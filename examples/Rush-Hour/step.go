// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

// One-cell steps.
//
// TryMove takes an absolute destination and slides as far as it can toward it,
// which is what a drag does. The experiment and the agent environment both work
// in single cells instead: one click, or one action, is one cell. That is the
// vocabulary in this file, so that a human trace and an agent trace count the
// same events.
//
// A direction is -1 (left for a horizontal vehicle, up for a vertical one) or
// +1 (right, down).

// Left is the direction of a one-cell step towards column 0 (horizontal
// vehicles) or row 0 (vertical vehicles).
const Left = -1

// Right is the direction of a one-cell step towards the last column
// (horizontal vehicles) or the last row (vertical vehicles).
const Right = 1

// Up is Left seen from a vertical vehicle: both move towards index 0.
const Up = Left

// Down is Right seen from a vertical vehicle.
const Down = Right

// StepTarget returns the head cell the vehicle would occupy after a one-cell
// step in dir. The result is not checked against the board.
func (c *Car) StepTarget(dir int) (row, col int) {
	if c.Horizontal {
		return c.Row, c.Col + dir
	}
	return c.Row + dir, c.Col
}

// CanStep reports whether a one-cell step in dir is legal, without touching the
// board. Only the single cell the vehicle would newly occupy can obstruct it,
// so this is a wall test plus one lookup — and, unlike a move-then-undo, it
// cannot leave the board perturbed if the caller forgets to undo.
func (b *Board) CanStep(c *Car, dir int) bool {
	if dir != Left && dir != Right {
		return false
	}

	// The leading cell: just beyond the head when stepping towards index 0,
	// just beyond the tail when stepping the other way.
	offset := -1
	if dir == Right {
		offset = c.Length
	}
	row, col := c.Row, c.Col
	if c.Horizontal {
		col += offset
	} else {
		row += offset
	}

	if row < 0 || row >= GridSize || col < 0 || col >= GridSize {
		return false
	}
	return b.CarAt(row, col) == nil
}

// Step slides the vehicle one cell in dir and reports whether it moved. It is
// TryMove with a one-cell destination, which cannot stop short: it either
// completes the step or changes nothing.
func (b *Board) Step(c *Car, dir int) bool {
	row, col := c.StepTarget(dir)
	return b.TryMove(c, row, col)
}

// Clone returns a deep copy: Board holds pointers to its cars, so a shallow
// copy would share — and move — the originals.
func (b *Board) Clone() *Board {
	cars := make([]*Car, len(b.Cars))
	for i, c := range b.Cars {
		cp := *c
		cars[i] = &cp
	}
	return &Board{Cars: cars}
}

// SlideCounter turns a stream of one-cell steps into a count of slides:
// consecutive steps of the same vehicle in the same direction are one slide.
// That is the unit puzzles.txt counts in its minimum-move column, and the unit
// the Rush Hour literature reports, whereas a click (or an agent action) is one
// cell. Humans and agents share this type so the two traces stay comparable.
//
// The zero value is ready to use: a direction is never 0, so the first Add
// always opens a new slide.
type SlideCounter struct {
	lastID  int
	lastDir int
	n       int
}

// Add records one successful step. Steps that moved nothing must not be passed:
// they interrupt nothing, and a blocked attempt between two steps of the same
// slide should not split it in two.
func (s *SlideCounter) Add(carID, dir int) {
	if carID != s.lastID || dir != s.lastDir {
		s.n++
	}
	s.lastID, s.lastDir = carID, dir
}

// N returns the number of slides so far.
func (s *SlideCounter) N() int { return s.n }
