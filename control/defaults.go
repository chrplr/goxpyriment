// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package control manages the overall state and initialization of an experiment.
package control

import (
	"errors"
	"fmt"
	"strings"

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

// Key codes (re-exported from SDL for convenience). The full alphabet, digit
// row, keypad, and common navigation/punctuation keys are re-exported so
// experiment code never needs to import go-sdl3 just to name a key.
const (
	// Navigation and control keys.
	K_SPACE     = sdl.K_SPACE
	K_ESCAPE    = sdl.K_ESCAPE
	K_RETURN    = sdl.K_RETURN
	K_BACKSPACE = sdl.K_BACKSPACE
	K_TAB       = sdl.K_TAB
	K_UP        = sdl.K_UP
	K_DOWN      = sdl.K_DOWN
	K_LEFT      = sdl.K_LEFT
	K_RIGHT     = sdl.K_RIGHT
	K_HOME      = sdl.K_HOME
	K_END       = sdl.K_END
	K_DELETE    = sdl.K_DELETE

	// Letters A–Z.
	K_A = sdl.K_A
	K_B = sdl.K_B
	K_C = sdl.K_C
	K_D = sdl.K_D
	K_E = sdl.K_E
	K_F = sdl.K_F
	K_G = sdl.K_G
	K_H = sdl.K_H
	K_I = sdl.K_I
	K_J = sdl.K_J
	K_K = sdl.K_K
	K_L = sdl.K_L
	K_M = sdl.K_M
	K_N = sdl.K_N
	K_O = sdl.K_O
	K_P = sdl.K_P
	K_Q = sdl.K_Q
	K_R = sdl.K_R
	K_S = sdl.K_S
	K_T = sdl.K_T
	K_U = sdl.K_U
	K_V = sdl.K_V
	K_W = sdl.K_W
	K_X = sdl.K_X
	K_Y = sdl.K_Y
	K_Z = sdl.K_Z

	// Digit row 0–9.
	K_0 = sdl.K_0
	K_1 = sdl.K_1
	K_2 = sdl.K_2
	K_3 = sdl.K_3
	K_4 = sdl.K_4
	K_5 = sdl.K_5
	K_6 = sdl.K_6
	K_7 = sdl.K_7
	K_8 = sdl.K_8
	K_9 = sdl.K_9

	// Numeric keypad.
	K_KP_0     = sdl.K_KP_0
	K_KP_1     = sdl.K_KP_1
	K_KP_2     = sdl.K_KP_2
	K_KP_3     = sdl.K_KP_3
	K_KP_4     = sdl.K_KP_4
	K_KP_5     = sdl.K_KP_5
	K_KP_6     = sdl.K_KP_6
	K_KP_7     = sdl.K_KP_7
	K_KP_8     = sdl.K_KP_8
	K_KP_9     = sdl.K_KP_9
	K_KP_ENTER = sdl.K_KP_ENTER
	K_KP_PLUS  = sdl.K_KP_PLUS
	K_KP_MINUS = sdl.K_KP_MINUS

	// Punctuation.
	K_MINUS        = sdl.K_MINUS
	K_PLUS         = sdl.K_PLUS
	K_EQUALS       = sdl.K_EQUALS
	K_LEFTBRACKET  = sdl.K_LEFTBRACKET
	K_RIGHTBRACKET = sdl.K_RIGHTBRACKET
)

// Mouse button constants (re-exported from SDL).
const (
	BUTTON_LEFT  = uint32(sdl.BUTTON_LEFT)
	BUTTON_RIGHT = uint32(sdl.BUTTON_RIGHT)
)

// Event queue primitives (re-exported from SDL).
//
// These let experiment code hand-roll a custom input loop — most notably a
// text-input/typing loop that needs per-keystroke hardware timestamps and a
// blinking cursor — without importing go-sdl3. Start and stop IME text input
// with exp.Screen.Window.StartTextInput() / StopTextInput(); drain the queue
// with PollEvent; read events via Event.KeyboardEvent() and
// Event.TextInputEvent(); and time onsets/blinks with TicksNS(). See
// examples/Typing-Speed for a complete worked loop.

// Event re-exports the SDL event union so callers can declare `var ev control.Event`.
type Event = sdl.Event

// EventType re-exports the SDL event-type enum (the type of Event.Type).
type EventType = sdl.EventType

// KeyboardEvent re-exports the SDL keyboard event returned by Event.KeyboardEvent().
// Its Key, Timestamp (nanoseconds), Repeat and Down fields are the ones typically read.
type KeyboardEvent = sdl.KeyboardEvent

// TextInputEvent re-exports the SDL text-input event returned by Event.TextInputEvent().
// Its Text (UTF-8) and Timestamp (nanoseconds) fields carry the composed input.
type TextInputEvent = sdl.TextInputEvent

// Event types (re-exported from SDL) needed to classify polled events in a
// hand-rolled loop: window close, key press/release, and IME text input.
const (
	EVENT_QUIT       = sdl.EVENT_QUIT
	EVENT_KEY_DOWN   = sdl.EVENT_KEY_DOWN
	EVENT_KEY_UP     = sdl.EVENT_KEY_UP
	EVENT_TEXT_INPUT = sdl.EVENT_TEXT_INPUT
)

// PollEvent dequeues the next pending event into *event, returning false when
// the queue is empty. Mirrors sdl.PollEvent so experiment code can drain the
// event queue without importing go-sdl3.
func PollEvent(event *Event) bool {
	return sdl.PollEvent(event)
}

// PumpEvents services the SDL backend without dequeuing anything: it collects
// events from the input devices and, on the Wayland and X11 video drivers,
// dispatches the pending protocol traffic that SDL needs to keep the window in
// sync with the compositor. Under a compositor, a per-frame loop that only
// calls Clear + Update never reaches SDL_PumpEvents, and the frames a client
// commits can stop matching what the compositor scans out even though the flip
// timing looks correct (the EGL frame callback still throttles to the refresh
// rate). Call it once per flip in any VSYNC-locked loop that does not otherwise
// poll; it does not consume events, so a less frequent PollEvents still sees
// ESC and quit.
//
// The kmsdrm backend has the same requirement for a different reason — see the
// warm-up loop in apparatus.NewScreen.
func PumpEvents() {
	sdl.PumpEvents()
}

// TicksNS returns the SDL high-resolution clock in nanoseconds — the same
// reference frame as event timestamps (KeyboardEvent.Timestamp,
// TextInputEvent.Timestamp) and Screen.FlipTS(). Use it for stimulus-onset
// references and cursor-blink timing in custom loops.
func TicksNS() uint64 {
	return sdl.TicksNS()
}

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
		return nil, fmt.Errorf("control.FontFromMemory: %w", err)
	}
	return ttf.OpenFontIO(ioStream, true, size)
}

