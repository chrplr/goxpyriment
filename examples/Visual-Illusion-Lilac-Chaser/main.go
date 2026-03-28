// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"log"
	"math"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	radiusFlag := flag.Float64("r", 40.0, "Radius of the lilac circles (pixels)")
	rFlag := flag.Int("R", 250, "Red component of circle color (0-255)")
	gFlag := flag.Int("G", 217, "Green component of circle color (0-255)")
	bFlag := flag.Int("B", 248, "Blue component of circle color (0-255)")

	// 1. Create and initialize the experiment
	exp := control.NewExperimentFromFlags("Lilac Chaser", control.White, control.Black, 32)
	defer exp.End()

	radius := float32(*radiusFlag)
	rose := control.Color{R: uint8(*rFlag), G: uint8(*gFlag), B: uint8(*bFlag), A: 255}

	// 2. Constants for the Lilac Chaser
	n := 12
	distance := float32(300)

	// 3. Prepare stimuli
	fixation := stimuli.NewFixCross(40, 5, control.Black)

	circles := make([]*stimuli.Circle, n)
	for i := 0; i < n; i++ {
		circles[i] = stimuli.NewCircle(radius, rose)
		// Calculate position in polar coordinates
		angle := 2 * math.Pi * float64(i) / float64(n)
		x := float32(distance * float32(math.Cos(angle)))
		y := float32(distance * float32(math.Sin(angle)))
		circles[i].SetPosition(control.FPoint{X: x, Y: y})
	}

	currentPos := 0

	// 4. Run the animation logic
	err := exp.Run(func() error {
		for {
			// Handle events (checking if ESC or QUIT is requested)
			if _, _, err := exp.HandleEvents(); err != nil {
				return err // returns control.EndLoop if ESC or QUIT
			}

			// Render the frame directly on the main thread
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
			if err := exp.Wait(100); err != nil {
				return err
			}
		}
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}