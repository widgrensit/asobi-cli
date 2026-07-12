// Package template scaffolds a runnable game project by fetching one of the asobi
// demo repositories at a pinned commit: an engine template (client + backend) or
// the backend template (server-Lua backend only). The repos are the single source
// of truth (no parallel templates to drift); each carries its own README
// documenting how to run the backend and, for engine templates, open the project.
package template

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Template is a demo repository pinned to an immutable release tag. Bump Ref to
// adopt a newer demo; the demos auto-cut tags via their release workflow on merge
// to main, so a stable tag always exists to pin to.
type Template struct {
	Repo string
	Ref  string
	Name string
}

var registry = map[string]Template{
	"defold":  {Repo: "asobi-defold-demo", Ref: "v0.2.0", Name: "Defold"},
	"godot":   {Repo: "asobi-godot-demo", Ref: "v0.2.0", Name: "Godot"},
	"unity":   {Repo: "asobi-unity-demo", Ref: "v0.2.0", Name: "Unity"},
	"backend": {Repo: "sdk_demo_backend", Ref: "v0.1.0", Name: "Backend"},
}

const (
	maxFiles = 20000
	maxBytes = 500 << 20 // 500 MiB, generous for a Unity demo
)

// Names returns the known template names, sorted (e.g. defold, godot, unity,
// backend).
func Names() []string {
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Get returns the template for an engine and whether it is known.
func Get(engine string) (Template, bool) {
	t, ok := registry[engine]
	return t, ok
}

// Fetch downloads the template for engine and extracts it into dir. dir must be
// empty. It returns the sorted top-level entries created.
func Fetch(engine, dir string) ([]string, error) {
	t, ok := registry[engine]
	if !ok {
		return nil, fmt.Errorf("unknown template %q; available: %s", engine, strings.Join(Names(), ", "))
	}
	if err := requireEmptyDir(dir); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://github.com/widgrensit/%s/archive/%s.tar.gz", t.Repo, t.Ref)
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s template: %w", t.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s template: %s returned %s", t.Name, url, resp.Status)
	}

	return extractTarGz(resp.Body, dir)
}

// requireEmptyDir returns an error unless dir does not exist or is an empty
// directory. Extracting a template over existing files would clobber them.
func requireEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty; a template needs a fresh directory (try: asobi init mygame --template <engine>)", dir)
	}
	return nil
}

// extractTarGz extracts a GitHub archive tarball into dir, stripping the single
// top-level "<repo>-<ref>/" directory. It writes only regular files and
// directories: symlinks, hardlinks, devices and paths escaping dir are refused,
// so a hostile archive cannot write outside the target tree.
func extractTarGz(r io.Reader, dir string) ([]string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gunzip template: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	top := map[string]struct{}{}
	var files, total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read template archive: %w", err)
		}

		rel := stripLeadingComponent(hdr.Name)
		if rel == "" {
			continue
		}
		dest, err := safeJoin(dir, rel)
		if err != nil {
			return nil, err
		}
		if i := strings.IndexByte(rel, '/'); i >= 0 {
			top[rel[:i]] = struct{}{}
		} else {
			top[rel] = struct{}{}
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return nil, fmt.Errorf("create %s: %w", rel, err)
			}
		case tar.TypeReg:
			files++
			if files > maxFiles {
				return nil, fmt.Errorf("template archive has too many files (>%d)", maxFiles)
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return nil, fmt.Errorf("create dir for %s: %w", rel, err)
			}
			n, err := writeFile(dest, tr, maxBytes-total)
			if err != nil {
				return nil, fmt.Errorf("write %s: %w", rel, err)
			}
			total += n
		default:
			// Skip symlinks, hardlinks, devices, fifos: never materialise a link
			// from a downloaded archive.
			continue
		}
	}

	names := make([]string, 0, len(top))
	for n := range top {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

func writeFile(dest string, r io.Reader, budget int64) (int64, error) {
	if budget <= 0 {
		return 0, fmt.Errorf("template archive exceeds size limit (%d bytes)", int64(maxBytes))
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(r, budget))
	if err != nil {
		return n, err
	}
	return n, nil
}

// stripLeadingComponent drops the first path element (the archive's
// "<repo>-<ref>/" wrapper) and returns a clean slash path.
func stripLeadingComponent(name string) string {
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	i := strings.IndexByte(name, '/')
	if i < 0 {
		return ""
	}
	return strings.Trim(name[i+1:], "/")
}

// safeJoin joins rel onto dir, refusing absolute paths or any that escape dir.
func safeJoin(dir, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in template archive: %q", rel)
	}
	dest := filepath.Join(dir, clean)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	if absDest != absDir && !strings.HasPrefix(absDest, absDir+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in template archive: %q", rel)
	}
	return dest, nil
}
