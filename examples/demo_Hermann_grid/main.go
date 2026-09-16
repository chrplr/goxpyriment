// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// Hermann grid illusion (https://en.wikipedia.org/wiki/Grid_illusion), ported
// from grid_expyriment.py.
//
// A regular array of black squares is drawn on a gray background. The gaps
// between the squares form "streets", and grayish smudges appear at their
// intersections — everywhere except the one you fixate. LEFT/RIGHT widen and
// narrow the gaps interactively, which changes how strong the illusion is.
package main

import (
	"flag"
	"fmt"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	minSpacing = 1
	maxSpacing = 100
)

// gridExtent returns the total width (or height) covered by n cells of side
// length cell separated by gaps of size space.
func gridExtent(n int, cell, space float32) float32 {
	return float32(n)*cell + float32(n-1)*space
}

// drawGrid draws the nRows x nCols array of black squares, centered on the
// screen. Remember that +Y points UP in goxpyriment coordinates.
func drawGrid(exp *control.Experiment, nRows, nCols int, cell, space float32) error {
	w := gridExtent(nCols, cell, space)
	h := gridExtent(nRows, cell, space)

	for row := 0; row < nRows; row++ {
		for col := 0; col < nCols; col++ {
			x := float32(col)*(cell+space) + cell/2 - w/2
			y := h/2 - (float32(row)*(cell+space) + cell/2)
			r := stimuli.NewRectangle(x, y, cell, cell, control.Black)
			if err := r.Draw(exp.Screen); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	rows := flag.Int("rows", 10, "Number of rows of cells")
	cols := flag.Int("cols", 10, "Number of columns of cells")
	cellSide := flag.Float64("cell", 50, "Side length of a cell (pixels)")
	spacing := flag.Float64("space", 10, "Initial gap between cells (pixels)")
	noHint := flag.Bool("nohint", false, "Hide the on-screen key legend")

	exp := control.NewExperimentFromFlags("Hermann's Grid", control.Gray, control.White, 20)
	defer exp.End()

	cell := float32(*cellSide)
	space := float32(*spacing)

	err := exp.Run(func() error {
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := drawGrid(exp, *rows, *cols, cell, space); err != nil {
			return err
		}
		if !*noHint {
			h := gridExtent(*rows, cell, space)
			hint := stimuli.NewTextLine(
				fmt.Sprintf("gap = %.0f px   LEFT/RIGHT to change, ESC to quit", space),
				0, -h/2-40, control.White)
			if err := hint.Draw(exp.Screen); err != nil {
				return err
			}
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		// Blocks until LEFT or RIGHT; returns control.EndLoop on ESC/quit.
		key, err := exp.Keyboard.WaitKeys([]control.Keycode{control.K_LEFT, control.K_RIGHT}, -1)
		if err != nil {
			return err
		}
		switch key {
		case control.K_RIGHT:
			if space < maxSpacing {
				space++
			}
		case control.K_LEFT:
			if space > minSpacing {
				space--
			}
		}
		return nil // draw the next frame
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
