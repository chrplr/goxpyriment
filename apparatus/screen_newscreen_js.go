// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

//go:build js

package apparatus

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
)

// ListDisplays returns an empty slice on WASM — display enumeration is not
// available in the browser context.
func ListDisplays() ([]DisplayInfo, error) {
	return nil, nil
}

// NewScreen creates an SDL window and renderer for the WASM/browser target.
//
// fullscreen and displayIndex are ignored in WASM; the window is always
// created as a 1024×768 windowed canvas (or the requested width×height).
// When width/height are 0, defaults of 1024×768 are used.
func NewScreen(title string, width, height int, bgColor sdl.Color, fullscreen bool, displayIndex int) (*Screen, error) {
	if width == 0 {
		width = 1024
	}
	if height == 0 {
		height = 768
	}

	window, renderer, err := sdl.CreateWindowAndRenderer(title, width, height, 0)
	if err != nil {
		return nil, fmt.Errorf("apparatus.NewScreen: creating window and renderer: %w", err)
	}

	logW, logH, _ := window.Size()
	if logW <= 0 {
		logW = int32(width)
	}
	if logH <= 0 {
		logH = int32(height)
	}

	s := &Screen{
		Window:      window,
		Renderer:    renderer,
		BgColor:     bgColor,
		Width:       int(logW),
		Height:      int(logH),
		LogicalSize: &sdl.FPoint{X: float32(logW), Y: float32(logH)},
	}

	return s, nil
}
