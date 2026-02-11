package vl1

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
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
	// GamingKeepaliveInterval is a shorter keepalive for gaming/streaming scenarios
	// where NAT mappings must be kept alive more aggressively.
	GamingKeepaliveInterval = 5 * time.Second
	// PeerTimeout is when a peer is considered dead.
	PeerTimeout = 60 * time.Second
	// HandshakeTimeout is the max time to complete a handshake.
	HandshakeTimeout = 10 * time.Second
	// HandshakeRetryInterval is delay between handshake retries.
	HandshakeRetryInterval = 3 * time.Second
)

// ICEState represents the ICE negotiation state.
type ICEState int

const (
	ICEStateNone       ICEState = iota // No ICE negotiation
	ICEStateGathering                  // Gathering candidates
	ICEStateSignaling                  // Exchanging offers/answers
	ICEStateConnecting                 // Connectivity checks in progress
	ICEStateConnected                  // ICE connection established
	ICEStateFailed                     // ICE negotiation failed
	ICEStateClosed                     // ICE connection closed
)

func (s ICEState) String() string {
	switch s {
	case ICEStateNone:
		return "none"
	case ICEStateGathering:
		return "gathering"
	case ICEStateSignaling:
		return "signaling"
	case ICEStateConnecting:
		return "connecting"
	case ICEStateConnected:
		return "connected"
	case ICEStateFailed:
		return "failed"
	case ICEStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Peer represents a remote node we communicate with.
type Peer struct {
	// Identity
	Address   identity.Address
	PublicKey [32]byte

	// Connection state
	State    PeerState
	Endpoint *net.UDPAddr // Current best endpoint

	// Encryption — cipher is stored atomically so EncryptTo can be lock-free.
	cipher atomic.Pointer[NoiseCipher]

	// ICE connection
	iceConn  net.Conn // ICE connection (set after successful ICE negotiation)
	iceState ICEState

	// Timing
	LastSeen          time.Time
	LastSend          time.Time
	LatencyMs         int64
	HandshakeAt       time.Time
	KeepaliveInterval time.Duration // configurable keepalive interval (0 = default)

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
func (p *Peer) SetCipher(c *NoiseCipher) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cipher.Store(c)
	p.State = PeerStateConnected
	p.LastSeen = time.Now()
	p.log.Info("peer connected", "endpoint", p.Endpoint)
}

// Encrypt encrypts a payload for this peer.
func (p *Peer) Encrypt(plaintext []byte) ([]byte, error) {
	c := p.cipher.Load()
	if c == nil {
		return nil, fmt.Errorf("peer %s: no cipher (not connected)", p.Address)
	}
	return c.Encrypt(plaintext)
}

// Decrypt decrypts a payload from this peer.
func (p *Peer) Decrypt(ciphertext []byte) ([]byte, error) {
	c := p.cipher.Load()
	if c == nil {
		return nil, fmt.Errorf("peer %s: no cipher (not connected)", p.Address)
	}
	return c.Decrypt(ciphertext)
}

// EncryptTo encrypts plaintext into dst for this peer (zero-allocation, lock-free path).
// Safe because sendAEAD is immutable after construction and sendNonce uses atomic operations.
func (p *Peer) EncryptTo(dst, plaintext []byte) (int, error) {
	c := p.cipher.Load()
	if c == nil {
		return 0, fmt.Errorf("peer %s: no cipher (not connected)", p.Address)
	}
	return c.EncryptTo(dst, plaintext)
}

// DecryptTo decrypts ciphertext into dst for this peer (zero-allocation path).
func (p *Peer) DecryptTo(dst, ciphertext []byte) ([]byte, error) {
	c := p.cipher.Load()
	if c == nil {
		return nil, fmt.Errorf("peer %s: no cipher (not connected)", p.Address)
	}
	return c.DecryptTo(dst, ciphertext)
}

// IsConnected returns true if the peer has an active connection.
func (p *Peer) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State == PeerStateConnected && p.cipher.Load() != nil
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
// If recent data was sent (within the keepalive interval), the data itself
// serves as a keepalive and no explicit keepalive packet is needed.
func (p *Peer) NeedsKeepalive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	interval := p.KeepaliveInterval
	if interval == 0 {
		interval = KeepaliveInterval
	}
	return p.State == PeerStateConnected && time.Since(p.LastSend) > interval
}

// SetICEConn sets the ICE connection for this peer.
func (p *Peer) SetICEConn(conn net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.iceConn = conn
	p.iceState = ICEStateConnected
	p.log.Info("ICE connection established")
}

// ICEConn returns the ICE connection, or nil if not established.
func (p *Peer) ICEConn() net.Conn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.iceConn
}

