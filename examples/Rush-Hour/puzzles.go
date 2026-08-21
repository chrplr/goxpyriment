// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	_ "embed"
	"os"
	"sync"
)

// PuzzleFile is the shipped puzzle library, compiled into the binary. The embed
// directive has to sit next to the data, which is why puzzles.txt lives in this
// package rather than at the repository root.
//
//go:embed puzzles.txt
var PuzzleFile string

var defaultPuzzles = sync.OnceValues(func() ([]Puzzle, error) {
	return ParsePuzzleFile(PuzzleFile)
})

// DefaultPuzzles returns the embedded library, parsed once and shared. Callers
// must not mutate the returned Puzzle.Board — use Puzzle.Fresh for a board to
// play on.
func DefaultPuzzles() ([]Puzzle, error) { return defaultPuzzles() }

// LoadPuzzleFile reads an alternative library from disk, for custom curricula.
func LoadPuzzleFile(path string) ([]Puzzle, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePuzzleFile(string(content))
}
