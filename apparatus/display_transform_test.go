// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"math"
	"os"
	"testing"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
)

// displayView must put p at c + m·(p − c) + d under SDL's mapping
// q = m·(viewport origin + p).
func TestDisplayViewMapping(t *testing.T) {
	for _, tc := range []struct {
		w, h      int
		dx, dy, m float32
	}{
		{200, 100, -30, 10, 1},
		{200, 100, 0, 0, 0.5},
		{1920, 1080, -80, 12, 0.9},
		{1024, 768, 5, -7, 1.25},
	} {
		vp := displayView(tc.w, tc.h, tc.dx, tc.dy, tc.m)
		cx, cy := float32(tc.w)/2, float32(tc.h)/2
		for _, p := range [][2]float32{{0, 0}, {cx, cy}, {float32(tc.w), float32(tc.h)}} {
			gotX, gotY := tc.m*(float32(vp.X)+p[0]), tc.m*(float32(vp.Y)+p[1])
			wantX, wantY := cx+tc.m*(p[0]-cx)+tc.dx, cy+tc.m*(p[1]-cy)+tc.dy
			// The viewport is integer: off by at most m/2.
			if math.Abs(float64(gotX-wantX)) > float64(tc.m)/2+1e-3 || math.Abs(float64(gotY-wantY)) > float64(tc.m)/2+1e-3 {
				t.Errorf("%+v: p=%v lands at (%v,%v), want (%v,%v)", tc, p, gotX, gotY, wantX, wantY)
			}
		}
		if vp.W != int32(tc.w) || vp.H != int32(tc.h) {
			t.Errorf("%+v: viewport size %dx%d, want the drawing space", tc, vp.W, vp.H)
		}
	}
}

// newOffscreenScreen opens a w×h window on SDL's offscreen video driver with
// the software renderer, set up as NewScreen sets up a fullscreen one (a
// STRETCH presentation of the window's size). It skips the test where SDL
// cannot run.
func newOffscreenScreen(t *testing.T, w, h int32) *Screen {
	t.Helper()
	t.Setenv("SDL_VIDEO_DRIVER", "offscreen")
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		t.Skipf("SDL offscreen video unavailable: %v", err)
	}
	win, err := sdl.CreateWindow("display-transform-test", int(w), int(h), 0)
	if err != nil {
		sdl.Quit()
		t.Skipf("no offscreen window: %v", err)
	}
	r, err := win.CreateRenderer("software")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetLogicalPresentation(w, h, sdl.LOGICAL_PRESENTATION_STRETCH); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		r.Destroy()
		win.Destroy()
		sdl.Quit()
	})
	return &Screen{
		Window: win, Renderer: r, BgColor: sdl.Color{R: 128, G: 128, B: 128, A: 255},
		Width: int(w), Height: int(h),
		LogicalSize: &sdl.FPoint{X: float32(w), Y: float32(h)},
		native:      presentation{w, h, sdl.LOGICAL_PRESENTATION_STRETCH},
	}
}

// box is the bounding box of the pixels of one class, in output pixels.
type box struct{ x0, y0, x1, y1 int }

func (b box) empty() bool { return b.x1 < b.x0 }

