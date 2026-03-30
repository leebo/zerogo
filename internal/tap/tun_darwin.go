//go:build darwin

package tap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/songgao/water"
)

var broadcastMAC = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// DarwinTUN implements Device using a TUN (utun) interface on macOS.
// It wraps raw IP packets with Ethernet headers so the rest of the stack
// (VL2 switch, ARP) works transparently.
type DarwinTUN struct {
	iface   *water.Interface
	name    string
	mac     net.HardwareAddr // virtual MAC for Ethernet header wrapping
	macMu   sync.RWMutex     // protects mac
	routeMu sync.Mutex       // protects AddRoute from concurrent calls
	closeMu sync.Mutex       // protects Close
	closed  bool
}

// NewTUN creates a new TUN (utun) device on macOS.
// The water library handles the 4-byte utun AF header transparently,
// so Read/Write operate on raw IP packets.
func NewTUN(name string) (*DarwinTUN, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}
	// On macOS, TUN names must be utun[0-9]+
	if name != "" && len(name) > 4 && name[:4] == "utun" {
		config.Name = name
	}
	iface, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("create TUN device: %w", err)
	}
	return &DarwinTUN{
		iface: iface,
		name:  iface.Name(),
		mac:   net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x01},
	}, nil
}

func (d *DarwinTUN) IsTUN() bool { return true }

func (d *DarwinTUN) Name() string { return d.name }

// Read reads a raw IP packet from TUN and wraps it in an Ethernet frame.
// Non-IP packets are throttled to prevent CPU spin.
func (d *DarwinTUN) Read(buf []byte) (int, error) {
	const maxConsecutiveInvalid = 1000
	invalidCount := 0

	for {
		n, err := d.iface.Read(buf[14:])
		if err != nil {
			return 0, err
		}
		if n < 1 {
			invalidCount++
			if invalidCount >= maxConsecutiveInvalid {
				time.Sleep(1 * time.Millisecond)
				invalidCount = 0
			}
			continue
		}

		version := buf[14] >> 4
		var etherType uint16
		switch version {
		case 4:
			etherType = 0x0800
		case 6:
			etherType = 0x86DD
		default:
			invalidCount++
			if invalidCount >= maxConsecutiveInvalid {
				time.Sleep(1 * time.Millisecond)
				invalidCount = 0
			}
			continue
		}

		d.macMu.RLock()
		mac := d.mac
		d.macMu.RUnlock()

		copy(buf[0:6], broadcastMAC)
		copy(buf[6:12], mac)
		binary.BigEndian.PutUint16(buf[12:14], etherType)

		return 14 + n, nil
	}
}

// Write strips the Ethernet header and writes raw IP to TUN.
// ARP and other non-IP frames are silently dropped.
func (d *DarwinTUN) Write(buf []byte) (int, error) {
	if len(buf) < 14 {
		return 0, fmt.Errorf("frame too short")
	}

	etherType := binary.BigEndian.Uint16(buf[12:14])
	if etherType != 0x0800 && etherType != 0x86DD {
		return len(buf), nil
	}

	n, err := d.iface.Write(buf[14:])
	if err != nil {
		return 0, err
	}
	return n + 14, nil
}

func (d *DarwinTUN) SetMTU(mtu int) error {
	cmd := exec.Command("ifconfig", d.name, "mtu", fmt.Sprintf("%d", mtu))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set MTU to %d: %w (stderr: %s)", mtu, err, stderr.String())
	}
	return nil
}

func (d *DarwinTUN) SetMACAddress(mac net.HardwareAddr) error {
	d.macMu.Lock()
	defer d.macMu.Unlock()
	d.mac = mac
	return nil
}

