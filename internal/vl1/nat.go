package vl1

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/stun/v3"
)

// NATTraversal manages ICE-based NAT traversal for a peer connection.
type NATTraversal struct {
	stunServers []string
	turnServers []TURNServer
	log         *slog.Logger
}

// TURNServer holds TURN server credentials.
type TURNServer struct {
	URL      string
	Username string
	Password string
}

// NewNATTraversal creates a new NAT traversal manager.
func NewNATTraversal(stunServers []string, turnServers []TURNServer, log *slog.Logger) *NATTraversal {
	return &NATTraversal{
		stunServers: stunServers,
		turnServers: turnServers,
		log:         log.With("component", "nat"),
	}
}

// DiscoverPublicAddr uses STUN to discover the public IP:port.
func (n *NATTraversal) DiscoverPublicAddr(localPort int) (*net.UDPAddr, error) {
	if len(n.stunServers) == 0 {
		return nil, fmt.Errorf("no STUN servers configured")
	}

	for _, server := range n.stunServers {
		addr, err := stunDiscover(server, localPort)
		if err != nil {
			n.log.Debug("STUN discovery failed", "server", server, "err", err)
			continue
		}
		n.log.Info("STUN discovered public address", "addr", addr, "server", server)
		return addr, nil
	}
	return nil, fmt.Errorf("all STUN servers failed")
}

// CreateICEAgent creates a pion/ice agent for a peer connection.
func (n *NATTraversal) CreateICEAgent() (*ice.Agent, error) {
	urls := make([]*stun.URI, 0)
	for _, s := range n.stunServers {
		u, err := stun.ParseURI(s)
		if err != nil {
			n.log.Debug("parse STUN URI", "uri", s, "err", err)
			continue
		}
		urls = append(urls, u)
	}
	for _, t := range n.turnServers {
		u, err := stun.ParseURI(t.URL)
		if err != nil {
			n.log.Debug("parse TURN URI", "uri", t.URL, "err", err)
			continue
		}
		u.Username = t.Username
		u.Password = t.Password
		urls = append(urls, u)
	}

	agent, err := ice.NewAgent(&ice.AgentConfig{
		Urls:              urls,
		NetworkTypes:      []ice.NetworkType{ice.NetworkTypeUDP4},
		CandidateTypes:    []ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypeServerReflexive, ice.CandidateTypeRelay},
		DisconnectedTimeout: ptrDuration(10 * time.Second),
		FailedTimeout:       ptrDuration(30 * time.Second),
		KeepaliveInterval:   ptrDuration(2 * time.Second),
	})
	if err != nil {
		return nil, fmt.Errorf("create ICE agent: %w", err)
	}

	return agent, nil
}

func ptrDuration(d time.Duration) *time.Duration {
	return &d
}

// stunDiscover performs a single STUN binding request.
func stunDiscover(serverAddr string, localPort int) (*net.UDPAddr, error) {
	conn, err := net.DialTimeout("udp", serverAddr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = ctx // STUN client handles timeout internally

	// Build STUN binding request
	msg := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(msg.Raw); err != nil {
		return nil, err
	}

	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	// Parse response
	resp := new(stun.Message)
	resp.Raw = buf[:n]
	if err := resp.Decode(); err != nil {
		return nil, err
	}

	var xorAddr stun.XORMappedAddress
	if err := xorAddr.GetFrom(resp); err != nil {
		// Try regular mapped address
		var mappedAddr stun.MappedAddress
		if err := mappedAddr.GetFrom(resp); err != nil {
			return nil, fmt.Errorf("no mapped address in STUN response")
		}
		return &net.UDPAddr{IP: mappedAddr.IP, Port: mappedAddr.Port}, nil
	}
	return &net.UDPAddr{IP: xorAddr.IP, Port: xorAddr.Port}, nil
}
