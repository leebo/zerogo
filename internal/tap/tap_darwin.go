//go:build darwin

package tap

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"

	"github.com/songgao/water"
)

// DarwinTAP implements Device using songgao/water for macOS.
type DarwinTAP struct {
	iface *water.Interface
	name  string
	mu    sync.Mutex // protects route operations to avoid races
}

// NewTAP creates a new TAP device.
func NewTAP(name string) (*DarwinTAP, error) {
	config := water.Config{
		DeviceType: water.TAP,
	}
	if name != "" {
		config.Name = name
	}
	iface, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("create TAP device: %w", err)
	}
	return &DarwinTAP{
		iface: iface,
		name:  iface.Name(),
	}, nil
}

func (d *DarwinTAP) IsTUN() bool { return false }

func (d *DarwinTAP) Name() string {
	return d.name
}

func (d *DarwinTAP) Read(buf []byte) (int, error) {
	return d.iface.Read(buf)
}

func (d *DarwinTAP) Write(buf []byte) (int, error) {
	return d.iface.Write(buf)
}

func (d *DarwinTAP) SetMTU(mtu int) error {
	return exec.Command("ifconfig", d.name, "mtu", fmt.Sprintf("%d", mtu)).Run()
}

func (d *DarwinTAP) SetMACAddress(mac net.HardwareAddr) error {
	// Bring interface down
	if err := exec.Command("ifconfig", d.name, "down").Run(); err != nil {
		return fmt.Errorf("bring down interface: %w", err)
	}

	// Ensure interface is brought back up even if MAC setting fails
	var macErr error
	defer func() {
		if err := exec.Command("ifconfig", d.name, "up").Run(); err != nil && macErr == nil {
			// Only report up error if MAC setting succeeded
			macErr = fmt.Errorf("bring up interface: %w", err)
		}
	}()

	if macErr = exec.Command("ifconfig", d.name, "lladdr", mac.String()).Run(); macErr != nil {
		return fmt.Errorf("set MAC address: %w", macErr)
	}

	return macErr
}

func (d *DarwinTAP) AddIPAddress(ip net.IP, mask net.IPMask) error {
	// macOS ifconfig syntax: ifconfig <iface> inet <ip> netmask <mask>
	maskStr := fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	return exec.Command("ifconfig", d.name, "inet", ip.String(), "netmask", maskStr).Run()
}

func (d *DarwinTAP) SetUp() error {
	return exec.Command("ifconfig", d.name, "up").Run()
}

func (d *DarwinTAP) AddRoute(destination, gateway string, metric int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if route exists before deleting to avoid unnecessary operations
	exists := d.routeExists(destination)

	if exists {
		// Route exists, delete it first for idempotent replace
		if err := exec.Command("route", "-n", "delete", "-net", destination).Run(); err != nil {
			// Log but continue - route might have been deleted by another process
			_ = err
		}
	}

	if gateway != "" {
		return exec.Command("route", "-n", "add", "-net", destination, gateway).Run()
	}
	return exec.Command("route", "-n", "add", "-net", destination, "-interface", d.name).Run()
}

// routeExists checks if a route exists for the given destination
func (d *DarwinTAP) routeExists(destination string) bool {
	// Try to get the route; if it fails, the route likely doesn't exist
	cmd := exec.Command("route", "-n", "get", "-net", destination)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	// Check if output contains valid route info
	return strings.Contains(string(output), "route to:")
}

func (d *DarwinTAP) EnableIPForwarding() error {
	if err := exec.Command("sysctl", "-w", "net.inet.ip.forwarding=1").Run(); err != nil {
		return fmt.Errorf("enable IP forwarding: %w", err)
	}
	return nil
}

func (d *DarwinTAP) RemoveRoute(destination string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if route exists before attempting delete
	if !d.routeExists(destination) {
		return nil // Route doesn't exist, nothing to do
	}

	return exec.Command("route", "-n", "delete", "-net", destination).Run()
}

func (d *DarwinTAP) AddBypassRoute(hostIP string) error {
	return fmt.Errorf("bypass routes not supported on darwin")
}

func (d *DarwinTAP) RemoveBypassRoute(hostIP string) error {
	return fmt.Errorf("bypass routes not supported on darwin")
}

func (d *DarwinTAP) Close() error {
	// Bring the interface down before closing
	_ = exec.Command("ifconfig", d.name, "down").Run()
	return d.iface.Close()
}

// SetPeerARP is a no-op on Darwin. The kernel handles ARP resolution
// via the network framework (ifmgr/ndp); userspace cannot manipulate the ARP
// table with "arp -s" equivalent on modern macOS for TAP devices.
func (d *DarwinTAP) SetPeerARP(ip net.IP, mac net.HardwareAddr) error {
	return nil
}
