package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/artipop/xciii/server/model"
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

// The split is the whole design: Wails owns /wails/ and the board owns
// everything else, /ws included — which is what lets the socket live on the
// page's own origin instead of being told a second address.
func TestFrontDoorSendsWailsToWailsAndTheRestToTheBoard(t *testing.T) {
	door := newFrontDoor(named("wails"), named("acp"), named("ingest"), named("board"), "127.0.0.1:9000", nil)

	for path, want := range map[string]string{
		"/wails/runtime.js":     "wails",
		"/wails/runtime":        "wails",
		"/acp/terminal/abc/ws":  "acp",
		"/acp/events/ws":        "acp",
		"/sources/ingest/phone": "ingest",
		"/":                     "board",
		"/ws":                   "board",
		"/api/v0/teams":         "board",
	} {
		if got := request(t, door, path, nil).Body.String(); got != want {
			t.Errorf("%s went to %q, want %q", path, got, want)
		}
	}
}

// The bound methods start agents and read the filesystem, and the runtime
// endpoint carries no credential of its own. Any page in any browser can reach
// a loopback port, so a call whose Origin is somebody else's page is refused —
// the response would be unreadable to them, the side effect would not.
func TestFrontDoorRefusesCrossOriginRuntimeCalls(t *testing.T) {
	door := newFrontDoor(named("wails"), named("acp"), named("ingest"), named("board"), "127.0.0.1:9000", nil)

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"no origin (a plain GET from our own page)", nil, http.StatusOK},
		{"our own origin", map[string]string{"Origin": "http://127.0.0.1:9000"}, http.StatusOK},
		{"same-origin fetch metadata", map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"another site", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"another site by fetch metadata", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
	}
	for _, c := range cases {
		if got := request(t, door, "/wails/runtime", c.headers).Code; got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
	}
}

// The door listens on the whole loopback, so the page can be opened as
// 127.0.0.1 while published as localhost; the host guard lets that in, and a
// runtime that then refused every binding call left the page with no terminal,
// no settings and no word of why. But a door published on every interface
// accepts any Host, and taking any Origin with it would hand the bindings to
// every site — the loopback allowance is for a loopback door only.
func TestFrontDoorTakesTheOtherLoopbackNameButNoOtherSite(t *testing.T) {
	cases := []struct {
		published, host, origin string
		want                    int
	}{
		{"localhost:9000", "127.0.0.1:9000", "http://127.0.0.1:9000", http.StatusOK},
		{"127.0.0.1:9000", "localhost:9000", "http://localhost:9000", http.StatusOK},
		{"127.0.0.1:9000", "[::1]:9000", "http://[::1]:9000", http.StatusOK},
		{"127.0.0.1:9000", "127.0.0.1:9000", "http://localhost:9001", http.StatusForbidden},
		{"127.0.0.1:9000", "127.0.0.1:9000", "http://evil.example:9000", http.StatusForbidden},
		{"0.0.0.0:9000", "evil.example:9000", "http://evil.example:9000", http.StatusForbidden},
		{"0.0.0.0:9000", "0.0.0.0:9000", "http://localhost:9000", http.StatusForbidden},
	}
	for _, c := range cases {
		door := newFrontDoor(named("wails"), named("acp"), named("ingest"), named("board"), c.published, nil)
		req := httptest.NewRequest(http.MethodPost, "/wails/runtime", nil)
		req.Host = c.host
		req.Header.Set("Origin", c.origin)
		rec := httptest.NewRecorder()
		door.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("published %s, Origin %s: status %d, want %d", c.published, c.origin, rec.Code, c.want)
		}
	}
}

// The tailnet door serves the same handler over TLS (tsnetdoor.go), so the
// page's own origin is https there. Without this the phone's every binding call
// and both sockets would be refused as somebody else's site — and the board
// would look broken in a way nothing in the page could explain.
func TestFrontDoorAcceptsItsOwnOriginOverTLS(t *testing.T) {
	door := newFrontDoor(named("wails"), named("acp"), named("ingest"), named("board"), "board.tail1234.ts.net", nil)

	for _, path := range []string{"/wails/runtime", "/acp/events/ws", "/acp/terminal/abc/ws"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = "board.tail1234.ts.net"
		req.Header.Set("Origin", "https://board.tail1234.ts.net")
		rec := httptest.NewRecorder()
		door.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, want 200", path, rec.Code)
		}
	}
}

// A name the attacker controls, repointed at 127.0.0.1, would otherwise be
// same-origin with the app for as long as it is running.
func TestFrontDoorRefusesAnUnexpectedHost(t *testing.T) {
	door := newFrontDoor(named("wails"), named("acp"), named("ingest"), named("board"), "127.0.0.1:9000", nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "rebind.example:9000"
	rec := httptest.NewRecorder()
	door.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status %d, want %d", rec.Code, http.StatusForbidden)
	}

	// localhost and 127.0.0.1 are the same machine on the same port, and the
	// user may well type either.
	for _, host := range []string{"localhost:9000", "127.0.0.1:9000", "[::1]:9000"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		door.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Host %s: status %d, want 200", host, rec.Code)
		}
	}
}

// A server build published on every interface is reached by whatever name the
// user's DNS gives it, so there is no authority left to check — only the port.
// Without this, `SERVER_HOST=0.0.0.0` would answer 403 to everyone
// but localhost, which is the opposite of what asking for it means.
func TestFrontDoorPublishedOnEveryInterfaceAcceptsAnyName(t *testing.T) {
	door := newFrontDoor(named("wails"), named("acp"), named("ingest"), named("board"), "0.0.0.0:8080", nil)

	for host, want := range map[string]int{
		"boards.internal:8080": http.StatusOK,
		"192.168.1.10:8080":    http.StatusOK,
		"boards.internal:9999": http.StatusForbidden, // a different port is a different server
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
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
