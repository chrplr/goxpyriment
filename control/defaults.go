// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Package control manages the overall state and initialization of an experiment.
package control

import "github.com/Zyko0/go-sdl3/sdl"

// Default Experiment settings
const (
	DefaultWindowWidth  = 800         // Default width of the experiment window in pixels.
	DefaultWindowHeight = 600         // Default height of the experiment window in pixels.
	DefaultWindowTitle  = "Goxpyriment" // Default title of the experiment window.
)

var (
	// DefaultBackgroundColor is the color used to clear the screen by default (Black).
	DefaultBackgroundColor = Black
	// DefaultTextColor is the color used for text stimuli if not specified (White).
	DefaultTextColor = White
)

// Common colors
var (
	Black   = sdl.Color{R: 0, G: 0, B: 0, A: 255}
	White   = sdl.Color{R: 255, G: 255, B: 255, A: 255}
	Red     = sdl.Color{R: 255, G: 0, B: 0, A: 255}
	Green   = sdl.Color{R: 0, G: 255, B: 0, A: 255}
	Blue    = sdl.Color{R: 0, G: 0, B: 255, A: 255}
	Yellow  = sdl.Color{R: 255, G: 255, B: 0, A: 255}
	Magenta = sdl.Color{R: 255, G: 0, B: 255, A: 255}
	Cyan    = sdl.Color{R: 0, G: 255, B: 255, A: 255}
	Gray    = sdl.Color{R: 128, G: 128, B: 128, A: 255}
)

