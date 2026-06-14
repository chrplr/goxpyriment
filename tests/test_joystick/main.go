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
		readAxes func() (int16, int16) // axes that drive the cursor
		axisDump func() string         // live "all axes" readout (joystick mode)
		mode     string
	)

	if pads, err := apparatus.GetGamePads(); err == nil && len(pads) > 0 {
		pad := pads[0]
		defer pad.Close()
		mode = "Gamepad: left analog stick"
		fmt.Printf("Input device: GAMEPAD API — %d gamepad(s) recognized, using gamepad[0] left analog stick (GAMEPAD_AXIS_LEFTX/LEFTY).\n", len(pads))
		readAxes = func() (int16, int16) {
			return pad.Axis(sdl.GAMEPAD_AXIS_LEFTX), pad.Axis(sdl.GAMEPAD_AXIS_LEFTY)
		}
		axisDump = func() string {
			return fmt.Sprintf("LX:%6d LY:%6d  RX:%6d RY:%6d",
				pad.Axis(sdl.GAMEPAD_AXIS_LEFTX), pad.Axis(sdl.GAMEPAD_AXIS_LEFTY),
				pad.Axis(sdl.GAMEPAD_AXIS_RIGHTX), pad.Axis(sdl.GAMEPAD_AXIS_RIGHTY))
		}
	} else if joys, err := apparatus.GetJoysticks(); err == nil && len(joys) > 0 {
		joy := joys[0]
		defer joy.Close()
		mode = "Joystick: raw axes 0/1 (may be a digital D-pad)"
		n, _ := joy.NumAxes()
		nb, _ := joy.NumButtons()
		name, _ := joy.Handle.Name()
		// NOTE: (*sdl.Joystick).GUID() panics ("not implemented") in
		// go-sdl3 v0.1.1, so identify the device by USB vendor/product instead.
		vid, pid := joy.Handle.Vendor(), joy.Handle.Product()

		fmt.Printf("Input device: JOYSTICK API (no gamepad mapping for this device)\n")
		fmt.Printf("  joysticks found : %d (using joystick[0])\n", len(joys))
		fmt.Printf("  name            : %q\n", name)
		fmt.Printf("  USB VID:PID     : %04x:%04x\n", vid, pid)
		fmt.Printf("  axes / buttons  : %d / %d\n", n, nb)
		fmt.Printf("  IsGamepad()     : %v  (false => SDL has no controller mapping for this device)\n", joy.ID.IsGamepad())
		fmt.Printf("  -> driving the cursor from raw axes 0/1; these may be a digital D-pad (8 directions only).\n")
		fmt.Printf("  -> wiggle each stick and watch the on-screen axis dump to find the analog stick indices.\n")
		fmt.Printf("  -> to get the proper gamepad path, add a controller mapping (SDL gamecontrollerdb / AddGamepadMapping).\n")

		readAxes = func() (int16, int16) {
			x, _ := joy.Axis(0)
			y, _ := joy.Axis(1)
			return x, y
		}
		axisDump = func() string {
			s := ""
			for i := int32(0); i < n; i++ {
				v, _ := joy.Axis(i)
				s += fmt.Sprintf(" a%d:%6d", i, v)
			}
			return s
		}
	} else {
		fmt.Println("Input device: NONE — no gamepad or joystick found.")
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

	fmt.Println("Move each stick/D-pad and watch the live axis values below")
	fmt.Println("(smooth sweep = analog stick; snapping to -32767/0/+32767 = digital):")

	runErr := exp.Run(func() error {
		prevTick := sdl.Ticks()
		var lastDump uint64

		for {
			// Delta time in seconds
			now := sdl.Ticks()
			dt := float32(now-prevTick) / 1000.0
			prevTick = now

			// Read the active device's axes (raw values, before dead zone).
			rawX, rawY := readAxes()

			// Live axis dump to the console (updates in place ~10x/sec) so all
			// axes — including a0 — are visible regardless of window width.
			if now-lastDump >= 100 {
				lastDump = now
				fmt.Printf("\r  %s        ", axisDump())
			}
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

			// Live readout: mode + cursor-driving axis values + direction,
			// plus a dump of every axis (to locate the analog stick indices).
			angle := math.Atan2(float64(-rawY), float64(rawX)) * 180 / math.Pi
			line1 := stimuli.NewTextLine(
				fmt.Sprintf("%s    X:%6d  Y:%6d    dir:%6.1f deg", mode, rawX, rawY, angle),
				0, -halfH+24, control.White)
			line2 := stimuli.NewTextLine(axisDump(), 0, -halfH+52, control.Gray)

			// Draw circle (clearing first), overlay the two readout lines, flip.
			if err := circle.Present(exp.Screen, true, false); err != nil {
				return err
			}
			if err := line1.Present(exp.Screen, false, false); err != nil {
				return err
			}
			if err := line2.Present(exp.Screen, false, false); err != nil {
				return err
			}
			if err := exp.Screen.Flip(); err != nil {
				return err
			}
			_ = line1.Unload() // free the per-frame text textures
			_ = line2.Unload()
		}
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		exp.Fatal("experiment error: %v", runErr)
	}

	done := stimuli.NewTextBox("Done. Press any key to quit.", 600, control.FPoint{}, control.White)
	exp.Show(done)
	exp.Keyboard.Wait()
}
