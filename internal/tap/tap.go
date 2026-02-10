package tap

import "net"

// Device is the cross-platform TAP device interface.
type Device interface {
	// Name returns the OS network interface name (e.g., "zt0").
	Name() string

	// Read reads an Ethernet frame from the TAP device into buf.
	// Returns the number of bytes read.
	Read(buf []byte) (int, error)

	// Write writes an Ethernet frame to the TAP device.
	// Returns the number of bytes written.
	Write(buf []byte) (int, error)

	// SetMTU sets the maximum transmission unit.
	SetMTU(mtu int) error

	// SetMACAddress sets the hardware (MAC) address.
	SetMACAddress(mac net.HardwareAddr) error

	// AddIPAddress assigns an IP address to the interface.
	AddIPAddress(ip net.IP, mask net.IPMask) error

	// SetUp brings the interface up.
	SetUp() error

	// Close shuts down and removes the TAP device.
	Close() error
}
