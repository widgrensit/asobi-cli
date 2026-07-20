package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A command that exits via fatal() must still surface the upgrade notice:
// os.Exit skips deferred main-level notices, so failing commands route
// through exit() -> notifyUpdate. This drives that wiring off a seeded
// cache so it never touches the network.
func TestNotifyUpdateWiring(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// os.UserHomeDir reads USERPROFILE on Windows, so HOME alone does not
	// redirect the cache dir there; set both to pin it to the temp dir.
	t.Setenv("USERPROFILE", dir)
	t.Setenv("ASOBI_NO_UPDATE_CHECK", "")
	t.Setenv("CI", "")
	asobiDir := filepath.Join(dir, ".asobi")
	if err := os.MkdirAll(asobiDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := `{"checked_at":"` + time.Now().Format(time.RFC3339) + `","latest":"v9.9.9"}`
	if err := os.WriteFile(filepath.Join(asobiDir, "version_check.json"), []byte(cache), 0o600); err != nil {
		t.Fatal(err)
	}

	oldVersion, oldArgs := version, os.Args
	version = "v0.0.1"
	os.Args = []string{"asobi", "health"}
	t.Cleanup(func() { version, os.Args = oldVersion, oldArgs })

	var buf bytes.Buffer
	notifyUpdateTo(&buf)
	if !strings.Contains(buf.String(), "v9.9.9") {
		t.Errorf("failing-command exit path did not surface upgrade notice; got %q", buf.String())
	}
}

func TestIsHealthy(t *testing.T) {
	for _, s := range []string{"ok", "healthy"} {
		if !isHealthy(s) {
			t.Errorf("isHealthy(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "degraded", "unhealthy", "OK"} {
		if isHealthy(s) {
			t.Errorf("isHealthy(%q) = true, want false", s)
		}
	}
}

func TestFirstArg(t *testing.T) {
	// firstArg runs on args after extractFlag has stripped known flags and
	// their values, so it only skips bare "--" tokens and returns the first
	// positional.
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"--verbose"}, ""},
		{[]string{"prod"}, "prod"},
		{[]string{"--verbose", "prod"}, "prod"},
		{[]string{"prod", "--verbose"}, "prod"},
	}
	for _, c := range cases {
		if got := firstArg(c.args); got != c.want {
			t.Errorf("firstArg(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestSelectEnv(t *testing.T) {
	prod := map[string]interface{}{"name": "prod"}
	staging := map[string]interface{}{"name": "staging"}

	if _, err := selectEnv(nil, ""); err == nil {
		t.Error("selectEnv(empty) should error")
	}

	got, err := selectEnv([]map[string]interface{}{prod}, "")
	if err != nil || got["name"] != "prod" {
		t.Errorf("single env: got %v, err %v", got, err)
	}

	got, err = selectEnv([]map[string]interface{}{prod, staging}, "staging")
	if err != nil || got["name"] != "staging" {
		t.Errorf("by name: got %v, err %v", got, err)
	}

	if _, err := selectEnv([]map[string]interface{}{prod, staging}, "nope"); err == nil {
		t.Error("unknown name should error")
	}

	if _, err := selectEnv([]map[string]interface{}{prod, staging}, ""); err == nil {
		t.Error("multiple envs without a name should be ambiguous")
	}
}
