package agent

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/unicornultrafoundation/zerogo/internal/identity"
	"github.com/unicornultrafoundation/zerogo/internal/protocol"
	"github.com/unicornultrafoundation/zerogo/internal/tap"
	"github.com/unicornultrafoundation/zerogo/internal/vl1"
	"github.com/unicornultrafoundation/zerogo/internal/vl2"
)

const (
	controllerReconnectDelay    = 5 * time.Second
	controllerPingInterval      = 30 * time.Second
	controllerWriteTimeout      = 10 * time.Second
	controllerMaxReconnectDelay = 60 * time.Second
)

// ControllerClient manages the WebSocket connection to the controller.
type ControllerClient struct {
	url       string
	agent     *Agent
	conn      *websocket.Conn
	mu        sync.Mutex
	connected bool
	log       *slog.Logger
}

// NewControllerClient creates a new controller client.
func NewControllerClient(url string, agent *Agent, log *slog.Logger) *ControllerClient {
	return &ControllerClient{
		url:   url,
		agent: agent,
		log:   log.With("component", "controller-client"),
	}
}

// Run starts the controller connection loop (blocking).
func (c *ControllerClient) Run(ctx context.Context) {
	delay := controllerReconnectDelay
	for {
		select {
		case <-ctx.Done():
			c.close()
			return
		default:
		}

		if err := c.connect(ctx); err != nil {
			c.log.Error("controller connect failed", "err", err, "retry_in", delay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			delay = delay * 2
			if delay > controllerMaxReconnectDelay {
				delay = controllerMaxReconnectDelay
			}
			continue
		}

		delay = controllerReconnectDelay

		if err := c.readLoop(ctx); err != nil {
			c.log.Warn("controller connection lost", "err", err)
		}
		c.close()
	}
}

func (c *ControllerClient) connect(ctx context.Context) error {
	wsURL := c.url + "/api/v1/agent/connect"
	c.log.Info("connecting to controller", "url", wsURL)

	header := http.Header{}
	header.Set("X-Node-Address", c.agent.identity.Address.String())
	header.Set("X-Public-Key", c.agent.identity.PublicKeyHex())

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("dial controller: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	// Determine which networks to join
	networks := c.agent.config.Networks
	if len(networks) == 0 && c.agent.config.NetworkID > 0 {
		networks = []string{fmt.Sprintf("%d", c.agent.config.NetworkID)}
	}

	// Send join message
	joinMsg := protocol.JoinMessage{
		Type:      protocol.MsgTypeJoin,
		NodeAddr:  c.agent.identity.Address.String(),
		PublicKey: c.agent.identity.PublicKeyHex(),
		Networks:  networks,
		Endpoints: []string{fmt.Sprintf(":%d", c.agent.transport.Port())},
		Platform:  "linux",
		Version:   "0.1.0",
	}
	if err := c.sendJSON(joinMsg); err != nil {
		return fmt.Errorf("send join: %w", err)
	}

	c.log.Info("connected to controller", "networks", networks)
	return nil
}

func (c *ControllerClient) readLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		var baseMsg protocol.Message
		if err := json.Unmarshal(message, &baseMsg); err != nil {
			c.log.Debug("unmarshal message", "err", err)
			continue
		}

		switch baseMsg.Type {
		case protocol.MsgTypeNetworkConfig:
			var msg protocol.NetworkConfigMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				c.log.Debug("unmarshal network config", "err", err)
				continue
			}
			c.handleNetworkConfig(&msg)

		case protocol.MsgTypePeerUpdate:
			var msg protocol.PeerUpdateMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				c.log.Debug("unmarshal peer update", "err", err)
				continue
			}
			c.handlePeerUpdate(&msg)

		case protocol.MsgTypeError:
			var msg protocol.ErrorMessage
			if err := json.Unmarshal(message, &msg); err == nil {
				c.log.Warn("controller error", "code", msg.Code, "message", msg.Message)
			}

		default:
			c.log.Debug("unknown message type", "type", baseMsg.Type)
		}
	}
}

