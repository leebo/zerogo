package tap

import "net"

// Device is the cross-platform TAP/TUN device interface.
type Device interface {
	// IsTUN returns true if this is a TUN (Layer 3) device rather than TAP (Layer 2).
	// TUN devices internally wrap IP packets with Ethernet headers for compatibility.
	IsTUN() bool

	// Name returns the OS network interface name (e.g., "zt0", "utun0").
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

	// AddRoute adds or replaces a managed route via this interface.
	// destination is a CIDR (e.g. "10.0.0.0/24"), gateway is an IP or empty for on-link.
	AddRoute(destination, gateway string, metric int) error

	// RemoveRoute removes a managed route from this interface.
	RemoveRoute(destination string) error

	// AddBypassRoute adds a /32 route for hostIP via the physical default gateway,
	// ensuring traffic to that IP bypasses any VPN managed routes. This prevents
	// ICE connectivity checks from being captured by the VPN tunnel.
	AddBypassRoute(hostIP string) error

	// RemoveBypassRoute removes a previously added bypass route.
	RemoveBypassRoute(hostIP string) error

	// EnableIPForwarding enables IP forwarding on this host (for gateway nodes).
	EnableIPForwarding() error

	// SetPeerARP adds a permanent ARP entry for a peer IP→MAC via this interface.
	// On Linux this uses "ip neigh add". On other platforms it is a no-op.
	// The kernel's ARP table is separate from the agent's ARP cache; without this,
	// the kernel cannot send IP packets to a peer it has never ARP-learned.
	SetPeerARP(ip net.IP, mac net.HardwareAddr) error

	// Close shuts down and removes the TAP device.
	Close() error
}