// FontFromFile opens a TTF font from a file path at the given point size.
func FontFromFile(path string, size float32) (*ttf.Font, error) {
	return ttf.OpenFont(path, size)
}

// DisplayInfo is re-exported from io so callers need not import that package.
type DisplayInfo = apparatus.DisplayInfo

// PacingStats is re-exported so callers need not import apparatus. It is what
// exp.Screen.PacingStats() returns: how many presents the driver blocked on its
// own versus how many Update had to hold to the frame boundary itself.
type PacingStats = apparatus.PacingStats

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

// FullscreenPolicy selects how a fullscreen window is presented: exclusive
// (a concrete display mode, taking the compositor out of the presentation
// path where the platform allows it) or fullscreen-desktop (borderless).
type FullscreenPolicy = apparatus.FullscreenPolicy

const (
	FullscreenAuto      = apparatus.FullscreenAuto
	FullscreenExclusive = apparatus.FullscreenExclusive
	FullscreenDesktop   = apparatus.FullscreenDesktop
)

// SetFullscreenPolicy overrides the automatic per-platform choice. Call it
// before Initialize() (NewExperimentFromFlags does this for you from
// -exclusive-fullscreen). The resolved value is recorded in every data file as
// "sys fullscreen_mode", because exclusive and desktop results are not
// comparable.
func SetFullscreenPolicy(p FullscreenPolicy) { apparatus.SetFullscreenPolicy(p) }

// ParseFullscreenPolicy maps the -exclusive-fullscreen values auto|on|off onto
// a FullscreenPolicy. Unrecognised input returns an error rather than silently
// defaulting, so a typo cannot quietly change what a session measures.
func ParseFullscreenPolicy(s string) (FullscreenPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return FullscreenAuto, nil
	case "on", "true", "1", "exclusive":
		return FullscreenExclusive, nil
	case "off", "false", "0", "desktop":
		return FullscreenDesktop, nil
	}
	return FullscreenAuto, fmt.Errorf("invalid fullscreen policy %q (want auto, on, or off)", s)
}
