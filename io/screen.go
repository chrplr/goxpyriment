// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Package io provides input/output handling for screen, keyboard, mouse, and other devices.
package io

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
)

// Re-export SDL types for convenience and to avoid direct SDL dependencies in user code.
type FRect = sdl.FRect
type FPoint = sdl.FPoint
type Color = sdl.Color

// Rendering type aliases — re-exported so callers need not import go-sdl3 directly.
type Texture = sdl.Texture
type Surface = sdl.Surface
type PixelFormat = sdl.PixelFormat
type TextureAccess = sdl.TextureAccess
type BlendMode = sdl.BlendMode

// Common pixel format / texture access / blend mode constants.
const (
	PIXELFORMAT_RGBA32      PixelFormat   = sdl.PIXELFORMAT_RGBA32
	TEXTUREACCESS_STREAMING TextureAccess = sdl.TEXTUREACCESS_STREAMING
	BLENDMODE_BLEND         BlendMode     = sdl.BLENDMODE_BLEND
)

// CreateSurfaceFrom allocates a Surface backed by existing pixel data.
// This wraps sdl.CreateSurfaceFrom so callers can avoid importing go-sdl3 directly.
func CreateSurfaceFrom(width, height int, format PixelFormat, pixels []byte, pitch int) (*Surface, error) {
	return sdl.CreateSurfaceFrom(width, height, format, pixels, pitch)
}

// Screen wraps the SDL window and hardware‑accelerated renderer.
// It is responsible for:
//   - managing the backbuffer / presenting frames (Clear, Update, Flip),
//   - tracking the logical coordinate system and conversion from centered
//     coordinates to SDL's top‑left space (CenterToSDL),
//   - holding the default font and optional canvas/logical size overrides.
type Screen struct {
	Window       *sdl.Window
	Renderer     *sdl.Renderer
	BgColor      sdl.Color
	Width        int
	Height       int
	DefaultFont  *ttf.Font
	CanvasOffset *sdl.FPoint // If not nil, use this instead of true center
	LogicalSize  *sdl.FPoint // If not nil, use this for CenterToSDL
}

// NewScreen initializes a new SDL window and renderer.
//
// width and height specify the logical experiment resolution. When fullscreen
// is true, or when width/height are 0, the physical window is created at the
// desktop's native resolution in exclusive fullscreen and the renderer is
// configured with a logical size matching the requested resolution (if > 0).
func NewScreen(title string, width, height int, bgColor sdl.Color, fullscreen bool) (*Screen, error) {
	// Exclusive fullscreen path: mimic tests/set_fullscreen/go_example/main.go.
	// When fullscreen is requested OR no explicit size is provided (0x0),
	// we ask SDL to create a fullscreen window at the native resolution
	// and then create a renderer for it.
	if fullscreen || (width == 0 && height == 0) {
		window, err := sdl.CreateWindow(
			title,
			0, 0,
			sdl.WINDOW_HIGH_PIXEL_DENSITY|sdl.WINDOW_FULLSCREEN,
		)
		if err != nil {
			return nil, err
		}

		renderer, err := window.CreateRenderer("")
		if err != nil {
			window.Destroy()
			return nil, err
		}

		// Query the actual physical pixel dimensions.
		w, h, err := window.SizeInPixels()
		if err != nil {
			w, h = 0, 0
		}

		return &Screen{
			Window:   window,
			Renderer: renderer,
			BgColor:  bgColor,
			Width:    int(w),
			Height:   int(h),
		}, nil
	}

	// Windowed path: create a hidden window+renderer pair and show it.
	window, renderer, err := sdl.CreateWindowAndRenderer(title, width, height, sdl.WINDOW_HIDDEN)
	if err != nil {
		return nil, err
	}

	s := &Screen{
		Window:   window,
		Renderer: renderer,
		BgColor:  bgColor,
		Width:    width,
		Height:   height,
	}

	if err := window.Show(); err != nil {
		window.Destroy()
		return nil, err
	}

	return s, nil
}

// CenterToSDL converts center‑based coordinates to SDL top‑left based
// coordinates using either the current logical size, canvas offset, or the
// renderer output size as a fallback.
func (s *Screen) CenterToSDL(x, y float32) (float32, float32) {
	if s.CanvasOffset != nil {
		return s.CanvasOffset.X + x, s.CanvasOffset.Y - y
	}
	if s.LogicalSize != nil {
		return s.LogicalSize.X/2 + x, s.LogicalSize.Y/2 - y
	}
	w, h, _ := s.Renderer.RenderOutputSize()
	return float32(w)/2 + x, float32(h)/2 - y
}

// LogicalCenterToSDL converts center-based coordinates to SDL top-left based coordinates using specified dimensions.
func (s *Screen) LogicalCenterToSDL(x, y float32, width, height float32) (float32, float32) {
	return width/2 + x, height/2 - y
}

// SetLogicalSize sets a device‑independent logical resolution for the
// renderer. All subsequent drawing operations are scaled to this size using
// SDL's logical presentation (letterboxed by default).
func (s *Screen) SetLogicalSize(width, height int32) error {
	s.LogicalSize = &sdl.FPoint{X: float32(width), Y: float32(height)}
	return s.Renderer.SetLogicalPresentation(width, height, sdl.LOGICAL_PRESENTATION_LETTERBOX)
}

// Size returns the current renderer output size.
func (s *Screen) Size() (int32, int32, error) {
	w, h, err := s.Renderer.RenderOutputSize()
	return w, h, err
}

// Clear clears the screen with the background color.
func (s *Screen) Clear() error {
	if err := s.Renderer.SetDrawColor(s.BgColor.R, s.BgColor.G, s.BgColor.B, s.BgColor.A); err != nil {
		return err
	}
	return s.Renderer.Clear()
}

// ClearAndUpdate clears the screen with the background color and presents the buffer.
// It is a convenience for the common pattern Clear() then Update().
func (s *Screen) ClearAndUpdate() error {
	if err := s.Clear(); err != nil {
		return err
	}
	return s.Update()
}

// Update presents the rendered buffer.
func (s *Screen) Update() error {
	// Ensure we are presenting the window, not a texture
	if s.Renderer.RenderTarget() != nil {
		if err := s.Renderer.SetRenderTarget(nil); err != nil {
			return err
		}
	}
	return s.Renderer.Present()
}

// Flip is an alias for Update and presents the backbuffer to the display.
// When VSync is enabled on the renderer, this call will typically block
// until the next vertical retrace, providing a well-defined stimulus onset.
func (s *Screen) Flip() error {
	return s.Update()
}

// SetVSync toggles vertical synchronization.
// vsync: 1 to enable, 0 to disable, -1 for adaptive vsync.
func (s *Screen) SetVSync(vsync int) error {
	return s.Renderer.SetVSync(int32(vsync))
}

// VSync returns the current VSync state.
func (s *Screen) VSync() (int, error) {
	v, err := s.Renderer.VSync()
	return int(v), err
}

// Destroy cleans up the window and renderer.
func (s *Screen) Destroy() {
	if s.Renderer != nil {
		s.Renderer.Destroy()
	}
	if s.Window != nil {
		s.Window.Destroy()
	}
}
