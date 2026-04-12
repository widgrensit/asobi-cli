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
