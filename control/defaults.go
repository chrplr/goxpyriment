// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Package control manages the overall state and initialization of an experiment.
package control

import "github.com/Zyko0/go-sdl3/sdl"

// Default Experiment settings
const (
	DefaultWindowWidth  = 800         // Default width of the experiment window in pixels.
	DefaultWindowHeight = 600         // Default height of the experiment window in pixels.
	DefaultWindowTitle  = "Expyriment" // Default title of the experiment window.
)

var (
	// DefaultBackgroundColor is the color used to clear the screen by default (Black).
	DefaultBackgroundColor = sdl.Color{R: 0, G: 0, B: 0, A: 255}
	// DefaultTextColor is the color used for text stimuli if not specified (White).
	DefaultTextColor = sdl.Color{R: 255, G: 255, B: 255, A: 255}
)

