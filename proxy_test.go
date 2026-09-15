package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// upstream stands in for the in-process board server.
func upstream(t *testing.T, contentType, body string) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func proxied(t *testing.T, port int) *http.Response {
	t.Helper()
	h, err := newServerProxy(port, "", "", func() string { return "su-token" })
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Result()
}

func TestProxyInjectsTheBootstrapIntoHTML(t *testing.T) {
	res := proxied(t, upstream(t, "text/html; charset=utf-8", "<html><head><title>x</title></head><body>hi</body></html>"))
	body, _ := io.ReadAll(res.Body)
	page := string(body)

	if !strings.Contains(page, "su-token") || !strings.Contains(page, "window.hasGoSide = true") {
		t.Fatalf("bootstrap missing from the page:\n%s", page)
	}
	// The page, the API and /ws share the front door's origin now, so nothing
	// has to tell the socket client where the board server lives.
	if strings.Contains(page, "webSocketBaseURL") {
		t.Error("bootstrap still overrides the WebSocket base URL")
	}
	// It has to land before the app's own scripts, which is what "right after
	// <head>" buys: the session token must exist by the time they run.
	if strings.Index(page, "boardSessionId") > strings.Index(page, "<title>") {
		t.Error("bootstrap injected after the head contents")
	}
	if got := res.Header.Get("Content-Length"); got != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length %q does not match the rewritten body (%d)", got, len(body))
	}
}

func TestProxyLeavesNonHTMLAlone(t *testing.T) {
	res := proxied(t, upstream(t, "application/json", `{"ok":true}`))
	body, _ := io.ReadAll(res.Body)
	if string(body) != `{"ok":true}` {
		t.Errorf("json response was rewritten: %s", body)
	}
}

// The page calls Go through bindings generated from App and loads them itself
// (goBridge.ts). What it cannot find out by itself is whether anything answers
// behind its origin — the same bundle runs in a plain browser — and that is the
// one thing the bootstrap has to say: without it every desktop-only screen
// silently decides it is not running in the app. A bridge built here as well
// would be a second, untyped way to the same methods.
func TestTheBootstrapTellsThePageThereIsAGoSide(t *testing.T) {
	script := bootstrapScript("su-token")

	if !strings.Contains(script, "window.hasGoSide = true") {
		t.Errorf("bootstrap does not mark the Go side:\n%s", script)
	}
	for _, unwanted := range []string{"Call.ByName", "/wails/runtime.js", "new Proxy", "window.runtime"} {
		if strings.Contains(script, unwanted) {
			t.Errorf("bootstrap still builds a bridge of its own (%q)", unwanted)
		}
	}
}

// The desktop webview cannot navigate to an outside origin, so every outward
// link has to be handed to the system browser. A previous version only caught
// target=_blank anchors carrying no inline onclick — which is exactly what a
// markdown link in a card comment is not, leaving such links dead on click.
func TestBootstrapSendsEveryOutwardLinkToTheBrowser(t *testing.T) {
	script := bootstrapScript("su-token")

	for _, want := range []string{
		`a[href]`, // every anchor, not only target=_blank ones
		`window.openInNewBrowser(href)`,
		`e.stopPropagation()`, // an inline onclick must not fire as well
		`, true)`,             // capture phase, so this runs first
	} {
		if !strings.Contains(script, want) {
			t.Errorf("bootstrap does not contain %q:\n%s", want, script)
		}
	}
	for _, unwanted := range []string{
		`a[target="_blank"]`,
		`getAttribute('onclick')`,
	} {
		if strings.Contains(script, unwanted) {
			t.Errorf("bootstrap still narrows the catch-all by %q", unwanted)
		}
	}
	// Same-origin navigation belongs to the app itself.
	if !strings.Contains(script, "window.location.origin") {
		t.Error("bootstrap does not keep in-app links inside the webview")
	}
}

// The page writes the cookie the guard reads, and the two names are in
// different languages: this is the one place they can be compared.
func TestTheBootstrapWritesTheCookieTheGuardReads(t *testing.T) {
	script := bootstrapScript("su-token")
	if !strings.Contains(script, sessionCookie+"=") {
		t.Fatalf("the bootstrap script writes no %q cookie", sessionCookie)
	}
}

// The token is asked for per response rather than baked in when the proxy is
// built: a hosted board's session is not known until somebody logs in, and it
// changes when they log out.
func TestTheBootstrapCarriesWhateverTokenIsCurrent(t *testing.T) {
	port := upstream(t, "text/html; charset=utf-8", "<html><head></head><body></body></html>")
	token := "first"
	h, err := newServerProxy(port, "", "", func() string { return token })
	if err != nil {
		t.Fatal(err)
	}

	serve := func() string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		body, _ := io.ReadAll(rec.Result().Body)
		return string(body)
	}

	if !strings.Contains(serve(), "first") {
		t.Fatal("the bootstrap did not carry the token")
	}
	token = "second"
	if !strings.Contains(serve(), "second") {
		t.Fatal("the bootstrap kept a token that is no longer current")
	}
}

