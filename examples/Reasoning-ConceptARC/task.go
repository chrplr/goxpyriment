// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"path"
	"sort"
	"strings"
	"sync"
)

// Pair is one input → output grid pair, a demonstration or a test item.
type Pair struct {
	Input  Grid `json:"input"`
	Output Grid `json:"output"`
}

// Task is one ConceptARC task: its demonstrations and its three test inputs.
type Task struct {
	Name    string // file stem, e.g. "AboveBelow7" or "CenterMinimal"
	Concept string // concept group, e.g. "AboveBelow"
	Minimal bool   // one of the 16 minimal (attention-check) tasks
	Train   []Pair `json:"train"`
	Test    []Pair `json:"test"`
}

// testsPerTask is fixed by the corpus: every task has exactly three test inputs.
const testsPerTask = 3

// taskFiles is the corpus, compiled into the binary. The embed directive has to
// sit next to the data, which is why tasks/ lives in this package.
//
//go:embed tasks/*.json
var taskFiles embed.FS

var loadTasksOnce = sync.OnceValues(func() ([]*Task, error) {
	return parseTaskFS(taskFiles, "tasks")
})

// LoadTasks returns the embedded corpus, parsed and validated once. Tasks are
// sorted by name; callers must not modify the grids.
func LoadTasks() ([]*Task, error) { return loadTasksOnce() }

// parseTaskFS reads every *.json file in dir and validates it. A single bad
// file aborts the program before the first trial rather than producing an
// unsolvable task mid-session.
func parseTaskFS(fsys embed.FS, dir string) ([]*Task, error) {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var tasks []*Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := fsys.ReadFile(path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		t, err := parseTask(strings.TrimSuffix(e.Name(), ".json"), raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		tasks = append(tasks, t)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name < tasks[j].Name })
	return tasks, nil
}

// parseTask decodes one task file and checks the corpus invariants.
func parseTask(name string, raw []byte) (*Task, error) {
	t := &Task{Name: name}
	if err := json.Unmarshal(raw, t); err != nil {
		return nil, err
	}
	t.Concept, t.Minimal = splitName(name)
	if len(t.Train) == 0 {
		return nil, fmt.Errorf("no demonstrations")
	}
	if len(t.Test) != testsPerTask {
		return nil, fmt.Errorf("%d test inputs, want %d", len(t.Test), testsPerTask)
	}
	for i, p := range append(append([]Pair{}, t.Train...), t.Test...) {
		if !p.Input.valid() || !p.Output.valid() {
			return nil, fmt.Errorf("pair %d: grid is empty, ragged, larger than %d, or has a colour outside 0-9", i, MaxGridSide)
		}
	}
	return t, nil
}

// splitName derives the concept group from a file stem: "AboveBelow7" →
// ("AboveBelow", false), "CenterMinimal" → ("Center", true).
func splitName(name string) (concept string, minimal bool) {
	if strings.HasSuffix(name, "Minimal") {
		return strings.TrimSuffix(name, "Minimal"), true
	}
	return strings.TrimRight(name, "0123456789"), false
}

// Concepts returns the sorted list of concept groups present in tasks.
func Concepts(tasks []*Task) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tasks {
		if !t.Minimal && !seen[t.Concept] {
			seen[t.Concept] = true
			out = append(out, t.Concept)
		}
	}
	sort.Strings(out)
	return out
}

// ── Session construction ────────────────────────────────────────────────────

// Trial kinds, written to the `kind` column.
const (
	KindTraining = "training" // minimal task, unlimited attempts, must be solved
	KindMinimal  = "minimal"  // minimal task used as an attention check
	KindCorpus   = "corpus"   // one of the 160 benchmark tasks
)

// TrialSpec is one test input to solve.
type TrialSpec struct {
	Task        *Task
	TestIndex   int    // 0, 1 or 2
	Kind        string // KindTraining, KindMinimal or KindCorpus
	MaxAttempts int    // 0 = unlimited
}

// SessionOptions selects which tasks a session presents. The zero value is not
// meaningful; start from DefaultSessionOptions.
type SessionOptions struct {
	Concept      string // non-empty: every task of that concept group, no attention checks
	NTraining    int    // minimal tasks presented first, unlimited attempts
	NChecks      int    // minimal tasks mixed in as attention checks (mixed mode only)
	NCorpus      int    // corpus tasks in the mixed session
	TestsPerTask int    // 1 (a random test input) or 3 (all, in order)
	MaxAttempts  int    // attempts per non-training test input
}

