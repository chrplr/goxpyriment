// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package apparatus

import (
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

// newLogicalScreen returns a Screen whose coordinate conversions use a fixed
// logical size, so CenterToSDL/CenteredRect can be tested without a live
// renderer.
func newLogicalScreen(w, h float32) *Screen {
	return &Screen{LogicalSize: &sdl.FPoint{X: w, Y: h}}
}

func TestCenterToSDL(t *testing.T) {
	s := newLogicalScreen(800, 600)
	for _, tc := range []struct {
		x, y, wantX, wantY float32
	}{
		{0, 0, 400, 300},      // center maps to the middle
		{100, 0, 500, 300},    // +x moves right
		{0, 100, 400, 200},    // +y moves up (SDL y is inverted)
		{-100, -50, 300, 350}, // negative quadrant
	} {
		gx, gy := s.CenterToSDL(tc.x, tc.y)
		if gx != tc.wantX || gy != tc.wantY {
			t.Errorf("CenterToSDL(%v,%v) = (%v,%v), want (%v,%v)", tc.x, tc.y, gx, gy, tc.wantX, tc.wantY)
		}
	}
}

func TestCenteredRect(t *testing.T) {
	s := newLogicalScreen(800, 600)
	r := s.CenteredRect(sdl.FPoint{X: 0, Y: 0}, 100, 50)
	want := sdl.FRect{X: 350, Y: 275, W: 100, H: 50}
	if *r != want {
		t.Errorf("CenteredRect = %+v, want %+v", *r, want)
	}

	// Off-center: the rect's center must land on CenterToSDL(pos).
	pos := sdl.FPoint{X: 120, Y: -40}
	r = s.CenteredRect(pos, 60, 80)
	cx, cy := s.CenterToSDL(pos.X, pos.Y)
	if gotCX := r.X + r.W/2; gotCX != cx {
		t.Errorf("rect center X = %v, want %v", gotCX, cx)
	}
	if gotCY := r.Y + r.H/2; gotCY != cy {
		t.Errorf("rect center Y = %v, want %v", gotCY, cy)
	}
}

func TestGammaCorrectorEndpointsAndIdentity(t *testing.T) {
	// gamma 1.0 is the identity transform.
	id := NewGammaCorrectorUniform(1.0)
	for _, v := range []uint8{0, 1, 64, 128, 200, 255} {
		got := id.CorrectColor(sdl.Color{R: v, G: v, B: v, A: 123})
		if got.R != v || got.G != v || got.B != v {
			t.Errorf("identity gamma changed %d -> (%d,%d,%d)", v, got.R, got.G, got.B)
		}
		if got.A != 123 {
			t.Errorf("alpha not passed through: got %d", got.A)
		}
	}

	// Endpoints are fixed for any gamma; 50%% maps to ≈186 at gamma 2.2.
	g := NewGammaCorrectorUniform(2.2)
	black := g.CorrectColor(sdl.Color{R: 0, G: 0, B: 0, A: 255})
	white := g.CorrectColor(sdl.Color{R: 255, G: 255, B: 255, A: 255})
	if black.R != 0 || white.R != 255 {
		t.Errorf("endpoints moved: black.R=%d white.R=%d", black.R, white.R)
	}
	mid := g.CorrectColor(sdl.Color{R: 128, G: 128, B: 128, A: 255})
	if mid.R < 184 || mid.R > 188 {
		t.Errorf("gamma 2.2 of 128 = %d, expected ≈186", mid.R)
	}
}
