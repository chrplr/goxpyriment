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

// stepFor decides the direction of a one-cell click-move: the half of the
// vehicle nearer its head sends it back, the half nearer its tail forward, and
// the middle cell of a 3-cell vehicle is a no-op. A click off the vehicle must
// never produce a step.
func TestStepForDirection(t *testing.T) {
	h2 := &Car{Row: 2, Col: 1, Length: 2, Horizontal: true} // cols 1-2
	v3 := &Car{Row: 1, Col: 3, Length: 3}                   // rows 1-3

	cases := []struct {
		name     string
		car      *Car
		row, col int
		want     int
	}{
		{"h2 head cell", h2, 2, 1, -1},
		{"h2 tail cell", h2, 2, 2, 1},
		{"h2 off the car", h2, 2, 4, 0},
		{"v3 head cell", v3, 1, 3, -1},
		{"v3 middle cell", v3, 2, 3, 0},
		{"v3 tail cell", v3, 3, 3, 1},
		{"v3 off the car", v3, 5, 3, 0},
	}
	for _, c := range cases {
		if got := stepFor(c.car, c.row, c.col); got != c.want {
			t.Errorf("%s: stepFor(%d,%d) = %d, want %d", c.name, c.row, c.col, got, c.want)
		}
	}
}