// DefaultSessionOptions is the protocol of Moskvichev et al. (2023), §5.1: two
// training tasks, then a shuffled mix of 3 minimal and 12 corpus tasks, one
// test input each, three attempts.
func DefaultSessionOptions() SessionOptions {
	return SessionOptions{NTraining: 2, NChecks: 3, NCorpus: 12, TestsPerTask: 1, MaxAttempts: 3}
}

// BuildSession draws the trial list for one participant. rng decides every
// random choice, so seeding it from the subject ID makes the session
// reproducible. Corpus tasks are drawn round-robin over a shuffled list of
// concepts, so a 12-task session covers 12 different concepts and a larger one
// spreads as evenly as possible.
func BuildSession(tasks []*Task, opts SessionOptions, rng *rand.Rand) ([]TrialSpec, error) {
	if opts.TestsPerTask != 1 && opts.TestsPerTask != testsPerTask {
		return nil, fmt.Errorf("tests per task must be 1 or %d, got %d", testsPerTask, opts.TestsPerTask)
	}

	var minimal []*Task
	byConcept := map[string][]*Task{}
	for _, t := range tasks {
		if t.Minimal {
			minimal = append(minimal, t)
		} else {
			byConcept[t.Concept] = append(byConcept[t.Concept], t)
		}
	}

	// Main block: either one whole concept group, or the mixed draw.
	var pool []*Task
	kinds := map[*Task]string{}
	if opts.Concept != "" {
		group := byConcept[opts.Concept]
		if len(group) == 0 {
			return nil, fmt.Errorf("unknown concept %q (known: %s)", opts.Concept, strings.Join(Concepts(tasks), ", "))
		}
		pool = append(pool, group...)
	} else {
		concepts := Concepts(tasks)
		rng.Shuffle(len(concepts), func(i, j int) { concepts[i], concepts[j] = concepts[j], concepts[i] })
		for _, c := range concepts {
			g := byConcept[c]
			rng.Shuffle(len(g), func(i, j int) { g[i], g[j] = g[j], g[i] })
		}
		total := 0
		for _, g := range byConcept {
			total += len(g)
		}
		if opts.NCorpus > total {
			return nil, fmt.Errorf("asked for %d corpus tasks, only %d exist", opts.NCorpus, total)
		}
		for round := 0; len(pool) < opts.NCorpus; round++ {
			for _, c := range concepts {
				if len(pool) == opts.NCorpus {
					break
				}
				if round < len(byConcept[c]) {
					pool = append(pool, byConcept[c][round])
				}
			}
		}
	}
	for _, t := range pool {
		kinds[t] = KindCorpus
	}

	// Minimal tasks: training first, then the attention checks.
	if opts.NTraining+opts.NChecks > len(minimal) {
		return nil, fmt.Errorf("asked for %d training + %d check tasks, only %d minimal tasks exist",
			opts.NTraining, opts.NChecks, len(minimal))
	}
	rng.Shuffle(len(minimal), func(i, j int) { minimal[i], minimal[j] = minimal[j], minimal[i] })
	training := minimal[:opts.NTraining]
	if opts.Concept == "" {
		for _, t := range minimal[opts.NTraining : opts.NTraining+opts.NChecks] {
			pool = append(pool, t)
			kinds[t] = KindMinimal
		}
	}
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	var specs []TrialSpec
	add := func(t *Task, kind string, maxAttempts int) {
		if opts.TestsPerTask == 1 {
			specs = append(specs, TrialSpec{Task: t, TestIndex: rng.Intn(testsPerTask), Kind: kind, MaxAttempts: maxAttempts})
			return
		}
		for i := 0; i < testsPerTask; i++ {
			specs = append(specs, TrialSpec{Task: t, TestIndex: i, Kind: kind, MaxAttempts: maxAttempts})
		}
	}
	for _, t := range training {
		add(t, KindTraining, 0)
	}
	for _, t := range pool {
		add(t, kinds[t], opts.MaxAttempts)
	}
	return specs, nil
}
