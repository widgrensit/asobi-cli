package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func tarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	tw.Write(content)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func zipArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(content)
	zw.Close()
	return buf.Bytes()
}

func TestAssetName(t *testing.T) {
	got := assetName()
	if !strings.HasPrefix(got, "asobi_"+runtime.GOOS+"_"+runtime.GOARCH) {
		t.Errorf("assetName = %q, missing os/arch", got)
	}
	wantExt := ".tar.gz"
	if runtime.GOOS == "windows" {
		wantExt = ".zip"
	}
	if !strings.HasSuffix(got, wantExt) {
		t.Errorf("assetName = %q, want suffix %q", got, wantExt)
	}
}

func TestVerifyChecksum(t *testing.T) {
	asset := []byte("pretend archive bytes")
	sum := sha256.Sum256(asset)
	good := fmt.Sprintf("%s  asobi_linux_amd64.tar.gz\n%s  other.zip\n",
		hex.EncodeToString(sum[:]), strings.Repeat("0", 64))

	if err := verifyChecksum(asset, []byte(good), "asobi_linux_amd64.tar.gz"); err != nil {
		t.Errorf("valid checksum rejected: %v", err)
	}
	if err := verifyChecksum([]byte("tampered"), []byte(good), "asobi_linux_amd64.tar.gz"); err == nil {
		t.Error("tampered asset must fail checksum")
	}
	if err := verifyChecksum(asset, []byte(good), "missing.tar.gz"); err == nil {
		t.Error("missing checksum entry must fail")
	}
}

func TestExtractBinary(t *testing.T) {
	want := []byte("#!/fake asobi binary\x00\x01")

	got, err := extractBinary(tarGz(t, "asobi", want), "asobi_linux_amd64.tar.gz")
	if err != nil {
		t.Fatalf("tar.gz: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("tar.gz binary content mismatch")
	}

	got, err = extractBinary(zipArchive(t, "asobi.exe", want), "asobi_windows_amd64.zip")
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("zip binary content mismatch")
	}

	if _, err := extractBinary(tarGz(t, "README.md", want), "asobi_linux_amd64.tar.gz"); err == nil {
		t.Error("archive without asobi binary must error")
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "asobi")
	if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(exe, []byte("new binary")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new binary" {
		t.Errorf("content = %q, want new binary", got)
	}
	// Windows has no Unix execute bit; executability is by file extension, so
	// this assertion only applies off Windows.
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(exe); info.Mode().Perm()&0o100 == 0 {
			t.Error("replaced binary is not executable")
		}
	}
	if err := replaceExecutable(exe, nil); err == nil {
		t.Error("empty binary must be refused")
	}
}

func TestManagedBy(t *testing.T) {
	cases := map[string]string{
		"/opt/homebrew/bin/asobi":                     "Homebrew",
		"/home/linuxbrew/.linuxbrew/bin/asobi":        "Homebrew",
		"/Users/x/scoop/apps/asobi/current/asobi.exe": "Scoop",
		"/nix/store/abc-asobi/bin/asobi":              "Nix",
		"/home/x/.local/bin/asobi":                    "",
		"/usr/local/bin/asobi":                        "",
	}
	for path, want := range cases {
		if got := managedBy(path); got != want {
			t.Errorf("managedBy(%q) = %q, want %q", path, got, want)
		}
	}
}

// Exercises the real download -> verify -> extract pipeline against a server,
// the security-critical path short of replacing the running binary.
func TestDownloadVerifyExtractPipeline(t *testing.T) {
	binary := []byte("the new asobi\x7fELF")
	name := assetName()
	var archive []byte
	if strings.HasSuffix(name, ".zip") {
		archive = zipArchive(t, "asobi.exe", binary)
	} else {
		archive = tarGz(t, "asobi", binary)
	}
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Write([]byte(checksums))
		case strings.HasSuffix(r.URL.Path, name):
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := releasesBaseURL
	releasesBaseURL = srv.URL
	defer func() { releasesBaseURL = old }()

	asset, err := download(fmt.Sprintf("%s/v9.9.9/%s", releasesBaseURL, name))
	if err != nil {
		t.Fatal(err)
	}
	sums, err := download(releasesBaseURL + "/v9.9.9/checksums.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(asset, sums, name); err != nil {
		t.Fatalf("checksum: %v", err)
	}
	got, err := extractBinary(asset, name)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, binary) {
		t.Error("extracted binary mismatch")
	}
}

func TestExtractBinaryRejectsOversized(t *testing.T) {
	old := maxDownload
	maxDownload = 8
	defer func() { maxDownload = old }()

	big := bytes.Repeat([]byte("A"), 100)
	if _, err := extractBinary(tarGz(t, "asobi", big), "asobi_linux_amd64.tar.gz"); err == nil {
		t.Error("oversized extracted binary must be rejected, not silently truncated")
	}
}

func TestDownloadRefusesHTTPDowngrade(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("payload"))
	}))
	defer target.Close()
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redir.Close()

	if _, err := download(redir.URL + "/asset"); err == nil {
		t.Error("redirect to a non-HTTPS URL must be refused")
	}
}
