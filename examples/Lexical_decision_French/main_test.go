// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"testing"
)

// The embedded materials must parse and fill every cell of the design. Exact
// counts are left out so that editing the materials doesn't break the test.
func TestEmbeddedMaterialsFillEveryCell(t *testing.T) {
	trials, err := parseTrials(materials)
	if err != nil {
		t.Fatal(err)
	}

	cells := map[string]int{}
	for _, tr := range trials {
		cells[fmt.Sprintf("%s-%d", tr.category, tr.length)]++
	}
	for _, cell := range []string{"HF-4", "HF-6", "LF-4", "LF-6", "PW-4", "PW-6"} {
		if cells[cell] == 0 {
			t.Errorf("no items in cell %s", cell)
		}
	}
}

func TestParseTrialsCountsCharactersNotBytes(t *testing.T) {
	trials, err := parseTrials("étui,4,LF\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(trials) != 1 || trials[0].length != 4 {
		t.Errorf("got %+v, want one item of length 4", trials)
	}
}

func TestParseTrialsRejectsBadRows(t *testing.T) {
	cases := map[string]string{
		"missing field":   "silo,4\n",
		"length not int":  "silo,four,LF\n",
		"length mismatch": "silo,6,LF\n",
		"unknown code":    "silo,4,XX\n",
	}
	for name, data := range cases {
		if _, err := parseTrials(data); err == nil {
			t.Errorf("%s: %q was accepted", name, data)
		}
	}
}

func TestLexicality(t *testing.T) {
	cases := map[string]string{categoryHF: labelWord, categoryLF: labelWord, categoryPseudo: labelPseudo}
	for category, want := range cases {
		if got := (trial{category: category}).lexicality(); got != want {
			t.Errorf("category %s: got %s, want %s", category, got, want)
		}
	}
}
