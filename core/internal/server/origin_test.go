package server

import (
	"net/http/httptest"
	"testing"
)

func TestRequireLocalOrigin(t *testing.T) {
	cases := []struct {
		name      string
		origin    string
		shouldErr bool
	}{
		// Programmatic clients (curl, Go HTTP client) usually omit
		// the Origin header on POST. This is the most common path
		// for non-browser callers and must be allowed.
		{"missing origin", "", false},

		// file:// loads (production Electron build) send "null".
		{"null origin", "null", false},

		// The Vite dev server: hostname is localhost or a loopback IP.
		{"localhost dev server", "http://localhost:5173", false},
		{"127.0.0.1 dev server", "http://127.0.0.1:5173", false},
		{"::1 ipv6 loopback", "http://[::1]:5173", false},

		// CSRF defense: any public origin must be rejected. The
		// authToken middleware already blocks unauthorized callers
		// when configured, but this is the second line.
		{"public https origin", "https://attacker.example.com", true},
		{"public http origin", "http://evil.test", true},

		// Malformed Origin headers — be conservative and reject
		// rather than try to interpret.
		{"garbage origin", "not-a-url", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/auth/chatgpt/start", nil)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			err := requireLocalOrigin(r)
			if tc.shouldErr && err == nil {
				t.Errorf("origin %q should have been rejected", tc.origin)
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("origin %q should have been accepted, got %v", tc.origin, err)
			}
		})
	}
}
