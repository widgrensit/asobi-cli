package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// maxDownload bounds a release asset or checksums download. Asset archives are
// a few MB; this is a generous ceiling against a hostile oversized response.
const maxDownload = 64 << 20

var downloadClient = &http.Client{Timeout: 2 * time.Minute}

// SelfUpgrade downloads the latest release for this platform, verifies it
// against the release checksums, and atomically replaces the running binary.
// It refuses to touch a package-manager-managed or read-only install.
func SelfUpgrade(current string, w io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	latest, err := LatestReleaseTag(context.Background())
	if err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}
	if !IsNewer(latest, current) {
		fmt.Fprintf(w, "asobi is already up to date (%s).\n", normalize(current))
		return nil
	}

	if pm := managedBy(exe); pm != "" {
		return fmt.Errorf("asobi was installed with %s; upgrade with it instead (e.g. `%s`)", pm, pmUpgradeHint(pm))
	}

	name := assetName()
	fmt.Fprintf(w, "Upgrading asobi %s -> %s ...\n", normalize(current), latest)

	asset, err := download(fmt.Sprintf("%s/%s/%s", releasesBaseURL, latest, name))
	if err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}
	sums, err := download(fmt.Sprintf("%s/%s/checksums.txt", releasesBaseURL, latest))
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	if err := verifyChecksum(asset, sums, name); err != nil {
		return err
	}

	bin, err := extractBinary(asset, name)
	if err != nil {
		return err
	}
	if err := replaceExecutable(exe, bin); err != nil {
		return err
	}

	fmt.Fprintf(w, "Upgraded to %s.\n", latest)
	return nil
}

// assetName is the release archive for the running platform, matching the
// goreleaser `asobi_{{.Os}}_{{.Arch}}` name template.
func assetName() string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("asobi_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

func download(url string) ([]byte, error) {
	resp, err := downloadClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownload {
		return nil, fmt.Errorf("%s exceeds %d byte limit", url, maxDownload)
	}
	return data, nil
}

// verifyChecksum fails closed: the asset's SHA-256 must appear in sums against
// its exact filename, or the upgrade is refused.
func verifyChecksum(asset, sums []byte, name string) error {
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum for %s in checksums.txt", name)
	}
	sum := sha256.Sum256(asset)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, want)
	}
	return nil
}

// extractBinary pulls the single `asobi` executable out of a verified archive.
func extractBinary(archive []byte, name string) ([]byte, error) {
	if strings.HasSuffix(name, ".zip") {
		return extractZip(archive)
	}
	return extractTarGz(archive)
}

func extractTarGz(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("gunzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if isBinaryEntry(hdr.Name) {
			return io.ReadAll(io.LimitReader(tr, maxDownload))
		}
	}
	return nil, fmt.Errorf("asobi binary not found in archive")
}

func extractZip(archive []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	for _, f := range zr.File {
		if isBinaryEntry(f.Name) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, maxDownload))
		}
	}
	return nil, fmt.Errorf("asobi binary not found in archive")
}

func isBinaryEntry(path string) bool {
	base := filepath.Base(filepath.ToSlash(path))
	return base == "asobi" || base == "asobi.exe"
}

// replaceExecutable writes bin next to exe and atomically renames it over the
// running binary. On Windows a running image cannot be overwritten, so the
// current binary is moved aside first.
func replaceExecutable(exe string, bin []byte) error {
	if len(bin) == 0 {
		return fmt.Errorf("refusing to install an empty binary")
	}
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".asobi-upgrade-*")
	if err != nil {
		return fmt.Errorf("write to %s: %w (need write access; use sudo or your package manager)", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return fmt.Errorf("move current binary aside: %w", err)
		}
		if err := os.Rename(tmpName, exe); err != nil {
			_ = os.Rename(old, exe)
			return fmt.Errorf("install new binary: %w", err)
		}
		return nil
	}
	if err := os.Rename(tmpName, exe); err != nil {
		return fmt.Errorf("install new binary: %w", err)
	}
	return nil
}

// managedBy returns the package manager that owns exe, or "" if it looks like a
// plain install we may replace in place.
func managedBy(exe string) string {
	p := filepath.ToSlash(exe)
	switch {
	case strings.Contains(p, "/Cellar/") || strings.Contains(p, "/homebrew/") || strings.Contains(p, "linuxbrew"):
		return "Homebrew"
	case strings.Contains(strings.ToLower(p), "/scoop/"):
		return "Scoop"
	case strings.Contains(p, "/nix/store/"):
		return "Nix"
	}
	return ""
}

func pmUpgradeHint(pm string) string {
	switch pm {
	case "Homebrew":
		return "brew upgrade asobi"
	case "Scoop":
		return "scoop update asobi"
	default:
		return "your package manager"
	}
}
