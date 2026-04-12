// Package auth implements the cryptographic primitives used by the
// asobi-cli device-code login flow.
//
// Protocol: clean-room reimplementation of the Supabase CLI device
// login using standard primitives (NIST SP 800-56A for ECDH key
// agreement on P-256, RFC 5869 HKDF-SHA256 for key derivation,
// NIST SP 800-38D AES-256-GCM for authenticated encryption).
//
// The CLI generates an ephemeral P-256 keypair and sends its public
// key to asobi_saas. The saas side generates its own ephemeral
// keypair, derives the ECDH shared secret, encrypts the issued API
// key with AES-256-GCM, and returns the ciphertext plus its public
// key. The CLI recomputes the shared secret from its private key and
// the saas public key, then decrypts.
//
// The Erlang counterpart lives in asobi_saas_cli_crypto. Both sides
// use the same HKDF info string ("asobi-cli-device-login-v1") and
// an empty salt to maintain domain-separation across future versions.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"fmt"

	"crypto/sha256"
	"golang.org/x/crypto/hkdf"
	"io"
)

// HKDFInfo is the domain-separated info string used by HKDF-SHA256.
// Must match the ?INFO macro in asobi_saas_cli_crypto.erl exactly.
const HKDFInfo = "asobi-cli-device-login-v1"

const (
	keyBytes   = 32
	nonceBytes = 12
	tagBytes   = 16
)

// Keypair holds a raw P-256 public key (65 bytes, uncompressed SEC1)
// and its corresponding private scalar (32 bytes).
type Keypair struct {
	Public  []byte
	Private []byte
}

// GenerateKeypair produces a fresh ephemeral P-256 keypair.
func GenerateKeypair() (*Keypair, error) {
	curve := ecdh.P256()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ecdh key: %w", err)
	}
	return &Keypair{
		Public:  priv.PublicKey().Bytes(),
		Private: priv.Bytes(),
	}, nil
}

// DeriveSharedSecret runs ECDH between a peer public key and our own
// private key, then stretches the raw shared secret into a 32-byte
// AES-256 key via HKDF-SHA256 with a fixed empty salt and the
// asobi-specific info string.
func DeriveSharedSecret(peerPublic, ownPrivate []byte) ([]byte, error) {
	curve := ecdh.P256()
	peerKey, err := curve.NewPublicKey(peerPublic)
	if err != nil {
		return nil, fmt.Errorf("parse peer public key: %w", err)
	}
	privKey, err := curve.NewPrivateKey(ownPrivate)
	if err != nil {
		return nil, fmt.Errorf("parse own private key: %w", err)
	}
	shared, err := privKey.ECDH(peerKey)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}
	return hkdfExpand(shared, nil, []byte(HKDFInfo), keyBytes)
}

// Encrypt seals a plaintext with AES-256-GCM. The caller supplies a
// 32-byte key (typically the output of DeriveSharedSecret). A fresh
// 12-byte random nonce is generated per call. Returns (nonce,
// ciphertext, tag) as three separate byte slices so the wire format
// matches the Erlang side's encrypt/2 return shape.
func Encrypt(plaintext, key []byte) (nonce, ciphertext, tag []byte, err error) {
	if len(key) != keyBytes {
		return nil, nil, nil, fmt.Errorf("encrypt: key must be %d bytes, got %d", keyBytes, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("aes new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("gcm new: %w", err)
	}
	nonce = make([]byte, nonceBytes)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, nil, fmt.Errorf("nonce random: %w", err)
	}
	sealed := aead.Seal(nil, nonce, plaintext, nil)
	// Seal returns ciphertext || tag; split them so the wire format
	// matches the Erlang side's {Nonce, Ciphertext, Tag} tuple shape.
	if len(sealed) < tagBytes {
		return nil, nil, nil, errors.New("encrypt: sealed output shorter than tag length")
	}
	ciphertext = sealed[:len(sealed)-tagBytes]
	tag = sealed[len(sealed)-tagBytes:]
	return nonce, ciphertext, tag, nil
}

// Decrypt opens a ciphertext with AES-256-GCM using the given key,
// nonce, and tag. Returns an error if the tag does not verify
// (tamper detection) or if the key length or nonce length is wrong.
func Decrypt(ciphertext, tag, nonce, key []byte) ([]byte, error) {
	if len(key) != keyBytes {
		return nil, fmt.Errorf("decrypt: key must be %d bytes, got %d", keyBytes, len(key))
	}
	if len(nonce) != nonceBytes {
		return nil, fmt.Errorf("decrypt: nonce must be %d bytes, got %d", nonceBytes, len(nonce))
	}
	if len(tag) != tagBytes {
		return nil, fmt.Errorf("decrypt: tag must be %d bytes, got %d", tagBytes, len(tag))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm new: %w", err)
	}
	// GCM expects ciphertext || tag concatenated; reassemble before Open.
	sealed := make([]byte, 0, len(ciphertext)+len(tag))
	sealed = append(sealed, ciphertext...)
	sealed = append(sealed, tag...)
	plaintext, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// hkdfExpand runs HKDF-SHA256 with the given input key material, salt,
// info, and output length. A thin wrapper over x/crypto/hkdf so callers
// don't have to care about reader plumbing.
func hkdfExpand(ikm, salt, info []byte, length int) ([]byte, error) {
	r := hkdf.New(sha256.New, ikm, salt, info)
	out := make([]byte, length)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, fmt.Errorf("hkdf expand: %w", err)
	}
	return out, nil
}
