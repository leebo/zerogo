package vl2

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

const (
	// EthernetHeaderSize is the minimum Ethernet header size (no VLAN tag).
	EthernetHeaderSize = 14
	// MinFrameSize is the minimum valid Ethernet frame size.
	MinFrameSize = EthernetHeaderSize
	// MaxFrameSize is the maximum Ethernet frame size (jumbo frame).
	MaxFrameSize = 9000
)

// Common EtherTypes
const (
	EtherTypeIPv4 = 0x0800
	EtherTypeARP  = 0x0806
	EtherTypeIPv6 = 0x86DD
	EtherTypeVLAN = 0x8100
)

// EthernetFrame represents a parsed Ethernet frame.
type EthernetFrame struct {
	DstMAC    net.HardwareAddr
	SrcMAC    net.HardwareAddr
	EtherType uint16
	Payload   []byte
	Raw       []byte // Original raw frame
}

// ParseEthernetFrame parses an Ethernet frame from raw bytes.
func ParseEthernetFrame(data []byte) (*EthernetFrame, error) {
	if len(data) < MinFrameSize {
		return nil, errors.New("frame too short")
	}
	f := &EthernetFrame{
		DstMAC:    net.HardwareAddr(data[0:6]),
		SrcMAC:    net.HardwareAddr(data[6:12]),
		EtherType: binary.BigEndian.Uint16(data[12:14]),
		Payload:   data[EthernetHeaderSize:],
		Raw:       data,
	}
	return f, nil
}

// IsBroadcast returns true if the destination is the broadcast address.
func (f *EthernetFrame) IsBroadcast() bool {
	return f.DstMAC[0] == 0xff && f.DstMAC[1] == 0xff && f.DstMAC[2] == 0xff &&
		f.DstMAC[3] == 0xff && f.DstMAC[4] == 0xff && f.DstMAC[5] == 0xff
}

// IsMulticast returns true if the destination is a multicast address.
// Multicast MACs have the least significant bit of the first byte set.
func (f *EthernetFrame) IsMulticast() bool {
	return f.DstMAC[0]&0x01 != 0
}

// IsUnicast returns true if the destination is a unicast address.
func (f *EthernetFrame) IsUnicast() bool {
	return !f.IsMulticast() // broadcast is a subset of multicast
}

// IsARP returns true if this is an ARP frame.
func (f *EthernetFrame) IsARP() bool {
	return f.EtherType == EtherTypeARP
}

// String returns a human-readable summary of the frame.
func (f *EthernetFrame) String() string {
	etherType := fmt.Sprintf("0x%04x", f.EtherType)
	switch f.EtherType {
	case EtherTypeIPv4:
		etherType = "IPv4"
	case EtherTypeARP:
		etherType = "ARP"
	case EtherTypeIPv6:
		etherType = "IPv6"
	}
	return fmt.Sprintf("%s → %s [%s] %d bytes", f.SrcMAC, f.DstMAC, etherType, len(f.Raw))
}
