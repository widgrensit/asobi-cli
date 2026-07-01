package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The refresh token is bound to the server-issued device secret, so refresh
// must present device_secret (not the old spoofable hostname fingerprint).
func TestRefreshAccessTokenSendsDeviceSecret(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "new-access"})
	}))
	defer srv.Close()

	creds := &Credentials{
		SaasURL:           srv.URL,
		RefreshToken:      "rt-123",
		DeviceSecret:      "server-issued-secret",
		DeviceFingerprint: "some-hostname",
	}

	tok, err := RefreshAccessToken(creds)
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if tok != "new-access" {
		t.Fatalf("token = %q, want new-access", tok)
	}
	if got["device_secret"] != "server-issued-secret" {
		t.Fatalf("device_secret = %q, want server-issued-secret", got["device_secret"])
	}
	if got["refresh_token"] != "rt-123" {
		t.Fatalf("refresh_token = %q, want rt-123", got["refresh_token"])
	}
	if _, ok := got["device_fingerprint"]; ok {
		t.Fatalf("device_fingerprint should no longer be sent, got %q", got["device_fingerprint"])
	}
}
