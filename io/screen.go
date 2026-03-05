// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Package io provides input/output handling for screen, keyboard, mouse, and other devices.
package io

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
)

// Screen represents the display window and the hardware-accelerated renderer.
// It manages the double-buffering scheme and global font settings.
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

// NewScreen initializes a new SDL window and renderer with specified dimensions, background color, and fullscreen mode.
func NewScreen(title string, width, height int, bgColor sdl.Color, fullscreen bool) (*Screen, error) {
	var flags sdl.WindowFlags
	if fullscreen {
		flags = sdl.WINDOW_FULLSCREEN
	}
	window, renderer, err := sdl.CreateWindowAndRenderer(title, width, height, flags)
	if err != nil {
		return nil, err
	}

	return &Screen{
		Window:   window,
		Renderer: renderer,
		BgColor:  bgColor,
		Width:    width,
		Height:   height,
	}, nil
}

// CenterToSDL converts center-based coordinates to SDL top-left based coordinates.
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

// SetLogicalSize sets a device-independent resolution for the renderer.
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
