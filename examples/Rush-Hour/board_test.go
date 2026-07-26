// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"strings"
	"testing"
)

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
	puzzles, err := ParsePuzzleFile(puzzleFile)
	if err != nil {
		t.Fatalf("ParsePuzzleFile: %v", err)
	}
	if len(puzzles) == 0 {
		t.Fatal("no puzzles in puzzles.txt")
	}

	prev := 0
	seen := make(map[string]string, len(puzzles))
	for _, p := range puzzles {
		n := minMoves(p.Fresh())
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

// ── Breadth-first solver, used only by the tests ─────────────────────────────

func encodeState(b *Board) string {
	var sb strings.Builder
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

// minMoves returns the length of the shortest solution, counting each slide of
// one vehicle (of any distance) as one move. Returns -1 if unsolvable.
func minMoves(b *Board) int {
	start := encodeState(b)
	dist := map[string]int{start: 0}
	queue := []string{start}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		d := dist[cur]

		decodeState(b, cur)
		if b.Solved() {
			return d
		}

		for id := range b.Cars {
			for _, delta := range []int{-1, 1} {
				for step := 1; step < GridSize; step++ {
					decodeState(b, cur)
					car := b.Cars[id]
					nr, nc := car.Row, car.Col
					if car.Horizontal {
						nc += delta * step
					} else {
						nr += delta * step
					}
					if nr < 0 || nc < 0 || nr >= GridSize || nc >= GridSize {
						break
					}
					if !b.TryMove(car, nr, nc) || car.Row != nr || car.Col != nc {
						break // wall or vehicle in the way: no longer slide possible
					}
					next := encodeState(b)
					if _, seen := dist[next]; !seen {
						dist[next] = d + 1
						queue = append(queue, next)
					}
				}
			}
		}
	}
	return -1
}
