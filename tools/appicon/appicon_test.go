package main

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The branding in the repository is what this command makes, pixel for pixel.
// A new picture or a changed mark that nobody re-ran is caught here the way a
// stale schema is caught in tools/schemagen.
func TestTheCommittedBrandingIsWhatTheMarkDraws(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	want, err := icons(root)
	if err != nil {
		t.Fatal(err)
	}
	for name, img := range trays() {
		want["build/tray/"+name+".png"] = img
	}

	for path, w := range want {
		f, err := os.Open(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if decoded.Bounds() != w.Bounds() {
			t.Errorf("%s is %v, the mark draws %v", path, decoded.Bounds(), w.Bounds())
			continue
		}
		// Compared as colours rather than as bytes of one encoding: a PNG
		// optimiser that drops the alpha channel of an opaque image leaves
		// every pixel what it was.
		differ := 0
		b := w.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if color.NRGBAModel.Convert(decoded.At(x, y)) != w.NRGBAAt(x, y) {
					differ++
				}
			}
		}
		if differ > 0 {
			t.Errorf("%s: %d pixels differ from what the mark draws — run `go run ./tools/appicon`", path, differ)
		}
	}
}
