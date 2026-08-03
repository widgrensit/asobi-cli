package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func mustCredentialsPath(t *testing.T) string {
	t.Helper()
	path, err := CredentialsPath()
	if err != nil {
		t.Fatalf("CredentialsPath: %v", err)
	}
	return path
}

// Runs on every platform. On Windows it exercises the DPAPI protect/unprotect
// round trip; on Unix the no-op layer. Either way, save-then-load must recover
// the credentials intact.
func TestCredentialsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	want := &Credentials{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-def",
		DeviceSecret: "device-ghi",
		SaasURL:      "https://console.asobi.dev",
		ActiveGame:   "space-corsairs",
	}
	if err := SaveCredentials(want); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	got, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got == nil {
		t.Fatal("LoadCredentials returned nil after save")
	}
	if got.AccessToken != want.AccessToken ||
		got.RefreshToken != want.RefreshToken ||
		got.DeviceSecret != want.DeviceSecret ||
		got.ActiveGame != want.ActiveGame {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", got, want)
	}
}

// #46: a second login must replace the stored tokens, never leave the
// previous ones in place.
func TestSaveCredentialsReplacesStaleTokens(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	if err := SaveCredentials(&Credentials{RefreshToken: "stale", AccessToken: "old"}); err != nil {
		t.Fatalf("first SaveCredentials: %v", err)
	}
	if err := SaveCredentials(&Credentials{RefreshToken: "fresh", AccessToken: "new"}); err != nil {
		t.Fatalf("second SaveCredentials: %v", err)
	}

	got, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got.RefreshToken != "fresh" || got.AccessToken != "new" {
		t.Fatalf("stale tokens survived re-login: %+v", got)
	}
}

// #46: with no resolvable home directory the CLI must fail loudly instead of
// writing credentials to a relative path under the current directory.
func TestCredentialsRequireAHomeDir(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Chdir(t.TempDir())

	if _, err := CredentialsPath(); err == nil {
		t.Fatal("expected CredentialsPath to fail without a home dir")
	}
	if err := SaveCredentials(&Credentials{AccessToken: "x"}); err == nil {
		t.Fatal("expected SaveCredentials to fail without a home dir")
	}
	if _, err := LoadCredentials(); err == nil {
		t.Fatal("expected LoadCredentials to fail without a home dir")
	}
	if _, err := os.Stat(".asobi"); !os.IsNotExist(err) {
		t.Fatal("credentials written to a relative .asobi dir")
	}
}

func TestLoginRemovesLegacyAuthFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	if err := SaveCredentials(&Credentials{AccessToken: "fresh"}); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	legacy := filepath.Join(dir, ".asobi", "auth")
	if err := os.WriteFile(legacy, []byte("old-token"), 0o600); err != nil {
		t.Fatalf("write legacy auth: %v", err)
	}

	if err := RemoveLegacyCredentials(); err != nil {
		t.Fatalf("RemoveLegacyCredentials: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy auth file still present")
	}
	if err := RemoveLegacyCredentials(); err != nil {
		t.Fatalf("RemoveLegacyCredentials on missing file: %v", err)
	}

	got, err := LoadCredentials()
	if err != nil || got == nil || got.AccessToken != "fresh" {
		t.Fatalf("credentials.json disturbed by legacy cleanup: %+v (%v)", got, err)
	}
}

func TestLoadCredentialsMissingFileIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	got, err := LoadCredentials()
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil credentials for missing file, got %+v", got)
	}
}
