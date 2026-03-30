//go:build linux && !android

package tap

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/songgao/water"
)

// LinuxTAP implements Device using songgao/water for Linux.
type LinuxTAP struct {
	iface *water.Interface
	name  string
}

// NewTAP creates a new TAP device.
// If name is empty, the OS assigns a name.
func NewTAP(name string) (*LinuxTAP, error) {
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
	return &LinuxTAP{
		iface: iface,
		name:  iface.Name(),
	}, nil
}

func (d *LinuxTAP) IsTUN() bool { return false }

func (d *LinuxTAP) Name() string {
	return d.name
}

func (d *LinuxTAP) Read(buf []byte) (int, error) {
	return d.iface.Read(buf)
}

func (d *LinuxTAP) Write(buf []byte) (int, error) {
	return d.iface.Write(buf)
}

func (d *LinuxTAP) SetMTU(mtu int) error {
	cmd := exec.Command("ip", "link", "set", "dev", d.name, "mtu", fmt.Sprintf("%d", mtu))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set MTU to %d: %w (stderr: %s)", mtu, err, stderr.String())
	}
	return nil
}

func (d *LinuxTAP) SetMACAddress(mac net.HardwareAddr) error {
	// Must bring interface down first to change MAC
	if err := exec.Command("ip", "link", "set", "dev", d.name, "down").Run(); err != nil {
		return fmt.Errorf("bring down interface: %w", err)
	}

	// Ensure we bring interface back up even if MAC change fails
	var macErr error
	defer func() {
		if upErr := exec.Command("ip", "link", "set", "dev", d.name, "up").Run(); upErr != nil {
			// If MAC setting already failed, combine errors
			if macErr != nil {
				macErr = fmt.Errorf("set MAC failed: %v; additionally, bring up failed: %w", macErr, upErr)
			} else {
				macErr = fmt.Errorf("bring up interface: %w", upErr)
			}
		}
	}()

	if err := exec.Command("ip", "link", "set", "dev", d.name, "address", mac.String()).Run(); err != nil {
		macErr = fmt.Errorf("set MAC address: %w", err)
		return macErr
	}

	return nil
}

func (d *LinuxTAP) AddIPAddress(ip net.IP, mask net.IPMask) error {
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

func (d *LinuxTAP) SetUp() error {
	cmd := exec.Command("ip", "link", "set", "dev", d.name, "up")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bring up interface: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

func (d *LinuxTAP) AddRoute(destination, gateway string, metric int) error {
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

func (d *LinuxTAP) EnableIPForwarding() error {
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

func (d *LinuxTAP) RemoveRoute(destination string) error {
	cmd := exec.Command("ip", "route", "del", destination, "dev", d.name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove route %s: %w (stderr: %s)", destination, err, stderr.String())
	}
	return nil
}

// getDefaultGateway returns the system's default IPv4 gateway IP.
func getDefaultGateway() (string, error) {
	cmd := exec.Command("ip", "route", "show", "default")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("get default route: %w", err)
	}
	// Output format: "default via 10.0.0.1 dev eth0"
	fields := strings.Fields(stdout.String())
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", fmt.Errorf("no default gateway found")
}

func (d *LinuxTAP) AddBypassRoute(hostIP string) error {
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

func (d *LinuxTAP) RemoveBypassRoute(hostIP string) error {
	cmd := exec.Command("ip", "route", "del", hostIP+"/32")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove bypass route %s: %w (stderr: %s)", hostIP, err, stderr.String())
	}
	return nil
}

// SetPeerARP adds a permanent ARP entry for peer IP→MAC via this TAP interface.
func (d *LinuxTAP) SetPeerARP(ip net.IP, mac net.HardwareAddr) error {
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

// Close closes the TAP device and cleans up resources.
func (d *LinuxTAP) Close() error {
	// Try to delete the interface, but don't fail if it doesn't exist
	cmd := exec.Command("ip", "link", "delete", d.name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // Ignore error - interface might already be gone

	return d.iface.Close()
}
