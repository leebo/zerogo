package vl2

import (
	"encoding/binary"
	"log/slog"
	"net"
	"sync"
	"time"
)

// ARP constants
const (
	ARPHeaderSize    = 28 // ARP header for IPv4/Ethernet
	ARPRequest       = 1
	ARPReply         = 2
	ARPCacheExpiry   = 5 * time.Minute
	ARPCacheMaxSize  = 1024
)

// ARPEntry maps an IP address to a MAC address.
type ARPEntry struct {
	MAC      net.HardwareAddr
	LastSeen time.Time
	Pinned   bool // If true, entry never expires (e.g. our own IP→MAC)
}

// ARPProxy intercepts ARP requests and replies from cache when possible,
// reducing broadcast traffic across the virtual network.
type ARPProxy struct {
	cache map[[4]byte]*ARPEntry // IPv4 → MAC
	mu    sync.RWMutex
	log   *slog.Logger
}

// NewARPProxy creates a new ARP proxy.
func NewARPProxy(log *slog.Logger) *ARPProxy {
	return &ARPProxy{
		cache: make(map[[4]byte]*ARPEntry),
		log:   log.With("component", "arp-proxy"),
	}
}

// HandleARP processes an ARP frame. If it's a request and we have the answer
// cached, returns a reply frame. Otherwise returns nil (let it flood).
func (a *ARPProxy) HandleARP(frame *EthernetFrame) []byte {
	if len(frame.Payload) < ARPHeaderSize {
		return nil
	}
	payload := frame.Payload

	// Parse ARP header
	htype := binary.BigEndian.Uint16(payload[0:2])  // Hardware type
	ptype := binary.BigEndian.Uint16(payload[2:4])  // Protocol type
	hlen := payload[4]                               // Hardware addr length
	plen := payload[5]                               // Protocol addr length
	oper := binary.BigEndian.Uint16(payload[6:8])    // Operation

	// We only handle Ethernet (1) + IPv4 (0x0800)
	if htype != 1 || ptype != 0x0800 || hlen != 6 || plen != 4 {
		return nil
	}

	senderMAC := net.HardwareAddr(payload[8:14])
	senderIP := [4]byte{payload[14], payload[15], payload[16], payload[17]}
	// targetMAC := payload[18:24] // not used for requests
	targetIP := [4]byte{payload[24], payload[25], payload[26], payload[27]}

	// Always learn from sender
	a.learn(senderIP, senderMAC)

	if oper == ARPRequest {
		// Check cache for target IP
		a.mu.RLock()
		entry, found := a.cache[targetIP]
		a.mu.RUnlock()

		if found && time.Since(entry.LastSeen) < ARPCacheExpiry {
			a.log.Debug("ARP proxy hit", "ip", net.IP(targetIP[:]), "mac", entry.MAC)
			return a.buildARPReply(frame, entry.MAC, senderMAC, senderIP, targetIP)
		}
		// Cache miss: let the ARP request flood
		return nil
	}

	if oper == ARPReply {
		// Learn from reply
		a.learn(senderIP, senderMAC)
	}

	return nil
}

// PeerFromARP extracts the sender IP and MAC from an ARP frame.
// Returns nil if the frame is not a valid ARP or not IPv4/Ethernet.
func (a *ARPProxy) PeerFromARP(frame *EthernetFrame) (net.IP, net.HardwareAddr) {
	if len(frame.Payload) < ARPHeaderSize {
		return nil, nil
	}
	payload := frame.Payload
	htype := binary.BigEndian.Uint16(payload[0:2])
	ptype := binary.BigEndian.Uint16(payload[2:4])
	hlen := payload[4]
	plen := payload[5]
	if htype != 1 || ptype != 0x0800 || hlen != 6 || plen != 4 {
		return nil, nil
	}
	senderMAC := net.HardwareAddr(payload[8:14])
	senderIP := net.IP(payload[14:18])
	return senderIP, senderMAC
}

// Lookup returns the cached MAC for an IP, or nil if not found.
func (a *ARPProxy) Lookup(ip net.IP) net.HardwareAddr {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}
	var key [4]byte
	copy(key[:], ip4)
	a.mu.RLock()
	entry, found := a.cache[key]
	a.mu.RUnlock()
	if found && time.Since(entry.LastSeen) < ARPCacheExpiry {
		return entry.MAC
	}
	return nil
}

// Learn adds or updates an ARP cache entry (public API for seeding).
// Seeded entries are pinned and never expire.
func (a *ARPProxy) Learn(ip net.IP, mac net.HardwareAddr) {
	ip4 := ip.To4()
	if ip4 == nil {
		return
	}
	var key [4]byte
	copy(key[:], ip4)
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.cache) >= ARPCacheMaxSize {
		a.evictOldest()
	}
	macCopy := make(net.HardwareAddr, 6)
	copy(macCopy, mac)
	a.cache[key] = &ARPEntry{
		MAC:      macCopy,
		LastSeen: time.Now(),
		Pinned:   true,
	}
}

// learn adds or updates an ARP cache entry.
func (a *ARPProxy) learn(ip [4]byte, mac net.HardwareAddr) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.cache) >= ARPCacheMaxSize {
		a.evictOldest()
	}
	macCopy := make(net.HardwareAddr, 6)
	copy(macCopy, mac)
	a.cache[ip] = &ARPEntry{
		MAC:      macCopy,
		LastSeen: time.Now(),
	}
}

// buildARPReply constructs an ARP reply Ethernet frame.
func (a *ARPProxy) buildARPReply(originalFrame *EthernetFrame, targetMAC, senderMAC net.HardwareAddr, senderIP, targetIP [4]byte) []byte {
	frame := make([]byte, EthernetHeaderSize+ARPHeaderSize)

	// Ethernet header
	copy(frame[0:6], senderMAC)   // dst: original sender
	copy(frame[6:12], targetMAC)  // src: the resolved MAC
	binary.BigEndian.PutUint16(frame[12:14], EtherTypeARP)

	// ARP reply
	arp := frame[EthernetHeaderSize:]
	binary.BigEndian.PutUint16(arp[0:2], 1)      // htype: Ethernet
	binary.BigEndian.PutUint16(arp[2:4], 0x0800)  // ptype: IPv4
	arp[4] = 6                                     // hlen
	arp[5] = 4                                     // plen
	binary.BigEndian.PutUint16(arp[6:8], ARPReply) // operation
	copy(arp[8:14], targetMAC)                     // sender MAC (the resolved one)
	copy(arp[14:18], targetIP[:])                  // sender IP
	copy(arp[18:24], senderMAC)                    // target MAC (original requester)
	copy(arp[24:28], senderIP[:])                  // target IP

	return frame
}

func (a *ARPProxy) evictOldest() {
	var oldestKey [4]byte
	var oldestTime time.Time
	first := true
	for k, v := range a.cache {
		if v.Pinned {
			continue // never evict pinned entries
		}
		if first || v.LastSeen.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.LastSeen
			first = false
		}
	}
	if !first {
		delete(a.cache, oldestKey)
	}
}

// CleanExpired removes expired entries from the ARP cache.
// Pinned entries (seeded via Learn) are never expired.
func (a *ARPProxy) CleanExpired() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	cutoff := time.Now().Add(-ARPCacheExpiry)
	removed := 0
	for k, v := range a.cache {
		if !v.Pinned && v.LastSeen.Before(cutoff) {
			delete(a.cache, k)
			removed++
		}
	}
	return removed
}
