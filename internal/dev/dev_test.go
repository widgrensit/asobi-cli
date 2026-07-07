package dev

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveLuaDir(t *testing.T) {
	t.Run("prefers lua over game", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, root, "lua")
		mustMkdir(t, root, "game")
		if got, err := resolveLuaDir(root, ""); err != nil || got != "lua" {
			t.Fatalf("got %q, %v; want lua", got, err)
		}
	})
	t.Run("falls back to game", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, root, "game")
		if got, err := resolveLuaDir(root, ""); err != nil || got != "game" {
			t.Fatalf("got %q, %v; want game", got, err)
		}
	})
	t.Run("honours override", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, root, "server")
		mustMkdir(t, root, "lua")
		if got, err := resolveLuaDir(root, "server"); err != nil || got != "server" {
			t.Fatalf("got %q, %v; want server", got, err)
		}
	})
	t.Run("errors when none", func(t *testing.T) {
		if _, err := resolveLuaDir(t.TempDir(), ""); err == nil {
			t.Fatal("expected error when no lua/ or game/")
		}
	})
}

func TestRenderCompose(t *testing.T) {
	out := renderCompose("/home/dev/mygame/lua", 8084)
	for _, want := range []string{
		`"8084:8084"`,
		`ghcr.io/widgrensit/asobi_lua:latest`,
		`"/home/dev/mygame/lua:/app/game:ro"`,
		`ASOBI_PORT: "8084"`,
		`condition: service_healthy`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("compose missing %q\n---\n%s", want, out)
		}
	}
}

func mustMkdir(t *testing.T, root, name string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
		t.Fatal(err)
	}
}
