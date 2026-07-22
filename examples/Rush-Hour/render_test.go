// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

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

// destinationFor must project onto the vehicle's own axis, leaving the other
// coordinate untouched — otherwise TryMove refuses every off-axis cursor.
func TestDestinationForProjectsOntoAxis(t *testing.T) {
	h := &Car{Row: 2, Col: 1, Length: 2, Horizontal: true}
	if got := destinationFor(h, 5, 4); got != [2]int{2, 4} {
		t.Errorf("horizontal: got %v, want [2 4]", got)
	}
	v := &Car{Row: 1, Col: 3, Length: 3}
	if got := destinationFor(v, 5, 0); got != [2]int{5, 3} {
		t.Errorf("vertical: got %v, want [5 3]", got)
	}
}
