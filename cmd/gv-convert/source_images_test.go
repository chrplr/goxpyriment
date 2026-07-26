// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"slices"
	"testing"
)

func TestNaturalCompareOrdersFrameSequences(t *testing.T) {
	// The case that motivates natural ordering: unpadded numbering, where a
	// plain string sort puts frame_10 before frame_2.
	in := []string{
		"frame_10.png", "frame_2.png", "frame_1.png", "frame_20.png",
		"frame_3.png", "frame_11.png",
	}
	want := []string{
		"frame_1.png", "frame_2.png", "frame_3.png",
		"frame_10.png", "frame_11.png", "frame_20.png",
	}

	got := slices.Clone(in)
	slices.SortFunc(got, naturalCompare)
	if !slices.Equal(got, want) {
		t.Errorf("natural sort:\n got %v\nwant %v", got, want)
	}
}

func TestNaturalComparePairs(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"frame_9", "frame_10", -1},       // the core case
		{"frame_10", "frame_9", 1},        // and its mirror
		{"a1", "a1", 0},                   // identical
		{"img2", "img10", -1},             // no separator
		{"f0009", "f10", -1},              // zero-padded vs not, by value
		{"f9", "f0010", -1},               // mixed padding
		{"f007", "f7", 1},                 // equal value, longer literal sorts after
		{"shot1a", "shot1b", -1},          // trailing text after equal numbers
		{"a", "a1", -1},                   // prefix is shorter
		{"1file", "afile", -1},            // digits before letters
		{"v1_2", "v1_10", -1},             // multiple numeric fields
		{"v2_1", "v1_10", 1},              // earlier field dominates
		{"", "a", -1},                     // empty string
		{"", "", 0},                       // both empty
		{"100", "99", 1},                  // pure numbers
		{"frame_1.png", "frame_1.jpg", 1}, // equal numbers, extension decides
	}

	for _, tt := range tests {
		if got := naturalCompare(tt.a, tt.b); got != tt.want {
			t.Errorf("naturalCompare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// A comparator must be antisymmetric or sorting is undefined.
func TestNaturalCompareIsAntisymmetric(t *testing.T) {
	names := []string{
		"frame_1", "frame_01", "frame_10", "frame_2", "a", "1", "",
		"v1_2", "v1_10", "shot1a", "f007", "f7", "img", "img0",
	}
	for _, a := range names {
		for _, b := range names {
			ab, ba := naturalCompare(a, b), naturalCompare(b, a)
			if ab != -ba {
				t.Errorf("not antisymmetric: cmp(%q,%q)=%d but cmp(%q,%q)=%d", a, b, ab, b, a, ba)
			}
			if (a == b) != (ab == 0) {
				t.Errorf("cmp(%q,%q)=%d: only equal strings may compare 0", a, b, ab)
			}
		}
	}
}
