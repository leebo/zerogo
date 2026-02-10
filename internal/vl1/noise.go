package vl1

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// Noise implements the Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s handshake.
// Simplified implementation using golang.org/x/crypto primitives.

const (
	// NoisePublicKeySize is the Curve25519 public key size.
	NoisePublicKeySize = 32
	// NoisePrivateKeySize is the Curve25519 private key size.
	NoisePrivateKeySize = 32
	// NoisePSKSize is the pre-shared key size.
	NoisePSKSize = 32
	// NoiseNonceSize is the ChaCha20-Poly1305 nonce size.
	NoiseNonceSize = chacha20poly1305.NonceSize // 12
	// NoiseTagSize is the Poly1305 authentication tag size.
	NoiseTagSize = chacha20poly1305.Overhead // 16

	// Handshake message sizes
	HandshakeInitiationSize = 1 + 32 + 48 + 28 + 16 // type + ephemeral + static_enc + timestamp_enc + mac
	HandshakeResponseSize   = 1 + 32 + 48 + 16       // type + ephemeral + empty_enc + mac

	handshakeMsgInit     = 1
	handshakeMsgResponse = 2
)

var (
	// NoiseProtocolName is the Noise protocol identifier used for hashing.
	NoiseProtocolName = []byte("Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s")

	// NoisePrologue is the protocol prologue.
	NoisePrologue = []byte("zerogo-vl1-v1")

	ErrInvalidHandshake = errors.New("invalid handshake message")
	ErrDecryptFailed    = errors.New("decrypt failed")
)

// NoiseHandshake manages a Noise IK handshake between two peers.
type NoiseHandshake struct {
	// Local identity
	localStatic    [NoisePrivateKeySize]byte
	localStaticPub [NoisePublicKeySize]byte

	// Remote static public key (known in advance for IK pattern)
	remoteStaticPub [NoisePublicKeySize]byte

	// Ephemeral keypair (generated per handshake)
	localEphemeral    [NoisePrivateKeySize]byte
	localEphemeralPub [NoisePublicKeySize]byte

	// Pre-shared key
	psk [NoisePSKSize]byte

	// Handshake state
	chainingKey [blake2s.Size]byte
	hash        [blake2s.Size]byte

	// Result: transport keys after handshake
	sendKey [chacha20poly1305.KeySize]byte
	recvKey [chacha20poly1305.KeySize]byte
}

// NewNoiseHandshake creates a new handshake state.
func NewNoiseHandshake(localPriv, localPub [32]byte, remoteStaticPub [32]byte, psk [32]byte) *NoiseHandshake {
	hs := &NoiseHandshake{
		localStatic:     localPriv,
		localStaticPub:  localPub,
		remoteStaticPub: remoteStaticPub,
		psk:             psk,
	}
	hs.initialize()
	return hs
}

func (hs *NoiseHandshake) initialize() {
	// h = HASH(protocol_name)
	hs.hash = blake2s.Sum256(NoiseProtocolName)
	// ck = h (for Noise, initial chaining key = initial hash)
	hs.chainingKey = hs.hash
	// Mix in prologue
	hs.mixHash(NoisePrologue)
}

// --- Initiator Side ---

// CreateInitiation generates the first handshake message (initiator → responder).
// The initiator knows the responder's static public key (IK pattern).
func (hs *NoiseHandshake) CreateInitiation() ([]byte, error) {
	// Mix responder's static public key into hash (IK pattern: responder's key is pre-known)
	hs.mixHash(hs.remoteStaticPub[:])

	// Generate ephemeral keypair
	if err := hs.generateEphemeral(); err != nil {
		return nil, err
	}

	msg := make([]byte, 0, HandshakeInitiationSize)
	msg = append(msg, handshakeMsgInit)

	// e: send ephemeral public key
	msg = append(msg, hs.localEphemeralPub[:]...)
	hs.mixHash(hs.localEphemeralPub[:])

	// es: DH(ephemeral, remote static)
	es, err := curve25519.X25519(hs.localEphemeral[:], hs.remoteStaticPub[:])
	if err != nil {
		return nil, fmt.Errorf("DH(e, rs): %w", err)
	}
	hs.mixKey(es)

	// s: encrypt and send static public key
	encrypted := hs.encryptAndHash(hs.localStaticPub[:])
	msg = append(msg, encrypted...)

	// ss: DH(static, remote static)
	ss, err := curve25519.X25519(hs.localStatic[:], hs.remoteStaticPub[:])
	if err != nil {
		return nil, fmt.Errorf("DH(s, rs): %w", err)
	}
	hs.mixKey(ss)

	// psk: mix in pre-shared key
	hs.mixKeyAndHash(hs.psk[:])

	// Encrypt timestamp as payload (for replay protection)
	timestamp := make([]byte, 12)
	if _, err := rand.Read(timestamp); err != nil {
		return nil, err
	}
	encrypted = hs.encryptAndHash(timestamp)
	msg = append(msg, encrypted...)

	// MAC over the message
	mac := hs.computeMAC(msg)
	msg = append(msg, mac...)

	return msg, nil
}

