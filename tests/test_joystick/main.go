// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// test_joystick demonstrates joystick input: use the joystick to move a red
// circle around the screen. Click on the circle to stop. ESC to quit.
package main

import (
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

	joysticks, err := apparatus.GetJoysticks()
	if err != nil {
		exp.Fatal("failed to enumerate joysticks: %v", err)
	}
	if len(joysticks) == 0 {
		msg := stimuli.NewTextBox("No joystick found. Connect a joystick and restart.\n\nPress any key to quit.", 600, control.FPoint{}, control.White)
		exp.Show(msg)
		exp.Keyboard.Wait()
		return
	}

	joy := joysticks[0]
	defer joy.Close()

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

			// Read joystick axes with dead zone
			axisX, _ := joy.Axis(0)
			axisY, _ := joy.Axis(1)
			if axisX > -deadZone && axisX < deadZone {
				axisX = 0
			}
			if axisY > -deadZone && axisY < deadZone {
				axisY = 0
			}

			// Update position
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
				case sdl.EVENT_JOYSTICK_BUTTON_DOWN, sdl.EVENT_QUIT:
					return control.EndLoop
				case sdl.EVENT_KEY_DOWN:
					if event.KeyboardEvent().Key == sdl.K_ESCAPE {
						return control.EndLoop
					}
				}
			}

			exp.Show(circle)
		}
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		exp.Fatal("experiment error: %v", runErr)
	}

	done := stimuli.NewTextBox("Done. Press any key to quit.", 600, control.FPoint{}, control.White)
	exp.Show(done)
	exp.Keyboard.Wait()
}
