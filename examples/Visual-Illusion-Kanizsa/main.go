// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// Kanizsa illusory square demo, ported from:
//
//	python_examples/kanizsa-expyriment_v2.py
//
// It draws four black disks on a gray background and overlays a gray
// central rectangle, producing the perception of an illusory square.
package main

import (
	"flag"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	defaultSquareSize   = 200.0
	defaultCircleRadius = 50.0
)

func main() {
	radiusFlag := flag.Float64("r", defaultCircleRadius, "Radius of the inducing circles (pixels)")
	sizeFlag := flag.Float64("w", defaultSquareSize, "Size of the central square (pixels)")

	exp := control.NewExperimentFromFlags("Kanizsa Square", control.LightGray, control.White, 16)
	defer exp.End()

	squareSize := float32(*sizeFlag)
	circleRadius := float32(*radiusFlag)

	// Optional logical size for nicer centering on large displays.
	if err := exp.SetLogicalSize(800, 600); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	left := -squareSize / 2
	right := squareSize / 2
	top := squareSize / 2
	bottom := -squareSize / 2

	cTL := stimuli.NewCircle(circleRadius, control.Black)
	cTL.SetPosition(control.Point(left, top))

	cTR := stimuli.NewCircle(circleRadius, control.Black)
	cTR.SetPosition(control.Point(right, top))

	cBL := stimuli.NewCircle(circleRadius, control.Black)
	cBL.SetPosition(control.Point(left, bottom))

	cBR := stimuli.NewCircle(circleRadius, control.Black)
	cBR.SetPosition(control.Point(right, bottom))

	// Central rectangle occluding the inner quadrants of the disks.
	rect := stimuli.NewRectangle(0, 0, squareSize, squareSize, control.LightGray)

	exp.Screen.Clear()
	cTL.Draw(exp.Screen)
	cTR.Draw(exp.Screen)
	cBL.Draw(exp.Screen)
	cBR.Draw(exp.Screen)
	rect.Draw(exp.Screen)

	box := stimuli.NewTextBox("Kanizsa illusory square –\npress any key to exit", 400, control.Point(0, -squareSize), control.White)
	box.Draw(exp.Screen)

	exp.Screen.Update()

	exp.Keyboard.Clear()
	exp.Keyboard.Wait()
}
