//go:build !frontend

// Package main, default (no `frontend` tag): nothing is embedded, so the server
// serves the page from frontend/dist on disk. This keeps
// `go build ./...` and tests working without a staged frontend bundle.
package main

import "io/fs"

// embeddedFrontend reports that no frontend is compiled in.
func embeddedFrontend() (fs.FS, bool) { return nil, false }
