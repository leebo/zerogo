package vl2

import (
	"encoding/binary"
	"net"

	"github.com/unicornultrafoundation/zerogo/internal/identity"
)

// GenerateMAC creates a deterministic locally-administered MAC address
// from a network ID and node address.
//
// Format: 02:XX:XX:XX:XX:XX
//   - Byte 0: 0x02 (locally administered, unicast)
//   - Bytes 1-2: derived from Network ID
//   - Bytes 3-5: derived from Node Address (first 3 bytes of 5-byte address)
func GenerateMAC(networkID uint32, nodeAddr identity.Address) net.HardwareAddr {
	mac := make(net.HardwareAddr, 6)

	// Byte 0: locally administered unicast
	mac[0] = 0x02

	// Bytes 1-2: from network ID
	var netIDBytes [4]byte
	binary.BigEndian.PutUint32(netIDBytes[:], networkID)
	mac[1] = netIDBytes[2]
	mac[2] = netIDBytes[3]

	// Bytes 3-5: from node address
	mac[3] = nodeAddr[0]
	mac[4] = nodeAddr[1]
	mac[5] = nodeAddr[2]

	return mac
}

// MACKey is a [6]byte type usable as a map key.
type MACKey [6]byte

// MACToKey converts a net.HardwareAddr to a map key.
func MACToKey(mac net.HardwareAddr) MACKey {
	var key MACKey
	copy(key[:], mac)
	return key
}