func (d *DarwinTUN) AddIPAddress(ip net.IP, mask net.IPMask) error {
	if len(mask) < 4 {
		return fmt.Errorf("invalid mask length: %d (expected at least 4 bytes)", len(mask))
	}

	maskStr := fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	cmd := exec.Command("ifconfig", d.name, "inet", ip.String(), ip.String(), "netmask", maskStr)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set IP %s/%s on %s: %w (stderr: %s)", ip.String(), maskStr, d.name, err, stderr.String())
	}

	// Add route for the subnet via this interface.
	// Without this, traffic to 10.147.17.2 (other host in subnet) falls through
	// to the default gateway instead of going through the VPN tunnel.
	ones, _ := mask.Size()
	network := ip.Mask(mask)
	cidr := fmt.Sprintf("%s/%d", network.String(), ones)

	if d.routeExists(cidr) {
		_ = exec.Command("route", "delete", "-net", cidr).Run()
	}
	cmd = exec.Command("route", "add", "-net", cidr, "-interface", d.name)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add subnet route %s via %s: %w (stderr: %s)", cidr, d.name, err, stderr.String())
	}

	return nil
}

func (d *DarwinTUN) SetUp() error {
	cmd := exec.Command("ifconfig", d.name, "up")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bring up interface %s: %w (stderr: %s)", d.name, err, stderr.String())
	}
	return nil
}

// routeExists checks if a route exists for the given destination.
func (d *DarwinTUN) routeExists(destination string) bool {
	cmd := exec.Command("route", "-n", "get", "-net", destination)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "route to:")
}

func (d *DarwinTUN) AddRoute(destination, gateway string, metric int) error {
	d.routeMu.Lock()
	defer d.routeMu.Unlock()

	if d.routeExists(destination) {
		_ = exec.Command("route", "-n", "delete", "-net", destination).Run()
	}

	var cmd *exec.Cmd
	if gateway != "" {
		cmd = exec.Command("route", "-n", "add", "-net", destination, gateway, "-interface", d.name)
	} else {
		cmd = exec.Command("route", "-n", "add", "-net", destination, "-interface", d.name)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add route %s: %w (stderr: %s)", destination, err, stderr.String())
	}
	return nil
}

func (d *DarwinTUN) EnableIPForwarding() error {
	cmd := exec.Command("sysctl", "-w", "net.inet.ip.forwarding=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("enable IP forwarding: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

func (d *DarwinTUN) RemoveRoute(destination string) error {
	d.routeMu.Lock()
	defer d.routeMu.Unlock()

	if !d.routeExists(destination) {
		return nil
	}
	cmd := exec.Command("route", "-n", "delete", "-net", destination)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove route %s: %w (stderr: %s)", destination, err, stderr.String())
	}
	return nil
}

// getDefaultGatewayDarwin returns the system's default IPv4 gateway on macOS.
func getDefaultGatewayDarwin() (string, error) {
	cmd := exec.Command("route", "-n", "get", "default")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("get default route: %w", err)
	}
	// Output format: "gateway: 10.0.0.1"
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "gateway:"); ok {
			gw := strings.TrimSpace(after)
			if gw != "" {
				return gw, nil
			}
		}
	}
	return "", fmt.Errorf("no default gateway found")
}

func (d *DarwinTUN) AddBypassRoute(hostIP string) error {
	gw, err := getDefaultGatewayDarwin()
	if err != nil {
		return fmt.Errorf("bypass route for %s: %w", hostIP, err)
	}
	cmd := exec.Command("route", "-n", "add", "-host", hostIP, gw)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add bypass route %s via %s: %w (stderr: %s)", hostIP, gw, err, stderr.String())
	}
	return nil
}

func (d *DarwinTUN) RemoveBypassRoute(hostIP string) error {
	cmd := exec.Command("route", "-n", "delete", "-host", hostIP)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove bypass route %s: %w (stderr: %s)", hostIP, err, stderr.String())
	}
	return nil
}

func (d *DarwinTUN) Close() error {
	d.closeMu.Lock()
	defer d.closeMu.Unlock()

	if d.closed {
		return nil
	}
	d.closed = true

	_ = exec.Command("ifconfig", d.name, "down").Run()
	return d.iface.Close()
}

// SetPeerARP is a no-op on Darwin TUN devices. The Darwin kernel handles
// peer ARP resolution internally via the ifmgr or ndp tables; there is no
// userspace "arp -s" equivalent needed for TUN interfaces.
func (d *DarwinTUN) SetPeerARP(ip net.IP, mac net.HardwareAddr) error {
	return nil
}
