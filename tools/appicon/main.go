// Command appicon regenerates the raster branding assets.
//
//	go run ./tools/appicon
//
// The app icon is a picture, build/appicon.icon/Assets/pourquoipas.png, with a
// night version beside it (pourquoipas-dark.png): square, full bleed, no
// corners. Icon Composer takes them as they are and masks them itself;
// everything else needs the corners baked in, so this writes the masked
// build/appicon.png and 64px copies of both: the page's favicon
// (frontend/public/favicon{,-dark}.png), the logo in its sidebar menu
// (frontend/src/assets/appLogo{,Dark}.png) and the guide's favicon
// (docs/guide/public/favicon{,-dark}.png). darwin/icons.icns and windows/icon.ico come
// from `wails3 task common:generate:icons`, which reads appicon.png and
// appicon.icon; run that after this.
//
// The four menu-bar marks under build/tray/ are drawn, not taken from the
// picture: they are one ink at 22pt, where an illustration is a smudge. The mark
// is the picture's bow in silhouette.
//
// Two images it deliberately does not draw. The disk image's own icon is
// rendered by create:dmg:icon from the built .app's icon. And
// build/darwin/dmg-background.png has been a picked image since d4849aa, not a
// rendering.
//
// The renderer is written out here rather than pulled from a library because
// the shapes are only rounded rectangles — a signed distance field with a
// one-pixel anti-aliasing band draws them exactly — and the only resize is by a
// whole factor, which a box filter does exactly.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type rgb [3]int

var (
	// The tray mark's inks: navy for a light menu bar, white for a dark one.
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

// iconSize is the side of the source picture and of build/appicon.png;
// pagePx is the page's copies, a whole factor of it.
const (
	iconSize = 1024
	pagePx   = 64
)

// maskIcon cuts the corners out of the full-bleed picture. 22.37% of the side
// is the corner radius Apple's icon grid uses, and the shape has to be baked
// in: .icns and .ico carry no mask of their own.
func maskIcon(art image.Image) (*image.NRGBA, error) {
	b := art.Bounds()
	if b.Dx() != iconSize || b.Dy() != iconSize {
		return nil, fmt.Errorf("the picture is %dx%d, want %dx%d", b.Dx(), b.Dy(), iconSize, iconSize)
	}
	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	draw.Draw(img, img.Bounds(), art, b.Min, draw.Src)
	fs := float64(iconSize)
	frame := rect{0, 0, fs, fs, fs * 0.2237}
	for py := 0; py < iconSize; py++ {
		for px := 0; px < iconSize; px++ {
			a := coverage(roundedRectSDF(float64(px)+0.5, float64(py)+0.5, frame))
			o := img.PixOffset(px, py)
			img.Pix[o+3] = uint8(round(float64(img.Pix[o+3]) * a))
		}
	}
	return img, nil
}

// shrink averages each factor×factor block into one pixel, weighting colour by
// alpha so the transparent corners do not darken the edge.
func shrink(src *image.NRGBA, factor int) *image.NRGBA {
	side := src.Bounds().Dx() / factor
	dst := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			var r, g, bl, a float64
			for dy := 0; dy < factor; dy++ {
				for dx := 0; dx < factor; dx++ {
					o := src.PixOffset(x*factor+dx, y*factor+dy)
					w := float64(src.Pix[o+3])
					r += float64(src.Pix[o]) * w
					g += float64(src.Pix[o+1]) * w
					bl += float64(src.Pix[o+2]) * w
					a += w
				}
			}
			o := dst.PixOffset(x, y)
			if a > 0 {
				dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2] = uint8(round(r/a)), uint8(round(g/a)), uint8(round(bl/a))
			}
			dst.Pix[o+3] = uint8(round(a / float64(factor*factor)))
		}
	}
	return dst
}

// icons are the masked icon and its page copies, keyed by path from the module
// root.
func icons(root string) (map[string]*image.NRGBA, error) {
	light, err := pageIcons(root, artPath)
	if err != nil {
		return nil, err
	}
	dark, err := pageIcons(root, darkArtPath)
	if err != nil {
		return nil, err
	}
	return map[string]*image.NRGBA{
		"build/appicon.png":                   light[0],
		"frontend/public/favicon.png":         light[1],
		"frontend/public/favicon-dark.png":    dark[1],
		"frontend/src/assets/appLogo.png":     light[1],
		"frontend/src/assets/appLogoDark.png": dark[1],
		"docs/guide/public/favicon.png":       light[1],
		"docs/guide/public/favicon-dark.png":  dark[1],
	}, nil
}

