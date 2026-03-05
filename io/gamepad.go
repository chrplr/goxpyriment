// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package io

import (
	"github.com/Zyko0/go-sdl3/sdl"
)

// GamePad represents a game controller.
type GamePad struct {
	ID     sdl.JoystickID
	Handle *sdl.Gamepad
}

// GetGamePads returns a list of connected gamepads.
func GetGamePads() ([]*GamePad, error) {
	ids, err := sdl.GetGamepads()
	if err != nil {
		return nil, err
	}
	
	res := make([]*GamePad, len(ids))
	for i, id := range ids {
		handle, err := id.OpenGamepad()
		if err != nil {
			continue
		}
		res[i] = &GamePad{ID: id, Handle: handle}
	}
	return res, nil
}

// WaitPress blocks until a gamepad button is pressed.
func (g *GamePad) WaitPress() (sdl.GamepadButton, error) {
	for {
		var event sdl.Event
		if sdl.WaitEvent(&event) == nil {
			if event.Type == sdl.EVENT_GAMEPAD_BUTTON_DOWN {
				if event.GamepadButtonEvent().Which == g.ID {
					return sdl.GamepadButton(event.GamepadButtonEvent().Button), nil
				}
			}
			if event.Type == sdl.EVENT_QUIT {
				return 0, sdl.EndLoop
			}
		}
	}
}

// Close closes the gamepad handle.
func (g *GamePad) Close() {
	if g.Handle != nil {
		g.Handle.Close()
	}
}