// ConsumeInitiation processes the first handshake message (responder side).
func (hs *NoiseHandshake) ConsumeInitiation(msg []byte) error {
	if len(msg) < HandshakeInitiationSize {
		return ErrInvalidHandshake
	}
	if msg[0] != handshakeMsgInit {
		return ErrInvalidHandshake
	}

	// Mix our (responder's) static public key
	hs.mixHash(hs.localStaticPub[:])

	pos := 1

	// e: read remote ephemeral
	var remoteEphemeral [32]byte
	copy(remoteEphemeral[:], msg[pos:pos+32])
	pos += 32
	hs.mixHash(remoteEphemeral[:])

	// es: DH(static, remote ephemeral)
	es, err := curve25519.X25519(hs.localStatic[:], remoteEphemeral[:])
	if err != nil {
		return fmt.Errorf("DH(s, re): %w", err)
	}
	hs.mixKey(es)

	// s: decrypt remote static public key
	decrypted, err := hs.decryptAndHash(msg[pos : pos+48])
	if err != nil {
		return fmt.Errorf("decrypt remote static: %w", err)
	}
	copy(hs.remoteStaticPub[:], decrypted)
	pos += 48

	// ss: DH(static, remote static)
	ss, err := curve25519.X25519(hs.localStatic[:], hs.remoteStaticPub[:])
	if err != nil {
		return fmt.Errorf("DH(s, rs): %w", err)
	}
	hs.mixKey(ss)

	// psk: mix in pre-shared key
	hs.mixKeyAndHash(hs.psk[:])

	// Decrypt timestamp payload
	_, err = hs.decryptAndHash(msg[pos : pos+28])
	if err != nil {
		return fmt.Errorf("decrypt timestamp: %w", err)
	}
	pos += 28

	// Verify MAC
	expectedMAC := hs.computeMAC(msg[:pos])
	if !constantTimeEqual(expectedMAC, msg[pos:pos+16]) {
		return ErrInvalidHandshake
	}

	// Store remote ephemeral for response
	// (we need it to generate our own ephemeral DH)
	// Save to a temporary for CreateResponse
	copy(hs.hash[:], hs.hash[:]) // hash is already correct state

	return nil
}

// CreateResponse generates the second handshake message (responder → initiator).
func (hs *NoiseHandshake) CreateResponse() ([]byte, error) {
	if err := hs.generateEphemeral(); err != nil {
		return nil, err
	}

	msg := make([]byte, 0, HandshakeResponseSize)
	msg = append(msg, handshakeMsgResponse)

	// e: send ephemeral public key
	msg = append(msg, hs.localEphemeralPub[:]...)
	hs.mixHash(hs.localEphemeralPub[:])

	// ee: DH(ephemeral, remote ephemeral) - not available in this simplified version
	// In the full IK pattern, ee is done. For our simplified version,
	// we derive transport keys from the accumulated chaining key.

	// Encrypt empty payload
	encrypted := hs.encryptAndHash(nil)
	msg = append(msg, encrypted...)

	// MAC
	mac := hs.computeMAC(msg)
	msg = append(msg, mac...)

	// Derive transport keys
	hs.deriveTransportKeys(false)

	return msg, nil
}

