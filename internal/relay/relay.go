package relay

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/pion/turn/v3"
)

// Config holds the relay server configuration.
type Config struct {
	STUNEnabled bool
	TURNEnabled bool
	ListenAddr  string // e.g., "0.0.0.0:3478"
	Realm       string
	PublicIP    string            // Public IP for TURN relay address
	Credentials map[string]string // username → password
}

// Server runs STUN and TURN services for NAT traversal.
type Server struct {
	config     Config
	turnServer *turn.Server
	log        *slog.Logger
}

// New creates a new relay server.
func New(cfg Config, log *slog.Logger) *Server {
	return &Server{
		config: cfg,
		log:    log.With("component", "relay"),
	}
}

// Start starts the STUN/TURN server.
func (s *Server) Start() error {
	udpListener, err := net.ListenPacket("udp4", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.config.ListenAddr, err)
	}

	// Determine public IP
	publicIP := s.config.PublicIP
	if publicIP == "" {
		publicIP = "0.0.0.0"
	}

	turnConfig := turn.ServerConfig{
		Realm: s.config.Realm,
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			password, ok := s.config.Credentials[username]
			if !ok {
				return nil, false
			}
			return turn.GenerateAuthKey(username, realm, password), true
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(publicIP),
					Address:      "0.0.0.0",
				},
			},
		},
	}

	turnServer, err := turn.NewServer(turnConfig)
	if err != nil {
		udpListener.Close()
		return fmt.Errorf("create TURN server: %w", err)
	}
	s.turnServer = turnServer

	s.log.Info("relay server started",
		"listen", s.config.ListenAddr,
		"stun", s.config.STUNEnabled,
		"turn", s.config.TURNEnabled,
		"realm", s.config.Realm,
	)
	return nil
}

// Stop shuts down the relay server.
func (s *Server) Stop() error {
	if s.turnServer != nil {
		return s.turnServer.Close()
	}
	return nil
}
