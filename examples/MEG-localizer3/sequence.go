// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math"
)

// Quadratic-residue (Legendre) binary sequences for multifocal designs.
//
// A multifocal mapping run stimulates every region on every trial according to
// its own binary on/off sequence. Deconvolving 24 region-specific responses
// from one continuous recording is only well-conditioned if those sequences are
// close to orthogonal, and the classical choice (James 2003; Vanni, Henriksson
// & James 2005, used by Kurki et al. 2022) is the quadratic-residue sequence.
//
// For a prime p ≡ 3 (mod 4), the length-p sequence q[i] = "i is a non-zero
// quadratic residue mod p" has, in ±1 coding, a periodic autocorrelation of
// exactly -1 at every non-zero lag. Two distinct cyclic shifts of it therefore
// correlate at -1/p — near-orthogonal by construction, with no search and no
// random seed. Region k is simply the sequence shifted by k*shift.
//
// Kurki et al. report 469 trials per run but give neither the prime nor the
// shift. 469 = 7x67 is not prime; 467 is the nearest prime congruent to 3
// (mod 4), and floor(467/24) = 19 spaces 24 shifts evenly over one period.
// Both are flags, and [Sequences.Report] measures what was actually built
// rather than trusting this reasoning.

// Sequences holds a multifocal design: which regions are on at each trial.
type Sequences struct {
	P        int // sequence period (prime, ≡ 3 mod 4)
	Shift    int // cyclic shift between consecutive regions
	NRegions int
	NTrials  int

	// Base is the length-P Legendre sequence.
	Base []bool
	// On[t][k] reports whether region k is stimulated on trial t.
	On [][]bool
}

// isPrime reports whether n is prime by trial division. n is a few hundred
// here, so nothing cleverer is warranted.
func isPrime(n int) bool {
	if n < 2 {
		return false
	}
	if n%2 == 0 {
		return n == 2
	}
	for d := 3; d*d <= n; d += 2 {
		if n%d == 0 {
			return false
		}
	}
	return true
}

// QRSequence returns the length-p Legendre sequence for a prime p ≡ 3 (mod 4).
//
// q[0] is false and q[i] is true iff i is a quadratic residue mod p. That
// convention (rather than q[0] = true) is the one that yields the ideal
// two-level autocorrelation; [Sequences.Report] checks it empirically.
func QRSequence(p int) ([]bool, error) {
	if !isPrime(p) {
		return nil, fmt.Errorf("sequence: p=%d is not prime", p)
	}
	if p%4 != 3 {
		return nil, fmt.Errorf("sequence: p=%d is not congruent to 3 (mod 4); "+
			"the ideal autocorrelation property does not hold", p)
	}
	q := make([]bool, p)
	// The residues are exactly the values j*j mod p for j in 1..(p-1)/2.
	for j := 1; j <= p/2; j++ {
		q[(j*j)%p] = true
	}
	q[0] = false
	return q, nil
}

// BuildSequences constructs the design. shift <= 0 selects floor(p/nRegions),
// which spreads the regions evenly over one period. nTrials <= 0 selects p,
// i.e. exactly one full period — the case in which the design is balanced.
func BuildSequences(p, nRegions, shift, nTrials int) (*Sequences, error) {
	if nRegions < 1 {
		return nil, fmt.Errorf("sequence: nRegions=%d must be positive", nRegions)
	}
	base, err := QRSequence(p)
	if err != nil {
		return nil, err
	}
	if shift <= 0 {
		shift = p / nRegions
	}
	// p is prime, so any shift not a multiple of p already gives every region a
	// distinct phase. This stricter condition enforces the design convention
	// that the regions tile one period without wrapping past its end, which is
	// what makes them evenly spread; it mostly catches a mistyped -shift.
	if shift*(nRegions-1) >= p {
		return nil, fmt.Errorf("sequence: shift=%d across %d regions spans %d, "+
			"which wraps past the period p=%d; regions would not be evenly spread",
			shift, nRegions, shift*(nRegions-1)+1, p)
	}
	if nTrials <= 0 {
		nTrials = p
	}
	s := &Sequences{P: p, Shift: shift, NRegions: nRegions, NTrials: nTrials, Base: base}
	s.On = make([][]bool, nTrials)
	for t := range s.On {
		row := make([]bool, nRegions)
		for k := range row {
			row[k] = base[(t+k*shift)%p]
		}
		s.On[t] = row
	}
	return s, nil
}

// Report summarises the measured properties of a design.
type Report struct {
	P, Shift, NRegions, NTrials int

	OnCount    []int     // per region, over the trials actually presented
	OnFraction []float64 // per region
	MinOnFrac  float64
	MaxOnFrac  float64

	// MaxPairXCorr is the largest |normalised cross-correlation| at lag 0 over
	// all distinct region pairs, in ±1 coding over one full period. It proves
	// the shifts are distinct and evenly spaced.
	MaxPairXCorr float64
	PairA, PairB int

	// MaxAutoCorr is the largest |normalised periodic autocorrelation| of the
	// base sequence over all non-zero lags. For a valid quadratic-residue
	// sequence it equals 1/p exactly, and this is the property the whole
	// design rests on.
	MaxAutoCorr float64
	AutoCorrLag int
}

