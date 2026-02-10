package protocol

import "time"

// MessageType identifies the control protocol message type.
type MessageType string

const (
	// Agent → Controller
	MsgTypeJoin         MessageType = "join"
	MsgTypeStatus       MessageType = "status"
	MsgTypeLeave        MessageType = "leave"

	// Controller → Agent
	MsgTypeNetworkConfig MessageType = "network_config"
	MsgTypePeerUpdate    MessageType = "peer_update"
	MsgTypeError         MessageType = "error"
)

// Message is the base control protocol message.
type Message struct {
	Type MessageType `json:"type"`
}

// JoinMessage is sent by agent to join a network.
type JoinMessage struct {
	Type      MessageType `json:"type"`
	NodeAddr  string      `json:"node_addr"`
	PublicKey string      `json:"public_key"`
	Networks  []string    `json:"networks"`
	Endpoints []string    `json:"endpoints"` // public-facing UDP endpoints
	Platform  string      `json:"platform"`
	Version   string      `json:"version"`
}

// StatusMessage is periodically sent by agent to report status.
type StatusMessage struct {
	Type  MessageType  `json:"type"`
	Peers []PeerStatus `json:"peers"`
}

// PeerStatus reports connection status with one peer.
type PeerStatus struct {
	Address   string `json:"address"`
	LatencyMs int64  `json:"latency_ms"`
	Path      string `json:"path"` // "direct" or "relay"
	BytesSent int64  `json:"bytes_sent"`
	BytesRecv int64  `json:"bytes_recv"`
}

// LeaveMessage is sent when agent leaves a network.
type LeaveMessage struct {
	Type     MessageType `json:"type"`
	Networks []string    `json:"networks"`
}

// NetworkConfigMessage is sent by controller with network details.
type NetworkConfigMessage struct {
	Type       MessageType `json:"type"`
	NetworkID  string      `json:"network_id"`
	Name       string      `json:"name"`
	IPRange    string      `json:"ip_range"`
	IP6Range   string      `json:"ip6_range,omitempty"`
	MTU        int         `json:"mtu"`
	Multicast  bool        `json:"multicast"`
	PSK        string      `json:"psk"`        // Network PSK for peer encryption (hex)
	AssignedIP string      `json:"assigned_ip"` // IP/mask assigned to this node (CIDR)
	Peers      []PeerInfo  `json:"peers"`
}

// PeerInfo contains information about a peer in a network.
type PeerInfo struct {
	Address   string   `json:"address"`
	PublicKey string   `json:"public_key"`
	Endpoints []string `json:"endpoints"`
	Name      string   `json:"name,omitempty"`
}

// PeerUpdateMessage is sent when peers join/leave a network.
type PeerUpdateMessage struct {
	Type   MessageType `json:"type"`
	Action string      `json:"action"` // "add" or "remove"
	Peer   PeerInfo    `json:"peer"`
}

// ErrorMessage reports an error from the controller.
type ErrorMessage struct {
	Type    MessageType `json:"type"`
	Code    int         `json:"code"`
	Message string      `json:"message"`
}

// --- REST API types ---

// Network represents a virtual network in API responses.
type Network struct {
	ID          uint32    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IPRange     string    `json:"ip_range"`
	IP6Range    string    `json:"ip6_range,omitempty"`
	MTU         int       `json:"mtu"`
	Multicast   bool      `json:"multicast"`
	MemberCount int       `json:"member_count,omitempty"`
	OnlineCount int       `json:"online_count,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateNetworkRequest is the request body for creating a network.
type CreateNetworkRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IPRange     string `json:"ip_range" binding:"required"`
	IP6Range    string `json:"ip6_range"`
	MTU         int    `json:"mtu"`
	Multicast   *bool  `json:"multicast"`
}

// Member represents a network member in API responses.
type Member struct {
	NetworkID   uint32    `json:"network_id"`
	NodeAddress string    `json:"node_address"`
	Authorized  bool      `json:"authorized"`
	IPAddress   string    `json:"ip_address,omitempty"`
	Name        string    `json:"name,omitempty"`
	Online      bool      `json:"online"`
	Platform    string    `json:"platform,omitempty"`
	LastSeen    time.Time `json:"last_seen,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// AuthorizeMemberRequest is the request body for authorizing a member.
type AuthorizeMemberRequest struct {
	NodeAddress string `json:"node_address" binding:"required"`
	Authorized  bool   `json:"authorized"`
	IPAddress   string `json:"ip_address"`
	Name        string `json:"name"`
}

// LoginRequest is the request body for authentication.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse contains the JWT token after successful login.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
