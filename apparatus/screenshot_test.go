// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Capture reads the frame being drawn, in the window's pixels, with the
// letterbox of SetLogicalSize and the display transform as on the screen; and
// ScreenshotNextFrame writes the frame Update presents, once.
func TestCaptureAndScreenshot(t *testing.T) {
	s := newOffscreenScreen(t, 200, 100)
	drawWhite(t, s, sdl.FRect{X: 10, Y: 20, W: 30, H: 40})
	img, err := s.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("capture %v, want 200x100", b)
	}
	for _, p := range []struct {
		x, y  int
		white bool
	}{{10, 20, true}, {39, 59, true}, {9, 20, false}, {40, 20, false}, {150, 50, false}} {
		r, g, b, a := img.At(p.x, p.y).RGBA()
		gotWhite := r>>8 == 255 && g>>8 == 255 && b>>8 == 255
		if gotWhite != p.white || a>>8 != 255 {
			t.Errorf("pixel (%d,%d) = %d,%d,%d,%d; white %v", p.x, p.y, r>>8, g>>8, b>>8, a>>8, p.white)
		}
	}
	// The background is the grey of the screen.
	if r, _, _, _ := img.At(150, 50).RGBA(); r>>8 != 128 {
		t.Errorf("background %d, want 128", r>>8)
	}

	// The display transform moves the square in the capture.
	if err := s.SetDisplayTransform(20, 0, 1); err != nil {
		t.Fatal(err)
	}
	drawWhite(t, s, sdl.FRect{X: 10, Y: 20, W: 30, H: 40})
	if white, _ := readBoxes(t, s); white.x0 != 30 {
		t.Errorf("with a 20 px shift the square starts at x=%d, want 30", white.x0)
	}
	if err := s.SetDisplayTransform(0, 0, 1); err != nil {
		t.Fatal(err)
	}

	// ScreenshotNextFrame: the next Update writes that frame, and only it.
	path := filepath.Join(t.TempDir(), "frame.png")
	s.ScreenshotNextFrame(path)
	drawWhite(t, s, sdl.FRect{X: 10, Y: 20, W: 30, H: 40})
	want, err := s.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatalf("screenshot %v, want %v", got.Bounds(), want.Bounds())
	}
	for y := range want.Bounds().Dy() {
		for x := range want.Bounds().Dx() {
			if got.At(x, y) != want.At(x, y) {
				gr, gg, gb, _ := got.At(x, y).RGBA()
				wr, wg, wb, _ := want.At(x, y).RGBA()
				t.Fatalf("screenshot pixel (%d,%d) = %d,%d,%d; want %d,%d,%d", x, y, gr>>8, gg>>8, gb>>8, wr>>8, wg>>8, wb>>8)
			}
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearAndUpdate(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a second Update wrote the screenshot again (%v)", err)
	}

	// An error writing the file is returned by Update.
	s.ScreenshotNextFrame(filepath.Join(t.TempDir(), "no-such-dir", "frame.png"))
	if err := s.ClearAndUpdate(); err == nil {
		t.Error("unwritable path: no error from Update")
	}
}
