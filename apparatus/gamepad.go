// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"fmt"
	"time"

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
		return nil, fmt.Errorf("apparatus.GetGamePads: %w", err)
	}

	res := make([]*GamePad, 0, len(ids))
	for _, id := range ids {
		handle, err := id.OpenGamepad()
		if err != nil {
			continue
		}
		res = append(res, &GamePad{ID: id, Handle: handle})
	}
	return res, nil
}

// Axis returns the current value of a gamepad axis in the range −32768..32767.
//
// Unlike the raw Joystick API, the gamepad axes are standardized via SDL's
// controller mapping database: the left analog stick is always
// GAMEPAD_AXIS_LEFTX / LEFTY and the right stick GAMEPAD_AXIS_RIGHTX / RIGHTY,
// regardless of how the underlying device numbers its raw HID axes. The D-pad
// is reported as buttons (GAMEPAD_BUTTON_DPAD_*), not as these analog axes.
func (g *GamePad) Axis(axis sdl.GamepadAxis) int16 {
	return g.Handle.Axis(axis)
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

// GetPressEventTS blocks until a button is pressed on this gamepad and
// returns both the button and the SDL3 event timestamp in nanoseconds (same
// clock as Screen.FlipTS and Keyboard.GetKeyEventTS).
//
// Pass timeoutMS = -1 for no timeout. On timeout, returns (0, 0, nil).
// On quit, returns sdl.EndLoop.
func (g *GamePad) GetPressEventTS(timeoutMS int) (sdl.GamepadButton, uint64, error) {
	start := sdl.Ticks()
	for {
		if timeoutMS >= 0 {
			if int(sdl.Ticks()-start) >= timeoutMS {
				return 0, 0, nil
			}
		}
		var event sdl.Event
		for sdl.PollEvent(&event) {
			switch event.Type {
			case sdl.EVENT_GAMEPAD_BUTTON_DOWN:
				ge := event.GamepadButtonEvent()
				if ge.Which == g.ID {
					return sdl.GamepadButton(ge.Button), ge.Timestamp, nil
				}
			case sdl.EVENT_QUIT:
				return 0, 0, sdl.EndLoop
			}
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// Close closes the gamepad handle.
func (g *GamePad) Close() {
	if g.Handle != nil {
		g.Handle.Close()
		g.Handle = nil
	}
}
