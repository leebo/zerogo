//go:build !linux && !darwin && !windows

package tap

import (
	"fmt"
	"net"
	"runtime"
)

// StubTAP is a placeholder for unsupported platforms during development.
type StubTAP struct {
	name string
}

func NewTAP(name string) (*StubTAP, error) {
	return nil, fmt.Errorf("TAP devices not supported on %s", runtime.GOOS)
}

func (d *StubTAP) IsTUN() bool                                 { return false }
func (d *StubTAP) Name() string                                { return d.name }
func (d *StubTAP) Read(buf []byte) (int, error)                { return 0, fmt.Errorf("stub") }
func (d *StubTAP) Write(buf []byte) (int, error)               { return 0, fmt.Errorf("stub") }
func (d *StubTAP) SetMTU(mtu int) error                        { return fmt.Errorf("stub") }
func (d *StubTAP) SetMACAddress(mac net.HardwareAddr) error     { return fmt.Errorf("stub") }
func (d *StubTAP) AddIPAddress(ip net.IP, mask net.IPMask) error { return fmt.Errorf("stub") }
func (d *StubTAP) SetUp() error                                { return fmt.Errorf("stub") }
func (d *StubTAP) EnableIPForwarding() error {
	return fmt.Errorf("IP forwarding not supported on %s", runtime.GOOS)
}
func (d *StubTAP) AddRoute(destination, gateway string, metric int) error {
	return fmt.Errorf("routes not supported on %s", runtime.GOOS)
}
func (d *StubTAP) RemoveRoute(destination string) error {
	return fmt.Errorf("routes not supported on %s", runtime.GOOS)
}
func (d *StubTAP) AddBypassRoute(hostIP string) error {
	return fmt.Errorf("bypass routes not supported on %s", runtime.GOOS)
}
func (d *StubTAP) RemoveBypassRoute(hostIP string) error {
	return fmt.Errorf("bypass routes not supported on %s", runtime.GOOS)
}
func (d *StubTAP) Close() error { return nil }
func (d *StubTAP) SetPeerARP(ip net.IP, mac net.HardwareAddr) error { return nil }

func NewTUN(name string) (*StubTAP, error) {
	return nil, fmt.Errorf("TUN devices not supported on %s", runtime.GOOS)
}
