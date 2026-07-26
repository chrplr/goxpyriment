// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"fmt"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Joystick represents a raw joystick device (axes, buttons, hats, balls).
// Unlike GamePad, no SDL controller mapping is applied — all axes and buttons
// are accessed by numeric index.
type Joystick struct {
	ID     sdl.JoystickID
	Handle *sdl.Joystick
}

// GetJoysticks returns a list of all connected joysticks.
func GetJoysticks() ([]*Joystick, error) {
	ids, err := sdl.GetJoysticks()
	if err != nil {
		return nil, fmt.Errorf("apparatus.GetJoysticks: %w", err)
	}

	res := make([]*Joystick, 0, len(ids))
	for _, id := range ids {
		handle, err := id.OpenJoystick()
		if err != nil {
			continue
		}
		res = append(res, &Joystick{ID: id, Handle: handle})
	}
	return res, nil
}

// NumAxes returns the number of axes on this joystick.
func (j *Joystick) NumAxes() (int32, error) {
	return j.Handle.NumAxes()
}

// NumButtons returns the number of buttons on this joystick.
func (j *Joystick) NumButtons() (int32, error) {
	return j.Handle.NumButtons()
}

// Axis returns the current value of the given axis in [-32768, 32767].
// axis 0 is typically the X axis, axis 1 the Y axis.
func (j *Joystick) Axis(axis int32) (int16, error) {
	return j.Handle.Axis(axis)
}

// WaitButtonPress blocks until any button on this joystick is pressed.
// Returns the button index (0-based) or sdl.EndLoop on quit.
func (j *Joystick) WaitButtonPress() (uint8, error) {
	for {
		var event sdl.Event
		if sdl.WaitEvent(&event) == nil {
			switch event.Type {
			case sdl.EVENT_JOYSTICK_BUTTON_DOWN:
				jbe := event.JoyButtonEvent()
				if jbe.Which == j.ID {
					return jbe.Button, nil
				}
			case sdl.EVENT_QUIT:
				return 0, sdl.EndLoop
			}
		}
	}
}

// GetButtonEventTS blocks until a button is pressed on this joystick and
// returns the button index and the SDL3 event timestamp in nanoseconds (same
// clock as Screen.FlipTS and Keyboard.GetKeyEventTS).
//
// Pass timeoutMS = -1 for no timeout. On timeout, returns (0, 0, nil).
// On quit, returns sdl.EndLoop.
func (j *Joystick) GetButtonEventTS(timeoutMS int) (uint8, uint64, error) {
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
			case sdl.EVENT_JOYSTICK_BUTTON_DOWN:
				jbe := event.JoyButtonEvent()
				if jbe.Which == j.ID {
					return jbe.Button, jbe.Timestamp, nil
				}
			case sdl.EVENT_QUIT:
				return 0, 0, sdl.EndLoop
			}
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// GetAxisEvent blocks until an axis-motion event arrives on this joystick and
// returns the axis index, value, and SDL3 event timestamp in nanoseconds.
//
// Pass timeoutMS = -1 for no timeout. On timeout, returns (0, 0, 0, nil).
// On quit, returns sdl.EndLoop.
func (j *Joystick) GetAxisEvent(timeoutMS int) (uint8, int16, uint64, error) {
	start := sdl.Ticks()
	for {
		if timeoutMS >= 0 {
			if int(sdl.Ticks()-start) >= timeoutMS {
				return 0, 0, 0, nil
			}
		}
		var event sdl.Event
		for sdl.PollEvent(&event) {
			switch event.Type {
			case sdl.EVENT_JOYSTICK_AXIS_MOTION:
				jae := event.JoyAxisEvent()
				if jae.Which == j.ID {
					return jae.Axis, jae.Value, jae.Timestamp, nil
				}
			case sdl.EVENT_QUIT:
				return 0, 0, 0, sdl.EndLoop
			}
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// Close closes the joystick handle.
func (j *Joystick) Close() {
	if j.Handle != nil {
		j.Handle.Close()
		j.Handle = nil
	}
}
