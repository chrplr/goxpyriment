// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func loadAll(t *testing.T) (sentences, nonwords []item) {
	t.Helper()
	var err error
	sentences, err = loadTable("stimuli/sentences_shuffled.csv", "S")
	if err != nil {
		t.Fatal(err)
	}
	nonwords, err = loadTable("stimuli/nonwords_shuffled.csv", "N")
	if err != nil {
		t.Fatal(err)
	}
	return sentences, nonwords
}

func TestLoadTables(t *testing.T) {
	for _, tc := range []struct {
		name, cond string
		n          int
	}{
		{"stimuli/sentences_shuffled.csv", "S", 139},
		{"stimuli/nonwords_shuffled.csv", "N", 139},
		// The training tables start with a UTF-8 byte-order mark.
		{"stimuli/training_sentences_shuffled.csv", "S", 101},
		{"stimuli/training_nonwords_shuffled.csv", "N", 101},
	} {
		items, err := loadTable(tc.name, tc.cond)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(items) != tc.n {
			t.Errorf("%s: %d items, want %d", tc.name, len(items), tc.n)
		}
	}
}

func TestLoadItemsRejects(t *testing.T) {
	for _, tc := range []struct{ name, csv string }{
		{"short item", "sentence,condition\nONE TWO THREE,S\n"},
		{"bad condition", "sentence,condition\nA B C D E F G H I J K L,X\n"},
		{"missing column", "words,condition\nA B C D E F G H I J K L,S\n"},
		{"no rows", "sentence,condition\n"},
	} {
		if _, err := loadItems(tc.name, []byte(tc.csv)); err == nil {
			t.Errorf("%s: accepted", tc.name)
		}
	}
}

func TestBuildRuns(t *testing.T) {
	sentences, nonwords := loadAll(t)
	runs, err := buildRuns(sentences, nonwords, 4, rand.New(rand.NewPCG(1, 999)))
	if err != nil {
		t.Fatal(err)
	}
	wantPerCond := []int{35, 35, 35, 34}
	seen := map[string]int{}
	for r, run := range runs {
		count := map[string]int{}
		for _, it := range run {
			count[it.cond]++
			seen[it.cond+strings.Join(it.words, " ")]++
		}
		if count["S"] != wantPerCond[r] || count["N"] != wantPerCond[r] {
			t.Errorf("run %d: %d S and %d N, want %d of each", r+1, count["S"], count["N"], wantPerCond[r])
		}
		if s := longestStreak(run); s > maxSameCond {
			t.Errorf("run %d: %d consecutive trials of one condition", r+1, s)
		}
	}
	if len(seen) != len(sentences)+len(nonwords) {
		t.Errorf("%d distinct items presented, want %d", len(seen), len(sentences)+len(nonwords))
	}
	for k, n := range seen {
		if n != 1 {
			t.Errorf("item presented %d times: %s", n, k)
		}
	}
}

// The runs are launched one at a time, so the same seed and subject must give
// the same lists, and another subject different ones.
func TestBuildRunsDeterministic(t *testing.T) {
	sentences, nonwords := loadAll(t)
	order := func(subject uint64) string {
		runs, err := buildRuns(sentences, nonwords, 4, rand.New(rand.NewPCG(1, subject)))
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, run := range runs {
			for _, it := range run {
				b.WriteString(it.cond + it.words[0] + it.words[1] + "|")
			}
		}
		return b.String()
	}
	first, again := order(7), order(7)
	if first != again {
		t.Error("same seed and subject gave different runs")
	}
	if first == order(8) {
		t.Error("subjects 7 and 8 got identical runs")
	}
}

func TestPracticeList(t *testing.T) {
	ts, err := loadTable("stimuli/training_sentences_shuffled.csv", "S")
	if err != nil {
		t.Fatal(err)
	}
	tn, err := loadTable("stimuli/training_nonwords_shuffled.csv", "N")
	if err != nil {
		t.Fatal(err)
	}
	list, err := practiceList(ts, tn, 5, rand.New(rand.NewPCG(1, 999)))
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, it := range list {
		count[it.cond]++
	}
	if count["S"] != 5 || count["N"] != 5 {
		t.Errorf("practice: %d S and %d N, want 5 of each", count["S"], count["N"])
	}
	if _, err := practiceList(ts, tn, 0, rand.New(rand.NewPCG(1, 1))); err == nil {
		t.Error("practice with 0 items per condition accepted")
	}
}

// TestInterleaveAcceptanceRate measures how often a plain shuffle of a 35+35
// run already satisfies the streak constraint; interleave's attempt bound and
// its comment rely on it.
func TestInterleaveAcceptanceRate(t *testing.T) {
	list := make([]item, 70)
	for i := range list {
		list[i].cond = "S"
		if i%2 == 1 {
			list[i].cond = "N"
		}
	}
	rng := rand.New(rand.NewPCG(3, 3))
	const n = 20000
	ok := 0
	for range n {
		rng.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
		if longestStreak(list) <= maxSameCond {
			ok++
		}
	}
	t.Logf("35+35 shuffles with no streak over %d: %d/%d (%.3f)", maxSameCond, ok, n, float64(ok)/n)
	if ok == 0 {
		t.Fatal("no shuffle satisfied the constraint")
	}
}
