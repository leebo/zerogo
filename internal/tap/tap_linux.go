//go:build linux

package tap

import (
	"fmt"
	"net"
	"os/exec"

	"github.com/songgao/water"
)

// LinuxTAP implements Device using songgao/water for Linux.
type LinuxTAP struct {
	iface *water.Interface
	name  string
}

// NewLinuxTAP creates a new TAP device on Linux.
// If name is empty, the OS assigns a name.
func NewLinuxTAP(name string) (*LinuxTAP, error) {
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
	return exec.Command("ip", "link", "set", "dev", d.name, "mtu", fmt.Sprintf("%d", mtu)).Run()
}

func (d *LinuxTAP) SetMACAddress(mac net.HardwareAddr) error {
	// Must bring interface down first to change MAC
	if err := exec.Command("ip", "link", "set", "dev", d.name, "down").Run(); err != nil {
		return fmt.Errorf("bring down interface: %w", err)
	}
	if err := exec.Command("ip", "link", "set", "dev", d.name, "address", mac.String()).Run(); err != nil {
		return fmt.Errorf("set MAC address: %w", err)
	}
	return exec.Command("ip", "link", "set", "dev", d.name, "up").Run()
}

func (d *LinuxTAP) AddIPAddress(ip net.IP, mask net.IPMask) error {
	ones, _ := mask.Size()
	cidr := fmt.Sprintf("%s/%d", ip.String(), ones)
	return exec.Command("ip", "addr", "add", cidr, "dev", d.name).Run()
}

func (d *LinuxTAP) SetUp() error {
	return exec.Command("ip", "link", "set", "dev", d.name, "up").Run()
}

func (d *LinuxTAP) Close() error {
	// Delete the interface
	_ = exec.Command("ip", "link", "delete", d.name).Run()
	return d.iface.Close()
}
