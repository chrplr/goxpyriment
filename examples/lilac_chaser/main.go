// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/misc"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/Zyko0/go-sdl3/sdl"
	"log"
	"math"
)

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 800x800)")
	flag.Parse()

	// 1. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 800, 800, false
	}
	exp := control.NewExperiment("Lilac Chaser", width, height, fullscreen)
	exp.BackgroundColor = control.White // background is white

	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// 2. Constants for the Lilac Chaser
	n := 12
	radius := float32(40)
	distance := float32(300)
	rose := sdl.Color{R: 250, G: 217, B: 248, A: 255}

	// 3. Prepare stimuli
	fixation := stimuli.NewFixCross(40, 5, control.Black)

	circles := make([]*stimuli.Circle, n)
	for i := 0; i < n; i++ {
		circles[i] = stimuli.NewCircle(radius, rose)
		// Calculate position in polar coordinates
		angle := 2 * math.Pi * float64(i) / float64(n)
		x := float32(distance * float32(math.Cos(angle)))
		y := float32(distance * float32(math.Sin(angle)))
		circles[i].SetPosition(sdl.FPoint{X: x, Y: y})
	}

	currentPos := 0

	// 4. Run the animation logic
	err := exp.Run(func() error {
		// Handle events (checking if ESC or QUIT is requested)
		if _, _, err := exp.HandleEvents(); err != nil {
			return err // returns sdl.EndLoop if ESC or QUIT
		}

		// Clear screen
		if err := exp.Screen.Clear(); err != nil {
			return err
		}

		// Draw fixation cross
		if err := fixation.Draw(exp.Screen); err != nil {
			return err
		}

		// Draw circles
		for i := 0; i < n; i++ {
			// Skip the circle at currentPos to create the illusion
			if i != currentPos {
				if err := circles[i].Draw(exp.Screen); err != nil {
					return err
				}
			}
		}

		// Present the frame
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		// Update position for next frame
		currentPos = (currentPos + 1) % n

		// Frame timing (approx 100ms per step)
		misc.Wait(100)

		return nil
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
