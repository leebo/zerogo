//go:build windows

package tap

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"

	"github.com/songgao/water"
)

// WindowsTAP implements Device using songgao/water for Windows.
type WindowsTAP struct {
	iface   *water.Interface
	name    string
	mac     net.HardwareAddr
	routeMu sync.Mutex // protects AddRoute from concurrent calls
	closeMu sync.Mutex // protects Close
	closed  bool
}

// NewTAP creates a new TAP device.
func NewTAP(name string) (*WindowsTAP, error) {
	config := water.Config{
		DeviceType: water.TAP,
	}
	if name != "" {
		config.PlatformSpecificParams = water.PlatformSpecificParams{
			ComponentID:   "tap0901",
			InterfaceName: name,
		}
	}
	iface, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("create TAP device: %w", err)
	}
	return &WindowsTAP{
		iface: iface,
		name:  iface.Name(),
	}, nil
}

func (d *WindowsTAP) IsTUN() bool { return false }

func (d *WindowsTAP) Name() string {
	return d.name
}

func (d *WindowsTAP) Read(buf []byte) (int, error) {
	return d.iface.Read(buf)
}

func (d *WindowsTAP) Write(buf []byte) (int, error) {
	return d.iface.Write(buf)
}

func (d *WindowsTAP) SetMTU(mtu int) error {
	cmd := exec.Command("netsh", "interface", "ipv4", "set", "subinterface",
		d.name, fmt.Sprintf("mtu=%d", mtu), "store=persistent")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set MTU to %d: %w (stderr: %s)", mtu, err, stderr.String())
	}
	return nil
}

func (d *WindowsTAP) SetMACAddress(mac net.HardwareAddr) error {
	// Store the MAC for local use (ARP cache, etc.).
	// Windows TAP adapter MAC changes require registry modification
	// which is fragile; the stored value is sufficient for the agent.
	d.mac = mac
	return nil
}

func (d *WindowsTAP) AddIPAddress(ip net.IP, mask net.IPMask) error {
	if len(mask) < 4 {
		return fmt.Errorf("invalid mask length: %d (expected at least 4 bytes)", len(mask))
	}
	maskStr := fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	cmd := exec.Command("netsh", "interface", "ip", "set", "address",
		d.name, "static", ip.String(), maskStr)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set IP %s/%s on %s: %w (stderr: %s)", ip.String(), maskStr, d.name, err, stderr.String())
	}
	return nil
}

func (d *WindowsTAP) SetUp() error {
	cmd := exec.Command("netsh", "interface", "set", "interface", d.name, "admin=enable")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bring up interface %s: %w (stderr: %s)", d.name, err, stderr.String())
	}
	return nil
}

// interfaceIndex returns the OS interface index for this TAP device.
func (d *WindowsTAP) interfaceIndex() (int, error) {
	iface, err := net.InterfaceByName(d.name)
	if err != nil {
		return 0, fmt.Errorf("get interface index for %s: %w", d.name, err)
	}
	return iface.Index, nil
}

func (d *WindowsTAP) AddRoute(destination, gateway string, metric int) error {
	d.routeMu.Lock()
	defer d.routeMu.Unlock()

	// Parse CIDR to get network and mask
	_, ipNet, err := net.ParseCIDR(destination)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}
	network := ipNet.IP.String()
	maskStr := net.IP(ipNet.Mask).String()

	// Use 0.0.0.0 as gateway for on-link routes
	gw := gateway
	if gw == "" {
		gw = "0.0.0.0"
	}

	// Delete existing route first (ignore error if it doesn't exist)
	_ = exec.Command("route", "DELETE", network, "MASK", maskStr).Run()

	// Build route ADD command with optional interface binding
	args := []string{"ADD", network, "MASK", maskStr, gw, "METRIC", fmt.Sprintf("%d", metric)}
	if idx, err := d.interfaceIndex(); err == nil {
		args = append(args, "IF", fmt.Sprintf("%d", idx))
	}

	cmd := exec.Command("route", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add route %s MASK %s %s: %w (stderr: %s)", network, maskStr, gw, err, stderr.String())
	}
	return nil
}

func (d *WindowsTAP) EnableIPForwarding() error {
	cmd := exec.Command("netsh", "interface", "ipv4", "set", "global", "forwarding=enabled")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("enable IP forwarding: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

func (d *WindowsTAP) RemoveRoute(destination string) error {
	_, ipNet, err := net.ParseCIDR(destination)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}
	network := ipNet.IP.String()
	maskStr := net.IP(ipNet.Mask).String()

	cmd := exec.Command("route", "DELETE", network, "MASK", maskStr)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove route %s: %w (stderr: %s)", destination, err, stderr.String())
	}
	return nil
}

// getDefaultGatewayWindows returns the system's default IPv4 gateway on Windows.
func getDefaultGatewayWindows() (string, error) {
	// Use "route PRINT 0.0.0.0" to find the default gateway
	cmd := exec.Command("route", "PRINT", "0.0.0.0")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("get default route: %w", err)
	}
	// Parse output. Lines look like:
	// Network Destination        Netmask          Gateway       Interface  Metric
	//           0.0.0.0          0.0.0.0      10.0.0.1       10.0.0.5     25
	for _, line := range strings.Split(stdout.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" {
			return fields[2], nil
		}
	}
	return "", fmt.Errorf("no default gateway found")
}

func (d *WindowsTAP) AddBypassRoute(hostIP string) error {
	gw, err := getDefaultGatewayWindows()
	if err != nil {
		return fmt.Errorf("bypass route for %s: %w", hostIP, err)
	}
	cmd := exec.Command("route", "ADD", hostIP, "MASK", "255.255.255.255", gw)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add bypass route %s via %s: %w (stderr: %s)", hostIP, gw, err, stderr.String())
	}
	return nil
}

func (d *WindowsTAP) RemoveBypassRoute(hostIP string) error {
	cmd := exec.Command("route", "DELETE", hostIP, "MASK", "255.255.255.255")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove bypass route %s: %w (stderr: %s)", hostIP, err, stderr.String())
	}
	return nil
}

func (d *WindowsTAP) Close() error {
	d.closeMu.Lock()
	defer d.closeMu.Unlock()

	if d.closed {
		return nil
	}
	d.closed = true

	// Disable the interface before closing
	_ = exec.Command("netsh", "interface", "set", "interface", d.name, "admin=disable").Run()

	return d.iface.Close()
}

// SetPeerARP adds a permanent ARP entry via the Windows TAP interface.
// Uses "netsh interface ip add neighbors" to populate the kernel ARP cache.
func (d *WindowsTAP) SetPeerARP(ip net.IP, mac net.HardwareAddr) error {
	cmd := exec.Command("netsh", "interface", "ip", "add", "neighbors",
		d.name, ip.String(), mac.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set peer ARP %s→%s: %w (stderr: %s)", ip, mac, err, stderr.String())
	}
	return nil
}

// NewTUN is not yet supported on Windows; use NewTAP instead.
func NewTUN(name string) (Device, error) {
	return nil, fmt.Errorf("TUN devices not yet supported on Windows, use TAP")
}
