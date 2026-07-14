package update

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	old := signingPublicKey
	signingPublicKey = hex.EncodeToString(pub)
	defer func() { signingPublicKey = old }()

	checksums := []byte("abc123  asobi_linux_amd64.tar.gz\n")
	sig := ed25519.Sign(priv, checksums)

	if err := verifySignature(checksums, sig); err != nil {
		t.Errorf("valid signature rejected: %v", err)
	}
	if err := verifySignature([]byte("tampered checksums"), sig); err == nil {
		t.Error("tampered checksums must fail signature")
	}
	if err := verifySignature(checksums, sig[:len(sig)-1]); err == nil {
		t.Error("wrong-size signature must fail")
	}
	_, otherPriv, _ := ed25519.GenerateKey(nil)
	if err := verifySignature(checksums, ed25519.Sign(otherPriv, checksums)); err == nil {
		t.Error("signature from a different key must fail")
	}
}
