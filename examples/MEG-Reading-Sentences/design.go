// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

// The design: loading the stimulus tables and dealing them into runs. Nothing
// here touches SDL, so it is unit-tested (design_test.go).

import (
	"bytes"
	"embed"
	"encoding/csv"
	"fmt"
	"math/rand/v2"
	"strings"
)

// The stimulus tables are embedded so the binary is
// self-contained (the browser build has no filesystem) and nothing is read
// from disk once a run has started.
//
//go:embed stimuli/*.csv
var stimFS embed.FS

// wordsPerItem is the length of every sentence and nonword string.
const wordsPerItem = 12

// maxSameCond is the longest run of consecutive trials of one condition
// allowed in a trial list, so the participant cannot predict the next one.
const maxSameCond = 3

// item is one row of a stimulus table.
type item struct {
	index int      // 1-based row number in its table, so a trial can be traced back
	cond  string   // "S" (sentence) or "N" (nonwords)
	words []string // the 12 words or nonwords
}

// loadItems parses a stimulus table with columns sentence,condition. raw is the
// file contents; name is used only in error messages. A UTF-8 byte-order mark
// (the training tables have one) is skipped.
func loadItems(name string, raw []byte) ([]item, error) {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	rows, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("%s: no data rows", name)
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range []string{"sentence", "condition"} {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("%s: missing column %q", name, required)
		}
	}

	items := make([]item, 0, len(rows)-1)
	for i, rec := range rows[1:] {
		lineno := i + 2 // 1-based, past the header
		words := strings.Fields(rec[col["sentence"]])
		if len(words) != wordsPerItem {
			return nil, fmt.Errorf("%s line %d: expected %d words, got %d",
				name, lineno, wordsPerItem, len(words))
		}
		cond := strings.ToUpper(strings.TrimSpace(rec[col["condition"]]))
		if cond != "S" && cond != "N" {
			return nil, fmt.Errorf("%s line %d: condition must be S or N, got %q", name, lineno, cond)
		}
		items = append(items, item{index: i + 1, cond: cond, words: words})
	}
	return items, nil
}

// loadTable reads an embedded stimulus table and checks that every row is of
// condition cond, so a table cannot be passed in the wrong role.
func loadTable(name, cond string) ([]item, error) {
	raw, err := stimFS.ReadFile(name)
	if err != nil {
		return nil, err
	}
	items, err := loadItems(name, raw)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if it.cond != cond {
			return nil, fmt.Errorf("%s line %d: condition %q in a table of condition %q",
				name, it.index+1, it.cond, cond)
		}
	}
	return items, nil
}

// buildRuns deals the sentences and the nonword strings into nRuns trial lists.
//
// Each condition is shuffled and split as evenly as possible across the runs,
// so every item is presented exactly once over the session and the runs differ
// in size by at most one trial per condition (139 + 139 in 4 runs: 35+35,
// 35+35, 35+35, 34+34). Within a run the two conditions are intermixed in
// random order, with no more than maxSameCond consecutive trials of one
// condition.
//
// The lists depend only on rng's state, so a participant's runs are the same
// every time the program is launched with the same seed and subject ID — which
// is what lets the runs be launched one at a time.
func buildRuns(sentences, nonwords []item, nRuns int, rng *rand.Rand) ([][]item, error) {
	if nRuns < 1 {
		return nil, fmt.Errorf("number of runs must be at least 1, got %d", nRuns)
	}
	if len(sentences) < nRuns || len(nonwords) < nRuns {
		return nil, fmt.Errorf("cannot deal %d sentences and %d nonword strings into %d runs",
			len(sentences), len(nonwords), nRuns)
	}
	s := shuffled(sentences, rng)
	n := shuffled(nonwords, rng)

	runs := make([][]item, nRuns)
	for r := range runs {
		run := append(split(s, nRuns, r), split(n, nRuns, r)...)
		if err := interleave(run, rng); err != nil {
			return nil, fmt.Errorf("run %d: %w", r+1, err)
		}
		runs[r] = run
	}
	return runs, nil
}

// practiceList draws perCond items of each condition from the practice tables
// and intermixes them like a run.
func practiceList(sentences, nonwords []item, perCond int, rng *rand.Rand) ([]item, error) {
	if perCond < 1 || perCond > len(sentences) || perCond > len(nonwords) {
		return nil, fmt.Errorf("practice items per condition must be between 1 and %d, got %d",
			min(len(sentences), len(nonwords)), perCond)
	}
	list := append(shuffled(sentences, rng)[:perCond], shuffled(nonwords, rng)[:perCond]...)
	if err := interleave(list, rng); err != nil {
		return nil, err
	}
	return list, nil
}

// shuffled returns a shuffled copy of items.
func shuffled(items []item, rng *rand.Rand) []item {
	c := append([]item(nil), items...)
	rng.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })
	return c
}

// split returns part k (0-based) of items cut into n nearly equal parts, the
// larger parts first.
func split(items []item, n, k int) []item {
	size, extra := len(items)/n, len(items)%n
	start := k*size + min(k, extra)
	end := start + size
	if k < extra {
		end++
	}
	return append([]item(nil), items[start:end]...)
}

// interleave shuffles list in place until no condition occurs more than
// maxSameCond times in a row. For a 35+35 run about one shuffle in 200
// qualifies (measured by TestInterleaveAcceptanceRate: 101 of 20000), so the
// bound on attempts is generous; hitting it means the conditions are too
// unbalanced for the constraint.
func interleave(list []item, rng *rand.Rand) error {
	const maxAttempts = 100000
	for range maxAttempts {
		rng.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
		if longestStreak(list) <= maxSameCond {
			return nil
		}
	}
	return fmt.Errorf("no order of %d trials keeps runs of one condition to %d", len(list), maxSameCond)
}

// longestStreak is the length of the longest run of consecutive items of one
// condition.
func longestStreak(list []item) int {
	best, cur := 0, 0
	for i := range list {
		if i > 0 && list[i].cond == list[i-1].cond {
			cur++
		} else {
			cur = 1
		}
		best = max(best, cur)
	}
	return best
}
