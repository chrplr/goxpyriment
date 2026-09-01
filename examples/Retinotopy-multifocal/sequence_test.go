// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math"
	"testing"
)

// The whole multifocal design rests on one number: the correlation between two
// regions must sit at the 1/p floor. These tests assert that directly, so a
// change to the generator cannot quietly degrade the design.

func TestQRSequenceRejectsBadPrimes(t *testing.T) {
	for _, p := range []int{1, 4, 9, 469, 461} {
		// 469 = 7x67 is composite; 461 is prime but ≡ 1 (mod 4).
		if _, err := QRSequence(p); err == nil {
			t.Errorf("QRSequence(%d): expected an error, got none", p)
		}
	}
}

func TestQRSequenceOnCount(t *testing.T) {
	const p = 467
	q, err := QRSequence(p)
	if err != nil {
		t.Fatalf("QRSequence(%d): %v", p, err)
	}
	if len(q) != p {
		t.Fatalf("length = %d, want %d", len(q), p)
	}
	if q[0] {
		t.Error("q[0] must be false for the ideal-autocorrelation convention")
	}
	n := 0
	for _, v := range q {
		if v {
			n++
		}
	}
	// Exactly (p-1)/2 of 1..p-1 are quadratic residues.
	if want := (p - 1) / 2; n != want {
		t.Errorf("on-count = %d, want %d", n, want)
	}
}

// TestIdealAutocorrelation is the property the design depends on: in ±1
// coding, the periodic autocorrelation is exactly -1 at every non-zero lag.
func TestIdealAutocorrelation(t *testing.T) {
	const p = 467
	q, err := QRSequence(p)
	if err != nil {
		t.Fatalf("QRSequence(%d): %v", p, err)
	}
	v := make([]int, p)
	for i, on := range q {
		if on {
			v[i] = 1
		} else {
			v[i] = -1
		}
	}
	for lag := 1; lag < p; lag++ {
		sum := 0
		for i := 0; i < p; i++ {
			sum += v[i] * v[(i+lag)%p]
		}
		if sum != -1 {
			t.Fatalf("autocorrelation at lag %d = %d, want -1", lag, sum)
		}
	}
}

func TestBuildSequencesRejectsWrappingShifts(t *testing.T) {
	// Region 23 would sit at offset 30*23 = 690, past the period 467, so the
	// regions would no longer be evenly spread over one cycle.
	if _, err := BuildSequences(467, 24, 30, 0); err == nil {
		t.Error("expected an error when the shifts wrap past the period")
	}
	// Shift 20 spans 460 < 467 and is therefore a legitimate, if uneven, design.
	if _, err := BuildSequences(467, 24, 20, 0); err != nil {
		t.Errorf("shift 20 fits within the period but was rejected: %v", err)
	}
}

func TestDefaultDesignIsOrthogonal(t *testing.T) {
	s, err := BuildSequences(DefaultPrime, NRegions, 0, 0)
	if err != nil {
		t.Fatalf("BuildSequences: %v", err)
	}
	if s.Shift != DefaultPrime/NRegions {
		t.Errorf("shift = %d, want %d", s.Shift, DefaultPrime/NRegions)
	}
	r := s.Report()
	for _, line := range r.Lines() {
		t.Log(line)
	}
	floor := 1.0 / float64(DefaultPrime)
	if math.Abs(r.MaxPairXCorr-floor) > 1e-9 {
		t.Errorf("max pairwise cross-correlation = %g, want %g", r.MaxPairXCorr, floor)
	}
	if math.Abs(r.MaxAutoCorr-floor) > 1e-9 {
		t.Errorf("max autocorrelation = %g, want %g", r.MaxAutoCorr, floor)
	}
	if !r.OK(1e-9) {
		t.Error("Report.OK rejected the default design")
	}
}

// TestRegionsAreDistinct guards the indexing: a bug in the shift arithmetic
// that gave two regions the same phase would leave the correlation checks
// above unchanged only if it also broke Report, so check the rows directly.
func TestRegionsAreDistinct(t *testing.T) {
	s, err := BuildSequences(DefaultPrime, NRegions, 0, 0)
	if err != nil {
		t.Fatalf("BuildSequences: %v", err)
	}
	for a := 0; a < NRegions; a++ {
		for b := a + 1; b < NRegions; b++ {
			same := true
			for tr := range s.On {
				if s.On[tr][a] != s.On[tr][b] {
					same = false
					break
				}
			}
			if same {
				t.Errorf("regions %d and %d have identical sequences", a, b)
			}
		}
	}
}

func TestTruncatedRunStaysBalanced(t *testing.T) {
	// A short debugging run must still stimulate every region roughly half
	// the time, or a quick sanity run would look broken for the wrong reason.
	s, err := BuildSequences(DefaultPrime, NRegions, 0, 60)
	if err != nil {
		t.Fatalf("BuildSequences: %v", err)
	}
	r := s.Report()
	if r.MinOnFrac < 0.3 || r.MaxOnFrac > 0.7 {
		t.Errorf("on-fraction range [%.3f, %.3f] is too wide for a 60-trial run",
			r.MinOnFrac, r.MaxOnFrac)
	}
}
