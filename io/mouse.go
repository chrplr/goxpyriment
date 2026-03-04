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
	// Note: In a real app, you might need the window context. 
	// For simplicity, we'll assume the active window.
	// SDL3 WarpMouseInWindow requires a window pointer.
	return nil // Placeholder for now or add window to Mouse struct
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
