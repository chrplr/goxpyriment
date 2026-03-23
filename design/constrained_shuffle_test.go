// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package design

import (
	"fmt"
	"testing"
)

// --- helpers ---

// checkTableMaxConsecutive returns the longest run of identical values in column col.
func checkTableMaxConsecutive(table [][]string, col int) int {
	max, run := 1, 1
	for i := 1; i < len(table); i++ {
		if col < len(table[i]) && col < len(table[i-1]) && table[i][col] == table[i-1][col] {
			run++
			if run > max {
				max = run
			}
		} else {
			run = 1
		}
	}
	return max
}

// checkTableMinGap returns the minimum distance between any two rows that share
// the same value in column col (returns len(table) when no value repeats).
func checkTableMinGap(table [][]string, col int) int {
	minGap := len(table)
	last := map[string]int{}
	for i, row := range table {
		if col >= len(row) {
			continue
		}
		v := row[col]
		if prev, ok := last[v]; ok {
			if i-prev < minGap {
				minGap = i - prev
			}
		}
		last[v] = i
	}
	return minGap
}

// checkTrialsMaxConsecutive mirrors checkTableMaxConsecutive for []*Trial.
func checkTrialsMaxConsecutive(trials []*Trial, factor string) int {
	if len(trials) == 0 {
		return 0
	}
	max, run := 1, 1
	for i := 1; i < len(trials); i++ {
		if trials[i].Factors[factor] == trials[i-1].Factors[factor] {
			run++
			if run > max {
				max = run
			}
		} else {
			run = 1
		}
	}
	return max
}

// checkTrialsMinGap mirrors checkTableMinGap for []*Trial.
func checkTrialsMinGap(trials []*Trial, factor string) int {
	minGap := len(trials)
	last := map[interface{}]int{}
	for i, t := range trials {
		v := t.Factors[factor]
		if prev, ok := last[v]; ok {
			if i-prev < minGap {
				minGap = i - prev
			}
		}
		last[v] = i
	}
	return minGap
}

// makeTable builds a [][]string by repeating each value 'reps' times as a single column.
func makeTable(values []string, reps int) [][]string {
	table := make([][]string, 0, len(values)*reps)
	for _, v := range values {
		for i := 0; i < reps; i++ {
			table = append(table, []string{v})
		}
	}
	return table
}

// makeBlock builds a Block with one trial per entry in values, setting the
// named factor to the corresponding value.
func makeBlock(factor string, values []string) *Block {
	b := NewBlock("test")
	for _, v := range values {
		t := NewTrial()
		t.SetFactor(factor, v)
		b.AddTrial(t, 1, false)
	}
	return b
}

// --- ShuffleTableConstrained ---

func TestShuffleTableConstrained_EmptyTable(t *testing.T) {
	var table [][]string
	if err := ShuffleTableConstrained(table, []Constraint{1}, 10); err != nil {
		t.Fatalf("unexpected error on empty table: %v", err)
	}
}

func TestShuffleTableConstrained_NoConstraints(t *testing.T) {
	table := makeTable([]string{"A", "B", "C"}, 2)
	orig := make([][]string, len(table))
	copy(orig, table)
	if err := ShuffleTableConstrained(table, nil, 10); err != nil {
		t.Fatalf("unexpected error with no constraints: %v", err)
	}
	if len(table) != len(orig) {
		t.Fatalf("row count changed: got %d, want %d", len(table), len(orig))
	}
}

func TestShuffleTableConstrained_MaxConsecutive1(t *testing.T) {
	// 3 A's and 3 B's: with Constraint(1) no two adjacent rows may share col 0.
	// The only valid orderings alternate A/B. Run many times to stress the RNG.
	for iter := 0; iter < 50; iter++ {
		table := makeTable([]string{"A", "A", "A", "B", "B", "B"}, 1)
		if err := ShuffleTableConstrained(table, []Constraint{1}, 200); err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if got := checkTableMaxConsecutive(table, 0); got > 1 {
			t.Fatalf("iter %d: max consecutive = %d, want ≤ 1; table = %v", iter, got, table)
		}
	}
}

func TestShuffleTableConstrained_MaxConsecutive2(t *testing.T) {
	// 4 A's and 4 B's: Constraint(2) allows runs of at most 2.
	for iter := 0; iter < 50; iter++ {
		table := makeTable([]string{"A", "A", "A", "A", "B", "B", "B", "B"}, 1)
		if err := ShuffleTableConstrained(table, []Constraint{2}, 200); err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if got := checkTableMaxConsecutive(table, 0); got > 2 {
			t.Fatalf("iter %d: max consecutive = %d, want ≤ 2; table = %v", iter, got, table)
		}
	}
}

