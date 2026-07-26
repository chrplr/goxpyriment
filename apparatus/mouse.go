// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"fmt"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Mouse provides methods for handling mouse input.
type Mouse struct {
	// PollButtons is injected by the control layer to avoid direct SDL polling
	// that discards non-mouse events. It returns (button, quitRequested).
	PollButtons func() (uint32, bool)

	// PollButtonsWithTS is like PollButtons but also returns the SDL3 event
	// timestamp (nanoseconds). Injected by the control layer; used by
	// GetPressEventTS.
	PollButtonsWithTS func() (uint32, uint64, bool)

	// PollButtonUps is like PollButtonsWithTS but returns the first
	// MOUSE_BUTTON_UP event seen in the current polling cycle. Injected by
	// the control layer; used by WaitButtonReleaseTS.
	PollButtonUps func() (uint32, uint64, bool)
}

// ShowCursor shows or hides the mouse cursor.
func (m *Mouse) ShowCursor(show bool) error {
	if show {
		return sdl.ShowCursor()
	}
	return sdl.HideCursor()
}

// Position returns the current (x, y) coordinates of the mouse.
func (m *Mouse) Position() (float32, float32) {
	_, x, y := sdl.GetMouseState()
	return x, y
}

// SetPosition moves the mouse cursor to the specified coordinates.
//
// TODO: not yet implemented — SDL3's WarpMouseInWindow requires a window
// reference. This method currently returns an error.
func (m *Mouse) SetPosition(x, y float32) error {
	return fmt.Errorf("Mouse.SetPosition: not yet implemented (requires SDL window reference)")
}

