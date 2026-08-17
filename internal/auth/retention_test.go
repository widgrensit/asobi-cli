package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetRetentionSendsThePeriod(t *testing.T) {
	var got map[string]string
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod/retention", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at-1" {
			w.WriteHeader(401)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &got)
		w.WriteHeader(200)
		w.Write([]byte(`{"retention":"Deleting unclaimed guests after 30 days of inactivity"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds := &Credentials{AccessToken: "at-1", SaasURL: srv.URL}
	if err := SetRetention(creds, "", "prod", "30"); err != nil {
		t.Fatalf("SetRetention: %v", err)
	}
	if got["after"] != "30" {
		t.Fatalf(`after = %q, want "30"`, got["after"])
	}
}

// "never" has to reach the server as a value rather than as an omitted field,
// or turning retention back off would read as a request that named no period
// and be refused.
func TestSetRetentionSendsNeverExplicitly(t *testing.T) {
	var got map[string]string
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod/retention", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &got)
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds := &Credentials{AccessToken: "at-1", SaasURL: srv.URL}
	if err := SetRetention(creds, "", "prod", "never"); err != nil {
		t.Fatalf("SetRetention: %v", err)
	}
	if _, ok := got["after"]; !ok {
		t.Fatal("after was omitted; it must be sent")
	}
	if got["after"] != "never" {
		t.Fatalf(`after = %q, want "never"`, got["after"])
	}
}

// A role refusal is not an expired session. Reporting it as one sends somebody
// off to re-run `asobi login` to fix a permission they do not have.
func TestSetRetentionReportsRoleRefusalPlainly(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod/retention", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error":"requires_owner_or_admin"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds := &Credentials{AccessToken: "at-1", SaasURL: srv.URL}
	err := SetRetention(creds, "", "prod", "30")
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "owner or admin") {
		t.Fatalf("error = %q, want it to name the role requirement", err)
	}
}

func TestSetRetentionPassesTheGame(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod/retention", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds := &Credentials{AccessToken: "at-1", SaasURL: srv.URL}
	if err := SetRetention(creds, "arena", "prod", "90"); err != nil {
		t.Fatalf("SetRetention: %v", err)
	}
	if !strings.Contains(gotQuery, "arena") {
		t.Fatalf("query = %q, want it to carry the game slug", gotQuery)
	}
}

// The list the CLI checks against locally must be the one the server accepts.
// `1` is the value that matters: a one-day retention would erase a working
// game's whole guest population overnight.
func TestRetentionPeriodsAreTheOfferedSet(t *testing.T) {
	want := []string{"never", "7", "30", "90", "365"}
	if len(RetentionPeriods) != len(want) {
		t.Fatalf("RetentionPeriods = %v, want %v", RetentionPeriods, want)
	}
	for i, v := range want {
		if RetentionPeriods[i] != v {
			t.Fatalf("RetentionPeriods[%d] = %q, want %q", i, RetentionPeriods[i], v)
		}
	}
	for _, bad := range []string{"1", "0", "-30", "31", "forever", ""} {
		for _, ok := range RetentionPeriods {
			if bad == ok {
				t.Fatalf("%q must not be an offered period", bad)
			}
		}
	}
}

// Destroy refusals have specific causes, and reporting them generically sends
// somebody to re-run `asobi login` for a permission they do not have, or to
// retry a call that will keep refusing.
func TestDeleteEnvReportsRoleRefusal(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error":"requires_owner_or_admin"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := DeleteEnv(&Credentials{AccessToken: "at-1", SaasURL: srv.URL}, "", "prod")
	if err == nil || !strings.Contains(err.Error(), "owner or admin") {
		t.Fatalf("error = %v, want it to name the role requirement", err)
	}
}

func TestDeleteEnvReportsProtection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/cli/envs/prod", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		w.Write([]byte(`{"error":"environment_protected"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	err := DeleteEnv(&Credentials{AccessToken: "at-1", SaasURL: srv.URL}, "", "prod")
	if err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("error = %v, want it to name the protection flag", err)
	}
}
