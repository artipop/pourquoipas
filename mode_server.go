//go:build server

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/artipop/pourquoipas/internal/apperr"
	"github.com/artipop/pourquoipas/internal/feedmcp"
)

// origin, in a server build, is Wails' own HTTP server. It is the door here:
// Wails binds the address, checks the Host on a loopback bind and the Origin of
// a runtime call against the request's own Host, and offers TLS and the
// origins a proxy in front may add (ServerOptions). Our routes ride it as asset
// middleware, which runs before Wails answers a WebSocket upgrade with 501, so
// the board's /ws and our sockets work through it.
//
// A desktop needs a door of its own (mode_desktop.go, frontdoor.go): there the
// page is not reached over the network at all, and a socket has nowhere to go.
//
// The address is SERVER_HOST/SERVER_PORT, or Wails' own WAILS_SERVER_* names.
type origin struct {
	board   http.Handler
	acp     http.Handler
	ingest  http.Handler
	host    string
	port    int
	// session says whether a request carries a live board session. See
	// identity.go.
	session func(string) bool
}

const defaultServerPort = 8080

func newOrigin(board, acp, ingest http.Handler, session func(string) bool) (*origin, error) {
	host := envOnce("SERVER_HOST", "WAILS_SERVER_HOST")
	if host == "" {
		host = "localhost"
	}
	port := defaultServerPort
	if raw := envOnce("SERVER_PORT", "WAILS_SERVER_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 65535 {
			return nil, fmt.Errorf("server: bad port %q", raw)
		}
		port = parsed
	}
	return &origin{board: board, acp: acp, ingest: ingest, host: host, port: port, session: session}, nil
}

// envOnce reads the first of the given variables that is set and unsets all of
// them: Wails reads the WAILS_SERVER_* pair itself and would let it override
// what ServerOptions says.
func envOnce(names ...string) string {
	value := ""
	for _, name := range names {
		if value == "" {
			value = os.Getenv(name)
		}
		_ = os.Unsetenv(name)
	}
	return value
}

func (o *origin) transport() application.Transport { return nil }

func (o *origin) serverOptions() application.ServerOptions {
	return application.ServerOptions{Host: o.host, Port: o.port}
}

// middleware puts our routes in front of Wails' asset chain. Wails' own
// /wails/events and /wails/stream/ws are on its router, outside this chain, so
// no session guards them; the emitter keeps our events off that bus in this
// build (emitsOnBus).
func (o *origin) middleware() application.Middleware {
	return func(wails http.Handler) http.Handler {
		mux := http.NewServeMux()
		// runtime.js and the bound methods. Wails checks a call's Origin
		// itself; the session is ours to ask.
		mux.Handle("/wails/", requireSession(wails, o.session))
		mux.Handle("/acp/events/ws", requireSession(requestOrigin(o.acp), o.session))
		mux.Handle("/acp/board/", requestOrigin(o.acp))
		mux.Handle(feedmcp.Path, requestOrigin(o.acp))
		mux.Handle("/sources/ingest/{name}", o.ingest)
		// The board takes everything else straight, as the desktop's door
		// gives it: its page, its API and its /ws socket, which the asset
		// server would refuse.
		mux.Handle("/", o.board)
		return requestLog(mux)
	}
}

// requestOrigin is sameOrigin keyed to the Host the request came in under, the
// rule Wails applies to its own routes: a server is reached by whatever name
// DNS or a proxy gives it, and a session, not the name, says who is calling.
func requestOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sameOrigin(next, r.Host).ServeHTTP(w, r)
	})
}

// start has nothing to start: Wails serves when the application runs.
func (o *origin) start() {}

func (o *origin) url() string {
	return "http://" + net.JoinHostPort(o.host, strconv.Itoa(o.port)) + "/"
}

// emitsOnBus: nothing in a server build listens on the Wails bus, and Wails
// broadcasts it on /wails/events to anybody who connects, logged in or not.
const emitsOnBus = false

// A headless build has no windows to outlive, so the question the desktop
// answers with a flag — does closing the last one end us — never arises, and
// the ordinary answer is the right one.
func appShouldQuit() bool { return true }

// And nothing to bring back when there is no Dock to click.
func watchDockReopen(*application.App, string) {}

// newMainWindow opens nothing: a server build has no webview, and the same
// page is reached with a browser instead.
func newMainWindow(_ *application.App, url string) {
	log.Printf("server mode: no window; open %s in a browser", url)
}

// openDiffWindow, likewise: the page is a route the board already serves, and a
// browser opens it itself.
func openDiffWindow(_ *application.App, _, _, _ string) {}

// openShareWindow has no window to open. A server build is reached with a
// browser, and the share dialog is a URL like any other page of it.
func openShareWindow(_ *application.App, url string) {
	log.Printf("server mode: no window; open %s in a browser", url)
}

// closeShareWindow has nothing to close: the dialog is a browser tab, and the
// page says what happened instead.
func closeShareWindow(_ *application.App) {}

// pickDirectory has no native dialog to open. Saying so is the point: an empty
// path would reach the UI as a cancelled picker and look like a bug.
func pickDirectory(_ *application.App, _ string) (string, error) {
	return "", apperr.New("folders.no-picker", "A server has no folder picker: type the path")
}
