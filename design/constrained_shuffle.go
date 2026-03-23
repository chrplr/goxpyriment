// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package design

import (
	"fmt"
	"math/rand"
)

// Constraint controls repetitions in a constrained shuffle.
//
// A positive value p means: at most p consecutive rows (or trials) may share
// the same value in the constrained column/factor.
// For example, Constraint(1) means no two adjacent rows may have the same value.
//
// A negative value -g means: the index distance between any two rows sharing
// the same value must be at least g.
// For example, Constraint(-3) means at least 2 other rows must lie between any
// two rows that share the same value (positions differ by ≥ 3).
//
// Zero means unconstrained.
type Constraint int

// ShuffleTableConstrained shuffles the rows of table in place using a
// constructive greedy algorithm that respects per-column constraints.
// constraints[i] applies to column i; columns beyond len(constraints) are
// unconstrained. If maxAttempts is 0 a default of len(table) is used.
//
// The algorithm (adapted from https://github.com/chrplr/shuffle-go) works by
// performing an initial random shuffle and then, for each position in turn,
// scanning the remaining rows for one that satisfies all constraints, swapping
// it into place. If no valid row is found the whole attempt is restarted from
// a fresh random shuffle. This is repeated up to maxAttempts times.
//
// Returns a non-nil error if no valid ordering can be found.
func ShuffleTableConstrained(table [][]string, constraints []Constraint, maxAttempts int) error {
	n := len(table)
	if n == 0 || len(constraints) == 0 {
		rand.Shuffle(n, func(i, j int) { table[i], table[j] = table[j], table[i] })
		return nil
	}
	if maxAttempts <= 0 {
		maxAttempts = n
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		rand.Shuffle(n, func(i, j int) { table[i], table[j] = table[j], table[i] })

		success := true
		for pos := 1; pos < n; pos++ {
			placed := false
			for candidateIdx := pos; candidateIdx < n; candidateIdx++ {
				if tableRowFitsAt(table, constraints, pos, candidateIdx) {
					table[pos], table[candidateIdx] = table[candidateIdx], table[pos]
					placed = true
					break
				}
			}
			if !placed {
				success = false
				break
			}
		}
		if success {
			return nil
		}
	}

	return fmt.Errorf("design: ShuffleTableConstrained: no valid permutation found after %d attempts", maxAttempts)
}

// tableRowFitsAt reports whether placing table[candidateIdx] at position pos
// would satisfy all constraints given what has already been placed at 0..pos-1.
func tableRowFitsAt(table [][]string, constraints []Constraint, pos, candidateIdx int) bool {
	row := table[candidateIdx]
	for col, c := range constraints {
		if c == 0 || col >= len(row) {
			continue
		}
		val := row[col]

		if c > 0 { // max-consecutive-repetitions constraint
			run := 0
			for i := pos - 1; i >= 0; i-- {
				if col < len(table[i]) && table[i][col] == val {
					run++
				} else {
					break
				}
			}
			if run+1 > int(c) {
				return false
			}
		}

		if c < 0 { // min-gap constraint
			gap := int(-c)
			start := pos - gap + 1 // Constraint(-g) → index distance ≥ g
			if start < 0 {
				start = 0
			}
			for i := start; i < pos; i++ {
				if col < len(table[i]) && table[i][col] == val {
					return false
				}
			}
		}
	}
	return true
}

// ShuffleTrialsConstrained shuffles the block's trials in place while respecting
// per-factor constraints. constraints maps a factor name to a Constraint value.
// If maxAttempts is 0 a default of len(b.Trials) is used.
//
// Example — ensure no two consecutive trials share the same "condition" factor,
// and that "target" values are at least 3 trials apart:
//
//	err := block.ShuffleTrialsConstrained(map[string]design.Constraint{
//	    "condition": 1,   // no consecutive repetition
//	    "target":   -3,   // min gap of 3
//	}, 0)
//
// Returns a non-nil error if no valid ordering can be found.
func (b *Block) ShuffleTrialsConstrained(constraints map[string]Constraint, maxAttempts int) error {
	n := len(b.Trials)
	if n == 0 || len(constraints) == 0 {
		ShuffleList(b.Trials)
		return nil
	}
	if maxAttempts <= 0 {
		maxAttempts = n
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		rand.Shuffle(n, func(i, j int) { b.Trials[i], b.Trials[j] = b.Trials[j], b.Trials[i] })

		success := true
		for pos := 1; pos < n; pos++ {
			placed := false
			for candidateIdx := pos; candidateIdx < n; candidateIdx++ {
				if trialFitsAt(b.Trials, constraints, pos, candidateIdx) {
					b.Trials[pos], b.Trials[candidateIdx] = b.Trials[candidateIdx], b.Trials[pos]
					placed = true
					break
				}
			}
			if !placed {
				success = false
				break
			}
		}
		if success {
			return nil
		}
	}

	return fmt.Errorf("design: ShuffleTrialsConstrained: no valid permutation found after %d attempts", maxAttempts)
}

// trialFitsAt reports whether placing trials[candidateIdx] at position pos
// satisfies all constraints given what has already been placed at 0..pos-1.
func trialFitsAt(trials []*Trial, constraints map[string]Constraint, pos, candidateIdx int) bool {
	candidate := trials[candidateIdx]
	for factorName, c := range constraints {
		if c == 0 {
			continue
		}
		val, ok := candidate.Factors[factorName]
		if !ok {
			continue
		}

		if c > 0 { // max-consecutive-repetitions constraint
			run := 0
			for i := pos - 1; i >= 0; i-- {
				if trials[i].Factors[factorName] == val {
					run++
				} else {
					break
				}
			}
			if run+1 > int(c) {
				return false
			}
		}

		if c < 0 { // min-gap constraint
			gap := int(-c)
			start := pos - gap + 1 // Constraint(-g) → index distance ≥ g
			if start < 0 {
				start = 0
			}
			for i := start; i < pos; i++ {
				if trials[i].Factors[factorName] == val {
					return false
				}
			}
		}
	}
	return true
}
