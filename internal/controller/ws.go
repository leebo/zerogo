package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/unicornultrafoundation/zerogo/internal/protocol"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allow all origins in MVP
}

// AgentConn represents a connected agent.
type AgentConn struct {
	NodeAddr  string
	PublicKey string
	Platform  string
	Endpoints []string
	Networks  []string
	Conn      *websocket.Conn
	LastSeen  time.Time
	mu        sync.Mutex
}

// SendJSON sends a JSON message to the agent.
func (ac *AgentConn) SendJSON(v interface{}) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return ac.Conn.WriteJSON(v)
}

// WSHandler manages WebSocket connections from agents.
type WSHandler struct {
	agents map[string]*AgentConn // nodeAddr → connection
	mu     sync.RWMutex
	ctrl   *Controller
	log    *slog.Logger
}

// NewWSHandler creates a new WebSocket handler.
func NewWSHandler(ctrl *Controller, log *slog.Logger) *WSHandler {
	return &WSHandler{
		agents: make(map[string]*AgentConn),
		ctrl:   ctrl,
		log:    log.With("component", "ws"),
	}
}

// HandleAgentConnect handles the agent WebSocket connection endpoint.
func (h *WSHandler) HandleAgentConnect(c *gin.Context) {
	nodeAddr := c.GetHeader("X-Node-Address")
	publicKey := c.GetHeader("X-Public-Key")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error("websocket upgrade failed", "err", err)
		return
	}

	agentConn := &AgentConn{
		NodeAddr:  nodeAddr,
		PublicKey: publicKey,
		Conn:      conn,
		LastSeen:  time.Now(),
	}

	h.mu.Lock()
	// Close existing connection from same node
	if old, exists := h.agents[nodeAddr]; exists {
		old.Conn.Close()
	}
	h.agents[nodeAddr] = agentConn
	h.mu.Unlock()

	h.log.Info("agent connected", "addr", nodeAddr, "remote", c.Request.RemoteAddr)

	// Read loop
	defer func() {
		h.mu.Lock()
		delete(h.agents, nodeAddr)
		h.mu.Unlock()
		conn.Close()
		h.log.Info("agent disconnected", "addr", nodeAddr)
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.log.Debug("agent websocket error", "addr", nodeAddr, "err", err)
			}
			return
		}

		agentConn.LastSeen = time.Now()
		h.handleMessage(agentConn, message)
	}
}

func (h *WSHandler) handleMessage(agent *AgentConn, message []byte) {
	var baseMsg protocol.Message
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		h.log.Debug("unmarshal agent message", "err", err)
		return
	}

	switch baseMsg.Type {
	case protocol.MsgTypeJoin:
		var msg protocol.JoinMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			return
		}
		h.handleJoin(agent, &msg)

	case protocol.MsgTypeStatus:
		var msg protocol.StatusMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			return
		}
		h.handleStatus(agent, &msg)

	case protocol.MsgTypeLeave:
		var msg protocol.LeaveMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			return
		}
		h.handleLeave(agent, &msg)

	default:
		h.log.Debug("unknown message type from agent", "type", baseMsg.Type, "addr", agent.NodeAddr)
	}
}

func (h *WSHandler) handleJoin(agent *AgentConn, msg *protocol.JoinMessage) {
	h.log.Info("agent join request",
		"addr", msg.NodeAddr,
		"networks", msg.Networks,
		"platform", msg.Platform,
	)

	agent.Platform = msg.Platform
	agent.Endpoints = msg.Endpoints
	agent.Networks = msg.Networks

	// Register/update node in database
	node := Node{
		Address:   msg.NodeAddr,
		PublicKey: msg.PublicKey,
		Platform:  msg.Platform,
		LastSeen:  time.Now(),
	}
	h.ctrl.db.Where("address = ?", msg.NodeAddr).Assign(node).FirstOrCreate(&node)

	// For each requested network, send config if authorized
	for _, netID := range msg.Networks {
		h.sendNetworkConfig(agent, netID)
	}
}

func (h *WSHandler) handleStatus(agent *AgentConn, msg *protocol.StatusMessage) {
	// Update last seen
	h.ctrl.db.Model(&Node{}).Where("address = ?", agent.NodeAddr).Update("last_seen", time.Now())
}

