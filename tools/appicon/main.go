// Command appicon regenerates the raster branding assets from the vector mark.
//
//	go run ./tools/appicon
//
// It writes build/appicon.png and the four menu-bar marks under build/tray/.
// darwin/icons.icns and windows/icon.ico come from
// `wails3 task common:generate:icons`, which reads appicon.png and
// appicon.icon; run that after this. The webapp's favicon
// (frontend/public/favicon.svg) draws the same shape by hand.
//
// Two images it deliberately does not draw, though the script it replaces did.
// The disk image's own icon is rendered by create:dmg:icon from the built
// .app's icon. And build/darwin/dmg-background.png has been a picked image
// since d4849aa, not a rendering — drawing it here would overwrite that choice
// with the navy wash it replaced.
//
// The renderer is written out here rather than pulled from a library because
// the mark is only rounded rectangles — a signed distance field with a
// one-pixel anti-aliasing band draws them exactly — and an SVG rasteriser would
// be a dependency for three rectangles. The geometry is the same as
// frontend/src/widgets/icons/appLogo.tsx and build/appicon.icon: three board
// columns, tallest first.
//
// It was a Python script, the last Python in the tree. The port draws the same
// pixels, and the test holds the committed files to it.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type rgb [3]int

var (
	// The product colours: white columns on deep navy.
	navy  = rgb{0x1B, 0x2A, 0x4A}
	white = rgb{0xFF, 0xFF, 0xFF}

	// The amber the board keeps for "an agent is asking" (--warning-rgb in
	// frontend/src/styles/_tokens.scss), in both of its values: the paper theme's
	// on a light menu bar, the screen theme's on a dark one.
	amberOnLight = rgb{0xA8, 0x70, 0x14}
	amberOnDark  = rgb{0xFF, 0xB4, 0x4C}
)

// The menu bar scales whatever it is handed to its own thickness, so 44px is
// 22pt drawn at 2x: crisp on a retina display and right-sized on one that is
// not.
const trayPx = 44

type rect struct{ x, y, w, h, r float64 }

func roundedRectSDF(px, py float64, c rect) float64 {
	dx := math.Abs(px-(c.x+c.w/2)) - (c.w/2 - c.r)
	dy := math.Abs(py-(c.y+c.h/2)) - (c.h/2 - c.r)
	return math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - c.r
}

func circleSDF(px, py, cx, cy, r float64) float64 {
	return math.Hypot(px-cx, py-cy) - r
}

func coverage(sdf float64) float64 {
	return math.Min(math.Max(0.5-sdf, 0), 1)
}

// round is Python's round(), which the committed files were drawn with: half
// to even, not half away from zero, and the difference is visible on an edge.
func round(v float64) int {
	return int(math.RoundToEven(v))
}

// columns are the three columns of the mark, in the proportions of
// appLogo.tsx.
func columns(left, top, width float64) []rect {
	s := width / 560.0
	return []rect{
		{left, top, 147 * s, 471 * s, 44 * s},
		{left + 206*s, top, 147 * s, 324 * s, 44 * s},
		{left + 412*s, top, 147 * s, 177 * s, 44 * s},
	}
}

func blend(dst, src rgb, a float64) rgb {
	var out rgb
	for i := range out {
		out[i] = round(float64(dst[i]) + float64(src[i]-dst[i])*a)
	}
	return out
}

func set(img *image.NRGBA, x, y int, c rgb, a int) {
	o := img.PixOffset(x, y)
	img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(c[0]), uint8(c[1]), uint8(c[2]), uint8(a)
}

func renderIcon(size int) *image.NRGBA {
	fs := float64(size)
	// 22.37% of the side is the corner radius Apple's icon grid uses, and the
	// shape has to be baked in: .icns and .ico carry no mask of their own.
	frame := rect{0, 0, fs, fs, fs * 0.2237}
	markW := fs * 0.547
	cols := columns((fs-markW)/2, (fs-fs*0.46)/2, markW)

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		yc := float64(py) + 0.5
		for px := 0; px < size; px++ {
			xc := float64(px) + 0.5
			a := coverage(roundedRectSDF(xc, yc, frame))
			if a <= 0 {
				continue
			}
			c := navy
			for _, col := range cols {
				if ca := coverage(roundedRectSDF(xc, yc, col)); ca > 0 {
					c = blend(c, white, ca)
				}
			}
			set(img, px, py, c, round(a*255))
		}
	}
	return img
}

// renderTray draws the mark for the system tray: the three columns in ink,
// plus — when dot is set — a badge saying an agent is waiting for a person.
//
// Two inks rather than one template icon, because the badge is the point: a
// macOS template icon is drawn from its alpha alone, so the amber would come
// out the same grey as the columns and the one thing worth noticing across a
// menu bar would stop being noticeable. The badge sits bottom-right, which is
// the corner the descending columns leave empty, so it needs no gap punched
// around it to read as a badge rather than as a fourth column.
func renderTray(ink rgb, dot *rgb) *image.NRGBA {
	cols := columns(4.0, 6.0, 36.0)

	img := image.NewNRGBA(image.Rect(0, 0, trayPx, trayPx))
	for py := 0; py < trayPx; py++ {
		yc := float64(py) + 0.5
		for px := 0; px < trayPx; px++ {
			xc := float64(px) + 0.5
			a, c := 0.0, ink
			for _, col := range cols {
				a = math.Max(a, coverage(roundedRectSDF(xc, yc, col)))
			}
			if dot != nil {
				if da := coverage(circleSDF(xc, yc, 36.5, 35.5, 6.5)); da > a {
					a, c = da, *dot
				}
			}
			if a <= 0 {
				continue
			}
			set(img, px, py, c, round(a*255))
		}
	}
	return img
}

// trays are the four menu-bar marks: idle and waiting, each in the ink for a
// light and for a dark menu bar.
func trays() map[string]*image.NRGBA {
	return map[string]*image.NRGBA{
		"idle-light":    renderTray(navy, nil),
		"idle-dark":     renderTray(white, nil),
		"waiting-light": renderTray(navy, &amberOnLight),
		"waiting-dark":  renderTray(white, &amberOnDark),
	}
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// moduleRoot walks up from the working directory to the go.mod, so the command
// writes to the same place from wherever it is run.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

func main() {
	out := flag.String("dir", "", "the directory to write into (default: build/ at the module root)")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "appicon:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	if dir == "" {
		root, err := moduleRoot()
		if err != nil {
			return err
		}
		dir = filepath.Join(root, "build")
	}
	if err := os.MkdirAll(filepath.Join(dir, "tray"), 0o755); err != nil {
		return err
	}
	if err := writePNG(filepath.Join(dir, "appicon.png"), renderIcon(1024)); err != nil {
		return err
	}
	for name, img := range trays() {
		if err := writePNG(filepath.Join(dir, "tray", name+".png"), img); err != nil {
			return err
		}
	}
	fmt.Println("wrote appicon.png, tray/{idle,waiting}-{light,dark}.png")
	fmt.Println("now run: wails3 task common:generate:icons")
	return nil
}
