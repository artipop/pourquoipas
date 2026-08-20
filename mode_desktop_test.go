//go:build !server

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Wails runtime probes for an optional custom.js on every page load, and a
// 404 there is written into the browser console on every launch. It has to read
// as "no such script" to the runtime and as nothing at all to a person.
func TestTheProbeForAnOptionalScriptIsAnsweredQuietly(t *testing.T) {
	handler := quietOptionalCustomJS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, customJSPath, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("custom.js answered %d, want %d", rec.Code, http.StatusNoContent)
	}
	// Anything the runtime would take for a script it has to load costs a
	// second request for a body that does not exist.
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Fatalf("custom.js named a content type %q, which asks the runtime to fetch it", got)
	}
}

// Everything else is the asset server's, including the runtime module itself.
func TestNothingElseIsAnsweredForTheAssetServer(t *testing.T) {
	handler := quietOptionalCustomJS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wails/runtime.js", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("runtime.js answered %d, want the asset server's own %d", rec.Code, http.StatusTeapot)
	}
}
