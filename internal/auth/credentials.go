package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials holds the CLI's saas session tokens and associated context.
// Stored in ~/.asobi/credentials.json with 0600 permissions.
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
}

func credentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".asobi", "credentials.json")
}

// LoadCredentials reads the stored CLI credentials. Returns nil if no
// credentials exist (not an error — user hasn't logged in yet).
// The ASOBI_ACCESS_TOKEN env var overrides the stored access token.
func LoadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read credentials: %w", err)
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
func SaveCredentials(creds *Credentials) error {
	dir := filepath.Dir(credentialsPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	return os.WriteFile(credentialsPath(), data, 0o600)
}

// DeleteCredentials removes the stored credentials file.
func DeleteCredentials() error {
	err := os.Remove(credentialsPath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove credentials: %w", err)
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