// Attached, the target is somebody else's server, and three things about the
// request have to stop describing our loopback door: the Host it names, the
// Origin it claims, and our own session cookie.
func TestARequestToAnAttachedServerDoesNotDescribeOurOwnDoor(t *testing.T) {
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
	}))
	t.Cleanup(srv.Close)

	h, err := newServerProxy(1, srv.URL, "", func() string { return "session" })
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v0/boards", nil)
	req.Host = "127.0.0.1:9000"
	req.Header.Set("Origin", "http://127.0.0.1:9000")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "ours"})
	req.AddCookie(&http.Cookie{Name: "somebody-elses", Value: "kept"})
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got == nil {
		t.Fatal("nothing reached the server")
	}
	if got.Host == "127.0.0.1:9000" {
		t.Errorf("Host still names our door: %q — the far side's hostGuard would refuse it", got.Host)
	}
	if origin := got.Header.Get("Origin"); strings.Contains(origin, "127.0.0.1:9000") {
		t.Errorf("Origin still claims our door: %q", origin)
	}
	if got.Header.Get("Sec-Fetch-Site") != "" {
		t.Error("Sec-Fetch-* is the browser's account of a fetch and means nothing after a hop")
	}
	if _, err := got.Cookie(sessionCookie); err == nil {
		t.Error("our own front-door cookie went to somebody else's server")
	}
	if _, err := got.Cookie("somebody-elses"); err != nil {
		t.Error("stripping one cookie took the others with it")
	}
}

// The split attaching makes, and the whole of it: the data goes to the server,
// the page stays here.
//
// The page is not fetched from the server because it is already on this disk —
// webapp/pack is embedded in this binary — and because it is *ours*: the
// bootstrap injected into it assumes a page that knows window.go.main.App, so a
// server on another version would hand back one this desktop does not match.
func TestAttachedTheDataGoesAcrossAndThePageStaysHere(t *testing.T) {
	var asked []string
	far := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"from":"the server"}`)
	}))
	t.Cleanup(far.Close)

	near := upstream(t, "text/html; charset=utf-8", "<html><head></head><body>ours</body></html>")
	h, err := newServerProxy(near, far.URL, "", func() string { return "session" })
	if err != nil {
		t.Fatal(err)
	}

	get := func(path string) string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		body, _ := io.ReadAll(rec.Result().Body)
		return string(body)
	}

	for _, path := range []string{"/api/v0/boards", "/api/v0/files/teams/0/b/f", "/ws"} {
		if got := get(path); !strings.Contains(got, "the server") {
			t.Errorf("%s was answered here: %q", path, got)
		}
	}
	if len(asked) != 3 {
		t.Errorf("the server was asked for %v, want the three data paths", asked)
	}

	// The page and its assets are this binary's own, and they carry the
	// bootstrap with the server's session in it.
	page := get("/")
	if !strings.Contains(page, "ours") {
		t.Errorf("the page came from somewhere else: %q", page)
	}
	if !strings.Contains(page, "session") {
		t.Error("the page was served without the attached server's session")
	}
	for _, path := range []string{"/static/main.js", "/anything/else"} {
		if got := get(path); !strings.Contains(got, "ours") {
			t.Errorf("%s left this machine: %q", path, got)
		}
	}
	if len(asked) != 3 {
		t.Errorf("the server was asked for the page too: %v", asked)
	}
}

// Under `wails3 dev` the page comes from Vite and the data from the board: Vite
// has the modules, the board has /api and /ws. The page gets the bootstrap
// either way, since that is what seeds the session and says there is a Go side.
func TestWithADevServerThePageComesFromViteAndTheDataFromTheBoard(t *testing.T) {
	board := upstream(t, "application/json", `{"from":"the board"}`)
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html><head></head><body>from vite</body></html>")
	}))
	t.Cleanup(vite.Close)

	h, err := newServerProxy(board, "", vite.URL, func() string { return "su-token" })
	if err != nil {
		t.Fatal(err)
	}
	get := func(path string) string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		body, _ := io.ReadAll(rec.Result().Body)
		return string(body)
	}

	for _, path := range []string{"/api/v0/boards", "/ws"} {
		if got := get(path); !strings.Contains(got, "the board") {
			t.Errorf("%s did not reach the board: %q", path, got)
		}
	}
	for _, path := range []string{"/", "/board/abc", "/src/main.tsx", "/@vite/client"} {
		if got := get(path); !strings.Contains(got, "from vite") {
			t.Errorf("%s did not come from Vite: %q", path, got)
		}
	}
	if page := get("/"); !strings.Contains(page, "su-token") {
		t.Error("the page from Vite came without the bootstrap")
	}
}

// Vite's HMR client opens its socket against the page's own origin, which is
// the front door, so the upgrade has to pass through the proxy the page came
// down. Without it the page loads and no edit ever arrives.
func TestTheHMRSocketGoesThroughToVite(t *testing.T) {
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "not an upgrade", http.StatusBadRequest)
			return
		}
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = buf.Flush()
	}))
	t.Cleanup(vite.Close)

	h, err := newServerProxy(upstream(t, "application/json", "{}"), "", vite.URL, func() string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	door := httptest.NewServer(h)
	t.Cleanup(door.Close)

	conn, err := net.Dial("tcp", door.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = fmt.Fprintf(conn, "GET /?token=t HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n"+
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Protocol: vite-hmr\r\n\r\n",
		door.Listener.Addr())
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("the HMR upgrade was answered %q", strings.TrimSpace(status))
	}
}
