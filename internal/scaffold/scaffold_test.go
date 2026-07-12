package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesWorkingGame(t *testing.T) {
	dir := t.TempDir()
	created, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	want := map[string]bool{
		filepath.Join("lua", "match.lua"): false,
		"README.md":                       false,
	}
	for _, f := range created {
		if _, ok := want[f]; !ok {
			t.Fatalf("unexpected created file %q", f)
		}
		want[f] = true
	}
	for f, seen := range want {
		if !seen {
			t.Fatalf("expected %q to be created", f)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "lua", "match.lua"))
	if err != nil {
		t.Fatalf("read match.lua: %v", err)
	}
	lua := string(data)
	for _, cb := range []string{"match_size", "function init(", "function join(", "function leave(", "function handle_input(", "function tick(", "function get_state("} {
		if !strings.Contains(lua, cb) {
			t.Fatalf("match.lua missing %q", cb)
		}
	}

	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	for _, step := range []string{"asobi login", "asobi use", "asobi create", "asobi deploy"} {
		if !strings.Contains(string(readme), step) {
			t.Fatalf("README.md missing deploy step %q", step)
		}
	}
	for _, hint := range []string{"asobi dev", "config.lua", "--template backend"} {
		if !strings.Contains(string(readme), hint) {
			t.Fatalf("README.md missing %q", hint)
		}
	}
}

func TestInitRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lua"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dir, "lua", "match.lua")
	if err := os.WriteFile(existing, []byte("-- mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Init(dir); err == nil {
		t.Fatal("expected error when match.lua already exists")
	}
	data, _ := os.ReadFile(existing)
	if string(data) != "-- mine\n" {
		t.Fatalf("existing match.lua was overwritten: %q", data)
	}
}
