// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/io"
	"github.com/chrplr/goxpyriment/misc"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
	"math"
)

// DrawEbbinghaus draws an Ebbinghaus illusion figure.
func DrawEbbinghaus(screen *io.Screen, n int, d float32, r1 float32, r2 float32, col1 sdl.Color, col2 sdl.Color, x float32, y float32) error {
	// draw inner circle
	inner := stimuli.NewCircle(r1, col1)
	inner.SetPosition(sdl.FPoint{X: x, Y: y})
	if err := inner.Draw(screen); err != nil {
		return err
	}

	// draw peripheral circles
	for i := 0; i < n; i++ {
		angle := (2 * math.Pi * float64(i)) / float64(n)
		x1 := x + d*float32(math.Cos(angle))
		y1 := y + d*float32(math.Sin(angle))
		outer := stimuli.NewCircle(r2, col2)
		outer.SetPosition(sdl.FPoint{X: x1, Y: y1})
		if err := outer.Draw(screen); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 700x500)")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 700, 500, false
	}

	// 1. Create and initialize the experiment
	exp := control.NewExperiment("Dynamic Ebbinghaus", width, height, fullscreen)
	exp.BackgroundColor = control.White

	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// 2. Constants and initial state for the illusion
	bigCirclesSize := float32(35)
	upperLimit := float32(35)
	smallCirclesSize := float32(15)
	lowerLimit := float32(15)
	timeBetweenRefresh := 200 // ms
	delta := float32(-1)

	// 3. Run the animation logic
	err := exp.Run(func() error {
		// Handle events (checking if ESC or QUIT is requested)
		if _, _, err := exp.HandleEvents(); err != nil {
			return err // returns sdl.EndLoop if ESC or QUIT
		}

		// Clear screen
		if err := exp.Screen.Clear(); err != nil {
			return err
		}

		// Draw Ebbinghaus figures
		// Right figure: inner circle 25, surrounded by 8 outer circles of size bigCirclesSize
		if err := DrawEbbinghaus(exp.Screen, 8, 100, 25, bigCirclesSize, control.Black, control.Black, 150, 0); err != nil {
			return err
		}

		// Left figure: inner circle 25, surrounded by 8 outer circles of size smallCirclesSize
		if err := DrawEbbinghaus(exp.Screen, 8, 100, 25, smallCirclesSize, control.Black, control.Black, -150, 0); err != nil {
			return err
		}

		// Present the frame
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		// Update sizes for next frame
		if bigCirclesSize >= upperLimit {
			delta = -1
		} else if bigCirclesSize <= lowerLimit {
			delta = 1
		}

		bigCirclesSize += delta
		smallCirclesSize -= delta

		// Frame timing
		misc.Wait(timeBetweenRefresh)

		return nil
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
