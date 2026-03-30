// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

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
	// WaitPressEventRT.
	PollButtonsWithTS func() (uint32, uint64, bool)
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

// WaitPressEventRT blocks until a mouse button is pressed and returns both
// the button index and the SDL3 event timestamp in nanoseconds (same reference
// clock as sdl.TicksNS() and Screen.FlipNS()).
//
// Pass timeoutMS = -1 for no timeout. On timeout, returns (0, 0, nil).
// On quit, returns sdl.EndLoop.
func (m *Mouse) WaitPressEventRT(timeoutMS int) (uint32, uint64, error) {
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
