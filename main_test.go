package main

import (
	"os"
	"testing"
)

// `wails3 dev` starts Vite and says where in FRONTEND_DEVSERVER_URL. A
// development build takes the page from there, which is what puts an edit to
// the page on screen without a rebuild. (Tests are built without the
// production tag, so this is the development build's answer.)
func TestADevelopmentBuildTakesThePageFromVite(t *testing.T) {
	t.Setenv("FRONTEND_DEVSERVER_URL", "http://192.0.2.1:9245")
	if got := viteDevServer(); got != "http://192.0.2.1:9245" {
		t.Fatalf("viteDevServer() = %q, want the address wails3 dev handed over", got)
	}
}

// wails3 dev says "localhost", which is ::1 to Node and whatever Go resolves to
// Go; where IPv6 loopback is refused, Wails waited for a Vite that was up and
// killed the app. The app and Wails' own wait are pointed at 127.0.0.1, where
// the dev:frontend task starts Vite.
func TestTheDevServerIsReachedOverIPv4Loopback(t *testing.T) {
	t.Setenv("FRONTEND_DEVSERVER_URL", "http://localhost:9245")
	if got := viteDevServer(); got != "http://127.0.0.1:9245" {
		t.Fatalf("viteDevServer() = %q, want 127.0.0.1", got)
	}
	if got := os.Getenv("FRONTEND_DEVSERVER_URL"); got != "http://127.0.0.1:9245" {
		t.Fatalf("Wails would still wait on %q", got)
	}
}

// With no Vite running the page is the board server's, as in a release build.
func TestWithoutViteThePageIsTheBoardServers(t *testing.T) {
	t.Setenv("FRONTEND_DEVSERVER_URL", "")
	if got := viteDevServer(); got != "" {
		t.Fatalf("viteDevServer() = %q with nothing set", got)
	}
}
