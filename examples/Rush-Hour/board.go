// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// GridSize is the side of the (square) Rush Hour board.
const GridSize = 6

// TargetRow is the row of the exit — the red car must reach the right wall on
// this row.
const TargetRow = 2

// Car is one vehicle on the board. Row/Col designate its top-left cell.
type Car struct {
	ID         int
	Label      byte // 'A'..'Z' from the board string; 'A' is the target car
	Row, Col   int
	Length     int
	Horizontal bool
	IsTarget   bool
}

// Orientation returns "H" or "V", for the data file.
func (c *Car) Orientation() string {
	if c.Horizontal {
		return "H"
	}
	return "V"
}

// Covers reports whether the car occupies cell (row, col).
func (c *Car) Covers(row, col int) bool {
	if c.Horizontal {
		return row == c.Row && col >= c.Col && col < c.Col+c.Length
	}
	return col == c.Col && row >= c.Row && row < c.Row+c.Length
}

// Board is a puzzle configuration: a set of cars on a GridSize×GridSize grid.
type Board struct {
	Cars []*Car
}

// ParseBoard builds a Board from the standard 36-character Rush Hour notation:
// one character per cell, row-major, 'o' or '.' for an empty cell and a letter
// for each vehicle. 'A' is the red target car.
//
// Whitespace (including newlines, so a 6-line block is accepted too) is
// ignored. Every letter must form a single straight contiguous run of 2 or 3
// cells, and 'A' must be horizontal on row TargetRow — anything else is an
// error, so a typo in puzzles.txt fails loudly instead of yielding an
// unsolvable board.
func ParseBoard(s string) (*Board, error) {
	var cells []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		cells = append(cells, c)
	}
	if len(cells) != GridSize*GridSize {
		return nil, fmt.Errorf("board must have %d cells, got %d", GridSize*GridSize, len(cells))
	}

	// Collect the cells of each letter.
	positions := make(map[byte][][2]int)
	for i, c := range cells {
		switch {
		case c == 'o' || c == 'O' || c == '.' || c == '-':
			continue
		case c >= 'A' && c <= 'Z':
			positions[c] = append(positions[c], [2]int{i / GridSize, i % GridSize})
		default:
			return nil, fmt.Errorf("invalid character %q in board", string(c))
		}
	}

	labels := make([]byte, 0, len(positions))
	for label := range positions {
		labels = append(labels, label)
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i] < labels[j] })

	b := &Board{}
	for _, label := range labels {
		cs := positions[label]
		if len(cs) < 2 || len(cs) > 3 {
			return nil, fmt.Errorf("car %q spans %d cells, must be 2 or 3", string(label), len(cs))
		}
		sort.Slice(cs, func(i, j int) bool {
			if cs[i][0] != cs[j][0] {
				return cs[i][0] < cs[j][0]
			}
			return cs[i][1] < cs[j][1]
		})

		sameRow := true
		sameCol := true
		for _, c := range cs {
			if c[0] != cs[0][0] {
				sameRow = false
			}
			if c[1] != cs[0][1] {
				sameCol = false
			}
		}
		if sameRow == sameCol { // neither a row nor a column, or (impossible) both
			return nil, fmt.Errorf("car %q is not aligned on a single row or column", string(label))
		}

		// Contiguity along the axis of alignment.
		for i := 1; i < len(cs); i++ {
			if sameRow && cs[i][1] != cs[i-1][1]+1 {
				return nil, fmt.Errorf("car %q is not contiguous", string(label))
			}
			if sameCol && cs[i][0] != cs[i-1][0]+1 {
				return nil, fmt.Errorf("car %q is not contiguous", string(label))
			}
		}

		car := &Car{
			ID:         len(b.Cars),
			Label:      label,
			Row:        cs[0][0],
			Col:        cs[0][1],
			Length:     len(cs),
			Horizontal: sameRow,
			IsTarget:   label == 'A',
		}
		b.Cars = append(b.Cars, car)
	}

	target := b.Target()
	if target == nil {
		return nil, fmt.Errorf("board has no target car 'A'")
	}
	if !target.Horizontal {
		return nil, fmt.Errorf("target car 'A' must be horizontal")
	}
	if target.Row != TargetRow {
		return nil, fmt.Errorf("target car 'A' must be on row %d, found on row %d", TargetRow, target.Row)
	}
	if b.Solved() {
		return nil, fmt.Errorf("board is already solved")
	}
	return b, nil
}

// Target returns the red car, or nil if the board has none.
func (b *Board) Target() *Car {
	for _, c := range b.Cars {
		if c.IsTarget {
			return c
		}
	}
	return nil
}

// Occupancy returns the grid of car IDs, with -1 for an empty cell. The
// optional exclude car is left out, which is what move testing needs.
func (b *Board) Occupancy(exclude *Car) [GridSize][GridSize]int {
	var grid [GridSize][GridSize]int
	for r := range grid {
		for c := range grid[r] {
			grid[r][c] = -1
		}
	}
	for _, car := range b.Cars {
		if car == exclude {
			continue
		}
		for i := 0; i < car.Length; i++ {
			r, c := car.Row, car.Col
			if car.Horizontal {
				c += i
			} else {
				r += i
			}
			if r >= 0 && r < GridSize && c >= 0 && c < GridSize {
				grid[r][c] = car.ID
			}
		}
	}
	return grid
}

