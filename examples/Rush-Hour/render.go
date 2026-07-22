// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"math"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Layout ───────────────────────────────────────────────────────────────────
//
// Coordinates are center-relative with +Y pointing UP (see apparatus/CLAUDE.md);
// row 0 is therefore the TOP row and gets the LARGEST Y.

const (
	logicalW = int32(1024)
	logicalH = int32(768)

	tile      = float32(90)                  // cell side, as in the pygame original
	boardHalf = float32(GridSize) * tile / 2 // 270 — half the 6×6 board
	boardTop  = boardHalf + 40               // board is shifted up to leave room for the status line

	carInset  = float32(4)  // colored body inset inside its black outline
	exitWidth = float32(10) // thickness of the exit marker at the right wall

	// Status line, 40 px below the bottom edge of the board.
	statusY = boardTop - float32(GridSize)*tile - 40
)

var (
	bgColor     = control.RGB(240, 240, 240)
	gridColor   = control.RGB(180, 180, 180)
	exitColor   = control.RGB(220, 50, 50)
	textColor   = control.RGB(30, 30, 30)
	outlineDark = control.RGB(0, 0, 0)
	selectColor = control.RGB(255, 255, 255)

	// Vehicle palette, ported from CAR_COLORS in rush.py. Index 0 is the red
	// target car; the others cycle over the remaining entries.
	carColors = []control.Color{
		control.RGB(220, 50, 50),  // red — target
		control.RGB(50, 120, 220), // blue
		control.RGB(50, 180, 80),  // green
		control.RGB(220, 160, 40), // orange
		control.RGB(140, 60, 200), // purple
		control.RGB(200, 200, 50), // yellow
		control.RGB(50, 180, 200), // cyan
	}
)

// carColor returns the drawing color of a vehicle.
func carColor(c *Car) control.Color {
	if c.IsTarget {
		return carColors[0]
	}
	return carColors[c.ID%(len(carColors)-1)+1]
}

// cellCenter returns the center of cell (row, col) in center-based coordinates.
func cellCenter(row, col int) control.FPoint {
	return control.FPoint{
		X: -boardHalf + tile*(float32(col)+0.5),
		Y: boardTop - tile*(float32(row)+0.5),
	}
}

// cellAt is the inverse of cellCenter: it maps a point to the cell containing
// it. ok is false when the point falls outside the board.
func cellAt(x, y float32) (row, col int, ok bool) {
	col = int(math.Floor(float64((x + boardHalf) / tile)))
	row = int(math.Floor(float64((boardTop - y) / tile)))
	if row < 0 || row >= GridSize || col < 0 || col >= GridSize {
		return row, col, false
	}
	return row, col, true
}

// clampedCellAt is like cellAt but clamps to the board, mirroring the pygame
// original's behaviour when the cursor leaves the grid mid-drag.
func clampedCellAt(x, y float32) (row, col int) {
	row, col, _ = cellAt(x, y)
	return clamp(row, 0, GridSize-1), clamp(col, 0, GridSize-1)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// carRect returns the center and size of a vehicle's full tile span.
func carRect(c *Car) (center control.FPoint, w, h float32) {
	w, h = tile, tile
	if c.Horizontal {
		w = tile * float32(c.Length)
	} else {
		h = tile * float32(c.Length)
	}
	head := cellCenter(c.Row, c.Col)
	if c.Horizontal {
		center = control.FPoint{X: head.X + tile*float32(c.Length-1)/2, Y: head.Y}
	} else {
		center = control.FPoint{X: head.X, Y: head.Y - tile*float32(c.Length-1)/2}
	}
	return center, w, h
}

// drawBoard renders one frame: grid, exit marker, vehicles, and the status
// line. It clears the screen but does not flip — the caller decides when to
// present (PacedFlip inside the trial loop).
func drawBoard(exp *control.Experiment, b *Board, selected *Car, status string) error {
	if err := exp.Screen.Clear(); err != nil {
		return err
	}

	// Grid lines.
	left, right := -boardHalf, boardHalf
	top, bottom := boardTop, boardTop-float32(GridSize)*tile
	for i := 0; i <= GridSize; i++ {
		y := boardTop - float32(i)*tile
		line := stimuli.NewLine(control.Point(left, y), control.Point(right, y), gridColor, 1)
		if err := line.Draw(exp.Screen); err != nil {
			return err
		}
		x := left + float32(i)*tile
		line = stimuli.NewLine(control.Point(x, top), control.Point(x, bottom), gridColor, 1)
		if err := line.Draw(exp.Screen); err != nil {
			return err
		}
	}

	// Exit marker on the right wall of the target row.
	exitCenter := cellCenter(TargetRow, GridSize-1)
	exit := stimuli.NewRectangle(right-exitWidth/2, exitCenter.Y, exitWidth, tile, exitColor)
	if err := exit.Draw(exp.Screen); err != nil {
		return err
	}

	// Vehicles: a black plate with the colored body inset on top. SDL's
	// RenderFillRect has no border radius, so the pygame rounded corners
	// become square.
	for _, car := range b.Cars {
		center, w, h := carRect(car)

		border := outlineDark
		if car == selected {
			border = selectColor
		}
		plate := stimuli.NewRectangle(center.X, center.Y, w-carInset, h-carInset, border)
		if err := plate.Draw(exp.Screen); err != nil {
			return err
		}
		body := stimuli.NewRectangle(center.X, center.Y, w-3*carInset, h-3*carInset, carColor(car))
		if err := body.Draw(exp.Screen); err != nil {
			return err
		}
	}

	if status != "" {
		if err := stimuli.NewTextLine(status, 0, statusY, textColor).Draw(exp.Screen); err != nil {
			return err
		}
	}
	return nil
}
