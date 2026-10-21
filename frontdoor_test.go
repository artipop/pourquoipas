package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/artipop/pourquoipas/server/model"
)

func named(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(name))
	})
}

func request(t *testing.T, h http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "127.0.0.1:9000"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The loopback door is for other processes; the page is on wails:// and has no
// business here, so nothing of it is served — not the HTML, which carries the
// session token, not the API, not the bindings.
func TestTheLoopbackDoorServesProcessesAndNotThePage(t *testing.T) {
	door := newFrontDoor(named("acp"), named("ingest"), "127.0.0.1:9000")

	for path, want := range map[string]string{
		"/acp/board/mcp":        "acp",
		"/sources/ingest/phone": "ingest",
	} {
		if got := request(t, door, path, nil).Body.String(); got != want {
			t.Errorf("%s went to %q, want %q", path, got, want)
		}
	}
	for _, path := range []string{"/", "/api/v0/teams", "/ws", "/wails/runtime", "/acp/events/ws"} {
		if code := request(t, door, path, nil).Code; code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, code)
		}
	}
}

// Any page in any browser can reach a loopback port; the agents' tools are not
// for it.
func TestTheLoopbackDoorRefusesAnotherSite(t *testing.T) {
	door := newFrontDoor(named("acp"), named("ingest"), "127.0.0.1:9000")

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"a process, no origin", nil, http.StatusOK},
		{"another site", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"another site by fetch metadata", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"another loopback port", map[string]string{"Origin": "http://localhost:9001"}, http.StatusForbidden},
	}
	for _, c := range cases {
		if got := request(t, door, "/acp/board/mcp", c.headers).Code; got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
	}
}

// A name the attacker controls, repointed at 127.0.0.1, would otherwise reach
// the door as if it were ours.
func TestTheLoopbackDoorRefusesAnUnexpectedHost(t *testing.T) {
	door := newFrontDoor(named("acp"), named("ingest"), "127.0.0.1:9000")

	for host, want := range map[string]int{
		"rebind.example:9000": http.StatusForbidden,
		"localhost:9000":      http.StatusOK,
		"127.0.0.1:9000":      http.StatusOK,
		"[::1]:9000":          http.StatusOK,
	} {
		req := httptest.NewRequest(http.MethodGet, "/acp/board/mcp", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		door.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %s: status %d, want %d", host, rec.Code, want)
		}
	}
}

// sessionsOf stands in for the board's session table: the guard asks it one
// question and the answer is all that decides.
type sessionsOf map[string]bool

func (s sessionsOf) GetSession(token string) (*model.Session, error) {
	if !s[token] {
		return nil, model.NewErrNotFound("session")
	}
	return &model.Session{ID: token, Token: token, UserID: "somebody"}, nil
}

func teapot() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
}

// The routes behind the guard start agents and open shells. On a hosted server
// a request that carries no live session is not one of the team's.
func TestTheGuardRefusesARequestWithNoSession(t *testing.T) {
	guarded := requireSession(teapot(), boardSessionGuard(sessionsOf{"live": true}))

	for _, tc := range []struct {
		name   string
		cookie *http.Cookie
		want   int
	}{
		{"no cookie at all", nil, http.StatusUnauthorized},
		{"a token nobody issued", &http.Cookie{Name: sessionCookie, Value: "made-up"}, http.StatusUnauthorized},
		{"a live session", &http.Cookie{Name: sessionCookie, Value: "live"}, http.StatusTeapot},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The call, not the script: the module itself is open (below).
			req := httptest.NewRequest(http.MethodPost, "/wails/runtime", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			rec := httptest.NewRecorder()
			guarded.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// Nothing under /wails/ is served to a page with no session. The runtime module
// was the exception while the page imported it before a login; the page bundles
// the runtime now, so an exception there would be a hole nothing uses.
func TestNothingOfTheRuntimeIsServedToAPageWithNoSession(t *testing.T) {
	guarded := requireSession(teapot(), boardSessionGuard(sessionsOf{}))

	for _, tc := range []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"the runtime module", http.MethodGet, "/wails/runtime.js", http.StatusUnauthorized},
		{"a call made through it", http.MethodPost, "/wails/runtime", http.StatusUnauthorized},
		{"anything else under it", http.MethodGet, "/wails/events", http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			guarded.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// On a desktop there is no session to carry, and the guard has to be absent
// rather than closed — otherwise the app locks itself out of its own bound
// methods. newSessionGuard is what answers nil there.
func TestOnADesktopTheGuardStandsAside(t *testing.T) {
	guarded := requireSession(teapot(), nil)

	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/wails/runtime", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("got %d, want the handler to have run", rec.Code)
	}
}
