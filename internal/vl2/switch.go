package vl2

import (
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/unicornultrafoundation/zerogo/internal/identity"
)

const (
	// MACTableExpiry is how long a MAC table entry lives without refresh.
	MACTableExpiry = 5 * time.Minute
	// MACTableMaxSize limits MAC table size to prevent memory exhaustion.
	MACTableMaxSize = 4096
)

// MACEntry tracks where a MAC address was last seen.
type MACEntry struct {
	// PeerAddr is the remote peer's node address (zero = local).
	PeerAddr identity.Address
	// LastSeen is when this entry was last updated.
	LastSeen time.Time
	// IsLocal indicates the MAC belongs to the local TAP.
	IsLocal bool
}

// PeerSender is the interface for sending frames to a remote peer.
type PeerSender interface {
	// SendToPeer sends an Ethernet frame to the specified peer.
	SendToPeer(peerAddr identity.Address, networkID uint32, frame []byte) error
	// BroadcastToPeers sends an Ethernet frame to all peers in the network.
	BroadcastToPeers(networkID uint32, frame []byte, excludePeer identity.Address) error
}

// Switch implements a virtual Ethernet learning switch for one network.
type Switch struct {
	networkID uint32
	macTable  map[MACKey]*MACEntry
	mu        sync.RWMutex
	sender    PeerSender
	log       *slog.Logger
}

// NewSwitch creates a new virtual switch for the given network.
func NewSwitch(networkID uint32, sender PeerSender, log *slog.Logger) *Switch {
	return &Switch{
		networkID: networkID,
		macTable:  make(map[MACKey]*MACEntry),
		sender:    sender,
		log:       log.With("component", "switch", "network", networkID),
	}
}

// HandleLocalFrame processes a frame coming from the local TAP device.
// It learns the source MAC and forwards based on destination.
func (sw *Switch) HandleLocalFrame(frame []byte) error {
	parsed, err := ParseEthernetFrame(frame)
	if err != nil {
		return err
	}

	// Learn source MAC as local
	sw.learn(parsed.SrcMAC, identity.Address{}, true)

	// Forward based on destination
	if parsed.IsBroadcast() || parsed.IsMulticast() {
		// Flood to all peers
		return sw.sender.BroadcastToPeers(sw.networkID, frame, identity.Address{})
	}

	// Unicast: lookup MAC table
	sw.mu.RLock()
	entry, found := sw.macTable[MACToKey(parsed.DstMAC)]
	sw.mu.RUnlock()

	if found && !entry.IsLocal {
		// Known remote peer: send directly
		return sw.sender.SendToPeer(entry.PeerAddr, sw.networkID, frame)
	}

	if !found {
		// Unknown destination: flood (will learn on reply)
		sw.log.Debug("unknown dst MAC, flooding", "dst", parsed.DstMAC)
		return sw.sender.BroadcastToPeers(sw.networkID, frame, identity.Address{})
	}

	// Destination is local — drop (shouldn't happen for TAP-originated frames)
	return nil
}

// HandleRemoteFrame processes a frame received from a remote peer via VL1.
// Returns the raw frame to inject into the local TAP device.
func (sw *Switch) HandleRemoteFrame(peerAddr identity.Address, frame []byte) ([]byte, error) {
	parsed, err := ParseEthernetFrame(frame)
	if err != nil {
		return nil, err
	}

	// Learn source MAC → remote peer
	sw.learn(parsed.SrcMAC, peerAddr, false)

	// If broadcast/multicast or destined for a local MAC, inject into TAP
	if parsed.IsBroadcast() || parsed.IsMulticast() {
		// Also flood to other remote peers (not back to sender)
		_ = sw.sender.BroadcastToPeers(sw.networkID, frame, peerAddr)
		return frame, nil
	}

	// Unicast: check if destination is local
	sw.mu.RLock()
	entry, found := sw.macTable[MACToKey(parsed.DstMAC)]
	sw.mu.RUnlock()

	if found && entry.IsLocal {
		// Destination is local: inject into TAP
		return frame, nil
	}

	if found && !entry.IsLocal {
		// Destination is another remote peer: forward
		_ = sw.sender.SendToPeer(entry.PeerAddr, sw.networkID, frame)
		return nil, nil // Don't inject into local TAP
	}

	// Unknown: inject locally (might be for us) and flood
	_ = sw.sender.BroadcastToPeers(sw.networkID, frame, peerAddr)
	return frame, nil
}

// learn adds or updates a MAC table entry.
func (sw *Switch) learn(mac net.HardwareAddr, peerAddr identity.Address, isLocal bool) {
	key := MACToKey(mac)
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Enforce table size limit
	if len(sw.macTable) >= MACTableMaxSize {
		sw.evictOldest()
	}

	sw.macTable[key] = &MACEntry{
		PeerAddr: peerAddr,
		LastSeen: time.Now(),
		IsLocal:  isLocal,
	}
}

// evictOldest removes the oldest entry from the MAC table.
func (sw *Switch) evictOldest() {
	var oldestKey MACKey
	var oldestTime time.Time
	first := true
	for k, v := range sw.macTable {
		if first || v.LastSeen.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.LastSeen
			first = false
		}
	}
	if !first {
		delete(sw.macTable, oldestKey)
	}
}

// CleanExpired removes expired MAC table entries.
func (sw *Switch) CleanExpired() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	cutoff := time.Now().Add(-MACTableExpiry)
	removed := 0
	for k, v := range sw.macTable {
		if v.LastSeen.Before(cutoff) && !v.IsLocal {
			delete(sw.macTable, k)
			removed++
		}
	}
	return removed
}

// MACTableSize returns the current MAC table size.
func (sw *Switch) MACTableSize() int {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return len(sw.macTable)
}
