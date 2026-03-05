// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package io

import (
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
func (m *Mouse) SetPosition(x, y float32) error {
	return nil // Placeholder
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
func (m *Mouse) Check() (uint32, error) {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		if event.Type == sdl.EVENT_MOUSE_BUTTON_DOWN {
			return uint32(event.MouseButtonEvent().Button), nil
		}
		if event.Type == sdl.EVENT_QUIT {
			return 0, sdl.EndLoop
		}
		// Note: Keyboard events might be here too, but we are draining the queue.
		// If we want both, we need a unified Event handler in Experiment.
	}
	return 0, nil
}
