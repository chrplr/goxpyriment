// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package apparatus

import (
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
		return nil, err
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

// WaitPressEventRT blocks until a button is pressed on this gamepad and
// returns both the button and the SDL3 event timestamp in nanoseconds (same
// clock as Screen.FlipNS and Keyboard.WaitKeysEventRT).
//
// Pass timeoutMS = -1 for no timeout. On timeout, returns (0, 0, nil).
// On quit, returns sdl.EndLoop.
func (g *GamePad) WaitPressEventRT(timeoutMS int) (sdl.GamepadButton, uint64, error) {
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
