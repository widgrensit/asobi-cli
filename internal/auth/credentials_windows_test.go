//go:build windows

package auth

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// The token must never sit in the clear on disk on Windows.
func TestWindowsEncryptsCredentialsAtRest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOME", dir)

	secret := "supersecret-access-token"
	if err := SaveCredentials(&Credentials{AccessToken: secret}); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	raw, err := os.ReadFile(mustCredentialsPath(t))
	if err != nil {
		t.Fatalf("read raw file: %v", err)
	}
	if bytes.Contains(raw, []byte(secret)) {
		t.Fatal("access token found in plaintext on disk; DPAPI encryption not applied")
	}
}

// A pre-#27 plaintext credentials.json must still load, then re-encrypt on the
// next save (seamless migration).
func TestWindowsLoadsLegacyPlaintextThenReEncrypts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOME", dir)

	secret := "legacy-plaintext-token"
	plain, err := json.MarshalIndent(&Credentials{AccessToken: secret}, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(dir+"/.asobi", 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(mustCredentialsPath(t), plain, 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	got, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials on legacy file: %v", err)
	}
	if got == nil || got.AccessToken != secret {
		t.Fatalf("legacy plaintext not loaded: %+v", got)
	}

	if err := SaveCredentials(got); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	raw, err := os.ReadFile(mustCredentialsPath(t))
	if err != nil {
		t.Fatalf("read re-saved: %v", err)
	}
	if bytes.Contains(raw, []byte(secret)) {
		t.Fatal("token still plaintext after re-save; migration did not encrypt")
	}
}
