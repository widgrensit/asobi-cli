package config

import (
	"fmt"
	"net"
	"net/url"
)

// RequireSecureURL rejects URLs that would send credentials over cleartext.
// https is always accepted; http is accepted only for loopback hosts
// (localhost / 127.0.0.1 / ::1) to keep local development working.
//
// This is the enforcement behind the "TLS is always required" promise:
// the device-login flow and every authenticated API call derive their
// endpoint from a URL that must pass this check first.
func RequireSecureURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopbackHost(u.Hostname()) {
			return nil
		}
		return fmt.Errorf(
			"refusing to use insecure URL %q: https is required for non-localhost hosts",
			raw,
		)
	case "":
		return fmt.Errorf("invalid URL %q: missing scheme (expected https://)", raw)
	default:
		return fmt.Errorf("unsupported URL scheme %q in %q (expected https)", u.Scheme, raw)
	}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
