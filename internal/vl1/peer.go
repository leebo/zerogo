package vl1

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/unicornultrafoundation/zerogo/internal/identity"
)

// PeerState represents the connection state of a peer.
type PeerState int

const (
	PeerStateNew        PeerState = iota // Just discovered, no handshake yet
	PeerStateHandshake                   // Handshake in progress
	PeerStateConnected                   // Handshake complete, exchanging data
	PeerStateDead                        // Connection lost
)

func (s PeerState) String() string {
	switch s {
	case PeerStateNew:
		return "new"
	case PeerStateHandshake:
		return "handshake"
	case PeerStateConnected:
		return "connected"
	case PeerStateDead:
		return "dead"
	default:
		return "unknown"
	}
}

const (
	// KeepaliveInterval is how often to send keepalive packets.
	KeepaliveInterval = 15 * time.Second
	// PeerTimeout is when a peer is considered dead.
	PeerTimeout = 60 * time.Second
	// HandshakeTimeout is the max time to complete a handshake.
	HandshakeTimeout = 10 * time.Second
	// HandshakeRetryInterval is delay between handshake retries.
	HandshakeRetryInterval = 3 * time.Second
)

// Peer represents a remote node we communicate with.
type Peer struct {
	// Identity
	Address   identity.Address
	PublicKey [32]byte

	// Connection state
	State    PeerState
	Endpoint *net.UDPAddr // Current best endpoint

	// Encryption
	cipher *NoiseCipher

	// Timing
	LastSeen     time.Time
	LastSend     time.Time
	LatencyMs    int64
	HandshakeAt  time.Time

	mu  sync.RWMutex
	log *slog.Logger
}

// NewPeer creates a new peer instance.
func NewPeer(addr identity.Address, pubKey [32]byte, endpoint *net.UDPAddr, log *slog.Logger) *Peer {
	return &Peer{
		Address:   addr,
		PublicKey: pubKey,
		State:     PeerStateNew,
		Endpoint:  endpoint,
		log:       log.With("peer", addr.String()),
	}
}

// SetCipher sets the transport cipher after handshake completes.
func (p *Peer) SetCipher(cipher *NoiseCipher) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cipher = cipher
	p.State = PeerStateConnected
	p.LastSeen = time.Now()
	p.log.Info("peer connected", "endpoint", p.Endpoint)
}

// Encrypt encrypts a payload for this peer.
func (p *Peer) Encrypt(plaintext []byte) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cipher == nil {
		return nil, fmt.Errorf("peer %s: no cipher (not connected)", p.Address)
	}
	return p.cipher.Encrypt(plaintext)
}

// Decrypt decrypts a payload from this peer.
func (p *Peer) Decrypt(ciphertext []byte) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cipher == nil {
		return nil, fmt.Errorf("peer %s: no cipher (not connected)", p.Address)
	}
	return p.cipher.Decrypt(ciphertext)
}

// IsConnected returns true if the peer has an active connection.
func (p *Peer) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State == PeerStateConnected && p.cipher != nil
}

// IsAlive returns true if the peer has been seen recently.
func (p *Peer) IsAlive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return time.Since(p.LastSeen) < PeerTimeout
}

// Touch updates the last seen timestamp.
func (p *Peer) Touch() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LastSeen = time.Now()
}

// NeedsKeepalive returns true if it's time to send a keepalive.
func (p *Peer) NeedsKeepalive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State == PeerStateConnected && time.Since(p.LastSend) > KeepaliveInterval
}

// PeerManager manages all known peers.
type PeerManager struct {
	peers map[identity.Address]*Peer
	mu    sync.RWMutex
	log   *slog.Logger
}

// NewPeerManager creates a new peer manager.
func NewPeerManager(log *slog.Logger) *PeerManager {
	return &PeerManager{
		peers: make(map[identity.Address]*Peer),
		log:   log.With("component", "peer-manager"),
	}
}

// AddPeer adds or updates a peer.
func (pm *PeerManager) AddPeer(addr identity.Address, pubKey [32]byte, endpoint *net.UDPAddr) *Peer {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p, exists := pm.peers[addr]; exists {
		// Update endpoint if changed
		if endpoint != nil {
			p.mu.Lock()
			p.Endpoint = endpoint
			p.mu.Unlock()
		}
		return p
	}
	p := NewPeer(addr, pubKey, endpoint, pm.log)
	pm.peers[addr] = p
	pm.log.Info("peer added", "addr", addr, "endpoint", endpoint)
	return p
}

// GetPeer returns a peer by address.
func (pm *PeerManager) GetPeer(addr identity.Address) *Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.peers[addr]
}

// GetPeerByEndpoint finds a peer by UDP endpoint.
func (pm *PeerManager) GetPeerByEndpoint(addr *net.UDPAddr) *Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for _, p := range pm.peers {
		p.mu.RLock()
		if p.Endpoint != nil && p.Endpoint.IP.Equal(addr.IP) && p.Endpoint.Port == addr.Port {
			p.mu.RUnlock()
			return p
		}
		p.mu.RUnlock()
	}
	return nil
}

// RemovePeer removes a peer by address.
func (pm *PeerManager) RemovePeer(addr identity.Address) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.peers, addr)
	pm.log.Info("peer removed", "addr", addr)
}

// ConnectedPeers returns all peers in connected state.
func (pm *PeerManager) ConnectedPeers() []*Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var result []*Peer
	for _, p := range pm.peers {
		if p.IsConnected() {
			result = append(result, p)
		}
	}
	return result
}

// AllPeers returns all peers.
func (pm *PeerManager) AllPeers() []*Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*Peer, 0, len(pm.peers))
	for _, p := range pm.peers {
		result = append(result, p)
	}
	return result
}

// CleanDead removes dead peers.
func (pm *PeerManager) CleanDead() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	removed := 0
	for addr, p := range pm.peers {
		if !p.IsAlive() && p.State == PeerStateDead {
			delete(pm.peers, addr)
			removed++
		}
	}
	return removed
}
