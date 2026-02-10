package vl1

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	// HeaderSize is the VL1 packet header length.
	HeaderSize = 8

	// MaxPacketSize is the maximum VL1 packet size (UDP-safe).
	MaxPacketSize = 65535

	// MaxPayloadSize is the maximum payload after header.
	MaxPayloadSize = MaxPacketSize - HeaderSize

	// Version is the current protocol version.
	Version = 1
)

// PacketType identifies the VL1 packet type.
type PacketType uint8

const (
	PacketTypeData      PacketType = 0x01
	PacketTypeControl   PacketType = 0x02
	PacketTypeKeepalive PacketType = 0x03
	PacketTypeHandshake PacketType = 0x04
)

func (t PacketType) String() string {
	switch t {
	case PacketTypeData:
		return "data"
	case PacketTypeControl:
		return "control"
	case PacketTypeKeepalive:
		return "keepalive"
	case PacketTypeHandshake:
		return "handshake"
	default:
		return fmt.Sprintf("unknown(0x%02x)", uint8(t))
	}
}

// Header is the VL1 packet header (8 bytes).
//
//	┌─────────────────────────────────────────┐
//	│ Version (1B) | Type (1B) | NetworkID (4B) | Reserved (2B) │
//	└─────────────────────────────────────────┘
type Header struct {
	Version   uint8
	Type      PacketType
	NetworkID uint32
	Reserved  uint16
}

// Encode writes the header into buf (must be >= HeaderSize).
func (h *Header) Encode(buf []byte) {
	buf[0] = h.Version
	buf[1] = uint8(h.Type)
	binary.BigEndian.PutUint32(buf[2:6], h.NetworkID)
	binary.BigEndian.PutUint16(buf[6:8], h.Reserved)
}

// DecodeHeader parses a header from buf.
func DecodeHeader(buf []byte) (Header, error) {
	if len(buf) < HeaderSize {
		return Header{}, errors.New("packet too short for header")
	}
	h := Header{
		Version:   buf[0],
		Type:      PacketType(buf[1]),
		NetworkID: binary.BigEndian.Uint32(buf[2:6]),
		Reserved:  binary.BigEndian.Uint16(buf[6:8]),
	}
	if h.Version != Version {
		return h, fmt.Errorf("unsupported version: %d", h.Version)
	}
	return h, nil
}

// Packet represents a complete VL1 packet with header and payload.
type Packet struct {
	Header  Header
	Payload []byte
}

// Encode serializes the packet into a byte slice.
func (p *Packet) Encode() []byte {
	buf := make([]byte, HeaderSize+len(p.Payload))
	p.Header.Encode(buf[:HeaderSize])
	copy(buf[HeaderSize:], p.Payload)
	return buf
}

// DecodePacket parses a complete packet from raw bytes.
func DecodePacket(data []byte) (*Packet, error) {
	hdr, err := DecodeHeader(data)
	if err != nil {
		return nil, err
	}
	return &Packet{
		Header:  hdr,
		Payload: data[HeaderSize:],
	}, nil
}

// NewDataPacket creates a data packet for carrying VL2 Ethernet frames.
func NewDataPacket(networkID uint32, payload []byte) *Packet {
	return &Packet{
		Header: Header{
			Version:   Version,
			Type:      PacketTypeData,
			NetworkID: networkID,
		},
		Payload: payload,
	}
}

// NewKeepalivePacket creates a keepalive packet.
func NewKeepalivePacket() *Packet {
	return &Packet{
		Header: Header{
			Version: Version,
			Type:    PacketTypeKeepalive,
		},
	}
}

// NewHandshakePacket creates a handshake packet carrying Noise protocol messages.
func NewHandshakePacket(payload []byte) *Packet {
	return &Packet{
		Header: Header{
			Version: Version,
			Type:    PacketTypeHandshake,
		},
		Payload: payload,
	}
}
