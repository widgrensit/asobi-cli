package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LogLine is one engine log line as Loki returned it.
type LogLine struct {
	Ts   string `json:"ts"`
	Line string `json:"line"`
}

// LogOptions narrows a log query. Zero values mean "use the server's default";
// the control plane clamps the range to 24h and the count to 500, so there is
// no point enforcing either here as well.
type LogOptions struct {
	Filter string
	Since  time.Duration
	Limit  int
}

// ParseSince turns "30m", "2h", "90s" or a bare number of seconds into a
// duration. Accepting a bare number matters because the API takes seconds, so
// somebody reading the docs and passing 3600 should get what they expect
// rather than an error.
func ParseSince(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n <= 0 {
			return 0, fmt.Errorf("--since must be positive")
		}
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("--since %q: use 30m, 2h, or a number of seconds", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("--since must be positive")
	}
	return d, nil
}

// EnvLogs fetches engine logs for one environment. The tenant and environment
// are resolved server-side from the caller's token and the environment name -
// nothing here selects whose logs come back.
func EnvLogs(creds *Credentials, game, name string, opts LogOptions) ([]LogLine, error) {
	q := url.Values{}
	if game != "" {
		q.Set("game", game)
	}
	if opts.Filter != "" {
		q.Set("filter", opts.Filter)
	}
	if opts.Since > 0 {
		q.Set("since", strconv.Itoa(int(opts.Since.Seconds())))
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	endpoint := creds.SaasURL + "/internal/cli/envs/" + url.PathEscape(name) + "/logs"
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		refreshed, err := RefreshAccessToken(creds)
		if err != nil {
			return nil, err
		}
		creds.AccessToken = refreshed
		_ = SaveCredentials(creds)
		return EnvLogs(creds, game, name, opts)
	}
	// Both of these are worth naming: one is a filter the caller can fix, the
	// other is a backend that is down and has nothing to do with them.
	if resp.StatusCode == 422 {
		return nil, fmt.Errorf("invalid --filter")
	}
	if resp.StatusCode == 503 {
		return nil, fmt.Errorf("logs are temporarily unavailable")
	}
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("logs failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var body struct {
		Lines []LogLine `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode logs: %w", err)
	}
	return body.Lines, nil
}
