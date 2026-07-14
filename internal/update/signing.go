package update

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

// signingPublicKey is the raw 32-byte ed25519 public key whose private half
// signs checksums.txt for every release (held as the ASOBI_SIGNING_KEY CI
// secret, never in the repo). Because the asset is verified against
// checksums.txt and checksums.txt is verified against this key, a GitHub or CDN
// compromise alone cannot forge an upgrade. Rotating the key requires shipping a
// release whose checksums are signed by the new key. Var, not const, so tests
// substitute an ephemeral key.
var signingPublicKey = "d96d1bd8642b31937478086325c9b23c42dacddce866e55654e1e2ebbdf1713a"

// verifySignature fails closed unless sig is a valid ed25519 signature of
// checksums by the release signing key.
func verifySignature(checksums, sig []byte) error {
	pub, err := hex.DecodeString(signingPublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid embedded signing key")
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("checksums signature has wrong size (%d bytes)", len(sig))
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), checksums, sig) {
		return fmt.Errorf("checksums signature does not verify against the release signing key")
	}
	return nil
}
