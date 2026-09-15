// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math/rand"
	"testing"
)

// TestEmbeddedCorpus checks the invariants the experiment relies on: 16 concept
// groups of 10 tasks, 16 minimal tasks, three test inputs each, valid grids.
func TestEmbeddedCorpus(t *testing.T) {
	tasks, err := LoadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 176 {
		t.Errorf("%d tasks, want 176", len(tasks))
	}
	perConcept := map[string]int{}
	minimalPerConcept := map[string]int{}
	for _, task := range tasks {
		if task.Minimal {
			minimalPerConcept[task.Concept]++
		} else {
			perConcept[task.Concept]++
		}
		if len(task.Test) != testsPerTask {
			t.Errorf("%s: %d test inputs", task.Name, len(task.Test))
		}
	}
	concepts := Concepts(tasks)
	if len(concepts) != 16 {
		t.Errorf("%d concepts, want 16: %v", len(concepts), concepts)
	}
	for _, c := range concepts {
		if perConcept[c] != 10 {
			t.Errorf("concept %s has %d tasks, want 10", c, perConcept[c])
		}
		if minimalPerConcept[c] != 1 {
			t.Errorf("concept %s has %d minimal tasks, want 1", c, minimalPerConcept[c])
		}
	}
}

func TestSplitName(t *testing.T) {
	cases := map[string]struct {
		concept string
		minimal bool
	}{
		"AboveBelow7":        {"AboveBelow", false},
		"TopBottom3D10":      {"TopBottom3D", false},
		"CenterMinimal":      {"Center", true},
		"TopBottom2DMinimal": {"TopBottom2D", true},
	}
	for name, want := range cases {
		c, m := splitName(name)
		if c != want.concept || m != want.minimal {
			t.Errorf("splitName(%q) = (%q, %v), want (%q, %v)", name, c, m, want.concept, want.minimal)
		}
	}
}

func TestBuildSessionDefault(t *testing.T) {
	tasks, err := LoadTasks()
	if err != nil {
		t.Fatal(err)
	}
	specs, err := BuildSession(tasks, DefaultSessionOptions(), rand.New(rand.NewSource(7)))
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 17 {
		t.Fatalf("%d trials, want 17", len(specs))
	}
	kinds := map[string]int{}
	seenTask := map[*Task]bool{}
	seenConcept := map[string]bool{}
	for i, s := range specs {
		kinds[s.Kind]++
		if seenTask[s.Task] {
			t.Errorf("trial %d repeats task %s", i, s.Task.Name)
		}
		seenTask[s.Task] = true
		if s.TestIndex < 0 || s.TestIndex >= testsPerTask {
			t.Errorf("trial %d: test index %d", i, s.TestIndex)
		}
		switch {
		case i < 2:
			if s.Kind != KindTraining || s.MaxAttempts != 0 {
				t.Errorf("trial %d should be unlimited training, got %s/%d", i, s.Kind, s.MaxAttempts)
			}
		default:
			if s.MaxAttempts != 3 {
				t.Errorf("trial %d: %d attempts, want 3", i, s.MaxAttempts)
			}
		}
		if s.Kind == KindCorpus {
			if seenConcept[s.Task.Concept] {
				t.Errorf("concept %s drawn twice in a 12-task session", s.Task.Concept)
			}
			seenConcept[s.Task.Concept] = true
		}
	}
	if kinds[KindTraining] != 2 || kinds[KindMinimal] != 3 || kinds[KindCorpus] != 12 {
		t.Errorf("kinds = %v", kinds)
	}

	// Same seed, same session.
	again, _ := BuildSession(tasks, DefaultSessionOptions(), rand.New(rand.NewSource(7)))
	for i := range specs {
		if specs[i].Task != again[i].Task || specs[i].TestIndex != again[i].TestIndex {
			t.Fatalf("session is not reproducible from the seed (trial %d)", i)
		}
	}
}

func TestBuildSessionConcept(t *testing.T) {
	tasks, err := LoadTasks()
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultSessionOptions()
	opts.Concept = "Center"
	opts.TestsPerTask = 3
	opts.NTraining = 0
	specs, err := BuildSession(tasks, opts, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 30 {
		t.Fatalf("%d trials, want 30", len(specs))
	}
	for i, s := range specs {
		if s.Task.Concept != "Center" || s.Task.Minimal || s.Kind != KindCorpus {
			t.Errorf("trial %d: %s (%s)", i, s.Task.Name, s.Kind)
		}
		if s.TestIndex != i%3 {
			t.Errorf("trial %d: test index %d, want %d", i, s.TestIndex, i%3)
		}
	}

	opts.Concept = "NoSuchConcept"
	if _, err := BuildSession(tasks, opts, rand.New(rand.NewSource(1))); err == nil {
		t.Error("unknown concept accepted")
	}
}

func TestBuildSessionLargeDraw(t *testing.T) {
	tasks, err := LoadTasks()
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultSessionOptions()
	opts.NCorpus = 160
	specs, err := BuildSession(tasks, opts, rand.New(rand.NewSource(3)))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[*Task]bool{}
	for _, s := range specs {
		if seen[s.Task] {
			t.Fatalf("task %s repeated", s.Task.Name)
		}
		seen[s.Task] = true
	}
	if len(specs) != 2+3+160 {
		t.Errorf("%d trials", len(specs))
	}
	opts.NCorpus = 161
	if _, err := BuildSession(tasks, opts, rand.New(rand.NewSource(3))); err == nil {
		t.Error("over-draw accepted")
	}
}
