// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math/rand"
	"testing"
)

// testWords returns n distinct placeholder words.
func testWords(n int) []string {
	w := make([]string, n)
	for i := range w {
		w[i] = fmt.Sprintf("W%04d", i)
	}
	return w
}

// checkStream asserts every invariant the experiment relies on.
func checkStream(t *testing.T, spec StreamSpec, events []Event) {
	t.Helper()

	if len(events) != spec.Len() {
		t.Fatalf("stream length = %d, want %d", len(events), spec.Len())
	}

	studyPos := map[int]int{}
	testPos := map[int]int{}
	perLagVF := map[int]map[string]int{}
	seen := map[string]int{} // word -> number of presentations
	newWords := map[string]bool{}
	studiedWords := map[string]bool{}

	for i, ev := range events {
		if ev.Word == "" {
			t.Fatalf("slot %d is empty", i)
		}
		seen[ev.Word]++
		switch ev.Kind {
		case KindStudy:
			if ev.PairID < 0 {
				t.Fatalf("slot %d: study event with no pair ID", i)
			}
			if prev, dup := studyPos[ev.PairID]; dup {
				t.Fatalf("pair %d studied twice (slots %d and %d)", ev.PairID, prev, i)
			}
			studyPos[ev.PairID] = i
			studiedWords[ev.Word] = true
			if ev.VF != "LVF" && ev.VF != "RVF" {
				t.Fatalf("slot %d: study event with VF %q", i, ev.VF)
			}
		case KindTest:
			if ev.IsOld {
				if prev, dup := testPos[ev.PairID]; dup {
					t.Fatalf("pair %d tested twice (slots %d and %d)", ev.PairID, prev, i)
				}
				testPos[ev.PairID] = i
				if perLagVF[ev.Lag] == nil {
					perLagVF[ev.Lag] = map[string]int{}
				}
				perLagVF[ev.Lag][ev.VF]++
			} else {
				if ev.PairID != -1 {
					t.Fatalf("slot %d: unstudied word with pair ID %d", i, ev.PairID)
				}
				if ev.Lag != 0 || ev.VF != "" {
					t.Fatalf("slot %d: unstudied word carries lag %d / VF %q", i, ev.Lag, ev.VF)
				}
				newWords[ev.Word] = true
			}
		}
	}

	if len(studyPos) != spec.NStudied() {
		t.Errorf("studied words = %d, want %d", len(studyPos), spec.NStudied())
	}
	if len(newWords) != spec.NNew {
		t.Errorf("unstudied words = %d, want %d", len(newWords), spec.NNew)
	}

	// The realised lag must equal the intended lag, and study must precede test.
	for pairID, s := range studyPos {
		tpos, ok := testPos[pairID]
		if !ok {
			t.Fatalf("pair %d studied at %d but never tested", pairID, s)
		}
		if tpos <= s {
			t.Fatalf("pair %d: test at %d does not follow study at %d", pairID, tpos, s)
		}
		if got, want := tpos-s, events[tpos].Lag; got != want {
			t.Errorf("pair %d: realised lag %d, logged lag %d", pairID, got, want)
		}
	}

	// Each lag condition gets PerLag items, split evenly between visual fields.
	for _, lag := range spec.Lags {
		vf := perLagVF[lag]
		if n := vf["LVF"] + vf["RVF"]; n != spec.PerLag {
			t.Errorf("lag %d: %d items, want %d", lag, n, spec.PerLag)
		}
		if vf["LVF"] != vf["RVF"] {
			t.Errorf("lag %d: %d LVF vs %d RVF, want an even split", lag, vf["LVF"], vf["RVF"])
		}
	}

	// A word is either studied-and-tested (twice) or unstudied (once) — never both.
	for word, n := range seen {
		want := 1
		if studiedWords[word] {
			want = 2
		}
		if n != want {
			t.Errorf("word %q appears %d times, want %d", word, n, want)
		}
		if studiedWords[word] && newWords[word] {
			t.Errorf("word %q is used both as studied and as unstudied", word)
		}
	}
}

func TestBuildStreamSpecs(t *testing.T) {
	specs := map[string]StreamSpec{
		"paper":    PaperSpec,
		"short":    ShortSpec,
		"practice": PracticeSpec,
	}
	for name, spec := range specs {
		for seed := int64(0); seed < 5; seed++ {
			t.Run(fmt.Sprintf("%s/seed%d", name, seed), func(t *testing.T) {
				events, err := BuildStream(testWords(spec.NWords()), spec, rand.New(rand.NewSource(seed)))
				if err != nil {
					t.Fatalf("BuildStream: %v", err)
				}
				checkStream(t, spec, events)
			})
		}
	}
}

// TestBuildStreamDeterministic checks that the same seed reproduces the same
// stream — the property the per-subject list assignment in main.go relies on to
// give two participants the same list with the visual fields swapped.
func TestBuildStreamDeterministic(t *testing.T) {
	words := testWords(PaperSpec.NWords())
	a, err := BuildStream(words, PaperSpec, rand.New(rand.NewSource(7)))
	if err != nil {
		t.Fatalf("BuildStream: %v", err)
	}
	b, err := BuildStream(words, PaperSpec, rand.New(rand.NewSource(7)))
	if err != nil {
		t.Fatalf("BuildStream: %v", err)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("slot %d differs between runs with the same seed: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// TestBuildStreamLagPositions checks that no lag condition is bunched at one end
// of the run — the paper distributed test items from each lag across the whole
// experimental run, and its footnote 1 reports per-lag mean list positions
// between 176 and 232 for a 400-presentation stream.
func TestBuildStreamLagPositions(t *testing.T) {
	const nRuns = 20
	sum := map[int]float64{}
	for seed := int64(0); seed < nRuns; seed++ {
		events, err := BuildStream(testWords(PaperSpec.NWords()), PaperSpec, rand.New(rand.NewSource(seed)))
		if err != nil {
			t.Fatalf("BuildStream: %v", err)
		}
		perLag := map[int][]int{}
		for i, ev := range events {
			if ev.Kind == KindTest && ev.IsOld {
				perLag[ev.Lag] = append(perLag[ev.Lag], i+1)
			}
		}
		for lag, positions := range perLag {
			total := 0
			for _, p := range positions {
				total += p
			}
			sum[lag] += float64(total) / float64(len(positions))
		}
	}
	for _, lag := range PaperSpec.Lags {
		mean := sum[lag] / nRuns
		if mean < 150 || mean > 260 {
			t.Errorf("lag %d: mean test position %.0f over %d runs, outside the plausible band [150, 260]",
				lag, mean, nRuns)
		}
	}
}

// TestBuildStreamRejectsBadSpecs checks the guards rather than letting a
// mis-specified design produce a silently wrong stream.
func TestBuildStreamRejectsBadSpecs(t *testing.T) {
	cases := map[string]struct {
		words int
		spec  StreamSpec
	}{
		"odd PerLag":      {words: 100, spec: StreamSpec{Lags: []int{1, 2}, PerLag: 3, NNew: 10}},
		"too few words":   {words: 10, spec: PaperSpec},
		"lag exceeds run": {words: 100, spec: StreamSpec{Lags: []int{100}, PerLag: 2, NNew: 4}},
		"no lags":         {words: 100, spec: StreamSpec{Lags: nil, PerLag: 2, NNew: 4}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildStream(testWords(tc.words), tc.spec, rand.New(rand.NewSource(1))); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}
