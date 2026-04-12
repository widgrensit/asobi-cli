package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestLoginHappyPath(t *testing.T) {
	// Generate a known server keypair for the mock saas.
	serverKP, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}

	var (
		mu              sync.Mutex
		storedClientPub []byte
		sessionID       = "test-session-12345"
		approved        bool
	)

	// Mock saas that handles start + poll.
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			pubB64 := body["client_public_key"]
			pubRaw, _ := base64.StdEncoding.DecodeString(pubB64)
			mu.Lock()
			storedClientPub = pubRaw
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{
				"session_id": sessionID,
				"expires_in": 600,
			})
			return
		}
	})
	mux.HandleFunc("/internal/cli/login/"+sessionID, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if !approved {
			json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
			return
		}
		// Build encrypted payload using the stored client public key.
		sharedSecret, _ := DeriveSharedSecret(storedClientPub, serverKP.Private)
		payload, _ := json.Marshal(map[string]any{
			"access_token":   "at_test_123",
			"refresh_token":  "rt_test_456",
			"saas_url":       "http://localhost",
			"engine_url":     "http://localhost:8090",
			"tenant_id":      "t-1",
			"game_id":        "g-1",
			"environment_id": "e-1",
			"env_name":       "dev",
			"scopes":         []string{"deploy"},
		})
		nonce, ciphertext, tag, _ := Encrypt(payload, sharedSecret)
		json.NewEncoder(w).Encode(map[string]string{
			"status":            "ok",
			"server_public_key": base64.StdEncoding.EncodeToString(serverKP.Public),
			"nonce":             base64.StdEncoding.EncodeToString(nonce),
			"ciphertext":        base64.StdEncoding.EncodeToString(ciphertext),
			"tag":               base64.StdEncoding.EncodeToString(tag),
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Auto-approve after a short delay (simulates browser approval).
	go func() {
		// Wait for the CLI to have called start.
		for {
			mu.Lock()
			if storedClientPub != nil {
				approved = true
				mu.Unlock()
				return
			}
			mu.Unlock()
		}
	}()

	creds, err := Login(server.URL, "test-host")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if creds.AccessToken != "at_test_123" {
		t.Errorf("access_token = %q, want at_test_123", creds.AccessToken)
	}
	if creds.RefreshToken != "rt_test_456" {
		t.Errorf("refresh_token = %q, want rt_test_456", creds.RefreshToken)
	}
	if creds.EngineURL != "http://localhost:8090" {
		t.Errorf("engine_url = %q, want http://localhost:8090", creds.EngineURL)
	}
	if creds.TenantID != "t-1" {
		t.Errorf("tenant_id = %q, want t-1", creds.TenantID)
	}
	if creds.EnvName != "dev" {
		t.Errorf("env_name = %q, want dev", creds.EnvName)
	}
}

func TestLoginDenied(t *testing.T) {
	sessionID := "deny-session"
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/login", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"session_id": sessionID,
			"expires_in": 600,
		})
	})
	mux.HandleFunc("/internal/cli/login/"+sessionID, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "denied"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	_, err := Login(server.URL, "test")
	if err == nil {
		t.Fatal("expected error for denied login")
	}
	want := "login denied by user"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestMintKeyHappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/mint-key", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-access-token" {
			w.WriteHeader(401)
			fmt.Fprintf(w, `{"error":"invalid_token"}`)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"raw_key":    "ak_test1234_deadbeef",
			"expires_in": 3600,
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	creds := &Credentials{
		AccessToken:       "test-access-token",
		SaasURL:           server.URL,
		DeviceFingerprint: "test-host",
	}

	key, err := MintKey(creds)
	if err != nil {
		t.Fatalf("MintKey: %v", err)
	}
	if key != "ak_test1234_deadbeef" {
		t.Errorf("key = %q, want ak_test1234_deadbeef", key)
	}
}

func TestMintKeyBadToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/mint-key", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprintf(w, `{"error":"invalid_token"}`)
	})
	mux.HandleFunc("/internal/cli/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprintf(w, `{"error":"refresh_expired"}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	creds := &Credentials{
		AccessToken:       "bad-token",
		RefreshToken:      "also-bad",
		SaasURL:           server.URL,
		DeviceFingerprint: "test-host",
	}

	_, err := MintKey(creds)
	if err == nil {
		t.Fatal("expected error for bad token")
	}
}
