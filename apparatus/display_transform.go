// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"fmt"
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
)

// SetDisplayTransform moves and scales everything drawn on the screen, for a
// display whose centre is not where the participant looks — a projector that
// cannot be moved, say. shiftX and shiftY are in screen pixels, the units of
// the photodiode square, with +Y up as everywhere else in goxpyriment;
// magnification scales the image about the centre of the screen before the
// shift. A drawing-space point p lands at c + m·(p − c) + d.
//
// It is SDL's own view transform (render viewport and scale) on the window, so
// it applies to every draw call, stimuli and raw Renderer calls alike, and to
// nothing else: the coordinate space (Width, Height, CenterToSDL) is
// unchanged, and so is layout code. Clear still paints the whole window, the
// strip the image uncovers included. The photodiode square stays at the corner
// of the screen. MousePosition and MarkerRect account for the transform.
// Texture render targets have views of their own and are not transformed until
// they are drawn on the window.
//
// The viewport is an integer rectangle, so the shift is rounded to the nearest
// drawing pixel (scaled by magnification). (0, 0, 1) removes the transform.
func (s *Screen) SetDisplayTransform(shiftX, shiftY, magnification float32) error {
	if !(magnification > 0) || math.IsInf(float64(magnification), 0) {
		return fmt.Errorf("apparatus.Screen.SetDisplayTransform: magnification %v must be a positive number", magnification)
	}
	if math.IsNaN(float64(shiftX)) || math.IsNaN(float64(shiftY)) ||
		math.IsInf(float64(shiftX), 0) || math.IsInf(float64(shiftY), 0) {
		return fmt.Errorf("apparatus.Screen.SetDisplayTransform: shift (%v, %v) must be finite", shiftX, shiftY)
	}
	s.shiftX, s.shiftY, s.mag = shiftX, shiftY, magnification
	return s.applyTransform()
}

// DisplayTransform returns what SetDisplayTransform set: (0, 0, 1) when it was
// never called.
func (s *Screen) DisplayTransform() (shiftX, shiftY, magnification float32) {
	return s.shiftX, s.shiftY, s.Magnification()
}

// Magnification is the factor SetDisplayTransform set, 1 by default. A drawing
// pixel covers that many screen pixels, which a size in degrees of visual angle
// computed from the screen's physical width must divide by.
func (s *Screen) Magnification() float32 {
	if s.mag == 0 {
		return 1
	}
	return s.mag
}

// transformed reports whether a display transform other than the identity is
// set.
func (s *Screen) transformed() bool {
	return s.shiftX != 0 || s.shiftY != 0 || s.Magnification() != 1
}

// displayView returns the render viewport that, with a render scale of m, puts
// the drawing-space point p (SDL top-left coordinates, w×h space) at
// c + m·(p − c) + (dx, dy), c being the centre and (dx, dy) in drawing units,
// SDL y-down.
//
// SDL maps p to m·(viewport origin + p) — measured with SDL 3.4.0: a rect at
// x=10 under viewport x=30 and scale 2 is drawn at x=80 — so the origin is the
// wanted offset divided by m. The viewport keeps the size of the drawing space,
// so what falls outside it is clipped as it is without a transform.
func displayView(w, h int, dx, dy, m float32) sdl.Rect {
	cx, cy := float32(w)/2, float32(h)/2
	return sdl.Rect{
		X: int32(math.Round(float64(((1-m)*cx + dx) / m))),
		Y: int32(math.Round(float64(((1-m)*cy + dy) / m))),
		W: int32(w),
		H: int32(h),
	}
}

// windowPerScreenPixel is the number of window coordinates in one screen
// pixel. The units of the photodiode square, and of the shift, are those of the
// presentation NewScreen set: window coordinates when it set one (fullscreen),
// output pixels when it did not (a window), which differ on a HiDPI display.
func (s *Screen) windowPerScreenPixel() float32 {
	if s.native.mode != sdl.LOGICAL_PRESENTATION_DISABLED {
		return 1
	}
	ow, _, oerr := s.Renderer.RenderOutputSize()
	ww, _, werr := s.Window.Size()
	if oerr != nil || werr != nil || ow <= 0 {
		return 1
	}
	return float32(ww) / float32(ow)
}

// applyTransform sets the window's view to the display transform. The view
// belongs to a render target, so the window is made the target for the time
// of the call. The Screen keeps the transform itself rather than reading the
// view back, because SDL_GetRenderViewport is a panic stub in the browser
// build.
func (s *Screen) applyTransform() (err error) {
	if prev := s.Renderer.RenderTarget(); prev != nil {
		if err := s.Renderer.SetRenderTarget(nil); err != nil {
			return fmt.Errorf("apparatus.Screen: selecting the window for the display transform: %w", err)
		}
		defer func() {
			if rerr := s.Renderer.SetRenderTarget(prev); rerr != nil && err == nil {
				err = fmt.Errorf("apparatus.Screen: restoring the render target: %w", rerr)
			}
		}()
	}
	if err := s.resetView(); err != nil {
		return err
	}
	if !s.transformed() {
		return nil
	}
	// Screen pixels to drawing units, measured on the untransformed view: 1 in
	// plain fullscreen, the inverse of the letterbox scale under SetLogicalSize.
	ws := 100 * s.windowPerScreenPixel()
	x0, y0, err0 := s.Renderer.RenderCoordinatesFromWindow(0, 0)
	x1, y1, err1 := s.Renderer.RenderCoordinatesFromWindow(ws, ws)
	if err0 != nil || err1 != nil {
		return fmt.Errorf("apparatus.Screen: converting the display shift to drawing units: %v", firstErr(err0, err1))
	}
	kx, ky := (x1-x0)/100, (y1-y0)/100
	m := s.Magnification()
	vp := displayView(s.Width, s.Height, kx*s.shiftX, -ky*s.shiftY, m)
	if err := s.Renderer.SetScale(m, m); err != nil {
		return fmt.Errorf("apparatus.Screen: setting the display magnification: %w", err)
	}
	if err := s.Renderer.SetViewport(&vp); err != nil {
		return fmt.Errorf("apparatus.Screen: setting the display shift: %w", err)
	}
	return nil
}

// resetView removes any view transform from the current render target.
func (s *Screen) resetView() error {
	if err := s.Renderer.SetViewport(nil); err != nil {
		return fmt.Errorf("apparatus.Screen: resetting the viewport: %w", err)
	}
	if err := s.Renderer.SetScale(1, 1); err != nil {
		return fmt.Errorf("apparatus.Screen: resetting the render scale: %w", err)
	}
	return nil
}

// WithoutDisplayTransform runs fn with the display transform lifted, for what
// is drawn at fixed screen positions rather than on the image — the photodiode
// square, or a fill that must cover the strip a shift uncovers — and puts the
// transform back after. Without a transform it just runs fn.
func (s *Screen) WithoutDisplayTransform(fn func() error) error {
	return s.withoutTransform(fn)
}

// withoutTransform is WithoutDisplayTransform. The reset acts on the current
// render target, which is the window inside Update.
func (s *Screen) withoutTransform(fn func() error) error {
	if !s.transformed() {
		return fn()
	}
	if err := s.resetView(); err != nil {
		return err
	}
	err := fn()
	if aerr := s.applyTransform(); aerr != nil && err == nil {
		err = aerr
	}
	return err
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
