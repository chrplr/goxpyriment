// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "testing"

// cellAt must be the exact inverse of cellCenter for every cell. This is the
// place where the +Y-is-UP convention is easiest to get backwards (a mirrored
// board would still look plausible but respond to clicks on the wrong row).
func TestCellAtInvertsCellCenter(t *testing.T) {
	for row := 0; row < GridSize; row++ {
		for col := 0; col < GridSize; col++ {
			p := cellCenter(row, col)
			gotRow, gotCol, ok := cellAt(p.X, p.Y)
			if !ok || gotRow != row || gotCol != col {
				t.Errorf("cellAt(cellCenter(%d,%d)) = (%d,%d,%v), want (%d,%d,true)",
					row, col, gotRow, gotCol, ok, row, col)
			}
		}
	}
}

// Row 0 must be the top row: larger Y.
func TestRowZeroIsAtTheTop(t *testing.T) {
	if cellCenter(0, 0).Y <= cellCenter(GridSize-1, 0).Y {
		t.Error("row 0 should be higher on screen (larger Y) than the last row")
	}
	if cellCenter(0, 0).X >= cellCenter(0, GridSize-1).X {
		t.Error("column 0 should be left (smaller X) of the last column")
	}
}

func TestCellAtOutsideBoard(t *testing.T) {
	corners := [][2]float32{
		{-boardHalf - 1, boardTop - 1},    // left of the board
		{boardHalf + 1, boardTop - 1},     // right of the board
		{0, boardTop + 1},                 // above the board
		{0, boardTop - GridSize*tile - 1}, // below the board
		{0, statusY},                      // on the status line
	}
	for _, c := range corners {
		if _, _, ok := cellAt(c[0], c[1]); ok {
			t.Errorf("cellAt(%v, %v) reported a cell, want outside", c[0], c[1])
		}
	}
}

// carRect must cover exactly the cells the car occupies: its bounding box has
// to span from the head cell's outer edge to the tail cell's outer edge.
func TestCarRectSpansItsCells(t *testing.T) {
	b, err := ParseBoard(classic)
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	for _, car := range b.Cars {
		center, w, h := carRect(car)
		head := cellCenter(car.Row, car.Col)
		tailRow, tailCol := car.Row, car.Col
		if car.Horizontal {
			tailCol += car.Length - 1
		} else {
			tailRow += car.Length - 1
		}
		tail := cellCenter(tailRow, tailCol)

		wantX := (head.X + tail.X) / 2
		wantY := (head.Y + tail.Y) / 2
		if center.X != wantX || center.Y != wantY {
			t.Errorf("car %s: center = (%v,%v), want (%v,%v)",
				string(car.Label), center.X, center.Y, wantX, wantY)
		}

		wantW, wantH := tile, tile
		if car.Horizontal {
			wantW = tile * float32(car.Length)
		} else {
			wantH = tile * float32(car.Length)
		}
		if w != wantW || h != wantH {
			t.Errorf("car %s: size = (%v,%v), want (%v,%v)", string(car.Label), w, h, wantW, wantH)
		}
	}
}

// stepForPoint decides the direction of a one-cell click-move from the side of
// the vehicle's midline the click landed on. The middle cell of a 3-cell
// vehicle must NOT be inert: its two halves give opposite directions.
func TestStepForPointDirection(t *testing.T) {
	h2 := &Car{Row: 2, Col: 1, Length: 2, Horizontal: true} // cols 1-2
	v3 := &Car{Row: 1, Col: 3, Length: 3}                   // rows 1-3

	// Each case clicks the center of cell (row, col), offset by (dx, dy).
	cases := []struct {
		name     string
		car      *Car
		row, col int
		dx, dy   float32
		want     int
	}{
		{"h2 head cell", h2, 2, 1, 0, 0, -1},
		{"h2 tail cell", h2, 2, 2, 0, 0, 1},
		{"v3 head cell", v3, 1, 3, 0, 0, -1},
		{"v3 tail cell", v3, 3, 3, 0, 0, 1},
		// The formerly-inert middle cell: above the midline sends the vehicle
		// up (+Y is up), below sends it down.
		{"v3 middle cell, upper half", v3, 2, 3, 0, tile / 4, -1},
		{"v3 middle cell, lower half", v3, 2, 3, 0, -tile / 4, 1},
	}
	for _, c := range cases {
		p := cellCenter(c.row, c.col)
		x, y := p.X+c.dx, p.Y+c.dy
		if got := stepForPoint(c.car, x, y); got != c.want {
			t.Errorf("%s: stepForPoint(%v,%v) = %d, want %d", c.name, x, y, got, c.want)
		}
	}

	// A click exactly on the midline is the only inert point.
	center, _, _ := carRect(v3)
	if got := stepForPoint(v3, center.X, center.Y); got != 0 {
		t.Errorf("midline: stepForPoint = %d, want 0", got)
	}
}
