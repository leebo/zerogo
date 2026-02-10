package identity

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/blake2s"
)

const (
	// AddressSize is the byte length of a node address (40 bits = 5 bytes).
	AddressSize = 5
)

// Address is a 40-bit node address derived from the public key.
type Address [AddressSize]byte

// AddressFromPublicKey derives a 40-bit address from a Curve25519 public key
// using BLAKE2s. The first 5 bytes of the hash become the address.
func AddressFromPublicKey(pubKey []byte) Address {
	hash := blake2s.Sum256(pubKey)
	var addr Address
	copy(addr[:], hash[:AddressSize])
	// Ensure first byte is non-zero (reserved addresses start with 0x00)
	if addr[0] == 0 {
		addr[0] = 1
	}
	return addr
}

// AddressFromHex parses a hex-encoded address string.
func AddressFromHex(s string) (Address, error) {
	var addr Address
	b, err := hex.DecodeString(s)
	if err != nil {
		return addr, fmt.Errorf("invalid hex address: %w", err)
	}
	if len(b) != AddressSize {
		return addr, fmt.Errorf("address must be %d bytes, got %d", AddressSize, len(b))
	}
	copy(addr[:], b)
	return addr, nil
}

// String returns the hex-encoded address.
func (a Address) String() string {
	return hex.EncodeToString(a[:])
}

// IsZero returns true if the address is all zeros.
func (a Address) IsZero() bool {
	return a == Address{}
}

// Uint64 converts the address to a uint64 for use as a map key.
func (a Address) Uint64() uint64 {
	// Pad to 8 bytes, big-endian
	var buf [8]byte
	copy(buf[3:], a[:])
	return binary.BigEndian.Uint64(buf[:])
}
