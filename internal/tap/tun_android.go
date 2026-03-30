//go:build android

package tap

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"
)

// AndroidTUN implements Device using a TUN file descriptor provided by
// Android's VpnService.  On Android, the Java/Kotlin layer calls
// VpnService.Builder.establish() to obtain a TUN fd, then passes it to
// Go via gomobile.  All interface configuration (IP, routes, MTU, DNS)
// is declared on the VpnService.Builder before establish(), so the
// config methods here are intentional no-ops.
//
// Like the other TUN implementations, Read/Write wrap raw IP packets
// with Ethernet headers so the VL2 switch works transparently.
type AndroidTUN struct {
	file *os.File
	name string
	mac  net.HardwareAddr // virtual MAC for Ethernet header wrapping
	mu   sync.Mutex       // protects file during UpdateFD

	// OnRouteChange is called when the agent wants to add/remove a managed
	// route.  The Java layer should rebuild the VPN session with updated
	// routes when this fires.  May be nil if dynamic route updates are not
	// supported.
	OnRouteChange func(action, destination, gateway string)
}

var androidBroadcastMAC = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// NewTUNFromFD creates an AndroidTUN from an existing file descriptor
// obtained from VpnService.Builder.establish().  The caller (Java/Kotlin
// via gomobile) is responsible for keeping the VpnService alive.
func NewTUNFromFD(fd int, name string) (Device, error) {
	if fd < 0 {
		return nil, fmt.Errorf("invalid TUN file descriptor: %d", fd)
	}
	// Dup the fd so Go's garbage collector / finalizer won't close the
	// original Java-owned descriptor.
	newFD, err := syscall.Dup(fd)
	if err != nil {
		return nil, fmt.Errorf("dup fd: %w", err)
	}
	file := os.NewFile(uintptr(newFD), "tun-android")
	if file == nil {
		syscall.Close(newFD)
		return nil, fmt.Errorf("os.NewFile returned nil for fd %d", newFD)
	}
	return &AndroidTUN{
		file: file,
		name: name,
		mac:  net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x01},
	}, nil
}

// NewTUN cannot create a TUN on Android from Go — the fd must come from
// VpnService via NewTUNFromFD.
func NewTUN(name string) (*AndroidTUN, error) {
	return nil, fmt.Errorf("on Android, use NewTUNFromFD with a VpnService file descriptor")
}

// NewTAP is not available on Android.
func NewTAP(name string) (*AndroidTUN, error) {
	return nil, fmt.Errorf("TAP devices are not supported on Android, use TUN via VpnService")
}

func (d *AndroidTUN) IsTUN() bool { return true }

func (d *AndroidTUN) Name() string { return d.name }

// Read reads a raw IP packet from the TUN fd and wraps it in an Ethernet
// frame.  Output: [dst MAC (6)][src MAC (6)][EtherType (2)][IP packet].
// Non-IP packets are silently skipped.
func (d *AndroidTUN) Read(buf []byte) (int, error) {
	for {
		d.mu.Lock()
		f := d.file

		if f == nil {
			d.mu.Unlock()
			return 0, fmt.Errorf("tun device closed")
		}

		// Read raw IP into buf[14:] leaving room for Ethernet header.
		n, err := f.Read(buf[14:])

		// Unlock after read is complete to prevent UpdateFD from
		// closing the file descriptor mid-read
		d.mu.Unlock()

		if err != nil {
			return 0, err
		}
		if n < 1 {
			continue
		}

		// Determine EtherType from IP version nibble.
		version := buf[14] >> 4
		var etherType uint16
		switch version {
		case 4:
			etherType = 0x0800 // IPv4
		case 6:
			etherType = 0x86DD // IPv6
		default:
			continue // skip unknown, read next
		}

		// Build Ethernet header.
		copy(buf[0:6], androidBroadcastMAC) // dst: broadcast
		copy(buf[6:12], d.mac)              // src: our virtual MAC
		binary.BigEndian.PutUint16(buf[12:14], etherType)

		return 14 + n, nil
	}
}

// Write strips the Ethernet header and writes the raw IP packet to the
// TUN fd.  Non-IP frames (ARP, etc.) are silently consumed.
func (d *AndroidTUN) Write(buf []byte) (int, error) {
	if len(buf) < 14 {
		return 0, fmt.Errorf("frame too short")
	}

	etherType := binary.BigEndian.Uint16(buf[12:14])

	// TUN only handles IP; silently drop ARP and others.
	if etherType != 0x0800 && etherType != 0x86DD {
		return len(buf), nil
	}

	d.mu.Lock()
	f := d.file
	d.mu.Unlock()

	n, err := f.Write(buf[14:])
	if err != nil {
		return 0, err
	}
	return n + 14, nil
}

// --- Configuration no-ops ---
// On Android all interface config is done via VpnService.Builder before
// establish().  These methods are no-ops so the agent code doesn't need
// platform-specific branches.

func (d *AndroidTUN) SetMTU(mtu int) error { return nil }

func (d *AndroidTUN) SetMACAddress(mac net.HardwareAddr) error {
	d.mac = mac
	return nil
}

func (d *AndroidTUN) AddIPAddress(ip net.IP, mask net.IPMask) error { return nil }

func (d *AndroidTUN) SetUp() error { return nil }

func (d *AndroidTUN) EnableIPForwarding() error { return nil }

// AddRoute is a no-op on Android (routes are declared in VpnService.Builder).
// If OnRouteChange is set, it notifies the Java layer so it can rebuild the
// VPN session with the new route.
func (d *AndroidTUN) AddRoute(destination, gateway string, metric int) error {
	if d.OnRouteChange != nil {
		d.OnRouteChange("add", destination, gateway)
	}
	return nil
}

// RemoveRoute is a no-op on Android.  See AddRoute.
func (d *AndroidTUN) RemoveRoute(destination string) error {
	if d.OnRouteChange != nil {
		d.OnRouteChange("remove", destination, "")
	}
	return nil
}

func (d *AndroidTUN) AddBypassRoute(hostIP string) error {
	return fmt.Errorf("bypass routes not supported on android")
}

func (d *AndroidTUN) RemoveBypassRoute(hostIP string) error {
	return fmt.Errorf("bypass routes not supported on android")
}

// UpdateFD replaces the underlying TUN file descriptor.  This is needed
// when the Java layer rebuilds the VPN session (e.g. after a route change)
// and obtains a new fd from VpnService.Builder.establish().
// The old fd is closed.
func (d *AndroidTUN) UpdateFD(fd int) error {
	newFile := os.NewFile(uintptr(fd), "tun-android")
	if newFile == nil {
		return fmt.Errorf("os.NewFile returned nil for fd %d", fd)
	}
	d.mu.Lock()
	old := d.file
	d.file = newFile
	d.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

// Close closes the TUN file descriptor.
func (d *AndroidTUN) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file != nil {
		return d.file.Close()
	}
	return nil
}

// SetPeerARP is a no-op on Android. The kernel ARP table is managed by the
// VpnService, and the Java layer would need to handle peer ARP resolution.
func (d *AndroidTUN) SetPeerARP(ip net.IP, mac net.HardwareAddr) error {
	return nil
}