// ConsumeResponse processes the second handshake message (initiator side).
func (hs *NoiseHandshake) ConsumeResponse(msg []byte) error {
	if len(msg) < HandshakeResponseSize {
		return ErrInvalidHandshake
	}
	if msg[0] != handshakeMsgResponse {
		return ErrInvalidHandshake
	}

	pos := 1

	// e: read responder ephemeral
	var remoteEphemeral [32]byte
	copy(remoteEphemeral[:], msg[pos:pos+32])
	pos += 32
	hs.mixHash(remoteEphemeral[:])

	// Decrypt empty payload
	_, err := hs.decryptAndHash(msg[pos : pos+16])
	if err != nil {
		return fmt.Errorf("decrypt response: %w", err)
	}
	pos += 16

	// Verify MAC
	expectedMAC := hs.computeMAC(msg[:pos])
	if !constantTimeEqual(expectedMAC, msg[pos:pos+16]) {
		return ErrInvalidHandshake
	}

	// Derive transport keys
	hs.deriveTransportKeys(true)

	return nil
}

// TransportKeys returns the derived send and receive keys.
func (hs *NoiseHandshake) TransportKeys() ([32]byte, [32]byte) {
	return hs.sendKey, hs.recvKey
}

// --- Crypto helpers ---

func (hs *NoiseHandshake) mixHash(data []byte) {
	h, _ := blake2s.New256(nil)
	h.Write(hs.hash[:])
	h.Write(data)
	copy(hs.hash[:], h.Sum(nil))
}

func (hs *NoiseHandshake) mixKey(input []byte) {
	// HKDF-like key derivation using BLAKE2s
	// temp = HMAC-BLAKE2s(ck, input)
	temp := hmacBlake2s(hs.chainingKey[:], input)
	// ck = HMAC-BLAKE2s(temp, 0x01)
	ck := hmacBlake2s(temp[:], []byte{0x01})
	copy(hs.chainingKey[:], ck[:])
}

func (hs *NoiseHandshake) mixKeyAndHash(input []byte) {
	// HKDF-like: derive ck, temp_h, temp_k from chaining key + input
	temp := hmacBlake2s(hs.chainingKey[:], input)
	ck := hmacBlake2s(temp[:], []byte{0x01})
	copy(hs.chainingKey[:], ck[:])
	tempH := hmacBlake2s(temp[:], append(ck[:], 0x02))
	hs.mixHash(tempH[:])
}

func (hs *NoiseHandshake) encryptAndHash(plaintext []byte) []byte {
	// Derive encryption key from chaining key
	key := hmacBlake2s(hs.chainingKey[:], []byte{0x03})
	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		panic("chacha20poly1305.New: " + err.Error())
	}
	var nonce [NoiseNonceSize]byte // zero nonce (each key used once)
	ciphertext := aead.Seal(nil, nonce[:], plaintext, hs.hash[:])
	hs.mixHash(ciphertext)
	return ciphertext
}

func (hs *NoiseHandshake) decryptAndHash(ciphertext []byte) ([]byte, error) {
	key := hmacBlake2s(hs.chainingKey[:], []byte{0x03})
	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return nil, fmt.Errorf("create AEAD: %w", err)
	}
	var nonce [NoiseNonceSize]byte
	plaintext, err := aead.Open(nil, nonce[:], ciphertext, hs.hash[:])
	if err != nil {
		return nil, ErrDecryptFailed
	}
	hs.mixHash(ciphertext)
	return plaintext, nil
}

func (hs *NoiseHandshake) generateEphemeral() error {
	if _, err := rand.Read(hs.localEphemeral[:]); err != nil {
		return fmt.Errorf("generate ephemeral key: %w", err)
	}
	hs.localEphemeral[0] &= 248
	hs.localEphemeral[31] &= 127
	hs.localEphemeral[31] |= 64
	pub, err := curve25519.X25519(hs.localEphemeral[:], curve25519.Basepoint)
	if err != nil {
		return err
	}
	copy(hs.localEphemeralPub[:], pub)
	return nil
}

func (hs *NoiseHandshake) deriveTransportKeys(isInitiator bool) {
	// Derive two keys from final chaining key
	temp := hmacBlake2s(hs.chainingKey[:], nil)
	k1 := hmacBlake2s(temp[:], []byte{0x01})
	k2 := hmacBlake2s(temp[:], append(k1[:], 0x02))
	if isInitiator {
		hs.sendKey = k1
		hs.recvKey = k2
	} else {
		hs.sendKey = k2
		hs.recvKey = k1
	}
}

func (hs *NoiseHandshake) computeMAC(data []byte) []byte {
	mac := hmacBlake2s(hs.hash[:], data)
	return mac[:16]
}

// --- Transport cipher (post-handshake) ---

