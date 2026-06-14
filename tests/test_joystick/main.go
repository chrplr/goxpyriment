// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// test_joystick demonstrates analog stick input: use the controller to move a
// red circle around the screen. Click on the circle (or press a controller
// button) to stop. ESC to quit.
//
// It prefers SDL's high-level *gamepad* API, which exposes a standardized
// analog left stick (GAMEPAD_AXIS_LEFTX/LEFTY) regardless of how the device
// numbers its raw HID axes — this gives smooth, any-angle motion. For devices
// SDL does not recognize as gamepads it falls back to the raw *joystick* API
// (axes 0/1), whose meaning is device-specific and may be a digital D-pad
// (only 8 directions). A live axis readout at the top of the screen shows
// which path is active and the current axis values.
package main

import (
	"fmt"
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	radius   float32 = 20
	maxSpeed float32 = 400 // pixels per second
	deadZone int16   = 2000
)

func clamp(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func main() {
	exp := control.NewExperimentFromFlags("Joystick Cursor", control.Black, control.White, 32)
	defer exp.End()

	// Prefer the gamepad API (standardized analog left stick); fall back to the
	// raw joystick API for devices SDL does not recognize as gamepads.
	var (
		readAxes func() (int16, int16)
		mode     string
	)

	if pads, err := apparatus.GetGamePads(); err == nil && len(pads) > 0 {
		pad := pads[0]
		defer pad.Close()
		mode = "Gamepad: left analog stick"
		readAxes = func() (int16, int16) {
			return pad.Axis(sdl.GAMEPAD_AXIS_LEFTX), pad.Axis(sdl.GAMEPAD_AXIS_LEFTY)
		}
	} else if joys, err := apparatus.GetJoysticks(); err == nil && len(joys) > 0 {
		joy := joys[0]
		defer joy.Close()
		mode = "Joystick: raw axes 0/1 (may be a digital D-pad)"
		readAxes = func() (int16, int16) {
			x, _ := joy.Axis(0)
			y, _ := joy.Axis(1)
			return x, y
		}
	} else {
		msg := stimuli.NewTextBox("No gamepad or joystick found. Connect one and restart.\n\nPress any key to quit.", 600, control.FPoint{}, control.White)
		exp.Show(msg)
		exp.Keyboard.Wait()
		return
	}

	w, h, _ := exp.Screen.Size()
	halfW := float32(w) / 2
	halfH := float32(h) / 2

	circle := stimuli.NewCircle(radius, control.Red)
	var pos sdl.FPoint // starts at screen center

	runErr := exp.Run(func() error {
		prevTick := sdl.Ticks()

		for {
			// Delta time in seconds
			now := sdl.Ticks()
			dt := float32(now-prevTick) / 1000.0
			prevTick = now

			// Read the active device's axes (raw values, before dead zone).
			rawX, rawY := readAxes()
			axisX, axisY := rawX, rawY
			if axisX > -deadZone && axisX < deadZone {
				axisX = 0
			}
			if axisY > -deadZone && axisY < deadZone {
				axisY = 0
			}

			// Update position (axis up = negative, so subtract for Y).
			pos.X += float32(axisX) / 32768.0 * maxSpeed * dt
			pos.Y -= float32(axisY) / 32768.0 * maxSpeed * dt
			pos.X = clamp(pos.X, -halfW+radius, halfW-radius)
			pos.Y = clamp(pos.Y, -halfH+radius, halfH-radius)
			circle.SetPosition(pos)

			// Poll events
			var event sdl.Event
			for sdl.PollEvent(&event) {
				switch event.Type {
				case sdl.EVENT_MOUSE_BUTTON_DOWN:
					// Hit-test: click inside the circle stops the demo
					mx, my := exp.Screen.MousePosition()
					dx := mx - pos.X
					dy := my - pos.Y
					if float32(math.Sqrt(float64(dx*dx+dy*dy))) <= radius {
						return control.EndLoop
					}
				case sdl.EVENT_GAMEPAD_BUTTON_DOWN, sdl.EVENT_JOYSTICK_BUTTON_DOWN, sdl.EVENT_QUIT:
					return control.EndLoop
				case sdl.EVENT_KEY_DOWN:
					if event.KeyboardEvent().Key == sdl.K_ESCAPE {
						return control.EndLoop
					}
				}
			}

			// Live readout: mode + current axis values + resulting direction.
			angle := math.Atan2(float64(-rawY), float64(rawX)) * 180 / math.Pi
			readout := stimuli.NewTextLine(
				fmt.Sprintf("%s    X:%6d  Y:%6d    dir:%6.1f deg", mode, rawX, rawY, angle),
				0, -halfH+24, control.White)

			// Draw circle (clearing first) then overlay the readout, then flip.
			if err := circle.Present(exp.Screen, true, false); err != nil {
				return err
			}
			if err := readout.Present(exp.Screen, false, false); err != nil {
				return err
			}
			if err := exp.Screen.Flip(); err != nil {
				return err
			}
			_ = readout.Unload() // free the per-frame text texture
		}
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		exp.Fatal("experiment error: %v", runErr)
	}

	done := stimuli.NewTextBox("Done. Press any key to quit.", 600, control.FPoint{}, control.White)
	exp.Show(done)
	exp.Keyboard.Wait()
}