// HasICE returns true if this peer has an active ICE connection.
func (p *Peer) HasICE() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.iceConn != nil && p.iceState == ICEStateConnected
}

// SetICEState sets the ICE negotiation state.
func (p *Peer) SetICEState(state ICEState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.iceState = state
}

// ICEState returns the current ICE state.
func (p *Peer) GetICEState() ICEState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.iceState
}

// CloseICE closes the ICE connection and resets state.
func (p *Peer) CloseICE() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.iceConn != nil {
		p.iceConn.Close()
		p.iceConn = nil
	}
	p.iceState = ICEStateClosed
}

// PeerManager manages all known peers.
type PeerManager struct {
	peers       map[identity.Address]*Peer
	endpointIdx map[string]*Peer // "ip:port" → Peer
	mu          sync.RWMutex
	log         *slog.Logger
}

// NewPeerManager creates a new peer manager.
func NewPeerManager(log *slog.Logger) *PeerManager {
	return &PeerManager{
		peers:       make(map[identity.Address]*Peer),
		endpointIdx: make(map[string]*Peer),
		log:         log.With("component", "peer-manager"),
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
			// Remove old endpoint index entry
			if p.Endpoint != nil {
				delete(pm.endpointIdx, p.Endpoint.String())
			}
			p.Endpoint = endpoint
			pm.endpointIdx[endpoint.String()] = p
			p.mu.Unlock()
		}
		return p
	}
	p := NewPeer(addr, pubKey, endpoint, pm.log)
	pm.peers[addr] = p
	if endpoint != nil {
		pm.endpointIdx[endpoint.String()] = p
	}
	pm.log.Info("peer added", "addr", addr, "endpoint", endpoint)
	return p
}

// GetPeer returns a peer by address.
func (pm *PeerManager) GetPeer(addr identity.Address) *Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.peers[addr]
}

// GetPeerByEndpoint finds a peer by UDP endpoint (O(1) lookup).
func (pm *PeerManager) GetPeerByEndpoint(addr *net.UDPAddr) *Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.endpointIdx[addr.String()]
}

// UpdatePeerEndpoint atomically updates a peer's endpoint and the endpoint index.
func (pm *PeerManager) UpdatePeerEndpoint(addr identity.Address, newEndpoint *net.UDPAddr) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, exists := pm.peers[addr]
	if !exists {
		return
	}
	p.mu.Lock()
	if p.Endpoint != nil {
		delete(pm.endpointIdx, p.Endpoint.String())
	}
	p.Endpoint = newEndpoint
	if newEndpoint != nil {
		pm.endpointIdx[newEndpoint.String()] = p
	}
	p.mu.Unlock()
}

// GetPeerByNodeAddr finds a peer by its string node address (hex-encoded).
func (pm *PeerManager) GetPeerByNodeAddr(nodeAddr string) *Peer {
	addr, err := identity.AddressFromHex(nodeAddr)
	if err != nil {
		return nil
	}
	return pm.GetPeer(addr)
}

// RemovePeer removes a peer by address.
func (pm *PeerManager) RemovePeer(addr identity.Address) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p, exists := pm.peers[addr]; exists {
		p.mu.RLock()
		if p.Endpoint != nil {
			delete(pm.endpointIdx, p.Endpoint.String())
		}
		p.mu.RUnlock()
		delete(pm.peers, addr)
	}
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
			p.mu.RLock()
			if p.Endpoint != nil {
				delete(pm.endpointIdx, p.Endpoint.String())
			}
			p.mu.RUnlock()
			delete(pm.peers, addr)
			removed++
		}
	}
	return removed
}
