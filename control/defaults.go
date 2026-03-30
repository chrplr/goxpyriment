// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// Package control manages the overall state and initialization of an experiment.
package control

import (
	"errors"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Default Experiment settings
const (
	DefaultWindowWidth  = 800           // Default width of the experiment window in pixels.
	DefaultWindowHeight = 600           // Default height of the experiment window in pixels.
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
	Black     = sdl.Color{R: 0, G: 0, B: 0, A: 255}
	White     = sdl.Color{R: 255, G: 255, B: 255, A: 255}
	Red       = sdl.Color{R: 255, G: 0, B: 0, A: 255}
	Green     = sdl.Color{R: 0, G: 255, B: 0, A: 255}
	Blue      = sdl.Color{R: 0, G: 0, B: 255, A: 255}
	Yellow    = sdl.Color{R: 255, G: 255, B: 0, A: 255}
	Magenta   = sdl.Color{R: 255, G: 0, B: 255, A: 255}
	Cyan      = sdl.Color{R: 0, G: 255, B: 255, A: 255}
	Gray      = sdl.Color{R: 128, G: 128, B: 128, A: 255}
	DarkGray  = sdl.Color{R: 50, G: 50, B: 50, A: 255}
	LightGray = sdl.Color{R: 200, G: 200, B: 200, A: 255}
)

// Keycode re-exports SDL key codes so callers can use control.K_SPACE etc. without importing go-sdl3.
type Keycode = sdl.Keycode

// Color re-exports SDL color struct so callers can use control.Color{R:..., G:..., B:..., A:...} without importing go-sdl3.
type Color = sdl.Color

// FPoint re-exports SDL FPoint struct so callers can use control.FPoint{X:..., Y:...} without importing go-sdl3.
type FPoint = sdl.FPoint

// FRect re-exports SDL FRect struct so callers can use control.FRect{X:..., Y:..., W:..., H:...} without importing go-sdl3.
type FRect = sdl.FRect

// Common key codes (re-exported from SDL for convenience).
const (
	K_SPACE     = sdl.K_SPACE
	K_ESCAPE    = sdl.K_ESCAPE
	K_RETURN    = sdl.K_RETURN
	K_BACKSPACE = sdl.K_BACKSPACE
	K_UP        = sdl.K_UP
	K_DOWN      = sdl.K_DOWN
	K_LEFT      = sdl.K_LEFT
	K_RIGHT     = sdl.K_RIGHT
	K_S         = sdl.K_S
	K_D         = sdl.K_D
	K_F         = sdl.K_F
	K_J         = sdl.K_J
	K_K         = sdl.K_K
	K_L         = sdl.K_L
	K_Q         = sdl.K_Q
	K_R         = sdl.K_R
	K_G         = sdl.K_G
	K_B         = sdl.K_B
	K_Y         = sdl.K_Y
	K_N         = sdl.K_N
	K_P         = sdl.K_P
	K_1         = sdl.K_1
	K_2         = sdl.K_2
	K_3         = sdl.K_3
	K_4         = sdl.K_4
	K_KP_1      = sdl.K_KP_1
	K_KP_2      = sdl.K_KP_2
	K_KP_3      = sdl.K_KP_3
	K_KP_4      = sdl.K_KP_4
)

// Mouse button constants (re-exported from SDL).
const (
	BUTTON_LEFT  = uint32(sdl.BUTTON_LEFT)
	BUTTON_RIGHT = uint32(sdl.BUTTON_RIGHT)
)

// Point returns an sdl.FPoint so callers can use control.Point(x,y) without importing go-sdl3.
func Point(x, y float32) sdl.FPoint {
	return sdl.FPoint{X: x, Y: y}
}

// Origin returns the center-origin point (0, 0).
func Origin() sdl.FPoint {
	return sdl.FPoint{X: 0, Y: 0}
}

// RGB returns an opaque sdl.Color with the given 0–255 components.
func RGB(r, g, b uint8) sdl.Color {
	return sdl.Color{R: r, G: g, B: b, A: 255}
}

// RGBA returns an sdl.Color with the given 0–255 components.
func RGBA(r, g, b, a uint8) sdl.Color {
	return sdl.Color{R: r, G: g, B: b, A: a}
}

// EndLoop is the sentinel error returned when the run loop should exit (e.g. ESC or window close).
// Re-exported from SDL so callers can return control.EndLoop from exp.Run(...) without importing go-sdl3.
var EndLoop = sdl.EndLoop

// IsEndLoop reports whether err is the sentinel used for graceful run-loop exit (ESC or window close).
// Use it to avoid importing go-sdl3 just to check: if err != nil && !control.IsEndLoop(err) { log.Fatal(err) }.
func IsEndLoop(err error) bool {
	return err != nil && errors.Is(err, sdl.EndLoop)
}

// FontFromMemory opens a TTF font from embedded bytes at the given point size.
// Use this instead of sdl.IOFromBytes + ttf.OpenFontIO to avoid a direct SDL dependency.
func FontFromMemory(data []byte, size float32) (*ttf.Font, error) {
	ioStream, err := sdl.IOFromBytes(data)
	if err != nil {
		return nil, err
	}
	return ttf.OpenFontIO(ioStream, true, size)
}

// FontFromFile opens a TTF font from a file path at the given point size.
func FontFromFile(path string, size float32) (*ttf.Font, error) {
	return ttf.OpenFont(path, size)
}

// DisplayInfo is re-exported from io so callers need not import that package.
type DisplayInfo = apparatus.DisplayInfo

// ListDisplays returns metadata for all connected displays, ordered so that
// index 0 is the primary display. Assign an index to exp.ScreenNumber before
// calling Initialize() (or before NewExperimentFromFlags) to open the
// experiment window on a specific monitor.
//
//	displays, _ := control.ListDisplays()
//	for i, d := range displays {
//	    fmt.Printf("display %d: %s (%dx%d)\n", i, d.Name, d.NativeW, d.NativeH)
//	}
//	exp.ScreenNumber = 1 // secondary monitor
func ListDisplays() ([]DisplayInfo, error) {
	return apparatus.ListDisplays()
}