// WaitPress blocks until a mouse button is pressed.
func (m *Mouse) WaitPress() (uint32, error) {
	if m.PollButtons != nil {
		for {
			btn, quit := m.PollButtons()
			if quit {
				return 0, sdl.EndLoop
			}
			if btn != 0 {
				return btn, nil
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	for {
		var event sdl.Event
		if sdl.WaitEvent(&event) == nil {
			if event.Type == sdl.EVENT_MOUSE_BUTTON_DOWN {
				return uint32(event.MouseButtonEvent().Button), nil
			}
			if event.Type == sdl.EVENT_QUIT {
				return 0, sdl.EndLoop
			}
		}
	}
}

// WaitPressRT blocks until a mouse button is pressed (or a timeout occurs)
// and returns the button index and the reaction time in milliseconds measured
// from the moment WaitPressRT was called. This mirrors Keyboard.WaitKeysRT.
//
// Pass timeoutMS = -1 for no timeout. On timeout, returns (0, 0, nil).
// On quit, returns sdl.EndLoop.
func (m *Mouse) WaitPressRT(timeoutMS int) (uint32, int64, error) {
	start := sdl.Ticks()

	if m.PollButtons != nil {
		for {
			if timeoutMS >= 0 {
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, 0, nil
				}
			}
			btn, quit := m.PollButtons()
			if quit {
				return 0, 0, sdl.EndLoop
			}
			if btn != 0 {
				return btn, int64(sdl.Ticks() - start), nil
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Fallback: direct SDL event polling.
	for {
		var event sdl.Event
		if timeoutMS < 0 {
			if sdl.WaitEvent(&event) == nil {
				if event.Type == sdl.EVENT_MOUSE_BUTTON_DOWN {
					return uint32(event.MouseButtonEvent().Button), int64(sdl.Ticks() - start), nil
				}
				if event.Type == sdl.EVENT_QUIT {
					return 0, 0, sdl.EndLoop
				}
			}
		} else {
			elapsed := int(sdl.Ticks() - start)
			remaining := timeoutMS - elapsed
			if remaining <= 0 {
				return 0, 0, nil
			}
			if sdl.WaitEventTimeout(&event, int32(remaining)) {
				if event.Type == sdl.EVENT_MOUSE_BUTTON_DOWN {
					return uint32(event.MouseButtonEvent().Button), int64(sdl.Ticks() - start), nil
				}
				if event.Type == sdl.EVENT_QUIT {
					return 0, 0, sdl.EndLoop
				}
			} else if int(sdl.Ticks()-start) >= timeoutMS {
				return 0, 0, nil
			}
		}
	}
}

// GetPressEventTS blocks until a mouse button is pressed and returns both
// the button index and the SDL3 event timestamp in nanoseconds (same reference
// clock as sdl.TicksNS() and Screen.FlipTS()).
//
// Pass timeoutMS = -1 for no timeout. On timeout, returns (0, 0, nil).
// On quit, returns sdl.EndLoop.
func (m *Mouse) GetPressEventTS(timeoutMS int) (uint32, uint64, error) {
	start := sdl.Ticks()

	if m.PollButtonsWithTS != nil {
		for {
			if timeoutMS >= 0 {
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, 0, nil
				}
			}
			btn, ts, quit := m.PollButtonsWithTS()
			if quit {
				return 0, 0, sdl.EndLoop
			}
			if btn != 0 {
				return btn, ts, nil
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Fallback: direct SDL event polling.
	for {
		var event sdl.Event
		if timeoutMS < 0 {
			if sdl.WaitEvent(&event) == nil {
				if event.Type == sdl.EVENT_MOUSE_BUTTON_DOWN {
					me := event.MouseButtonEvent()
					return uint32(me.Button), me.Timestamp, nil
				}
				if event.Type == sdl.EVENT_QUIT {
					return 0, 0, sdl.EndLoop
				}
			}
		} else {
			elapsed := int(sdl.Ticks() - start)
			remaining := timeoutMS - elapsed
			if remaining <= 0 {
				return 0, 0, nil
			}
			if sdl.WaitEventTimeout(&event, int32(remaining)) {
				if event.Type == sdl.EVENT_MOUSE_BUTTON_DOWN {
					me := event.MouseButtonEvent()
					return uint32(me.Button), me.Timestamp, nil
				}
				if event.Type == sdl.EVENT_QUIT {
					return 0, 0, sdl.EndLoop
				}
			} else if int(sdl.Ticks()-start) >= timeoutMS {
				return 0, 0, nil
			}
		}
	}
}

// Check polls for mouse button events without blocking.
func (m *Mouse) Check() (uint32, error) {
	if m.PollButtons != nil {
		btn, quit := m.PollButtons()
		if quit {
			return 0, sdl.EndLoop
		}
		return btn, nil
	}

	var event sdl.Event
	for sdl.PollEvent(&event) {
		if event.Type == sdl.EVENT_MOUSE_BUTTON_DOWN {
			return uint32(event.MouseButtonEvent().Button), nil
		}
		if event.Type == sdl.EVENT_QUIT {
			return 0, sdl.EndLoop
		}
	}
	return 0, nil
}

// IsPressed reports whether the given mouse button is physically held down at
// the moment of the call. It uses sdl.GetMouseState which returns a bitmask of
// currently-pressed buttons — no event queue involvement.
//
// button should be one of sdl.BUTTON_LEFT, sdl.BUTTON_MIDDLE, sdl.BUTTON_RIGHT,
// sdl.BUTTON_X1, or sdl.BUTTON_X2.
func (m *Mouse) IsPressed(button uint32) bool {
	flags, _, _ := sdl.GetMouseState()
	return flags&sdl.ButtonMask(sdl.MouseButtonFlags(button)) != 0
}

// waitSDLMouseUpEvent is the fallback SDL event loop for WaitButtonReleaseTS
// when no injected PollButtonUps callback is available. It blocks until the
// specified button generates a MOUSE_BUTTON_UP event, ESC/quit, or timeout.
//
// Return values:
//   - (ts, nil)     — MOUSE_BUTTON_UP for button; ts is its hardware timestamp
//   - (0, EndLoop)  — ESC KEY_DOWN or window-close quit event
//   - (0, nil)      — timeout
func waitSDLMouseUpEvent(button uint32, start uint64, timeoutMS int) (uint64, error) {
	for {
		var event sdl.Event
		var hasEvent bool

		if timeoutMS < 0 {
			if sdl.WaitEvent(&event) == nil {
				hasEvent = true
			}
		} else {
			elapsed := int(sdl.Ticks() - start)
			remaining := timeoutMS - elapsed
			if remaining <= 0 {
				return 0, nil
			}
			if sdl.WaitEventTimeout(&event, int32(remaining)) {
				hasEvent = true
			} else {
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, nil
				}
				continue
			}
		}

		if !hasEvent {
			continue
		}

		switch event.Type {
		case sdl.EVENT_QUIT:
			return 0, sdl.EndLoop
		case sdl.EVENT_KEY_DOWN:
			if event.KeyboardEvent().Key == sdl.K_ESCAPE {
				return 0, sdl.EndLoop
			}
		case sdl.EVENT_MOUSE_BUTTON_UP:
			me := event.MouseButtonEvent()
			if uint32(me.Button) == button {
				return me.Timestamp, nil
			}
		}
	}
}

// WaitButtonReleaseTS blocks until the given mouse button is released
// (MOUSE_BUTTON_UP event) and returns the SDL3 hardware event timestamp in
// nanoseconds.
//
// Combined with the button-down timestamp from GetPressEventTS, this gives
// nanosecond-precision press duration:
//
//	btn, downTS, _ := exp.Mouse.GetPressEventTS(-1)
//	upTS, _        := exp.Mouse.WaitButtonReleaseTS(btn, 5000)
//	durationNS     := upTS - downTS
//
// Pass timeoutMS = -1 for no timeout. On timeout returns (0, nil).
// On ESC or quit returns (0, sdl.EndLoop).
func (m *Mouse) WaitButtonReleaseTS(button uint32, timeoutMS int) (uint64, error) {
	start := sdl.Ticks()

	if m.PollButtonUps != nil {
		for {
			if timeoutMS >= 0 {
				if int(sdl.Ticks()-start) >= timeoutMS {
					return 0, nil
				}
			}
			btnUp, ts, quit := m.PollButtonUps()
			if quit {
				return 0, sdl.EndLoop
			}
			if btnUp == button {
				return ts, nil
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Fallback: direct SDL event polling when no callback is injected.
	return waitSDLMouseUpEvent(button, start, timeoutMS)
}
