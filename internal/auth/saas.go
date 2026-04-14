package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type mintKeyResponse struct {
	RawKey    string `json:"raw_key"`
	ExpiresIn int    `json:"expires_in"`
	Error     string `json:"error,omitempty"`
}

// EphemeralDeploy creates a fresh ephemeral environment + API key with 1h TTL.
type EphemeralDeployResponse struct {
	EnvID     string `json:"env_id"`
	RawKey    string `json:"raw_key"`
	ExpiresIn int    `json:"expires_in"`
	Error     string `json:"error,omitempty"`
}

// Environment is a single env returned by ListEnvs.
type Environment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	IsEphemeral bool   `json:"is_ephemeral"`
	ExpiresAt   string `json:"expires_at"`
	InsertedAt  string `json:"inserted_at"`
}

type listEnvsResponse struct {
	Environments []Environment `json:"environments"`
	Error        string        `json:"error,omitempty"`
}

type destroyResponse struct {
	Status string `json:"status"`
	EnvID  string `json:"env_id"`
	Error  string `json:"error,omitempty"`
}

type refreshResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	Error       string `json:"error,omitempty"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// MintKey calls the saas to create a short-lived engine API key.
// Uses the CLI's access token as a bearer credential.
func MintKey(creds *Credentials) (string, error) {
	url := creds.SaasURL + "/internal/cli/mint-key"
	req, err := http.NewRequest("POST", url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST /internal/cli/mint-key: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		refreshed, refreshErr := RefreshAccessToken(creds)
		if refreshErr != nil {
			return "", fmt.Errorf("access token expired and refresh failed: %w (original: %s)", refreshErr, data)
		}
		creds.AccessToken = refreshed
		if saveErr := SaveCredentials(creds); saveErr != nil {
			return "", fmt.Errorf("save refreshed credentials: %w", saveErr)
		}
		return MintKey(creds)
	}

	var result mintKeyResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse mint-key response: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("mint-key failed (%d): %s", resp.StatusCode, result.Error)
	}
	return result.RawKey, nil
}

// RefreshAccessToken exchanges a refresh token for a new access token.
// The device fingerprint must match what was stored at login time.
func RefreshAccessToken(creds *Credentials) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"refresh_token":      creds.RefreshToken,
		"device_fingerprint": creds.DeviceFingerprint,
	})
	url := creds.SaasURL + "/internal/cli/refresh"
	resp, err := httpClient.Do(mustNewRequest("POST", url, body))
	if err != nil {
		return "", fmt.Errorf("POST /internal/cli/refresh: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var result refreshResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse refresh response: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, result.Error)
	}
	return result.AccessToken, nil
}

func mustNewRequest(method, url string, body []byte) *http.Request {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		panic(fmt.Sprintf("bad request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

// EphemeralDeploy creates a fresh ephemeral env + API key with 1h TTL.
func EphemeralDeploy(creds *Credentials, name string) (*EphemeralDeployResponse, error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	req, err := http.NewRequest("POST", creds.SaasURL+"/internal/cli/ephemeral-deploy", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST /internal/cli/ephemeral-deploy: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		refreshed, refreshErr := RefreshAccessToken(creds)
		if refreshErr != nil {
			return nil, fmt.Errorf("access token expired and refresh failed: %w", refreshErr)
		}
		creds.AccessToken = refreshed
		_ = SaveCredentials(creds)
		return EphemeralDeploy(creds, name)
	}

	var result EphemeralDeployResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ephemeral-deploy failed (%d): %s", resp.StatusCode, result.Error)
	}
	return &result, nil
}

// Destroy deletes an environment by ID. Idempotent.
func Destroy(creds *Credentials, envID string) error {
	url := creds.SaasURL + "/internal/cli/environments/" + envID
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE /internal/cli/environments/%s: %w", envID, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		refreshed, refreshErr := RefreshAccessToken(creds)
		if refreshErr != nil {
			return fmt.Errorf("access token expired and refresh failed: %w", refreshErr)
		}
		creds.AccessToken = refreshed
		_ = SaveCredentials(creds)
		return Destroy(creds, envID)
	}

	var result destroyResponse
	if err := json.Unmarshal(data, &result); err == nil && result.Error != "" {
		return fmt.Errorf("destroy failed (%d): %s", resp.StatusCode, result.Error)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("destroy failed (%d): %s", resp.StatusCode, data)
	}
	return nil
}

// ListEnvs returns all environments for the current game.
// If ephemeralOnly is true, only ephemeral envs are returned.
func ListEnvs(creds *Credentials, ephemeralOnly bool) ([]Environment, error) {
	url := creds.SaasURL + "/internal/cli/environments"
	if ephemeralOnly {
		url += "?ephemeral=true"
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET /internal/cli/environments: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		refreshed, refreshErr := RefreshAccessToken(creds)
		if refreshErr != nil {
			return nil, fmt.Errorf("access token expired and refresh failed: %w", refreshErr)
		}
		creds.AccessToken = refreshed
		_ = SaveCredentials(creds)
		return ListEnvs(creds, ephemeralOnly)
	}

	var result listEnvsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list envs failed (%d): %s", resp.StatusCode, result.Error)
	}
	return result.Environments, nil
}
