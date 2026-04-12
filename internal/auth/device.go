package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

type startResponse struct {
	SessionID string `json:"session_id"`
	ExpiresIn int    `json:"expires_in"`
}

type pollResponse struct {
	Status          string `json:"status"`
	ServerPublicKey string `json:"server_public_key,omitempty"`
	Nonce           string `json:"nonce,omitempty"`
	Ciphertext      string `json:"ciphertext,omitempty"`
	Tag             string `json:"tag,omitempty"`
}

const (
	pollInterval    = 2 * time.Second
	pollMaxInterval = 10 * time.Second
	pollTimeout     = 10 * time.Minute
)

// Login runs the full ECDH-encrypted device-code login flow.
// It returns credentials on success or an error.
func Login(saasURL, tokenName string) (*Credentials, error) {
	kp, err := GenerateKeypair()
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}

	pubB64 := base64.StdEncoding.EncodeToString(kp.Public)
	startResp, err := startSession(saasURL, tokenName, pubB64)
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}

	approvalURL := fmt.Sprintf("%s/dashboard/cli/login?session=%s", saasURL, startResp.SessionID)
	fmt.Printf("\nOpen this URL to approve the CLI:\n\n  %s\n\n", approvalURL)
	openBrowser(approvalURL)
	fmt.Printf("Waiting for approval (expires in %ds)...\n", startResp.ExpiresIn)

	payload, err := pollForApproval(saasURL, startResp.SessionID, kp.Private)
	if err != nil {
		return nil, err
	}

	creds := &Credentials{
		AccessToken:       payload.AccessToken,
		RefreshToken:      payload.RefreshToken,
		SaasURL:           coalesce(payload.SaasURL, saasURL),
		EngineURL:         payload.EngineURL,
		TenantID:          payload.TenantID,
		GameID:            payload.GameID,
		EnvironmentID:     payload.EnvironmentID,
		EnvName:           payload.EnvName,
		DeviceFingerprint: DeviceFingerprint(),
	}
	return creds, nil
}

type decryptedPayload struct {
	AccessToken   string   `json:"access_token"`
	RefreshToken  string   `json:"refresh_token"`
	SaasURL       string   `json:"saas_url"`
	EngineURL     string   `json:"engine_url"`
	TenantID      string   `json:"tenant_id"`
	GameID        string   `json:"game_id"`
	EnvironmentID string   `json:"environment_id"`
	EnvName       string   `json:"env_name"`
	Scopes        []string `json:"scopes"`
}

func startSession(saasURL, tokenName, pubB64 string) (*startResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"token_name":        tokenName,
		"client_public_key": pubB64,
	})
	resp, err := http.Post(saasURL+"/internal/cli/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("POST /internal/cli/login: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("start session failed (%d): %s", resp.StatusCode, data)
	}
	var sr startResponse
	if err := json.Unmarshal(data, &sr); err != nil {
		return nil, fmt.Errorf("parse start response: %w", err)
	}
	return &sr, nil
}

func pollForApproval(saasURL, sessionID string, clientPriv []byte) (*decryptedPayload, error) {
	interval := pollInterval
	deadline := time.Now().Add(pollTimeout)
	url := saasURL + "/internal/cli/login/" + sessionID

	for time.Now().Before(deadline) {
		time.Sleep(interval)
		if interval < pollMaxInterval {
			interval = interval + time.Second
		}

		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("  poll error: %v (retrying)\n", err)
			continue
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var pr pollResponse
		if err := json.Unmarshal(data, &pr); err != nil {
			continue
		}

		switch pr.Status {
		case "pending":
			fmt.Print(".")
			continue
		case "denied":
			return nil, fmt.Errorf("login denied by user")
		case "expired":
			return nil, fmt.Errorf("session expired — run asobi login again")
		case "ok":
			fmt.Println(" approved!")
			return decryptApproval(pr, clientPriv)
		default:
			continue
		}
	}
	return nil, fmt.Errorf("login timed out after %s", pollTimeout)
}

func decryptApproval(pr pollResponse, clientPriv []byte) (*decryptedPayload, error) {
	serverPub, err := base64.StdEncoding.DecodeString(pr.ServerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode server public key: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(pr.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(pr.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	tag, err := base64.StdEncoding.DecodeString(pr.Tag)
	if err != nil {
		return nil, fmt.Errorf("decode tag: %w", err)
	}

	sharedSecret, err := DeriveSharedSecret(serverPub, clientPriv)
	if err != nil {
		return nil, fmt.Errorf("derive shared secret: %w", err)
	}

	plaintext, err := Decrypt(ciphertext, tag, nonce, sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt approval payload: %w", err)
	}

	var payload decryptedPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, fmt.Errorf("parse decrypted payload: %w", err)
	}
	return &payload, nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}
	_ = cmd.Start()
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
