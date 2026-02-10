package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/curve25519"
)

const (
	PrivateKeySize = 32
	PublicKeySize  = 32
)

// Identity holds a node's Curve25519 keypair and derived address.
type Identity struct {
	PrivateKey [PrivateKeySize]byte
	PublicKey  [PublicKeySize]byte
	Address    Address
}

// Generate creates a new random identity.
func Generate() (*Identity, error) {
	id := &Identity{}
	if _, err := rand.Read(id.PrivateKey[:]); err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}
	// Clamp private key per Curve25519 convention
	id.PrivateKey[0] &= 248
	id.PrivateKey[31] &= 127
	id.PrivateKey[31] |= 64

	pub, err := curve25519.X25519(id.PrivateKey[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("derive public key: %w", err)
	}
	copy(id.PublicKey[:], pub)
	id.Address = AddressFromPublicKey(id.PublicKey[:])
	return id, nil
}

// FromPrivateKey recreates an identity from a private key.
func FromPrivateKey(privKey [PrivateKeySize]byte) (*Identity, error) {
	id := &Identity{PrivateKey: privKey}
	pub, err := curve25519.X25519(id.PrivateKey[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("derive public key: %w", err)
	}
	copy(id.PublicKey[:], pub)
	id.Address = AddressFromPublicKey(id.PublicKey[:])
	return id, nil
}

// LoadOrGenerate loads an identity from file, or generates a new one.
func LoadOrGenerate(path string) (*Identity, error) {
	data, err := os.ReadFile(path)
	if err == nil && len(data) == PrivateKeySize {
		var privKey [PrivateKeySize]byte
		copy(privKey[:], data)
		return FromPrivateKey(privKey)
	}
	// Generate new identity
	id, err := Generate()
	if err != nil {
		return nil, err
	}
	// Save to file
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create identity directory: %w", err)
	}
	if err := os.WriteFile(path, id.PrivateKey[:], 0600); err != nil {
		return nil, fmt.Errorf("save identity: %w", err)
	}
	return id, nil
}

// PublicKeyHex returns the public key as a hex string.
func (id *Identity) PublicKeyHex() string {
	return hex.EncodeToString(id.PublicKey[:])
}

// String returns a human-readable identity summary.
func (id *Identity) String() string {
	return fmt.Sprintf("Identity{addr=%s, pubkey=%s...}", id.Address, id.PublicKeyHex()[:16])
}
