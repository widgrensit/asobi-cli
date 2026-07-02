package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListGames(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/games", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at-1" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"games": []map[string]string{
				{"id": "g-1", "slug": "arena", "name": "Arena"},
				{"id": "g-2", "slug": "ctf", "name": "Capture the Flag"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds := &Credentials{AccessToken: "at-1", SaasURL: srv.URL}
	games, err := ListGames(creds)
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("len(games) = %d, want 2", len(games))
	}
	if games[0].Slug != "arena" || games[1].Slug != "ctf" {
		t.Fatalf("slugs = %q,%q, want arena,ctf", games[0].Slug, games[1].Slug)
	}
	if games[1].Name != "Capture the Flag" {
		t.Fatalf("name = %q, want Capture the Flag", games[1].Name)
	}
}

func TestListGamesNoTenant(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/games", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error":"no_tenant"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds := &Credentials{AccessToken: "at-1", SaasURL: srv.URL}
	if _, err := ListGames(creds); err == nil {
		t.Fatal("expected error for no_tenant")
	}
}

func TestCreateEnvSendsGame(t *testing.T) {
	var body map[string]string
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"environment": map[string]any{"id": "e-1"}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds := &Credentials{AccessToken: "at-1", SaasURL: srv.URL}
	if _, err := CreateEnv(creds, "arena", "prod", "xs"); err != nil {
		t.Fatalf("CreateEnv: %v", err)
	}
	if body["game"] != "arena" {
		t.Fatalf("game = %q, want arena", body["game"])
	}
	if body["name"] != "prod" || body["size"] != "xs" {
		t.Fatalf("name/size = %q/%q, want prod/xs", body["name"], body["size"])
	}
}

func TestDeployBundlePassesGameQuery(t *testing.T) {
	var gotGame string
	var gotBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod/deploy", func(w http.ResponseWriter, r *http.Request) {
		gotGame = r.URL.Query().Get("game")
		gotBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{"generation": 1, "sha256": "abcdef012345"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds := &Credentials{AccessToken: "at-1", SaasURL: srv.URL}
	if _, err := DeployBundle(creds, "arena", "prod", []byte("zip-bytes")); err != nil {
		t.Fatalf("DeployBundle: %v", err)
	}
	if gotGame != "arena" {
		t.Fatalf("game query = %q, want arena", gotGame)
	}
	if string(gotBody) != "zip-bytes" {
		t.Fatalf("body = %q, want zip-bytes", gotBody)
	}
}

func TestEnvActionPassesGameQuery(t *testing.T) {
	var gotGame string
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod/stop", func(w http.ResponseWriter, r *http.Request) {
		gotGame = r.URL.Query().Get("game")
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds := &Credentials{AccessToken: "at-1", SaasURL: srv.URL}
	if err := EnvAction(creds, "arena", "prod", "stop"); err != nil {
		t.Fatalf("EnvAction: %v", err)
	}
	if gotGame != "arena" {
		t.Fatalf("game query = %q, want arena", gotGame)
	}
}
