// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// Kanizsa illusory square demo, ported from:
//
//	python_examples/kanizsa-expyriment_v2.py
//
// It draws four black disks on a gray background and overlays a gray
// central rectangle, producing the perception of an illusory square.
//
// The participant adjusts the brightness of the central rectangle with the
// Up/Down arrow keys (one RGB level per press, key repeat when held) until
// it looks the same as the background, then presses Enter. The signed
// difference (rectangle minus background, in RGB levels) is then displayed
// and printed to stdout.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	defaultSquareSize   = 200.0
	defaultCircleRadius = 50.0
)

func main() {
	radiusFlag := flag.Float64("radius", defaultCircleRadius, "Radius of the inducing circles (pixels)")
	sizeFlag := flag.Float64("squaresize", defaultSquareSize, "Size of the central square (pixels)")

	background := control.LightGray
	exp := control.NewExperimentFromFlags("Kanizsa Square", background, control.White, 16)
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

	// Central rectangle occluding the inner quadrants of the disks. It starts
	// at the background level (difference 0) and is adjusted by the participant.
	level := int(background.R)
	rect := stimuli.NewRectangle(0, 0, squareSize, squareSize, background)

	instructions := stimuli.NewTextBox("Kanizsa illusory square –\n↑ / ↓ adjust the brightness of the square\nEnter to finish",
		600, control.Point(0, -squareSize), control.White)

	drawScene := func(text *stimuli.TextBox) error {
		exp.Screen.Clear()
		for _, c := range []*stimuli.Circle{cTL, cTR, cBL, cBR} {
			if err := c.Draw(exp.Screen); err != nil {
				return err
			}
		}
		if err := rect.Draw(exp.Screen); err != nil {
			return err
		}
		if err := text.Draw(exp.Screen); err != nil {
			return err
		}
		return exp.Screen.Update()
	}

	exp.Keyboard.Clear()
	err := exp.Run(func() error {
		// Adjustment phase: SDL emits repeated KEY_DOWN events while a key is
		// held, so holding an arrow key ramps the level continuously.
		for {
			if err := drawScene(instructions); err != nil {
				return err
			}
			key, err := exp.Keyboard.WaitKeys([]control.Keycode{control.K_UP, control.K_DOWN, control.K_RETURN}, -1)
			if err != nil {
				return err
			}
			switch key {
			case control.K_UP:
				level = min(level+1, 255)
			case control.K_DOWN:
				level = max(level-1, 0)
			case control.K_RETURN:
				diff := level - int(background.R)
				fmt.Printf("rectangle level = %d, background level = %d, difference = %+d\n",
					level, background.R, diff)
				result := stimuli.NewTextBox(
					fmt.Sprintf("Rectangle − background = %+d RGB levels\n(rectangle %d, background %d)\n\npress any key to exit",
						diff, level, background.R),
					600, control.Point(0, -squareSize), control.White)
				if err := drawScene(result); err != nil {
					return err
				}
				if _, err := exp.Keyboard.Wait(); err != nil {
					return err
				}
				return control.EndLoop
			}
			g := uint8(level)
			rect.Color = control.RGB(g, g, g)
		}
	})
	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("kanizsa: %v", err)
	}
}
