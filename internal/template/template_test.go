package template

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type entry struct {
	name string
	typ  byte
	body string
	link string
}

func makeTarGz(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o644}
		switch e.typ {
		case tar.TypeDir:
			hdr.Mode = 0o755
		case tar.TypeReg:
			hdr.Size = int64(len(e.body))
		case tar.TypeSymlink:
			hdr.Linkname = e.link
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", e.name, err)
		}
		if e.typ == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("write body %s: %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestEnginesKnown(t *testing.T) {
	got := Engines()
	want := []string{"defold", "godot", "unity"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Engines() = %v, want %v", got, want)
	}
	for _, e := range want {
		if _, ok := Get(e); !ok {
			t.Errorf("Get(%q) not found", e)
		}
	}
	if _, ok := Get("bogus"); ok {
		t.Error("Get(bogus) should not be found")
	}
}

func TestExtractStripsPrefixAndWrites(t *testing.T) {
	data := makeTarGz(t, []entry{
		{name: "asobi-defold-demo-abc/", typ: tar.TypeDir},
		{name: "asobi-defold-demo-abc/README.md", typ: tar.TypeReg, body: "hi"},
		{name: "asobi-defold-demo-abc/lua/", typ: tar.TypeDir},
		{name: "asobi-defold-demo-abc/lua/match.lua", typ: tar.TypeReg, body: "return 1"},
	})
	dir := t.TempDir()
	top, err := extractTarGz(bytes.NewReader(data), dir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if want := []string{"README.md", "lua"}; !reflect.DeepEqual(top, want) {
		t.Errorf("top entries = %v, want %v", top, want)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "README.md")); err != nil || string(b) != "hi" {
		t.Errorf("README.md = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "lua", "match.lua")); err != nil {
		t.Errorf("lua/match.lua missing: %v", err)
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	data := makeTarGz(t, []entry{
		{name: "repo-abc/", typ: tar.TypeDir},
		{name: "repo-abc/../evil.txt", typ: tar.TypeReg, body: "pwned"},
	})
	parent := t.TempDir()
	dir := filepath.Join(parent, "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := extractTarGz(bytes.NewReader(data), dir); err == nil {
		t.Fatal("expected traversal to be rejected")
	} else if !strings.Contains(err.Error(), "unsafe path") {
		t.Errorf("error = %v, want unsafe path", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "evil.txt")); !os.IsNotExist(err) {
		t.Error("traversal wrote a file outside the target dir")
	}
}

func TestExtractSkipsSymlinks(t *testing.T) {
	data := makeTarGz(t, []entry{
		{name: "repo-abc/", typ: tar.TypeDir},
		{name: "repo-abc/link", typ: tar.TypeSymlink, link: "/etc/passwd"},
		{name: "repo-abc/real.txt", typ: tar.TypeReg, body: "ok"},
	})
	dir := t.TempDir()
	if _, err := extractTarGz(bytes.NewReader(data), dir); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "link")); !os.IsNotExist(err) {
		t.Error("symlink from archive should not be materialised")
	}
	if _, err := os.Stat(filepath.Join(dir, "real.txt")); err != nil {
		t.Errorf("regular file after skipped symlink missing: %v", err)
	}
}

func TestRequireEmptyDir(t *testing.T) {
	if err := requireEmptyDir(filepath.Join(t.TempDir(), "does-not-exist")); err != nil {
		t.Errorf("missing dir should be allowed: %v", err)
	}
	empty := t.TempDir()
	if err := requireEmptyDir(empty); err != nil {
		t.Errorf("empty dir should be allowed: %v", err)
	}
	nonEmpty := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonEmpty, "x"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := requireEmptyDir(nonEmpty); err == nil {
		t.Error("non-empty dir should be refused")
	}
}

func TestFetchUnknownEngine(t *testing.T) {
	if _, err := Fetch("nope", t.TempDir()); err == nil || !strings.Contains(err.Error(), "unknown template") {
		t.Fatalf("Fetch(nope) err = %v, want unknown template", err)
	}
}

// TestFetchE2E actually downloads a pinned template. It is network-bound and off
// by default; run with ASOBI_TEMPLATE_E2E=1. It is the anti-drift guard: it
// fails if a pinned demo no longer extracts into a plausible project.
func TestFetchE2E(t *testing.T) {
	if os.Getenv("ASOBI_TEMPLATE_E2E") == "" {
		t.Skip("set ASOBI_TEMPLATE_E2E=1 to run the network fetch test")
	}
	dir := t.TempDir()
	top, err := Fetch("defold", dir)
	if err != nil {
		t.Fatalf("Fetch(defold): %v", err)
	}
	if len(top) == 0 {
		t.Fatal("Fetch(defold) created nothing")
	}
	if _, err := os.Stat(filepath.Join(dir, "game.project")); err != nil {
		t.Errorf("defold template missing game.project: %v", err)
	}
}
