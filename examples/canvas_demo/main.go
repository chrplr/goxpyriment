// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main
import (
	"flag"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"

	"github.com/Zyko0/go-sdl3/sdl"
)

func main() {
	fullscreen := flag.Bool("F", false, "Launch in fullscreen display mode")
	flag.Parse()

	// 1. Create and initialize the experiment
	exp := control.NewExperiment("Canvas Demo", 1368, 1024, *fullscreen)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// 2. Prepare Canvas
	canvas := stimuli.NewCanvas(400, 400, sdl.Color{R: 50, G: 50, B: 50, A: 255})
	
	// Prepare sub-stimuli to draw on canvas
	// Coordinates are relative to canvas center (0,0)
	rect := stimuli.NewRectangle(0, 0, 100, 100, sdl.Color{R: 200, G: 0, B: 0, A: 255})
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

		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