// pageIcons masks one picture and returns it at full size and at page size.
func pageIcons(root, path string) ([2]*image.NRGBA, error) {
	f, err := os.Open(filepath.Join(root, path))
	if err != nil {
		return [2]*image.NRGBA{}, err
	}
	defer f.Close()
	art, err := png.Decode(f)
	if err != nil {
		return [2]*image.NRGBA{}, fmt.Errorf("%s: %w", path, err)
	}
	icon, err := maskIcon(art)
	if err != nil {
		return [2]*image.NRGBA{}, fmt.Errorf("%s: %w", path, err)
	}
	return [2]*image.NRGBA{icon, shrink(icon, iconSize/pagePx)}, nil
}

const artPath = "build/appicon.icon/Assets/pourquoipas.png"

// darkArtPath is the same picture at night. Icon Composer shows it in dark
// appearance (icon.json); the page shows its small copy in the dark theme.
// build/appicon.png, which Windows and Linux take, stays the day one.
const darkArtPath = "build/appicon.icon/Assets/pourquoipas-dark.png"

// bow is the tray mark: the ship's bow as the icon draws it, climbing to the
// right, cut off by the left edge the way the picture cuts the liner off. In
// tray pixels; the tip is a few points rather than a curve, which at 44px is
// the same thing.
var bow = [][2]float64{
	{4, 20}, {38, 13.2}, {40.4, 12.9}, {41.0, 13.6}, {40.7, 14.9}, {35.2, 38}, {4, 38},
}

// funnel stands on the deck behind the bow, as on the icon, with a gap above
// the hull so the two read as separate shapes. Its top is the icon's dark cap;
// with one ink per mark, the cap is told apart by a cut, funnelBand.
var funnel = [][2]float64{
	{8, 5.5}, {17, 5.5}, {17, 15.4}, {8, 17.2},
}

var funnelBand = [][2]float64{
	{6, 10}, {19, 10}, {19, 11.6}, {6, 11.6},
}

// polygonSDF is the signed distance to a closed polygon, negative inside.
func polygonSDF(px, py float64, pts [][2]float64) float64 {
	d := math.Inf(1)
	inside := false
	for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
		ax, ay := pts[j][0], pts[j][1]
		bx, by := pts[i][0], pts[i][1]
		ex, ey := bx-ax, by-ay
		wx, wy := px-ax, py-ay
		t := math.Max(0, math.Min(1, (wx*ex+wy*ey)/(ex*ex+ey*ey)))
		d = math.Min(d, math.Hypot(wx-ex*t, wy-ey*t))
		if (ay > py) != (by > py) && px < ax+(py-ay)*ex/ey {
			inside = !inside
		}
	}
	if inside {
		return -d
	}
	return d
}

// renderTray draws the mark for the system tray: the bow and funnel in ink, plus — when
// dot is set — a badge saying an agent is waiting for a person.
//
// Two inks rather than one template icon, because the badge is the point: a
// macOS template icon is drawn from its alpha alone, so the amber would come
// out the same grey as the hull and the one thing worth noticing across a
// menu bar would stop being noticeable. The badge sits bottom-right, on the
// hull, so a ring is punched out around it or it would read as a rivet.
func renderTray(ink rgb, dot *rgb) *image.NRGBA {
	const dotX, dotY, dotR, gap = 36.5, 35.5, 6.5, 2.5

	img := image.NewNRGBA(image.Rect(0, 0, trayPx, trayPx))
	for py := 0; py < trayPx; py++ {
		yc := float64(py) + 0.5
		for px := 0; px < trayPx; px++ {
			xc := float64(px) + 0.5
			hull := coverage(polygonSDF(xc, yc, bow))
			stack := coverage(polygonSDF(xc, yc, funnel)) * (1 - coverage(polygonSDF(xc, yc, funnelBand)))
			a, c := math.Max(hull, stack), ink
			if dot != nil {
				a *= 1 - coverage(circleSDF(xc, yc, dotX, dotY, dotR+gap))
				if da := coverage(circleSDF(xc, yc, dotX, dotY, dotR)); da > a {
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
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "appicon:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	out, err := icons(root)
	if err != nil {
		return err
	}
	for name, img := range trays() {
		out["build/tray/"+name+".png"] = img
	}
	for path, img := range out {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := writePNG(full, img); err != nil {
			return err
		}
		fmt.Println("wrote", path)
	}
	fmt.Println("now run: wails3 task common:generate:icons")
	return nil
}