func (h *WSHandler) handleLeave(agent *AgentConn, msg *protocol.LeaveMessage) {
	h.log.Info("agent leaving networks", "addr", agent.NodeAddr, "networks", msg.Networks)
	// Remove from networks list
	for _, netID := range msg.Networks {
		for i, n := range agent.Networks {
			if n == netID {
				agent.Networks = append(agent.Networks[:i], agent.Networks[i+1:]...)
				break
			}
		}
	}
}

func (h *WSHandler) sendNetworkConfig(agent *AgentConn, networkID string) {
	var network Network
	if err := h.ctrl.db.First(&network, "id = ?", networkID).Error; err != nil {
		agent.SendJSON(protocol.ErrorMessage{
			Type:    protocol.MsgTypeError,
			Code:    404,
			Message: "network not found",
		})
		return
	}

	// Check membership
	var member Member
	if err := h.ctrl.db.First(&member, "network_id = ? AND node_address = ?", networkID, agent.NodeAddr).Error; err != nil {
		// Auto-create pending membership
		member = Member{
			NetworkID:   network.ID,
			NodeAddress: agent.NodeAddr,
			Authorized:  false,
		}
		h.ctrl.db.Create(&member)
		h.log.Info("new member pending authorization", "network", networkID, "node", agent.NodeAddr)
	}

	if !member.Authorized {
		agent.SendJSON(protocol.ErrorMessage{
			Type:    protocol.MsgTypeError,
			Code:    403,
			Message: "not authorized for this network",
		})
		return
	}

	// Gather peer list
	var members []Member
	h.ctrl.db.Where("network_id = ? AND node_address != ? AND authorized = ?", networkID, agent.NodeAddr, true).Find(&members)

	peers := make([]protocol.PeerInfo, 0, len(members))
	for _, m := range members {
		var node Node
		if err := h.ctrl.db.First(&node, "address = ?", m.NodeAddress).Error; err != nil {
			continue
		}
		// Get endpoints from connected agent
		h.mu.RLock()
		peerConn, online := h.agents[m.NodeAddress]
		h.mu.RUnlock()

		var endpoints []string
		if online {
			endpoints = peerConn.Endpoints
		}

		peers = append(peers, protocol.PeerInfo{
			Address:   m.NodeAddress,
			PublicKey: node.PublicKey,
			Endpoints: endpoints,
			Name:      m.Name,
		})
	}

	agent.SendJSON(protocol.NetworkConfigMessage{
		Type:       protocol.MsgTypeNetworkConfig,
		NetworkID:  networkID,
		Name:       network.Name,
		IPRange:    network.IPRange,
		IP6Range:   network.IP6Range,
		MTU:        network.MTU,
		Multicast:  network.Multicast,
		PSK:        network.PSK,
		AssignedIP: member.IPAddress,
		Peers:      peers,
	})
}

// SendNetworkConfigToAgent sends the full network config to a specific online agent.
func (h *WSHandler) SendNetworkConfigToAgent(nodeAddr string, networkID string) {
	h.mu.RLock()
	agent, ok := h.agents[nodeAddr]
	h.mu.RUnlock()
	if !ok {
		return // agent not online
	}
	h.sendNetworkConfig(agent, networkID)
}

// BroadcastPeerUpdate notifies all agents in a network about a peer change.
func (h *WSHandler) BroadcastPeerUpdate(networkID uint32, action string, peer protocol.PeerInfo) {
	msg := protocol.PeerUpdateMessage{
		Type:   protocol.MsgTypePeerUpdate,
		Action: action,
		Peer:   peer,
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, agent := range h.agents {
		for _, netID := range agent.Networks {
			if netID == fmt.Sprintf("%d", networkID) {
				agent.SendJSON(msg)
				break
			}
		}
	}
}

// GetOnlineAgents returns connected agent addresses.
func (h *WSHandler) GetOnlineAgents() map[string]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	online := make(map[string]bool, len(h.agents))
	for addr := range h.agents {
		online[addr] = true
	}
	return online
}