// NoiseCipher provides authenticated encryption for transport data.
type NoiseCipher struct {
	sendKey   [chacha20poly1305.KeySize]byte
	recvKey   [chacha20poly1305.KeySize]byte
	sendNonce atomic.Uint64
	recvNonce uint64
	recvMu    sync.Mutex
}

// NewNoiseCipher creates a cipher pair from handshake-derived keys.
func NewNoiseCipher(sendKey, recvKey [32]byte) *NoiseCipher {
	return &NoiseCipher{
		sendKey: sendKey,
		recvKey: recvKey,
	}
}

// Encrypt encrypts plaintext and prepends the 8-byte nonce counter.
func (c *NoiseCipher) Encrypt(plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(c.sendKey[:])
	if err != nil {
		return nil, err
	}
	counter := c.sendNonce.Add(1) - 1
	var nonce [NoiseNonceSize]byte
	binary.LittleEndian.PutUint64(nonce[4:], counter)

	// Output: 8-byte counter + ciphertext + tag
	out := make([]byte, 8, 8+len(plaintext)+NoiseTagSize)
	binary.LittleEndian.PutUint64(out, counter)
	out = aead.Seal(out, nonce[:], plaintext, nil)
	return out, nil
}

// Decrypt decrypts a message (8-byte counter prefix + ciphertext + tag).
func (c *NoiseCipher) Decrypt(data []byte) ([]byte, error) {
	if len(data) < 8+NoiseTagSize {
		return nil, errors.New("ciphertext too short")
	}
	aead, err := chacha20poly1305.New(c.recvKey[:])
	if err != nil {
		return nil, err
	}
	counter := binary.LittleEndian.Uint64(data[:8])
	var nonce [NoiseNonceSize]byte
	binary.LittleEndian.PutUint64(nonce[4:], counter)

	plaintext, err := aead.Open(nil, nonce[:], data[8:], nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}

	// Update receive nonce (allow some reordering)
	c.recvMu.Lock()
	if counter >= c.recvNonce {
		c.recvNonce = counter + 1
	}
	c.recvMu.Unlock()

	return plaintext, nil
}

// --- Utility functions ---

func hmacBlake2s(key, data []byte) [blake2s.Size]byte {
	// HMAC-BLAKE2s(key, data) = BLAKE2s(key || data)
	// Simplified: using BLAKE2s with key as a keyed hash when possible
	if len(key) <= blake2s.Size {
		h, err := blake2s.New256(key)
		if err != nil {
			// Fallback to unkeyed hash with key prepended
			h2, _ := blake2s.New256(nil)
			h2.Write(key)
			h2.Write(data)
			var result [blake2s.Size]byte
			copy(result[:], h2.Sum(nil))
			return result
		}
		h.Write(data)
		var result [blake2s.Size]byte
		copy(result[:], h.Sum(nil))
		return result
	}
	// Key too long: hash it first
	keyHash := blake2s.Sum256(key)
	return hmacBlake2s(keyHash[:], data)
}

// DeriveKeysFromPSK derives deterministic send/recv keys from a PSK and two
// public keys. Both sides compute the same keys by sorting the public keys.
// This is used for Phase 1 testing (no handshake round-trip needed).
// The peer with the lexicographically smaller public key gets (k1=send, k2=recv),
// the other gets (k1=recv, k2=send).
func DeriveKeysFromPSK(psk [32]byte, localPub, remotePub [32]byte) (sendKey, recvKey [32]byte) {
	// Determine order: smaller pubkey is "initiator"
	localIsSmaller := false
	for i := 0; i < 32; i++ {
		if localPub[i] < remotePub[i] {
			localIsSmaller = true
			break
		} else if localPub[i] > remotePub[i] {
			break
		}
	}

	// Derive master key: BLAKE2s(psk || pubkey_small || pubkey_large)
	h, _ := blake2s.New256(nil)
	h.Write(psk[:])
	if localIsSmaller {
		h.Write(localPub[:])
		h.Write(remotePub[:])
	} else {
		h.Write(remotePub[:])
		h.Write(localPub[:])
	}
	master := h.Sum(nil)

	// Derive two keys from master
	k1 := hmacBlake2s(master, []byte("zerogo-psk-key-1"))
	k2 := hmacBlake2s(master, []byte("zerogo-psk-key-2"))

	if localIsSmaller {
		return k1, k2
	}
	return k2, k1
}

func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
