package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/artipop/pourquoipas/internal/feedmcp"
)

// newFrontDoor is the desktop's loopback door, for other processes on this
// machine: the agents' board tools and the column feed (each with its own
// token, and refused to a browser by its Origin), and the ingest route, which
// a script or webhook posts to with a source token.
func newFrontDoor(acpRoutes, ingest http.Handler, allowedHost string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/acp/board/", sameOrigin(acpRoutes, allowedHost))
	mux.Handle(feedmcp.Path, sameOrigin(acpRoutes, allowedHost))
	mux.Handle("/sources/ingest/{name}", ingest)
	return hostGuard(requestLog(mux), allowedHost)
}

// requestLog prints every request the front door serves when
// FRONTDOOR_DEBUG is set. A window that shows nothing is otherwise
// silent — the webview has a console nobody can read — and the first question
// is always which of these three the page actually asked for.
func requestLog(next http.Handler) http.Handler {
	if os.Getenv("FRONTDOOR_DEBUG") == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("front door: %s %s (origin %q)", r.Method, r.URL.Path, r.Header.Get("Origin"))
		next.ServeHTTP(w, r)
	})
}

// sameOrigin refuses a cross-site request to the Wails runtime. The bound
// methods start agents and read the filesystem, and the endpoint carries no
// credential of its own: without this any page in any browser could POST to the
// front door and have the call go through — unreadable to it, but done.
//
// A same-origin fetch sends no Origin header (a GET) or sends ours.
//
// Both schemes count, because a server may be published behind TLS while the
// door itself speaks HTTP. The guard is about which *site* is calling, and
// that is the host.
func sameOrigin(next http.Handler, allowedHost string) http.Handler {
	allowedPort := portOf(allowedHost)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !originAllowed(origin, allowedHost, allowedPort) {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, "cross-site request refused", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originAllowed accepts the published authority, and — only when the door is
// published under a loopback name — the other loopback names on its port, so a
// page opened as 127.0.0.1 while published as localhost can still call the
// bindings. Not hostAllowed: its wildcard branch is right for Host and would
// let any site's Origin through here, and this check is what keeps a foreign
// page off the bindings (measured: a door on 0.0.0.0 took evil.example).
func originAllowed(origin, allowedHost, allowedPort string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if u.Host == allowedHost {
		return true
	}
	name, port, err := net.SplitHostPort(u.Host)
	if err != nil || port != allowedPort {
		return false
	}
	return isLoopbackName(strings.TrimSuffix(allowedHost, ":"+allowedPort)) && isLoopbackName(name)
}

func isLoopbackName(name string) bool {
	switch strings.ToLower(name) {
	case "localhost", "127.0.0.1", "[::1]", "::1":
		return true
	}
	return false
}

// hostGuard refuses a request whose Host is not the authority the front door
// was published under — the standard defence against DNS rebinding, where a
// name the attacker controls is repointed at 127.0.0.1 so their page becomes
// same-origin with ours.
func hostGuard(next http.Handler, allowedHost string) http.Handler {
	allowedPort := portOf(allowedHost)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host, allowedHost, allowedPort) {
			http.Error(w, "unexpected Host header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hostAllowed accepts the published authority and the other loopback names on
// the same port.
func hostAllowed(host, allowedHost, allowedPort string) bool {
	if host == allowedHost {
		return true
	}
	name, port, err := net.SplitHostPort(host)
	if err != nil || port != allowedPort {
		return false
	}
	return isLoopbackName(name)
}

func portOf(hostport string) string {
	if _, port, err := net.SplitHostPort(hostport); err == nil {
		return port
	}
	return ""
}

// listenLoopback binds a listener on the loopback interface. A port of 0 asks
// the kernel for a free one, which is what a desktop build wants: the front
// door is private to this process and its window.
func listenLoopback(port int) (net.Listener, error) {
	return net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
}

// sessionCookie is how the token reaches the front door: the Wails runtime
// makes its own fetches and a WebSocket takes no header a page can set. Written
// by the page (session.ts), so not HttpOnly.
const sessionCookie = "boardSession"

// requireSession refuses a request with no live board session. Installed only
// on a hosted server, where sessions exist.
//
// There is no exception under it any more. The runtime module used to be one —
// the page imported /wails/runtime.js before anybody had logged in — and the
// page now bundles the runtime with the bindings, so nothing it needs before a
// login lives behind this guard.
func requireSession(next http.Handler, valid func(string) bool) http.Handler {
	if valid == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || !valid(cookie.Value) {
			http.Error(w, "not logged in", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(withSessionToken(r.Context(), cookie.Value)))
	})
}
