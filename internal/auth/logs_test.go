package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The API takes seconds, so a reader who passes the number from the docs must
// get what they expect rather than a parse error.
func TestParseSinceAcceptsDurationsAndBareSeconds(t *testing.T) {
	cases := map[string]time.Duration{
		"":     0,
		"90s":  90 * time.Second,
		"30m":  30 * time.Minute,
		"2h":   2 * time.Hour,
		"3600": time.Hour,
	}
	for in, want := range cases {
		got, err := ParseSince(in)
		if err != nil {
			t.Fatalf("ParseSince(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseSince(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseSinceRejectsNonsense(t *testing.T) {
	for _, in := range []string{"0", "-5", "-1h", "yesterday", "1 hour"} {
		if _, err := ParseSince(in); err == nil {
			t.Fatalf("ParseSince(%q) should have failed", in)
		}
	}
}

func TestEnvLogsSendsTheNarrowingOptions(t *testing.T) {
	var got string
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod/logs", func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]any{
			"lines": []map[string]string{{"ts": "2026-08-17T10:00:00Z", "line": "boot"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	lines, err := EnvLogs(&Credentials{AccessToken: "at-1", SaasURL: srv.URL}, "arena", "prod",
		LogOptions{Filter: "error", Since: 30 * time.Minute, Limit: 50})
	if err != nil {
		t.Fatalf("EnvLogs: %v", err)
	}
	if len(lines) != 1 || lines[0].Line != "boot" {
		t.Fatalf("lines = %+v", lines)
	}
	for _, want := range []string{"game=arena", "filter=error", "since=1800", "limit=50"} {
		if !strings.Contains(got, want) {
			t.Fatalf("query %q missing %q", got, want)
		}
	}
}

// Zero values must be omitted rather than sent as 0, or they would shadow the
// server's own defaults with something it would clamp back up.
func TestEnvLogsOmitsUnsetOptions(t *testing.T) {
	var got string
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod/logs", func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		w.Write([]byte(`{"lines":[]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	if _, err := EnvLogs(&Credentials{AccessToken: "at-1", SaasURL: srv.URL}, "", "prod", LogOptions{}); err != nil {
		t.Fatalf("EnvLogs: %v", err)
	}
	for _, unwanted := range []string{"since=", "limit=", "filter=", "game="} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("query %q should not carry %q", got, unwanted)
		}
	}
}

// A bad filter is the caller's to fix; an unavailable backend is not. Reporting
// both as a generic failure makes the first one unfixable.
func TestEnvLogsDistinguishesBadFilterFromOutage(t *testing.T) {
	for code, want := range map[int]string{422: "filter", 503: "temporarily unavailable"} {
		mux := http.NewServeMux()
		status := code
		mux.HandleFunc("/internal/cli/envs/prod/logs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		})
		srv := httptest.NewServer(mux)
		_, err := EnvLogs(&Credentials{AccessToken: "at-1", SaasURL: srv.URL}, "", "prod", LogOptions{})
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("status %d gave %v, want it to mention %q", code, err, want)
		}
	}
}
