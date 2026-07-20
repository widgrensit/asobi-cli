package auth

import "testing"

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