// readBoxes returns the bounding boxes of the white (> 200) and black (< 50)
// pixels of the whole window.
func readBoxes(t *testing.T, s *Screen) (white, black box) {
	t.Helper()
	var surf *sdl.Surface
	// ReadPixels(nil) reads the viewport, which a letterbox insets too, so
	// the window's own presentation is put back for the read.
	err := s.withoutTransform(func() error {
		if s.logical != nil {
			if err := s.Renderer.SetLogicalPresentation(s.native.w, s.native.h, s.native.mode); err != nil {
				return err
			}
			defer s.Renderer.SetLogicalPresentation(s.logical.w, s.logical.h, s.logical.mode)
		}
		var err error
		surf, err = s.Renderer.ReadPixels(nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := surf.Convert(sdl.PIXELFORMAT_RGBA32)
	if err != nil {
		t.Fatal(err)
	}
	white, black = box{1 << 30, 1 << 30, -1, -1}, box{1 << 30, 1 << 30, -1, -1}
	px := c.Pixels()
	grow := func(b *box, x, y int) {
		b.x0, b.y0 = min(b.x0, x), min(b.y0, y)
		b.x1, b.y1 = max(b.x1, x), max(b.y1, y)
	}
	for y := 0; y < int(c.H); y++ {
		for x := 0; x < int(c.W); x++ {
			switch v := px[y*int(c.Pitch)+4*x]; {
			case v > 200:
				grow(&white, x, y)
			case v < 50:
				grow(&black, x, y)
			}
		}
	}
	return white, black
}

// drawWhite clears to the background and fills r (drawing space, SDL
// coordinates) in white.
func drawWhite(t *testing.T, s *Screen, r sdl.FRect) {
	t.Helper()
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := s.Renderer.SetDrawColor(255, 255, 255, 255); err != nil {
		t.Fatal(err)
	}
	if err := s.Renderer.RenderFillRect(&r); err != nil {
		t.Fatal(err)
	}
}

func wantBox(t *testing.T, label string, got box, want box) {
	t.Helper()
	if d := max(abs(got.x0-want.x0), abs(got.y0-want.y0), abs(got.x1-want.x1), abs(got.y1-want.y1)); d > 1 {
		t.Errorf("%s: drawn at x[%d,%d] y[%d,%d], want x[%d,%d] y[%d,%d]",
			label, got.x0, got.x1, got.y0, got.y1, want.x0, want.x1, want.y0, want.y1)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestDisplayTransformMovesWhatIsDrawn(t *testing.T) {
	s := newOffscreenScreen(t, 200, 100)
	// A 20×20 square centred on the screen centre (100, 50).
	sq := *s.CenteredRect(sdl.FPoint{}, 20, 20)

	drawWhite(t, s, sq)
	white, _ := readBoxes(t, s)
	wantBox(t, "identity", white, box{90, 40, 109, 59})

	// +Y is up: a shift of +10 moves the square 10 px up the screen.
	if err := s.SetDisplayTransform(-30, 10, 1); err != nil {
		t.Fatal(err)
	}
	drawWhite(t, s, sq)
	white, _ = readBoxes(t, s)
	wantBox(t, "shift (-30, +10)", white, box{60, 30, 79, 49})

	// Magnified about the centre, then shifted: the square is 10 px wide and
	// its centre at (100-30, 50-10).
	if err := s.SetDisplayTransform(-30, 10, 0.5); err != nil {
		t.Fatal(err)
	}
	drawWhite(t, s, sq)
	white, _ = readBoxes(t, s)
	wantBox(t, "shift (-30, +10) magnification 0.5", white, box{65, 35, 74, 44})

	// Clear paints the whole window, the strip the image uncovers included.
	if err := s.SetDisplayTransform(60, 0, 1); err != nil {
		t.Fatal(err)
	}
	s.BgColor = sdl.Color{R: 255, G: 255, B: 255, A: 255}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	white, _ = readBoxes(t, s)
	wantBox(t, "Clear under a shift", white, box{0, 0, 199, 99})
	s.BgColor = sdl.Color{R: 128, G: 128, B: 128, A: 255}

	// WithoutDisplayTransform draws at screen positions.
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := s.WithoutDisplayTransform(func() error {
		_ = s.Renderer.SetDrawColor(255, 255, 255, 255)
		return s.Renderer.RenderFillRect(nil)
	}); err != nil {
		t.Fatal(err)
	}
	white, _ = readBoxes(t, s)
	wantBox(t, "fill without the transform", white, box{0, 0, 199, 99})

	// Back to the identity.
	if err := s.SetDisplayTransform(0, 0, 1); err != nil {
		t.Fatal(err)
	}
	drawWhite(t, s, sq)
	white, _ = readBoxes(t, s)
	wantBox(t, "identity again", white, box{90, 40, 109, 59})

	if err := s.SetDisplayTransform(0, 0, 0); err == nil {
		t.Error("magnification 0 accepted")
	}
}

func TestDisplayTransformUnderLogicalSize(t *testing.T) {
	// 100×100 logical, letterboxed in 400×200: scale 2, bars 100 px wide.
	s := newOffscreenScreen(t, 400, 200)
	if err := s.SetLogicalSize(100, 100); err != nil {
		t.Fatal(err)
	}
	sq := *s.CenteredRect(sdl.FPoint{}, 10, 10) // (45..55) logical = (190..210) screen
	drawWhite(t, s, sq)
	white, _ := readBoxes(t, s)
	wantBox(t, "letterbox identity", white, box{190, 90, 209, 109})

	// The shift is in screen pixels whatever the logical size.
	if err := s.SetDisplayTransform(-40, -20, 1); err != nil {
		t.Fatal(err)
	}
	drawWhite(t, s, sq)
	white, _ = readBoxes(t, s)
	wantBox(t, "letterbox shift (-40, -20)", white, box{150, 110, 169, 129})

	// Set before the logical size, it is recomputed for the new space.
	s2 := newOffscreenScreen(t, 400, 200)
	if err := s2.SetDisplayTransform(-40, -20, 1); err != nil {
		t.Fatal(err)
	}
	if err := s2.SetLogicalSize(100, 100); err != nil {
		t.Fatal(err)
	}
	drawWhite(t, s2, sq)
	white, _ = readBoxes(t, s2)
	wantBox(t, "shift set before SetLogicalSize", white, box{150, 110, 169, 129})
}

func TestDisplayTransformLeavesThePhotodiodeSquare(t *testing.T) {
	for _, logical := range []bool{false, true} {
		s := newOffscreenScreen(t, 400, 200)
		if logical {
			if err := s.SetLogicalSize(100, 100); err != nil {
				t.Fatal(err)
			}
		}
		s.marker = NewMarker(MarkerOptions{Photodiode: true, Size: 30})
		if err := s.SetDisplayTransform(-50, 20, 0.8); err != nil {
			t.Fatal(err)
		}
		if err := s.Clear(); err != nil {
			t.Fatal(err)
		}
		if err := s.drawMarker(); err != nil {
			t.Fatal(err)
		}
		_, black := readBoxes(t, s)
		wantBox(t, "photodiode square", black, box{0, 0, 29, 29})

		// MarkerRect is the square in the drawing space: screen point Q is
		// at drawing point c + (q − c − d)/m, q being Q in drawing units.
		sc, offX := float32(1), float32(0)
		if logical {
			sc, offX = 2, 100 // 100×100 letterboxed in 400×200
		}
		cx, cy := float32(s.Width)/2, float32(s.Height)/2
		dx, dy := -50/sc, -20/sc
		toDrawing := func(X, Y float32) (float32, float32) {
			qx, qy := (X-offX)/sc, Y/sc
			return cx + (qx-cx-dx)/0.8, cy + (qy-cy-dy)/0.8
		}
		x0, y0 := toDrawing(0, 0)
		x1, y1 := toDrawing(30, 30)
		r := s.MarkerRect()
		if math.Abs(float64(r.X-x0)) > 1 || math.Abs(float64(r.Y-y0)) > 1 ||
			math.Abs(float64(r.X+r.W-x1)) > 1 || math.Abs(float64(r.Y+r.H-y1)) > 1 {
			t.Errorf("logical=%v: MarkerRect = %+v, want (%v,%v)-(%v,%v)", logical, r, x0, y0, x1, y1)
		}
		// Without a letterbox the square is on the image: filled there under
		// the transform, it covers the square on the screen.
		if !logical {
			drawWhite(t, s, r)
			white, _ := readBoxes(t, s)
			wantBox(t, "MarkerRect filled under the transform", white, box{0, 0, 29, 29})
		}
	}
}

func TestDisplayTransformKeepsTheRenderTarget(t *testing.T) {
	s := newOffscreenScreen(t, 200, 100)
	tex, err := s.Renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_TARGET, 50, 50)
	if err != nil {
		t.Fatal(err)
	}
	defer tex.Destroy()
	if err := s.Renderer.SetRenderTarget(tex); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDisplayTransform(-30, 0, 1); err != nil {
		t.Fatal(err)
	}
	if s.Renderer.RenderTarget() != tex {
		t.Fatal("SetDisplayTransform changed the render target")
	}
	if err := s.Renderer.SetRenderTarget(nil); err != nil {
		t.Fatal(err)
	}
	// The window got the transform although a texture was the target.
	sq := *s.CenteredRect(sdl.FPoint{}, 20, 20)
	drawWhite(t, s, sq)
	white, _ := readBoxes(t, s)
	wantBox(t, "transform set while a texture was the target", white, box{60, 40, 79, 59})
}

// TestMain loads the SDL library bundled in go-sdl3 once for the package's
// tests; the offscreen tests skip when it cannot run.
func TestMain(m *testing.M) {
	lib := binsdl.Load()
	code := m.Run()
	lib.Unload()
	os.Exit(code)
}

// TestVisibleRect checks the part of the drawing space that stays on screen:
// a shift to the left pushes the left edge off, a magnification the edges.
func TestVisibleRect(t *testing.T) {
	s := newOffscreenScreen(t, 200, 100)
	if r := s.VisibleRect(); r != (sdl.FRect{W: 200, H: 100}) {
		t.Errorf("no transform: %+v, want the whole drawing space", r)
	}
	if err := s.SetDisplayTransform(-80, 0, 1.05); err != nil {
		t.Fatal(err)
	}
	// Drawing point p lands at c + m(p - c) + d: the screen's left edge is
	// p = 100 + (0 - 100 + 80)/1.05 ≈ 81 (the viewport is integer), its top
	// and bottom edges 50 ∓ 50/1.05 ≈ 2.4 and 97.6. On the right the screen
	// edge is beyond the drawing space, whose own edge (200) clips.
	r := s.VisibleRect()
	near := func(got, want float32) bool { return math.Abs(float64(got-want)) <= 1.1 }
	if !near(r.X, 81) || !near(r.Y, 2.4) || !near(r.X+r.W, 200) || !near(r.Y+r.H, 97.6) {
		t.Errorf("shift -80, magnification 1.05: %+v, want about x 81-200, y 2.4-97.6", r)
	}
}
