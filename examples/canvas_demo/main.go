// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags("Canvas Demo", control.Black, control.White, 32)
	defer exp.End()

	// 2. Prepare Canvas
	canvas := stimuli.NewCanvas(400, 400, control.Color{R: 50, G: 50, B: 50, A: 255})

	// Prepare sub-stimuli to draw on canvas
	// Coordinates are relative to canvas center (0,0)
	rect := stimuli.NewRectangle(0, 0, 100, 100, control.Color{R: 200, G: 0, B: 0, A: 255})
	text := stimuli.NewTextLine("Inside Canvas", 0, -80, control.White)

	// Title
	title := stimuli.NewTextLine("Canvas Demo (Press Space)", 0, 250, control.DefaultTextColor)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Blit things onto the canvas
		if err := canvas.Blit(rect, exp.Screen); err != nil {
			return err
		}
		if err := canvas.Blit(text, exp.Screen); err != nil {
			return err
		}

		// Present the canvas on the screen
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := title.Draw(exp.Screen); err != nil {
			return err
		}
		if err := canvas.Draw(exp.Screen); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		_, err := exp.Keyboard.Wait()
		if err != nil {
			return err
		}

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
