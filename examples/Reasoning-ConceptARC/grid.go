// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "strings"

// Grid is an ARC grid: rows of colour indices 0–9, row 0 at the top. All rows
// have the same length. It has no SDL dependency, so grid logic is unit-tested
// without a display.
type Grid [][]int

// MaxGridSide is the largest grid the editor allows. ARC caps grids at 30×30;
// the ConceptARC corpus never exceeds 25×25.
const MaxGridSide = 30

// Blank returns a rows×cols grid filled with colour 0 (black).
func Blank(rows, cols int) Grid {
	g := make(Grid, rows)
	for r := range g {
		g[r] = make([]int, cols)
	}
	return g
}

// Rows returns the number of rows.
func (g Grid) Rows() int { return len(g) }

// Cols returns the number of columns (0 for an empty grid).
func (g Grid) Cols() int {
	if len(g) == 0 {
		return 0
	}
	return len(g[0])
}

// Clone returns a deep copy.
func (g Grid) Clone() Grid {
	c := make(Grid, len(g))
	for r := range g {
		c[r] = append([]int(nil), g[r]...)
	}
	return c
}

// Equal reports whether the two grids have the same size and contents.
func (g Grid) Equal(o Grid) bool {
	if g.Rows() != o.Rows() || g.Cols() != o.Cols() {
		return false
	}
	for r := range g {
		for c := range g[r] {
			if g[r][c] != o[r][c] {
				return false
			}
		}
	}
	return true
}

// Resized returns a rows×cols copy: cells that still fit keep their colour,
// new cells are black, cells outside the new bounds are dropped.
func (g Grid) Resized(rows, cols int) Grid {
	n := Blank(rows, cols)
	for r := 0; r < rows && r < g.Rows(); r++ {
		for c := 0; c < cols && c < g.Cols(); c++ {
			n[r][c] = g[r][c]
		}
	}
	return n
}

// String serialises the grid for the results file: one digit per cell, rows
// separated by '|' — "0000|0120|0000". Compact, unambiguous (colours are single
// digits), and safe inside a CSV field.
func (g Grid) String() string {
	var sb strings.Builder
	for r, row := range g {
		if r > 0 {
			sb.WriteByte('|')
		}
		for _, v := range row {
			sb.WriteByte(byte('0' + v))
		}
	}
	return sb.String()
}

// valid reports whether the grid is rectangular, non-empty, within the size
// limit, and uses only colours 0–9.
func (g Grid) valid() bool {
	if g.Rows() == 0 || g.Cols() == 0 || g.Rows() > MaxGridSide || g.Cols() > MaxGridSide {
		return false
	}
	for _, row := range g {
		if len(row) != g.Cols() {
			return false
		}
		for _, v := range row {
			if v < 0 || v > 9 {
				return false
			}
		}
	}
	return true
}
