package update

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.2.0", "v0.1.2", 1},
		{"0.2.0", "v0.2.0", 0},
		{"v1.0.0", "v0.9.9", 1},
		{"v0.2.0", "v0.2.1", -1},
		{"v0.2.10", "v0.2.9", 1},
		{"v0.2.0-rc1", "v0.2.0", 0},
		{"v2.0.0", "v10.0.0", -1},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("v0.3.0", "0.2.0") {
		t.Error("0.3.0 should be newer than 0.2.0")
	}
	if IsNewer("v0.2.0", "0.2.0") {
		t.Error("equal versions are not newer")
	}
	if IsNewer("v0.1.0", "0.2.0") {
		t.Error("older is not newer")
	}
}

func TestLatestReleaseTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"tag_name":"v0.9.0"}`))
	}))
	defer srv.Close()
	old := apiBaseURL
	apiBaseURL = srv.URL
	defer func() { apiBaseURL = old }()

	tag, err := LatestReleaseTag(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v0.9.0" {
		t.Errorf("tag = %q, want v0.9.0", tag)
	}
}

func TestNotify(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASOBI_NO_UPDATE_CHECK", "")
	t.Setenv("CI", "")
	// Fresh cache advertising a newer release; Notify must not hit the network.
	writeCache(cache{CheckedAt: time.Now(), Latest: "v0.9.0"})

	var buf bytes.Buffer
	Notify("0.2.0", "deploy", &buf)
	if !strings.Contains(buf.String(), "v0.9.0") {
		t.Errorf("expected upgrade notice, got %q", buf.String())
	}

	buf.Reset()
	Notify("0.9.0", "deploy", &buf)
	if buf.Len() != 0 {
		t.Errorf("no notice when current == latest, got %q", buf.String())
	}
}

func TestNotifySkips(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeCache(cache{CheckedAt: time.Now(), Latest: "v0.9.0"})

	cases := []struct {
		name    string
		current string
		cmd     string
		env     map[string]string
	}{
		{"opt-out", "0.2.0", "deploy", map[string]string{"ASOBI_NO_UPDATE_CHECK": "1"}},
		{"ci", "0.2.0", "deploy", map[string]string{"CI": "true"}},
		{"dev build", "dev", "deploy", nil},
		{"version cmd", "0.2.0", "version", nil},
		{"upgrade cmd", "0.2.0", "upgrade", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("ASOBI_NO_UPDATE_CHECK", "")
			t.Setenv("CI", "")
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			var buf bytes.Buffer
			Notify(c.current, c.cmd, &buf)
			if buf.Len() != 0 {
				t.Errorf("expected no notice, got %q", buf.String())
			}
		})
	}
}

func TestValidVersion(t *testing.T) {
	for _, ok := range []string{"v0.2.0", "0.2.0", "v1.2.3-rc1", "v0.2.0+build.1"} {
		if !validVersion(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "latest", "v1.2", "1.2.3.4", "v1.2.x", "0.2.0; rm -rf /", "../../etc"} {
		if validVersion(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestLatestReleaseTagRejectsGarbage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"not-a-version"}`))
	}))
	defer srv.Close()
	old := apiBaseURL
	apiBaseURL = srv.URL
	defer func() { apiBaseURL = old }()

	if _, err := LatestReleaseTag(context.Background()); err == nil {
		t.Error("non-semver tag_name must be rejected")
	}
}
