// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "testing"

// The board of the original pygame implementation (rush_hour/rush.py).
const classic = "BCCCoo BoooDo oAAEDo oooEoo FFoEoo ooGGGo"

func TestParseBoardClassic(t *testing.T) {
	b, err := ParseBoard(classic)
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	if len(b.Cars) != 7 {
		t.Fatalf("got %d cars, want 7", len(b.Cars))
	}

	target := b.Target()
	if target == nil || !target.Horizontal || target.Row != 2 || target.Col != 1 || target.Length != 2 {
		t.Fatalf("target = %+v, want horizontal 2-cell car at (2,1)", target)
	}

	// Round-trips through the notation.
	want := "BCCCoo\nBoooDo\noAAEDo\noooEoo\nFFoEoo\nooGGGo"
	if got := b.String(); got != want {
		t.Errorf("String() =\n%s\nwant\n%s", got, want)
	}
}

func TestParseBoardRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"too short":        "BCCCoo BoooDo oAAEDo oooEoo FFoEoo ooGGG",
		"bad character":    "BCCCo1 BoooDo oAAEDo oooEoo FFoEoo ooGGGo",
		"car of one cell":  "BCCCoo ooooDo oAAEDo oooEoo FFoEoo ooGGGo",
		"car of four":      "BBBBoo oooooo oAAooo oooooo oooooo oooooo",
		"non contiguous":   "BoBooo oooooo oAAooo oooooo oooooo oooooo",
		"L shaped":         "BBoooo Booooo oAAooo oooooo oooooo oooooo",
		"no target":        "BBoooo oooooo oooooo oooooo oooooo oooooo",
		"vertical target":  "oooooo Aooooo Aooooo oooooo oooooo oooooo",
		"target wrong row": "AAoooo oooooo oooooo oooooo oooooo oooooo",
		"already solved":   "oooooo oooooo ooooAA oooooo oooooo oooooo",
	}
	for name, spec := range cases {
		if _, err := ParseBoard(spec); err == nil {
			t.Errorf("%s: ParseBoard accepted %q, want an error", name, spec)
		}
	}
}

func TestTryMove(t *testing.T) {
	b, err := ParseBoard(classic)
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	target := b.Target() // horizontal, row 2, cols 1-2

	// Off-axis targets are rejected outright.
	if b.TryMove(target, 3, 1) {
		t.Error("moving a horizontal car off its row should be refused")
	}

	// Sliding left stops at the wall.
	if !b.TryMove(target, 2, 0) || target.Col != 0 {
		t.Errorf("target at col %d, want 0", target.Col)
	}

	// Sliding right parks flush against E at (2,3) — even though col 5 was asked.
	if !b.TryMove(target, 2, 5) {
		t.Fatal("expected the target to slide right")
	}
	if target.Col != 1 {
		t.Errorf("target parked at col %d, want 1 (blocked by E at col 3)", target.Col)
	}

	// A move that changes nothing reports false.
	if b.TryMove(target, 2, 5) {
		t.Error("a blocked move should report false")
	}
}

func TestSolvedRequiresRightWall(t *testing.T) {
	b, err := ParseBoard("oooooo oooooo AAoooo oooooo oooooo oooooo")
	if err != nil {
		t.Fatalf("ParseBoard: %v", err)
	}
	if b.Solved() {
		t.Fatal("board reported solved at the starting position")
	}
	b.TryMove(b.Target(), 2, 4)
	if !b.Solved() {
		t.Fatal("board should be solved once the target reaches the right wall")
	}
}

// TestEmbeddedPuzzles checks that every puzzle shipped in puzzles.txt parses
// and is actually solvable, and reports the minimum number of moves so the
// file's difficulty ordering can be verified.
func TestEmbeddedPuzzles(t *testing.T) {
	puzzles, err := ParsePuzzleFile(PuzzleFile)
	if err != nil {
		t.Fatalf("ParsePuzzleFile: %v", err)
	}
	if len(puzzles) == 0 {
		t.Fatal("no puzzles in puzzles.txt")
	}

	prev := 0
	seen := make(map[string]string, len(puzzles))
	for _, p := range puzzles {
		n := MinMoves(p.Fresh())
		if n < 0 {
			t.Errorf("%s: unsolvable\n%s", p.Name, p.Board.String())
			continue
		}
		t.Logf("%s: %d moves", p.Name, n)

		if p.MinMoves != 0 && p.MinMoves != n {
			t.Errorf("%s: puzzles.txt declares %d moves, solver finds %d", p.Name, p.MinMoves, n)
		}
		if n < prev {
			t.Errorf("%s needs %d moves but follows a puzzle needing %d — puzzles.txt should be ordered easy to hard",
				p.Name, n, prev)
		}
		prev = n

		if other, dup := seen[p.Board.String()]; dup {
			t.Errorf("%s duplicates %s", p.Name, other)
		}
		seen[p.Board.String()] = p.Name
	}
}
