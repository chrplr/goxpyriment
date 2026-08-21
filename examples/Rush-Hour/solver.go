// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "strings"

// Breadth-first optimal solver.
//
// It exists for three jobs, none of which is playing the game well: it checks
// that every board in puzzles.txt is solvable and that the declared move counts
// are right, it gives the environment an optimal-play oracle to validate agents
// against, and it supplies the potential for optional reward shaping.
//
// A move here is one slide of one vehicle over any distance — the unit
// puzzles.txt counts, not the one-cell step an agent takes.

// Slide is one vehicle moving any distance along its axis. Exactly one of DRow
// and DCol is non-zero.
type Slide struct {
	CarID      int
	DRow, DCol int
}

// Dir returns the one-cell direction of the slide, and its length in cells.
func (s Slide) Dir() (dir, cells int) {
	d := s.DCol
	if d == 0 {
		d = s.DRow
	}
	if d < 0 {
		return Left, -d
	}
	return Right, d
}

// MinMoves returns the length of the shortest solution in slides, or -1 if the
// board cannot be solved. The board is not modified.
func MinMoves(b *Board) int {
	slides, ok := SolveSlides(b)
	if !ok {
		return -1
	}
	return len(slides)
}

// SolveSlides returns one optimal sequence of slides and reports whether the
// board is solvable. An already-solved board yields an empty sequence and true.
// The board is not modified.
func SolveSlides(b *Board) ([]Slide, bool) {
	work := b.Clone()
	start := encodeState(work)

	seen := map[string]origin{start: {}}
	queue := []string{start}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		decodeState(work, cur)
		if work.Solved() {
			return replay(seen, start, cur), true
		}

		for id := range work.Cars {
			for _, dir := range []int{Left, Right} {
				for cells := 1; cells < GridSize; cells++ {
					decodeState(work, cur)
					car := work.Cars[id]
					fromR, fromC := car.Row, car.Col
					nr, nc := fromR, fromC
					if car.Horizontal {
						nc += dir * cells
					} else {
						nr += dir * cells
					}
					if nr < 0 || nc < 0 || nr >= GridSize || nc >= GridSize {
						break
					}
					// TryMove stops at the first obstacle and still reports
					// true, so arrival has to be checked: once this distance is
					// unreachable, every longer one is too.
					if !work.TryMove(car, nr, nc) || car.Row != nr || car.Col != nc {
						break
					}
					next := encodeState(work)
					if _, dup := seen[next]; dup {
						continue
					}
					seen[next] = origin{prev: cur, slide: Slide{CarID: id, DRow: nr - fromR, DCol: nc - fromC}}
					queue = append(queue, next)
				}
			}
		}
	}
	return nil, false
}

// origin records how a state was first reached, so an optimal path can be
// reconstructed once the goal is popped.
type origin struct {
	prev  string
	slide Slide
}

// replay walks the predecessor chain from goal back to start, then reverses it.
func replay(seen map[string]origin, start, goal string) []Slide {
	var reversed []Slide
	for s := goal; s != start; s = seen[s].prev {
		reversed = append(reversed, seen[s].slide)
	}
	slides := make([]Slide, len(reversed))
	for i, s := range reversed {
		slides[len(reversed)-1-i] = s
	}
	return slides
}

func encodeState(b *Board) string {
	var sb strings.Builder
	sb.Grow(2 * len(b.Cars))
	for _, c := range b.Cars {
		sb.WriteByte(byte('0' + c.Row))
		sb.WriteByte(byte('0' + c.Col))
	}
	return sb.String()
}

func decodeState(b *Board, s string) {
	for i, c := range b.Cars {
		c.Row = int(s[2*i] - '0')
		c.Col = int(s[2*i+1] - '0')
	}
}
