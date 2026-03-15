// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package io

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Mouse provides methods for handling mouse input.
type Mouse struct{}

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

// Check polls for mouse button events without blocking.
//
// WARNING: Because SDL uses a single shared event queue, calling Check drains
// ALL pending events — including keyboard events. If you need to process both
// keyboard and mouse input, use the unified Experiment.PollEvents/HandleEvents
// methods instead.
func (m *Mouse) Check() (uint32, error) {
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
