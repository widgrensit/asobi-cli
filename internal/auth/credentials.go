package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials holds the CLI's saas session tokens and associated context.
// Stored in ~/.asobi/credentials.json with 0600 permissions. On Windows,
// where the file mode does not create an ACL, the blob is additionally
// encrypted at rest with DPAPI scoped to the current user (see protect).
type Credentials struct {
	AccessToken       string `json:"access_token"`
	RefreshToken      string `json:"refresh_token"`
	SaasURL           string `json:"saas_url"`
	EngineURL         string `json:"engine_url"`
	TenantID          string `json:"tenant_id"`
	GameID            string `json:"game_id"`
	EnvironmentID     string `json:"environment_id"`
	EnvName           string `json:"env_name"`
	DeviceFingerprint string `json:"device_fingerprint"`
	// DeviceSecret is the server-issued secret that binds the refresh token
	// to this CLI install. Presented on refresh; never sent anywhere else.
	DeviceSecret string `json:"device_secret"`
	// ActiveGame is the slug of the game selected via `asobi use <slug>`.
	// The effective game for env operations resolves from --game, then this.
	ActiveGame string `json:"active_game"`
}

// CredentialsPath returns the absolute path of the credentials file.
// It fails rather than falling back to a relative path when the home
// directory cannot be resolved, so credentials are never written to
// whatever directory the CLI happened to be invoked from.
func CredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".asobi", "credentials.json"), nil
}

// LoadCredentials reads the stored CLI credentials. Returns nil if no
// credentials exist (not an error — user hasn't logged in yet).
// The ASOBI_ACCESS_TOKEN env var overrides the stored access token.
func LoadCredentials() (*Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, err
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	data, err := unprotect(blob)
	if err != nil {
		return nil, fmt.Errorf("unprotect credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if envToken := os.Getenv("ASOBI_ACCESS_TOKEN"); envToken != "" {
		creds.AccessToken = envToken
	}
	return &creds, nil
}

// SaveCredentials writes credentials to disk with 0600 permissions.
// The write goes to a temp file in the same directory and is renamed into
// place, so a failed write can never leave half a token behind or silently
// keep the previous credentials.
func SaveCredentials(creds *Credentials) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	blob, err := protect(data)
	if err != nil {
		return fmt.Errorf("protect credentials: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".credentials-*")
	if err != nil {
		return fmt.Errorf("create temp credentials: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(blob); err != nil {
		tmp.Close()
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

// DeleteCredentials removes the stored credentials file, including the
// legacy token file.
func DeleteCredentials() error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove credentials: %w", err)
	}
	return RemoveLegacyCredentials()
}

// RemoveLegacyCredentials deletes ~/.asobi/auth, the token file written by
// CLI versions before credentials.json. Left behind it is a live copy of
// expired tokens that an older binary still on PATH will keep using.
func RemoveLegacyCredentials() error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	legacy := filepath.Join(filepath.Dir(path), "auth")
	if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy credentials: %w", err)
	}
	return nil
}

// DeviceFingerprint returns a stable identifier for this machine.
func DeviceFingerprint() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
