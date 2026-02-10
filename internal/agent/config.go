package agent

import "net"

// PeerEndpoint defines a static peer endpoint for Phase 1 (no controller).
type PeerEndpoint struct {
	PublicKey string       `yaml:"public_key"`
	Endpoint  *net.UDPAddr `yaml:"-"`
	Address   string       `yaml:"address"` // host:port
}

// Config holds the agent runtime configuration.
type Config struct {
	IdentityPath string
	ListenPort   int
	TAPName      string // desired TAP device name (e.g., "zt0")
	TAPMTU       int
	TAPIPv4      string // IP/mask to assign (e.g., "10.147.17.1/24")
	NetworkID    uint32
	PSK          [32]byte // Pre-shared key for Noise handshake

	// Phase 1: static peers (no controller)
	StaticPeers []PeerEndpoint

	// Phase 3: controller
	ControllerURL string
	Networks      []string // network IDs to join via controller

	LogLevel string
}