// handleNetworkConfig applies the network configuration from the controller.
func (c *ControllerClient) handleNetworkConfig(msg *protocol.NetworkConfigMessage) {
	c.log.Info("received network config",
		"network", msg.NetworkID,
		"name", msg.Name,
		"assigned_ip", msg.AssignedIP,
		"peers", len(msg.Peers),
	)

	a := c.agent

	// Parse PSK
	var psk [32]byte
	if msg.PSK != "" {
		b, err := hex.DecodeString(msg.PSK)
		if err != nil || len(b) != 32 {
			c.log.Error("invalid PSK from controller", "err", err)
			return
		}
		copy(psk[:], b)
		a.config.PSK = psk
	}

	// Parse network ID
	var networkID uint32
	fmt.Sscanf(msg.NetworkID, "%d", &networkID)
	a.config.NetworkID = networkID

	// Setup TAP device if not already created
	if a.tapDev == nil {
		mtu := msg.MTU
		if mtu == 0 {
			mtu = 2800
		}
		a.config.TAPMTU = mtu

		tapName := a.config.TAPName
		if tapName == "" {
			tapName = "zt0"
		}

		tapDev, err := tap.NewLinuxTAP(tapName)
		if err != nil {
			c.log.Error("create TAP device", "err", err)
			return
		}
		a.tapDev = tapDev
		c.log.Info("TAP device created", "name", tapDev.Name())

		if err := tapDev.SetMTU(mtu); err != nil {
			c.log.Warn("set TAP MTU", "err", err)
		}

		// Create VL2 network
		netConfig := vl2.NetworkConfig{
			ID:        networkID,
			Name:      msg.Name,
			MTU:       mtu,
			Multicast: msg.Multicast,
		}
		a.network = vl2.NewNetwork(netConfig, a.identity.Address, a, a.log)

		// Set MAC
		mac := vl2.GenerateMAC(networkID, a.identity.Address)
		if err := tapDev.SetMACAddress(mac); err != nil {
			c.log.Warn("set TAP MAC", "err", err)
		}

		// Set IP from controller
		if msg.AssignedIP != "" {
			ip, ipNet, err := net.ParseCIDR(msg.AssignedIP)
			if err != nil {
				c.log.Warn("invalid assigned IP", "ip", msg.AssignedIP, "err", err)
			} else {
				if err := tapDev.AddIPAddress(ip, ipNet.Mask); err != nil {
					c.log.Warn("add TAP IP", "err", err)
				}
				c.log.Info("TAP IP configured", "ip", msg.AssignedIP)
			}
		}

		// Bring up
		if err := tapDev.SetUp(); err != nil {
			c.log.Warn("bring TAP up", "err", err)
		}

		// Start TAP read loop
		a.wg.Add(1)
		go a.tapReadLoop()

		c.log.Info("network configured",
			"network_id", networkID,
			"name", msg.Name,
			"ip", msg.AssignedIP,
			"tap", tapDev.Name(),
		)
	}

	// Connect to peers
	for _, peerInfo := range msg.Peers {
		c.addPeerFromInfo(peerInfo, psk)
	}
}

// handlePeerUpdate processes a peer add/remove notification from the controller.
func (c *ControllerClient) handlePeerUpdate(msg *protocol.PeerUpdateMessage) {
	c.log.Info("peer update",
		"action", msg.Action,
		"peer", msg.Peer.Address,
		"endpoints", msg.Peer.Endpoints,
	)

	switch msg.Action {
	case "add":
		c.addPeerFromInfo(msg.Peer, c.agent.config.PSK)
	case "remove":
		addr, err := identity.AddressFromHex(msg.Peer.Address)
		if err != nil {
			c.log.Warn("invalid peer address", "addr", msg.Peer.Address)
			return
		}
		c.agent.peers.RemovePeer(addr)
		c.log.Info("peer removed", "addr", msg.Peer.Address)
	}
}

// addPeerFromInfo adds a peer from PeerInfo and initiates handshake.
func (c *ControllerClient) addPeerFromInfo(info protocol.PeerInfo, psk [32]byte) {
	pubKeyBytes, err := hex.DecodeString(info.PublicKey)
	if err != nil || len(pubKeyBytes) != 32 {
		c.log.Warn("invalid peer public key", "peer", info.Address, "err", err)
		return
	}

	var pubKey [32]byte
	copy(pubKey[:], pubKeyBytes)
	peerAddr := identity.AddressFromPublicKey(pubKey[:])

	// Already connected?
	if existing := c.agent.peers.GetPeer(peerAddr); existing != nil && existing.IsConnected() {
		return
	}

	// Resolve endpoint
	var endpoint *net.UDPAddr
	for _, ep := range info.Endpoints {
		resolved, err := net.ResolveUDPAddr("udp", ep)
		if err == nil && resolved.IP != nil {
			endpoint = resolved
			break
		}
	}

	if endpoint == nil {
		c.log.Debug("no valid endpoint for peer", "peer", info.Address, "endpoints", info.Endpoints)
		return
	}

	peer := c.agent.peers.AddPeer(peerAddr, pubKey, endpoint)

	// Derive keys from PSK and initiate handshake
	sendKey, recvKey := vl1.DeriveKeysFromPSK(psk, c.agent.identity.PublicKey, pubKey)
	cipher := vl1.NewNoiseCipher(sendKey, recvKey)
	peer.SetCipher(cipher)

	c.agent.sendHello(peer)
	c.log.Info("peer connected via controller", "peer", info.Address, "endpoint", endpoint)
}

// SendStatus sends a status report to the controller.
func (c *ControllerClient) SendStatus() error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	c.mu.Unlock()

	peers := c.agent.peers.ConnectedPeers()
	peerStatuses := make([]protocol.PeerStatus, 0, len(peers))
	for _, p := range peers {
		peerStatuses = append(peerStatuses, protocol.PeerStatus{
			Address:   p.Address.String(),
			LatencyMs: p.LatencyMs,
			Path:      "direct",
		})
	}

	return c.sendJSON(protocol.StatusMessage{
		Type:  protocol.MsgTypeStatus,
		Peers: peerStatuses,
	})
}

func (c *ControllerClient) sendJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	c.conn.SetWriteDeadline(time.Now().Add(controllerWriteTimeout))
	return c.conn.WriteJSON(v)
}

func (c *ControllerClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
}
