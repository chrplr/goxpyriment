// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "testing"

// The solved sequence must actually solve the board, in exactly the declared
// number of slides. This is the oracle the agent environment is validated
// against, so it has to be right on every shipped puzzle, not just the easy end.
func TestSolveSlidesSolvesEveryPuzzle(t *testing.T) {
	puzzles, err := DefaultPuzzles()
	if err != nil {
		t.Fatalf("DefaultPuzzles: %v", err)
	}

	for _, p := range puzzles {
		slides, ok := SolveSlides(p.Fresh())
		if !ok {
			t.Errorf("%s: reported unsolvable", p.Name)
			continue
		}
		if p.MinMoves != 0 && len(slides) != p.MinMoves {
			t.Errorf("%s: solution of %d slides, puzzles.txt declares %d",
				p.Name, len(slides), p.MinMoves)
		}

		b := p.Fresh()
		for i, s := range slides {
			car := b.Cars[s.CarID]
			nr, nc := car.Row+s.DRow, car.Col+s.DCol
			if !b.TryMove(car, nr, nc) || car.Row != nr || car.Col != nc {
				t.Fatalf("%s: slide %d (%+v) is not legal\n%s", p.Name, i, s, b.String())
			}
		}
		if !b.Solved() {
			t.Errorf("%s: the solution left the board unsolved\n%s", p.Name, b.String())
		}
	}
}

// SolveSlides must not disturb the board it is given: callers hold live state.
func TestSolveSlidesLeavesTheBoardAlone(t *testing.T) {
	b, err := ParseBoard(classic)
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	before := b.String()
	if _, ok := SolveSlides(b); !ok {
		t.Fatal("the classic board should be solvable")
	}
	if after := b.String(); after != before {
		t.Errorf("board changed:\n%s\nwant\n%s", after, before)
	}
}

// Expanding a slide into one-cell steps — what the environment does to replay an
// optimal solution as agent actions — must reach the same board.
func TestSlidesExpandIntoSteps(t *testing.T) {
	puzzles, err := DefaultPuzzles()
	if err != nil {
		t.Fatalf("DefaultPuzzles: %v", err)
	}

	for _, p := range puzzles[:12] {
		slides, ok := SolveSlides(p.Fresh())
		if !ok {
			t.Fatalf("%s: unsolvable", p.Name)
		}

		b := p.Fresh()
		var counter SlideCounter
		for _, s := range slides {
			dir, cells := s.Dir()
			for i := 0; i < cells; i++ {
				if !b.Step(b.Cars[s.CarID], dir) {
					t.Fatalf("%s: step %d of slide %+v refused\n%s", p.Name, i, s, b.String())
				}
				counter.Add(s.CarID, dir)
			}
		}
		if !b.Solved() {
			t.Errorf("%s: stepwise replay did not solve the board", p.Name)
		}
		// The counter must recover the slide count from the step stream: that
		// equivalence is what makes agent and human move counts comparable.
		if counter.N() != len(slides) {
			t.Errorf("%s: SlideCounter saw %d slides, want %d", p.Name, counter.N(), len(slides))
		}
	}
}

// An unsolvable board is reported as such rather than looping or panicking.
func TestSolveSlidesUnsolvable(t *testing.T) {
	// The red car is walled in by three-cell vehicles that cannot move either.
	b, err := ParseBoard("oooDoo oooDoo AAoDoo oooEoo oooEoo oooEoo")
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	if _, ok := SolveSlides(b); ok {
		t.Error("board reported solvable")
	}
	if n := MinMoves(b); n != -1 {
		t.Errorf("MinMoves = %d, want -1", n)
	}
}
