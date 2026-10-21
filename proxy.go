package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// bootstrapScript is injected into the served index.html <head>, before any app
// JS runs. It
//   - seeds the single-user session token into localStorage, and
//   - tells the page there is a Go side behind this origin (window.hasGoSide),
//     which is what makes goApp (frontend/src/goBridge.ts) hand out the
//     bindings generated from App.
//
// The page calls Go through those bindings over the @wailsio/runtime it
// bundles, as a Wails template's page does, so there is no bridge to build
// here, and sending outward links to the system browser is the page's own
// business too (installGoBridge). What the page cannot find out by itself is
// whether anything answers behind its origin: nothing but the front door
// injects this script, so its presence is the answer.
//
// There is deliberately no window.webSocketBaseURL here, which under v2 told
// the webapp's socket client the board server's own address. The page, the API
// and /ws now share one origin — the front door (frontdoor.go) — so the socket
// connects to where the page came from, as it does in a browser.
func bootstrapScript(sessionToken string) string {
	return fmt.Sprintf(`<script>
(function () {
  // In single-user mode this is the session the API synthesizes, seeded before
  // any app script runs. On a hosted server it is empty and the page logs in.
  try {
    // A desktop page's storage outlives the app and its database; it is kept
    // for one launch, as it was when every launch had a new origin.
    var launch = %q;
    if (launch && localStorage.getItem('launch') !== launch) {
      localStorage.clear();
      localStorage.setItem('launch', launch);
    }
    var seeded = %q;
    if (seeded) { localStorage.setItem('boardSessionId', seeded); }
    // The same token a second way, for the front door (frontdoor.go): the Wails
    // runtime makes its own fetches and a WebSocket carries no header a page
    // can set, so what guards those routes is a cookie. Written here and by
    // octoClient at login, because a login navigates without reloading.
    var token = localStorage.getItem('boardSessionId') || '';
    if (token) {
      document.cookie = 'boardSession=' + token + '; path=/; SameSite=Strict' +
        (location.protocol === 'https:' ? '; Secure' : '');
    }
  } catch (e) {}

  window.hasGoSide = true;
  window.pageOnWails = %t;
})();
</script>`, pageLaunch, sessionToken, pageOnWails)
}