// CarAt returns the car covering cell (row, col), or nil.
func (b *Board) CarAt(row, col int) *Car {
	for _, car := range b.Cars {
		if car.Covers(row, col) {
			return car
		}
	}
	return nil
}

// TryMove slides car toward (newRow, newCol) one cell at a time, stopping at
// the first wall or collision, and reports whether the car actually moved.
//
// A drag past a blocker therefore parks the car flush against it rather than
// doing nothing. A target off the car's axis is rejected outright.
func (b *Board) TryMove(car *Car, newRow, newCol int) bool {
	if car.Horizontal && newRow != car.Row {
		return false
	}
	if !car.Horizontal && newCol != car.Col {
		return false
	}

	grid := b.Occupancy(car)

	stepR := sign(newRow - car.Row)
	stepC := sign(newCol - car.Col)

	currR, currC := car.Row, car.Col
	for currR != newRow || currC != newCol {
		nextR := currR + stepR
		nextC := currC + stepC

		// Boundary check for the whole vehicle span.
		maxR, maxC := GridSize-1, GridSize-1
		if car.Horizontal {
			maxC = GridSize - car.Length
		} else {
			maxR = GridSize - car.Length
		}
		if nextR < 0 || nextR > maxR || nextC < 0 || nextC > maxC {
			break
		}

		blocked := false
		for i := 0; i < car.Length; i++ {
			checkR, checkC := nextR, nextC
			if car.Horizontal {
				checkC += i
			} else {
				checkR += i
			}
			if grid[checkR][checkC] != -1 {
				blocked = true
				break
			}
		}
		if blocked {
			break
		}

		currR, currC = nextR, nextC
	}

	if currR == car.Row && currC == car.Col {
		return false
	}
	car.Row, car.Col = currR, currC
	return true
}

// Solved reports whether the red car has reached the right wall.
func (b *Board) Solved() bool {
	t := b.Target()
	return t != nil && t.Col+t.Length == GridSize
}

// String renders the board back to the 6-line notation — used in tests and
// error messages.
func (b *Board) String() string {
	var grid [GridSize][GridSize]byte
	for r := range grid {
		for c := range grid[r] {
			grid[r][c] = 'o'
		}
	}
	for _, car := range b.Cars {
		for i := 0; i < car.Length; i++ {
			r, c := car.Row, car.Col
			if car.Horizontal {
				c += i
			} else {
				r += i
			}
			grid[r][c] = car.Label
		}
	}
	var sb strings.Builder
	for r := range grid {
		if r > 0 {
			sb.WriteByte('\n')
		}
		sb.Write(grid[r][:])
	}
	return sb.String()
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

// ParsePuzzleFile extracts the puzzles from the embedded puzzles.txt. Blank
// lines and everything after a '#' are ignored; each remaining line is
//
//	<name>: <min-moves>: <board>
//
// The two shorter forms "<name>: <board>" and "<board>" alone are also
// accepted, with the minimum-move count left unknown (0).
func ParsePuzzleFile(content string) ([]Puzzle, error) {
	var puzzles []Puzzle
	for i, line := range strings.Split(content, "\n") {
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		name := fmt.Sprintf("puzzle%02d", len(puzzles)+1)
		minMoves := 0
		spec := line

		fields := strings.Split(line, ":")
		switch len(fields) {
		case 1:
		case 2:
			name, spec = strings.TrimSpace(fields[0]), strings.TrimSpace(fields[1])
		case 3:
			name, spec = strings.TrimSpace(fields[0]), strings.TrimSpace(fields[2])
			n, err := strconv.Atoi(strings.TrimSpace(fields[1]))
			if err != nil {
				return nil, fmt.Errorf("puzzles.txt line %d (%s): bad minimum-move count: %w", i+1, name, err)
			}
			minMoves = n
		default:
			return nil, fmt.Errorf("puzzles.txt line %d: expected at most 3 colon-separated fields", i+1)
		}

		board, err := ParseBoard(spec)
		if err != nil {
			return nil, fmt.Errorf("puzzles.txt line %d (%s): %w", i+1, name, err)
		}
		puzzles = append(puzzles, Puzzle{Name: name, Spec: spec, Board: board, MinMoves: minMoves})
	}
	if len(puzzles) == 0 {
		return nil, fmt.Errorf("no puzzles found")
	}
	return puzzles, nil
}

// Puzzle is one trial: a named board configuration, with the length of its
// shortest solution (0 when unknown).
type Puzzle struct {
	Name     string
	Spec     string
	Board    *Board
	MinMoves int
}

// Fresh returns a pristine copy of the puzzle's board, so a trial always
// starts from the original configuration.
func (p Puzzle) Fresh() *Board {
	b, err := ParseBoard(p.Spec)
	if err != nil {
		// Unreachable: Spec was validated at load time.
		panic(err)
	}
	return b
}
