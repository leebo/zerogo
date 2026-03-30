//go:build linux && !android

package tap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/songgao/water"
)

// LinuxTUN implements Device using a TUN interface on Linux.
// It wraps raw IP packets with Ethernet headers so the VL2 switch works transparently.
type LinuxTUN struct {
	iface      *water.Interface
	name       string
	mac        net.HardwareAddr // virtual MAC for Ethernet header wrapping
	macMu      sync.RWMutex     // protects mac
	routeMu    sync.Mutex       // protects AddRoute from concurrent calls
	closeMu    sync.Mutex       // protects Close
	closed     bool
}

const defaultMAC = "02:00:00:00:00:01"

// parseMAC parses a MAC address string, returning default on error.
func parseMAC(s string) net.HardwareAddr {
	mac, err := net.ParseMAC(s)
	if err != nil {
		mac, _ = net.ParseMAC(defaultMAC)
	}
	return mac
}

// NewTUN creates a new TUN device on Linux.
// The water library sets IFF_NO_PI, so Read/Write operate on raw IP packets.
func NewTUN(name string) (*LinuxTUN, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}
	if name != "" {
		config.Name = name
	}
	iface, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("create TUN device: %w", err)
	}
	return &LinuxTUN{
		iface: iface,
		name:  iface.Name(),
		mac:   parseMAC(defaultMAC),
	}, nil
}

func (d *LinuxTUN) IsTUN() bool { return true }

func (d *LinuxTUN) Name() string { return d.name }

// Read reads a raw IP packet from TUN and wraps it in an Ethernet frame.
// Non-IP packets are throttled to prevent 100% CPU spin when receiving
// invalid packets continuously.
func (d *LinuxTUN) Read(buf []byte) (int, error) {
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

		// Broadcast dst so switch floods to all peers
		copy(buf[0:6], net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
		copy(buf[6:12], mac)
		binary.BigEndian.PutUint16(buf[12:14], etherType)

		return 14 + n, nil
	}
}

// Write strips the Ethernet header and writes raw IP to TUN.
func (d *LinuxTUN) Write(buf []byte) (int, error) {
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

func (d *LinuxTUN) SetMTU(mtu int) error {
	cmd := exec.Command("ip", "link", "set", "dev", d.name, "mtu", fmt.Sprintf("%d", mtu))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set MTU to %d: %w (stderr: %s)", mtu, err, stderr.String())
	}
	return nil
}

func (d *LinuxTUN) SetMACAddress(mac net.HardwareAddr) error {
	d.macMu.Lock()
	defer d.macMu.Unlock()
	d.mac = mac
	return nil
}

func (d *LinuxTUN) AddIPAddress(ip net.IP, mask net.IPMask) error {
	ones, bits := mask.Size()
	cidr := fmt.Sprintf("%s/%d", ip.String(), ones)

	cmd := exec.Command("ip", "addr", "add", cidr, "dev", d.name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add IP address %s/%d: %w (stderr: %s)", ip.String(), bits, err, stderr.String())
	}
	return nil
}

func (d *LinuxTUN) SetUp() error {
	cmd := exec.Command("ip", "link", "set", "dev", d.name, "up")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bring up interface: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

func (d *LinuxTUN) AddRoute(destination, gateway string, metric int) error {
	d.routeMu.Lock()
	defer d.routeMu.Unlock()

	args := []string{"route", "replace", destination}
	if gateway != "" {
		args = append(args, "via", gateway)
	}
	args = append(args, "dev", d.name, "metric", fmt.Sprintf("%d", metric))

	cmd := exec.Command("ip", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add route to %s: %w (stderr: %s)", destination, err, stderr.String())
	}
	return nil
}

func (d *LinuxTUN) EnableIPForwarding() error {
	var errs []error

	// IPv4
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errs = append(errs, fmt.Errorf("IPv4: %w (stderr: %s)", err, stderr.String()))
	}

	// IPv6
	stderr.Reset()
	cmd = exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errs = append(errs, fmt.Errorf("IPv6: %w (stderr: %s)", err, stderr.String()))
	}

	if len(errs) > 0 {
		return fmt.Errorf("enable IP forwarding failed: %v", errs)
	}
	return nil
}

func (d *LinuxTUN) RemoveRoute(destination string) error {
	cmd := exec.Command("ip", "route", "del", destination, "dev", d.name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove route %s: %w (stderr: %s)", destination, err, stderr.String())
	}
	return nil
}

func (d *LinuxTUN) AddBypassRoute(hostIP string) error {
	gw, err := getDefaultGateway()
	if err != nil {
		return fmt.Errorf("bypass route for %s: %w", hostIP, err)
	}
	cmd := exec.Command("ip", "route", "replace", hostIP+"/32", "via", gw)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add bypass route %s via %s: %w (stderr: %s)", hostIP, gw, err, stderr.String())
	}
	return nil
}

func (d *LinuxTUN) RemoveBypassRoute(hostIP string) error {
	cmd := exec.Command("ip", "route", "del", hostIP+"/32")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove bypass route %s: %w (stderr: %s)", hostIP, err, stderr.String())
	}
	return nil
}

func (d *LinuxTUN) Close() error {
	d.closeMu.Lock()
	defer d.closeMu.Unlock()

	if d.closed {
		return nil
	}
	d.closed = true

	// Try to delete the interface, but don't fail if it doesn't exist
	cmd := exec.Command("ip", "link", "delete", d.name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // Ignore error - interface might already be gone

	return d.iface.Close()
}

// SetPeerARP adds a permanent ARP entry for peer IP→MAC via this TUN interface.
func (d *LinuxTUN) SetPeerARP(ip net.IP, mac net.HardwareAddr) error {
	var stderr bytes.Buffer

	// Use replace first — this handles both new and existing (STALE) entries,
	// refreshing STALE entries to REACHABLE/PERMANENT.
	cmd := exec.Command("ip", "neigh", "replace", ip.String(), "lladdr", mac.String(), "dev", d.name, "nud", "permanent")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Entry doesn't exist yet — add it
		cmd = exec.Command("ip", "neigh", "add", ip.String(), "lladdr", mac.String(), "dev", d.name, "nud", "permanent")
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("set peer ARP %s→%s: %w (stderr: %s)", ip, mac, err, stderr.String())
		}
	}
	return nil
}
