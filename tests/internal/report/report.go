// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package report captures a test's console output so it also reaches the data
// file.
//
// # Why this exists
//
// These tests are run by hand, often on a machine nobody is sitting at, and
// their results were reaching only stdout. Several wrote a full statistics
// report to the terminal beside a 0-byte CSV — the numbers existed until the
// scrollback rolled, and then did not.
//
// The first fix was to rewrite each summary as compact key=value comments. That
// saved every number and lost what made them usable: labels, units, the
// soundness checks, the verdict. Someone running a test remotely should be able
// to send back the file rather than a screenshot.
//
// So a Tee prints and records the same bytes. There is no second formatting
// path to fall out of step with the first — whatever the terminal showed is
// what the file holds.
//
// `Timing-Tests` does not need this: `run-timing-tests.sh` already pipes every
// step through `tee` into a per-step .txt. This is for the standalone tests,
// which have no such harness.
package report

import (
	"fmt"
	"strings"

	"github.com/chrplr/goxpyriment/results"
)

// Tee prints to stdout and keeps a copy for the data file.
//
// The zero value is ready to use:
//
//	out := &report.Tee{}
//	defer out.Flush(exp.Data, "tearing report")
//	out.Printf("mean %.3f ms\n", mean)
type Tee struct{ lines []string }

// Write makes a Tee an io.Writer, so helpers that render into a writer —
// timingstats.FprintStats and friends — can be captured without reformatting
// their output a second time.
func (t *Tee) Write(p []byte) (int, error) {
	t.lines = append(t.lines, string(p))
	return fmt.Print(string(p))
}

// Printf formats, prints to stdout, and records.
func (t *Tee) Printf(format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	fmt.Print(line)
	t.lines = append(t.lines, line)
}

// Println prints and records a single line.
func (t *Tee) Println(s string) { t.Printf("%s\n", s) }

// Flush writes everything recorded into the data file's companion info file,
// one comment line per line of output, under a title so a file holding several
// reports stays readable.
//
// It is safe to call with nothing recorded, and safe to defer before the work
// that might fail: a test that dies halfway still files the part it produced.
func (t *Tee) Flush(df *results.DataFile, title string) {
	if df == nil || len(t.lines) == 0 {
		return
	}
	df.WriteComment("--- " + title + " ---")
	for _, chunk := range t.lines {
		for _, line := range strings.Split(strings.TrimRight(chunk, "\n"), "\n") {
			df.WriteComment(line)
		}
	}
	t.lines = nil // so a deferred Flush after an explicit one does not double up
}
