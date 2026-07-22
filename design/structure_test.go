package design

import (
	"testing"
)

// TestExperimentStructure verifies that blocks and trials are correctly added.
func TestExperimentStructure(t *testing.T) {
	exp := NewExperiment("Test Exp")

	block := NewBlock("Block 1")
	trial := NewTrial()
	trial.SetFactor("type", "target")

	block.AddTrial(trial, 10, false)
	if len(block.Trials) != 10 {
		t.Errorf("Expected 10 trials in block, got %d", len(block.Trials))
	}

	exp.AddBlock(block, 2)
	if len(exp.Blocks) != 2 {
		t.Errorf("Expected 2 blocks in experiment, got %d", len(exp.Blocks))
	}

	// Verify deep copy
	if exp.Blocks[0].Trials[0] == exp.Blocks[1].Trials[0] {
		t.Error("Trials were not deep-copied between blocks")
	}
}

// TestGetPermutedBWSFactorCondition verifies 1-based counterbalancing
// assignment wraps with the number of conditions and never panics — in
// particular for the default subject 0 and negative IDs, which previously
// produced a -1 index and an out-of-range panic.
func TestGetPermutedBWSFactorCondition(t *testing.T) {
	exp := NewExperiment("BWS Test")
	conds := []interface{}{"A", "B", "C"}
	exp.AddBWSFactor("hand", conds)

	// 1-based: subject 1 -> first condition, wrapping every len(conds).
	cases := map[int]interface{}{
		1:  "A",
		2:  "B",
		3:  "C",
		4:  "A", // wraps
		0:  "C", // default subject ID: must not panic (floored modulo)
		-1: "B", // negative ID: normalized into range
	}
	for subj, want := range cases {
		if got := exp.GetPermutedBWSFactorCondition("hand", subj); got != want {
			t.Errorf("subject %d: got %v, want %v", subj, got, want)
		}
	}

	// Unknown factor returns nil rather than panicking.
	if got := exp.GetPermutedBWSFactorCondition("missing", 0); got != nil {
		t.Errorf("unknown factor: got %v, want nil", got)
	}
}

// TestTrialFactors verifies factor setting and getting.
func TestTrialFactors(t *testing.T) {
	trial := NewTrial()
	trial.SetFactor("intensity", 0.5)
	trial.SetFactor("label", "high")

	if trial.GetFactor("intensity") != 0.5 {
		t.Errorf("Expected intensity 0.5, got %v", trial.GetFactor("intensity"))
	}

	if trial.GetFactor("label") != "high" {
		t.Errorf("Expected label 'high', got %v", trial.GetFactor("label"))
	}

	if trial.GetFactor("unknown") != nil {
		t.Error("Expected nil for unknown factor")
	}
}

// TestLatinSquare verifies the between-subjects counterbalancing logic.
func TestLatinSquare(t *testing.T) {
	exp := NewExperiment("Counterbalancing")

	// Factor A: 2 conditions
	exp.AddBWSFactor("Group", []interface{}{"A", "B"})

	// Subject 1 -> Condition 0 (A)
	c1 := exp.GetPermutedBWSFactorCondition("Group", 1)
	if c1 != "A" {
		t.Errorf("Subject 1: expected condition A, got %v", c1)
	}

	// Subject 2 -> Condition 1 (B)
	c2 := exp.GetPermutedBWSFactorCondition("Group", 2)
	if c2 != "B" {
		t.Errorf("Subject 2: expected condition B, got %v", c2)
	}

	// Subject 3 -> Condition 0 (A) (Wrap around)
	c3 := exp.GetPermutedBWSFactorCondition("Group", 3)
	if c3 != "A" {
		t.Errorf("Subject 3: expected condition A (wrapped), got %v", c3)
	}
}

// TestShuffleList verifies that ShuffleList randomizes order.
func TestShuffleList(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	original := make([]int, len(items))
	copy(original, items)

	ShuffleList(items)

	// In very rare cases, the shuffle might result in the same order,
	// but for 10 items, it's 1/10! chance.
	same := true
	for i := range items {
		if items[i] != original[i] {
			same = false
			break
		}
	}

	if same {
		t.Error("ShuffleList did not change the order of 10 items")
	}

	// Ensure all original items are still present
	for _, orig := range original {
		found := false
		for _, item := range items {
			if item == orig {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Lost item %d during shuffle", orig)
		}
	}
}

// TestRepeatList verifies that RepeatList concatenates count copies of the input,
// mimicking slices.Repeat.
func TestRepeatList(t *testing.T) {
	got := RepeatList([]int{1, 2, 3}, 3)
	want := []int{1, 2, 3, 1, 2, 3, 1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("RepeatList length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("RepeatList[%d] = %d, want %d", i, got[i], want[i])
		}
	}

	// count == 0 yields an empty, non-nil slice (as slices.Repeat does).
	if got := RepeatList([]int{1, 2, 3}, 0); len(got) != 0 {
		t.Errorf("RepeatList with count 0 = %v, want empty", got)
	}

	// An empty input yields an empty result regardless of count.
	if got := RepeatList([]int{}, 5); len(got) != 0 {
		t.Errorf("RepeatList of empty slice = %v, want empty", got)
	}

	// The result must be independent of the source: mutating the input
	// afterwards must not change the returned slice.
	src := []int{7, 8}
	out := RepeatList(src, 2)
	src[0] = 99
	if out[0] != 7 || out[2] != 7 {
		t.Errorf("RepeatList result aliases the input: got %v", out)
	}

	// A negative count must panic, matching slices.Repeat.
	func() {
		defer func() {
			if recover() == nil {
				t.Error("RepeatList with negative count did not panic")
			}
		}()
		RepeatList([]int{1}, -1)
	}()
}
