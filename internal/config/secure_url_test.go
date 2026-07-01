package config

import "testing"

func TestRequireSecureURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https ok", "https://console.asobi.dev", false},
		{"https with path", "https://console.asobi.dev/api", false},
		{"http localhost ok", "http://localhost:8090", false},
		{"http 127.0.0.1 ok", "http://127.0.0.1:8090", false},
		{"http ipv6 loopback ok", "http://[::1]:8090", false},
		{"http public rejected", "http://evil.example.com", true},
		{"http public ip rejected", "http://93.184.216.34", true},
		{"missing scheme rejected", "console.asobi.dev", true},
		{"ftp scheme rejected", "ftp://console.asobi.dev", true},
		{"empty rejected", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireSecureURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("RequireSecureURL(%q) = nil, want error", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("RequireSecureURL(%q) = %v, want nil", tc.url, err)
			}
		})
	}
}