func TestShuffleTableConstrained_MinGap(t *testing.T) {
	// 3 A's, 3 B's, 3 C's: Constraint(-3) requires a gap of at least 3.
	for iter := 0; iter < 50; iter++ {
		table := makeTable([]string{"A", "A", "A", "B", "B", "B", "C", "C", "C"}, 1)
		if err := ShuffleTableConstrained(table, []Constraint{-3}, 500); err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if got := checkTableMinGap(table, 0); got < 3 {
			t.Fatalf("iter %d: min gap = %d, want ≥ 3; table = %v", iter, got, table)
		}
	}
}

func TestShuffleTableConstrained_MultiColumn(t *testing.T) {
	// Two columns, each with independent constraints.
	// Col 0: no consecutive repeats (Constraint 1).
	// Col 1: min gap of 2 (Constraint -2).
	for iter := 0; iter < 50; iter++ {
		table := [][]string{
			{"A", "X"}, {"A", "Y"}, {"A", "Z"},
			{"B", "X"}, {"B", "Y"}, {"B", "Z"},
			{"C", "X"}, {"C", "Y"}, {"C", "Z"},
		}
		if err := ShuffleTableConstrained(table, []Constraint{1, -2}, 500); err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if got := checkTableMaxConsecutive(table, 0); got > 1 {
			t.Fatalf("iter %d: col-0 max consecutive = %d, want ≤ 1", iter, got)
		}
		if got := checkTableMinGap(table, 1); got < 2 {
			t.Fatalf("iter %d: col-1 min gap = %d, want ≥ 2", iter, got)
		}
	}
}

func TestShuffleTableConstrained_ZeroConstraintIsIgnored(t *testing.T) {
	// Constraint(0) means unconstrained; should never cause an error.
	table := makeTable([]string{"A", "A", "A", "B", "B", "B"}, 1)
	if err := ShuffleTableConstrained(table, []Constraint{0}, 10); err != nil {
		t.Fatalf("unexpected error with zero constraint: %v", err)
	}
}

func TestShuffleTableConstrained_DefaultMaxAttempts(t *testing.T) {
	// maxAttempts=0 should use the default (len(table)); should still succeed.
	table := makeTable([]string{"A", "A", "A", "B", "B", "B"}, 1)
	if err := ShuffleTableConstrained(table, []Constraint{1}, 0); err != nil {
		t.Fatalf("unexpected error with maxAttempts=0: %v", err)
	}
}

func TestShuffleTableConstrained_PreservesRows(t *testing.T) {
	// All original rows must still be present after shuffling.
	orig := [][]string{
		{"A", "1"}, {"B", "2"}, {"C", "3"}, {"A", "4"}, {"B", "5"}, {"C", "6"},
	}
	table := make([][]string, len(orig))
	copy(table, orig)

	if err := ShuffleTableConstrained(table, []Constraint{1}, 200); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range table {
		seen[fmt.Sprintf("%v", row)] = true
	}
	for _, row := range orig {
		key := fmt.Sprintf("%v", row)
		if !seen[key] {
			t.Fatalf("row %v missing after shuffle", row)
		}
	}
}

func TestShuffleTableConstrained_ImpossibleReturnsError(t *testing.T) {
	// 4 A's and 1 B: Constraint(1) is impossible (can't avoid consecutive A's).
	table := makeTable([]string{"A", "A", "A", "A", "B"}, 1)
	err := ShuffleTableConstrained(table, []Constraint{1}, 50)
	if err == nil {
		t.Fatalf("expected error for impossible constraint, got nil; table = %v", table)
	}
}

// --- ShuffleTrialsConstrained ---

func TestShuffleTrialsConstrained_EmptyBlock(t *testing.T) {
	b := NewBlock("empty")
	if err := b.ShuffleTrialsConstrained(map[string]Constraint{"cond": 1}, 10); err != nil {
		t.Fatalf("unexpected error on empty block: %v", err)
	}
}

func TestShuffleTrialsConstrained_NoConstraints(t *testing.T) {
	b := makeBlock("cond", []string{"A", "B", "A", "B"})
	if err := b.ShuffleTrialsConstrained(map[string]Constraint{}, 10); err != nil {
		t.Fatalf("unexpected error with no constraints: %v", err)
	}
	if len(b.Trials) != 4 {
		t.Fatalf("trial count changed: got %d, want 4", len(b.Trials))
	}
}

func TestShuffleTrialsConstrained_MaxConsecutive1(t *testing.T) {
	for iter := 0; iter < 50; iter++ {
		b := makeBlock("cond", []string{"go", "go", "go", "nogo", "nogo", "nogo"})
		if err := b.ShuffleTrialsConstrained(map[string]Constraint{"cond": 1}, 200); err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if got := checkTrialsMaxConsecutive(b.Trials, "cond"); got > 1 {
			t.Fatalf("iter %d: max consecutive = %d, want ≤ 1", iter, got)
		}
	}
}

