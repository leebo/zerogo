//go:build !linux

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

func NewLinuxTAP(name string) (*StubTAP, error) {
	return nil, fmt.Errorf("TAP devices not supported on %s (Linux required)", runtime.GOOS)
}

func (d *StubTAP) Name() string                                { return d.name }
func (d *StubTAP) Read(buf []byte) (int, error)                { return 0, fmt.Errorf("stub") }
func (d *StubTAP) Write(buf []byte) (int, error)               { return 0, fmt.Errorf("stub") }
func (d *StubTAP) SetMTU(mtu int) error                        { return fmt.Errorf("stub") }
func (d *StubTAP) SetMACAddress(mac net.HardwareAddr) error     { return fmt.Errorf("stub") }
func (d *StubTAP) AddIPAddress(ip net.IP, mask net.IPMask) error { return fmt.Errorf("stub") }
func (d *StubTAP) SetUp() error                                { return fmt.Errorf("stub") }
func (d *StubTAP) Close() error                                { return nil }