// Report measures the design. Cost is O(nRegions^2 * P + P^2) — a few hundred
// thousand operations for the default parameters, so it runs at every startup
// rather than only under a flag.
func (s *Sequences) Report() Report {
	r := Report{P: s.P, Shift: s.Shift, NRegions: s.NRegions, NTrials: s.NTrials}

	// On-fractions over the trials actually presented (not the full period,
	// which would hide an unbalanced truncated run).
	r.OnCount = make([]int, s.NRegions)
	r.OnFraction = make([]float64, s.NRegions)
	r.MinOnFrac, r.MaxOnFrac = math.Inf(1), math.Inf(-1)
	for t := range s.On {
		for k, on := range s.On[t] {
			if on {
				r.OnCount[k]++
			}
		}
	}
	for k := range r.OnCount {
		f := float64(r.OnCount[k]) / float64(s.NTrials)
		r.OnFraction[k] = f
		r.MinOnFrac = math.Min(r.MinOnFrac, f)
		r.MaxOnFrac = math.Max(r.MaxOnFrac, f)
	}

	// ±1 coding of each region's full-period sequence.
	pm := make([][]float64, s.NRegions)
	for k := range pm {
		v := make([]float64, s.P)
		for i := range v {
			if s.Base[(i+k*s.Shift)%s.P] {
				v[i] = 1
			} else {
				v[i] = -1
			}
		}
		pm[k] = v
	}

	// Pairwise cross-correlation at lag 0.
	for a := 0; a < s.NRegions; a++ {
		for b := a + 1; b < s.NRegions; b++ {
			sum := 0.0
			for i := 0; i < s.P; i++ {
				sum += pm[a][i] * pm[b][i]
			}
			if c := math.Abs(sum) / float64(s.P); c > r.MaxPairXCorr {
				r.MaxPairXCorr, r.PairA, r.PairB = c, a, b
			}
		}
	}

	// Periodic autocorrelation of the base at every non-zero lag. Because the
	// regions are cyclic shifts of one sequence, this bounds the correlation
	// between any two regions at any lag, which is what the deconvolution
	// actually depends on.
	for lag := 1; lag < s.P; lag++ {
		sum := 0.0
		for i := 0; i < s.P; i++ {
			sum += pm[0][i] * pm[0][(i+lag)%s.P]
		}
		if c := math.Abs(sum) / float64(s.P); c > r.MaxAutoCorr {
			r.MaxAutoCorr, r.AutoCorrLag = c, lag
		}
	}
	return r
}

// Lines renders the report as one line per fact, for printing to the console
// and for storing as '#' comments in the data file.
func (r Report) Lines() []string {
	return []string{
		fmt.Sprintf("sequence: quadratic-residue (Legendre), p=%d, shift=%d, regions=%d, trials=%d",
			r.P, r.Shift, r.NRegions, r.NTrials),
		fmt.Sprintf("sequence: on-fraction per region min=%.4f max=%.4f (expected ~0.5)",
			r.MinOnFrac, r.MaxOnFrac),
		fmt.Sprintf("sequence: max |pairwise cross-correlation| at lag 0 = %.6f "+
			"(regions %d,%d; expected %.6f = 1/p)",
			r.MaxPairXCorr, r.PairA, r.PairB, 1.0/float64(r.P)),
		fmt.Sprintf("sequence: max |autocorrelation| over non-zero lags = %.6f "+
			"(lag %d; expected %.6f = 1/p)",
			r.MaxAutoCorr, r.AutoCorrLag, 1.0/float64(r.P)),
	}
}

// OK reports whether the sequences are orthogonal enough to deconvolve: no
// correlation materially above the theoretical 1/p floor. tol is the slack
// allowed above 1/p, in correlation units.
//
// This is a property of the sequence family alone, not of how many trials are
// presented, so it holds for a truncated run too and a failure here always
// means -p or -shift is wrong.
func (r Report) OK(tol float64) bool {
	floor := 1.0/float64(r.P) + tol
	return r.MaxPairXCorr <= floor && r.MaxAutoCorr <= floor
}

// Balanced reports whether every region is stimulated on roughly half the
// trials actually presented.
//
// A full-period run (NTrials == P) is balanced by construction. A truncated
// run is not, and that is a legitimate thing to do while testing — so this is
// reported separately from [Report.OK] rather than folded into it, and it is a
// warning rather than an error.
func (r Report) Balanced() bool {
	return r.MinOnFrac > 0.4 && r.MaxOnFrac < 0.6
}