func TestShuffleTrialsConstrained_MaxConsecutive2(t *testing.T) {
	for iter := 0; iter < 50; iter++ {
		b := makeBlock("cond", []string{"A", "A", "A", "A", "B", "B", "B", "B"})
		if err := b.ShuffleTrialsConstrained(map[string]Constraint{"cond": 2}, 200); err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if got := checkTrialsMaxConsecutive(b.Trials, "cond"); got > 2 {
			t.Fatalf("iter %d: max consecutive = %d, want ≤ 2", iter, got)
		}
	}
}

func TestShuffleTrialsConstrained_MinGap(t *testing.T) {
	for iter := 0; iter < 50; iter++ {
		b := makeBlock("target", []string{"cat", "cat", "cat", "dog", "dog", "dog", "fox", "fox", "fox"})
		if err := b.ShuffleTrialsConstrained(map[string]Constraint{"target": -3}, 500); err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if got := checkTrialsMinGap(b.Trials, "target"); got < 3 {
			t.Fatalf("iter %d: min gap = %d, want ≥ 3", iter, got)
		}
	}
}

func TestShuffleTrialsConstrained_MultipleFactor(t *testing.T) {
	// Two factors with independent constraints.
	for iter := 0; iter < 50; iter++ {
		b := NewBlock("multi")
		conds := []string{"go", "go", "go", "nogo", "nogo", "nogo"}
		targets := []string{"cat", "dog", "fox", "cat", "dog", "fox"}
		for i := range conds {
			tr := NewTrial()
			tr.SetFactor("cond", conds[i])
			tr.SetFactor("target", targets[i])
			b.AddTrial(tr, 1, false)
		}

		err := b.ShuffleTrialsConstrained(map[string]Constraint{
			"cond":   1,  // no consecutive same condition
			"target": -2, // target values at least 2 apart
		}, 500)
		if err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if got := checkTrialsMaxConsecutive(b.Trials, "cond"); got > 1 {
			t.Fatalf("iter %d: cond max consecutive = %d, want ≤ 1", iter, got)
		}
		if got := checkTrialsMinGap(b.Trials, "target"); got < 2 {
			t.Fatalf("iter %d: target min gap = %d, want ≥ 2", iter, got)
		}
	}
}

func TestShuffleTrialsConstrained_MissingFactorIgnored(t *testing.T) {
	// Trials that lack the constrained factor should not cause a crash.
	b := NewBlock("partial")
	for _, v := range []string{"A", "B", "A", "B"} {
		tr := NewTrial()
		tr.SetFactor("cond", v)
		// "other" factor is absent
		b.AddTrial(tr, 1, false)
	}
	if err := b.ShuffleTrialsConstrained(map[string]Constraint{"other": 1}, 20); err != nil {
		t.Fatalf("unexpected error when factor is absent: %v", err)
	}
}

func TestShuffleTrialsConstrained_PreservesAllTrials(t *testing.T) {
	// The set of trials must be unchanged (same factors, same count).
	values := []string{"A", "A", "A", "B", "B", "B"}
	b := makeBlock("cond", values)
	if err := b.ShuffleTrialsConstrained(map[string]Constraint{"cond": 1}, 200); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := map[interface{}]int{}
	for _, tr := range b.Trials {
		count[tr.Factors["cond"]]++
	}
	if count["A"] != 3 || count["B"] != 3 {
		t.Fatalf("trial counts wrong after shuffle: %v", count)
	}
}

func TestShuffleTrialsConstrained_DefaultMaxAttempts(t *testing.T) {
	b := makeBlock("cond", []string{"go", "go", "go", "nogo", "nogo", "nogo"})
	if err := b.ShuffleTrialsConstrained(map[string]Constraint{"cond": 1}, 0); err != nil {
		t.Fatalf("unexpected error with maxAttempts=0: %v", err)
	}
}

func TestShuffleTrialsConstrained_ImpossibleReturnsError(t *testing.T) {
	// 4 A's and 1 B: Constraint(1) cannot be satisfied.
	b := makeBlock("cond", []string{"A", "A", "A", "A", "B"})
	err := b.ShuffleTrialsConstrained(map[string]Constraint{"cond": 1}, 50)
	if err == nil {
		t.Fatalf("expected error for impossible constraint, got nil")
	}
}

func TestShuffleTrialsConstrained_IntFactorValues(t *testing.T) {
	// Factors are interface{}, test that integer values are compared correctly.
	for iter := 0; iter < 50; iter++ {
		b := NewBlock("int")
		for _, v := range []int{1, 1, 1, 2, 2, 2} {
			tr := NewTrial()
			tr.SetFactor("level", v)
			b.AddTrial(tr, 1, false)
		}
		if err := b.ShuffleTrialsConstrained(map[string]Constraint{"level": 1}, 200); err != nil {
			t.Fatalf("iter %d: unexpected error: %v", iter, err)
		}
		if got := checkTrialsMaxConsecutive(b.Trials, "level"); got > 1 {
			t.Fatalf("iter %d: max consecutive = %d, want ≤ 1", iter, got)
		}
	}
}
