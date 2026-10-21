//go:build server

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serverRoutesForTest() http.Handler {
	o := &origin{
		board:   named("board"),
		acp:     named("acp"),
		ingest:  named("ingest"),
		session: func(token string) bool { return token == "live" },
	}
	return o.middleware()(named("wails"))
}

func serve(h http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// A team server is opened by its domain, usually behind a proxy that ends TLS
// and passes the Host on without a port. The door that stood in front of Wails
// compared Host and Origin with the address it listened on, and refused all of
// it.
func TestAServerReachedByItsDomainServesEverything(t *testing.T) {
	routes := serverRoutesForTest()
	headers := map[string]string{
		"Host":   "board.example.com",
		"Origin": "https://board.example.com",
		"Cookie": sessionCookie + "=live",
	}
	for path, want := range map[string]string{
		"/":                   "board",
		"/api/v0/teams":       "board",
		"/ws":                 "board",
		"/wails/runtime":      "wails",
		"/acp/events/ws":      "acp",
		"/acp/board/mcp":      "acp",
		"/sources/ingest/sms": "ingest",
	} {
		rec := serve(routes, path, headers)
		if rec.Code != http.StatusOK || rec.Body.String() != want {
			t.Errorf("%s: %d %q, want 200 %q", path, rec.Code, rec.Body.String(), want)
		}
	}
}

// What is about this machine rather than about a board — the bound methods and
// the agents' events — needs a login, as it did behind our own door.
func TestTheBindingsAndTheEventSocketNeedASession(t *testing.T) {
	routes := serverRoutesForTest()
	for _, path := range []string{"/wails/runtime", "/wails/runtime.js", "/acp/events/ws"} {
		rec := serve(routes, path, map[string]string{"Host": "board.example.com"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a session: %d, want 401", path, rec.Code)
		}
	}
}

// Wails checks a runtime call's Origin itself; the routes of ours are checked
// the same way, against the Host the request came in under.
func TestAnotherSiteIsRefusedOurRoutes(t *testing.T) {
	routes := serverRoutesForTest()
	for _, path := range []string{"/acp/events/ws", "/acp/board/mcp"} {
		rec := serve(routes, path, map[string]string{
			"Host":   "board.example.com",
			"Origin": "https://evil.example",
			"Cookie": sessionCookie + "=live",
		})
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s from another site: %d, want 403", path, rec.Code)
		}
	}
}