// pageLaunch is this run of a desktop app; empty on a server.
var pageLaunch = func() string {
	if !pageOnWails {
		return ""
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}()

// newServerProxy builds what the front door puts behind everything that is not
// /wails/ or /acp/: the page, and the board the page talks to. In a release
// build and unattached those are the same thing.
//
// Two things pull them apart, and the split is the same both times: the data
// the page asks for lives under two prefixes, /api/v0 (with /files inside it)
// and /ws, and everything else is the page.
//
// Attached, **the page is not fetched from the server.** frontend/dist is
// embedded in this binary, so proxying somebody's HTML would ask the network
// for a file already on this disk — and it would be *their* page, while the
// bootstrap injected into it assumes one this desktop's Go side matches.
//
// With a Vite dev server (devServer, a development build under `wails3 dev`),
// the page and every module it imports come from Vite, and so does the socket
// its HMR client opens — against the page's own origin, which is this door, so
// it goes down the same proxy as an ordinary upgrade. The board keeps the data.
//
// The page never knows which is which — every path it builds is relative to
// window.location.origin, and that is the front door either way. It gets the
// bootstrap either way too, since that is what seeds the session.
//
// token is asked per response rather than baked in: the session for a hosted
// board is not known until somebody logs in. Which board is behind it can
// change while the app runs, which is what boardRouter below is for.
func newServerProxy(wire *boardWire, attached, devServer string, token func() string) (http.Handler, error) {
	local := &url.URL{Scheme: "http", Host: boardWireHost}
	board := wire.transport()
	pageOrigin, pageTransport := local, http.RoundTripper(board)
	if devServer != "" {
		var err error
		if pageOrigin, err = url.Parse(devServer); err != nil {
			return nil, fmt.Errorf("dev server %q: %w", devServer, err)
		}
		pageTransport = nil
	}
	page := pageProxy(pageOrigin, pageTransport, token)

	if attached == "" && devServer == "" {
		return page, nil
	}

	var data http.Handler = localProxy(local, board)
	var err error
	if attached != "" {
		if data, err = attachedProxy(attached); err != nil {
			return nil, err
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/api/v0/", data)
	mux.Handle("/ws", data)
	mux.Handle("/", page)
	return mux, nil
}

// pageProxy serves the page from origin with the bootstrap injected.
func pageProxy(origin *url.URL, transport http.RoundTripper, token func() string) *httputil.ReverseProxy {
	page := localProxy(origin, transport)
	director := page.Director
	page.Director = func(req *http.Request) {
		director(req)
		// Ask upstream for uncompressed HTML so we can inject the bootstrap script.
		req.Header.Del("Accept-Encoding")
	}
	page.ModifyResponse = injectBootstrap(token)
	return page
}

// localProxy forwards to a process of this machine: the board server, or Vite.
// Host is rewritten to the one it listens under — Vite refuses a Host it does
// not know, which is its own guard against DNS rebinding.
func localProxy(origin *url.URL, transport http.RoundTripper) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(origin)
	// nil is the default transport: Vite, on a real port.
	proxy.Transport = transport
	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		director(req)
		req.Host = origin.Host
	}
	return proxy
}

// attachedProxy forwards the data to a hosted board this desktop logged in to.
func attachedProxy(attached string) (*httputil.ReverseProxy, error) {
	remote, err := url.Parse(attached)
	if err != nil {
		return nil, fmt.Errorf("attached server %q: %w", attached, err)
	}
	data := httputil.NewSingleHostReverseProxy(remote)
	dataDirector := data.Director
	data.Director = func(req *http.Request) {
		dataDirector(req)
		// Load-bearing: the far side's own hostGuard checks the Host it was
		// published under and would refuse a loopback name. Do not "simplify"
		// this away.
		req.Host = remote.Host
		// Our loopback origin is not who is calling upstream, and saying so is
		// a lie a guard added there later would believe. Sec-Fetch-* is the
		// browser's own account of the fetch and means nothing after a hop.
		req.Header.Set("Origin", remote.Scheme+"://"+remote.Host)
		req.Header.Del("Sec-Fetch-Site")
		req.Header.Del("Sec-Fetch-Mode")
		req.Header.Del("Sec-Fetch-Dest")
		req.Header.Del("Sec-Fetch-User")
		// The front door's own cookie is the front door's business. The board
		// authenticates by the Authorization header the page sends.
		stripCookie(req, sessionCookie)
	}
	return data, nil
}

// injectBootstrap puts the shim into the served HTML, right after <head>, so
// the token is set before app scripts load.
func injectBootstrap(token func() string) func(*http.Response) error {
	return func(resp *http.Response) error {
		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
			return nil
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		if idx := bytes.Index(body, []byte("<head>")); idx != -1 {
			at := idx + len("<head>")
			inject := []byte(bootstrapScript(token()))
			body = append(body[:at], append(inject, body[at:]...)...)
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
		return nil
	}
}

// stripCookie removes one cookie from a request, leaving the others.
func stripCookie(req *http.Request, name string) {
	cookies := req.Cookies()
	req.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name == name {
			continue
		}
		req.AddCookie(cookie)
	}
}

// boardRouter is the handler the front door holds, so which board the page
// talks to can change without rebuilding the door. A switch is a rebuild and a
// reload: everything the page holds came from the board it loaded against.
type boardRouter struct {
	wire      *boardWire
	devServer string
	token     func() string

	current atomic.Pointer[http.Handler]
	// attached is the server the router points at, empty for this one.
	attached atomic.Pointer[string]
}

func newBoardRouter(wire *boardWire, attached, devServer string, token func() string) (*boardRouter, error) {
	r := &boardRouter{wire: wire, devServer: devServer, token: token}
	if err := r.point(attached); err != nil {
		return nil, err
	}
	return r, nil
}

// point aims the router at a board: a server's origin, or empty for the one
// this process serves.
func (r *boardRouter) point(attached string) error {
	next, err := newServerProxy(r.wire, attached, r.devServer, r.token)
	if err != nil {
		return err
	}
	r.current.Store(&next)
	r.attached.Store(&attached)
	return nil
}

// dialSocket opens a WebSocket at path on the board the router points at.
func (r *boardRouter) dialSocket(path string) (*websocket.Conn, error) {
	attached := ""
	if p := r.attached.Load(); p != nil {
		attached = *p
	}
	if attached == "" {
		return r.wire.dialSocket(path)
	}
	remote, err := url.Parse(attached)
	if err != nil {
		return nil, err
	}
	target := *remote
	target.Scheme = map[string]string{"https": "wss"}[remote.Scheme]
	if target.Scheme == "" {
		target.Scheme = "ws"
	}
	target.Path = strings.TrimSuffix(remote.Path, "/") + path
	header := http.Header{"Origin": {remote.Scheme + "://" + remote.Host}}
	dialer := websocket.Dialer{Proxy: http.ProxyFromEnvironment, HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.Dial(target.String(), header)
	return conn, err
}

func (r *boardRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// WKWebView sends no Content-Length through Wails' asset server, and
	// ReverseProxy drops a body of length 0.
	if req.ContentLength == 0 && req.Body != nil && req.Body != http.NoBody && req.Method != http.MethodGet && req.Method != http.MethodHead {
		req.ContentLength = -1
	}
	handler := r.current.Load()
	if handler == nil {
		http.Error(w, "no board", http.StatusServiceUnavailable)
		return
	}
	(*handler).ServeHTTP(w, req)
}
