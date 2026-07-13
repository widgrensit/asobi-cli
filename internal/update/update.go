// Package update checks for and installs newer asobi CLI releases. The version
// check (Notify) is best-effort and never blocks or fails a command; the
// self-upgrade (see upgrade.go) verifies downloads against the release
// checksums before replacing the running binary.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widgrensit/asobi-cli/internal/config"
)

const repo = "widgrensit/asobi-cli"

const installCmd = "curl -fsSL https://raw.githubusercontent.com/widgrensit/asobi-cli/main/install.sh | sh"

// checkInterval is how long a cached version-check result is trusted before we
// hit the network again.
const checkInterval = 24 * time.Hour

// Overridable in tests.
var (
	apiBaseURL      = "https://api.github.com"
	releasesBaseURL = "https://github.com/" + repo + "/releases/download"
	httpClient      = &http.Client{Timeout: 3 * time.Second}
)

// Notify prints a one-line notice to w when a newer release than current
// exists. It is best-effort: it consults a ~/.asobi cache first, refreshes at
// most once per checkInterval, and stays silent on any error, opt-out, in CI,
// or for a dev build. cmd is the invoked subcommand, used to skip commands
// where a notice is noise (version, upgrade, help).
func Notify(current, cmd string, w io.Writer) {
	if !shouldCheck(current, cmd) {
		return
	}
	latest, err := cachedOrFetch()
	if err != nil || latest == "" {
		return
	}
	if IsNewer(latest, current) {
		fmt.Fprintf(w, "\nA new asobi is available: %s (you have %s).\nUpgrade: asobi upgrade   (or: %s)\n",
			latest, normalize(current), installCmd)
	}
}

func shouldCheck(current, cmd string) bool {
	switch {
	case os.Getenv("ASOBI_NO_UPDATE_CHECK") != "":
		return false
	case os.Getenv("CI") != "":
		return false
	case current == "" || current == "dev":
		return false
	}
	switch cmd {
	case "version", "--version", "-v", "upgrade", "help", "--help", "-h":
		return false
	}
	return true
}

type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

func cachePath() string {
	return filepath.Join(config.Dir(), "version_check.json")
}

// cachedOrFetch returns the latest known release tag, using the cache when it
// is fresh and otherwise refreshing it from the releases API.
func cachedOrFetch() (string, error) {
	if c, ok := readCache(); ok && time.Since(c.CheckedAt) < checkInterval {
		return c.Latest, nil
	}
	latest, err := LatestReleaseTag(context.Background())
	if err != nil {
		return "", err
	}
	writeCache(cache{CheckedAt: time.Now(), Latest: latest})
	return latest, nil
}

func readCache() (cache, bool) {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return cache{}, false
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		return cache{}, false
	}
	return c, true
}

func writeCache(c cache) {
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(cachePath(), data, 0o600)
}

// LatestReleaseTag returns the tag_name of the newest published release.
func LatestReleaseTag(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", apiBaseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "asobi-cli")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("releases API returned %s", resp.Status)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return "", err
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("no tag_name in latest release")
	}
	return rel.TagName, nil
}

// IsNewer reports whether latest is a strictly higher version than current.
func IsNewer(latest, current string) bool {
	return CompareVersions(latest, current) > 0
}

// CompareVersions compares two vMAJOR.MINOR.PATCH strings (a leading "v" is
// optional), returning -1, 0 or 1. Any pre-release/build suffix is ignored;
// unparseable components sort as 0.
func CompareVersions(a, b string) int {
	as, bs := parse(a), parse(b)
	for i := 0; i < 3; i++ {
		if as[i] != bs[i] {
			if as[i] < bs[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parse(v string) [3]int {
	v = normalize(v)
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		out[i], _ = strconv.Atoi(part)
	}
	return out
}

func normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}
