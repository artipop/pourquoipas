package main

import (
	"net"
	"net/url"
	"os"
)

// viteDevServer is where `wails3 dev` started Vite, if it did: every app it
// starts is handed that address in FRONTEND_DEVSERVER_URL. The front door then
// takes the page and its modules from there rather than from the board server,
// and Vite's HMR socket rides the same origin (proxy.go), so an edit to the
// page is on screen without a rebuild.
//
// Only a development build listens. A release binary started from a shell that
// happens to carry the variable must still serve the page it embeds — the
// address names somebody else's process.
//
// The variable is otherwise left for Wails, whose own preRun reads it and waits
// for Vite to answer before the window opens — the wait this needs anyway.
func viteDevServer() string {
	if !appIsDev {
		return ""
	}
	return pinLoopbackIPv4("FRONTEND_DEVSERVER_URL")
}

// pinLoopbackIPv4 rewrites a "localhost" in the URL held by the variable to
// 127.0.0.1, in the environment as well, and returns the result.
//
// wails3 dev writes "localhost" and nothing else, and that name is two
// addresses: Node binds Vite to whichever the system lists first — ::1 on
// Windows — while Go dials what it resolves. On a machine where IPv6 loopback
// is refused, measured on the Windows laptop this is developed on, Wails'
// preRun waited five seconds for a Vite that was up and killed the app. So the
// two ends are pinned to one address: the dev:frontend task starts Vite on
// 127.0.0.1, and this points the app — and Wails' own wait — there too.
func pinLoopbackIPv4(variable string) string {
	raw := os.Getenv(variable)
	u, err := url.Parse(raw)
	if raw == "" || err != nil || u.Hostname() != "localhost" {
		return raw
	}
	u.Host = net.JoinHostPort("127.0.0.1", u.Port())
	_ = os.Setenv(variable, u.String())
	return u.String()
}
