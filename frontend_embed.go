//go:build frontend

// Package main, `frontend` build tag: the page's bundle is compiled into the
// binary straight from `frontend/dist`, where `build:frontend` leaves it, as in
// the Wails template. Release builds pass `-tags frontend`; plain `go
// build`/tests omit it and fall back to the bundle on disk (see
// frontend_disk.go), so the module still builds
// without a built frontend.
package main

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

// embeddedFrontend returns the compiled-in bundle rooted at its top level.
func embeddedFrontend() (fs.FS, bool) {
	sub, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		return nil, false
	}
	return sub, true
}
