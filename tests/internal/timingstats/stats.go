// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Package timingstats provides frame-interval statistics helpers shared by
// tearing_test and Timing-Tests.
package timingstats

import (
	"fmt"
	"math"
	"sort"
)

// Stats holds summary statistics for a slice of frame-interval measurements.
type Stats struct {
	Mean, SD, MinV, MaxV, P5, P95 float64
	Late05, Late1                 int // count > 0.5 ms and > 1 ms from target
	N                             int
	Vals                          []float64 // raw values, kept for histogram
}

// ComputeStats computes summary statistics for deltas (in ms).
// late05 / late1 count intervals that deviate more than 0.5 / 1.0 ms from targetMs.
func ComputeStats(deltas []float64, targetMs float64) Stats {
	n := len(deltas)
	if n == 0 {
		return Stats{}
	}
	var sum float64
	mn, mx := deltas[0], deltas[0]
	for _, v := range deltas {
		sum += v
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	mean := sum / float64(n)
	var sqSum float64
	var late05, late1 int
	for _, v := range deltas {
		sqSum += (v - mean) * (v - mean)
		dev := math.Abs(v - targetMs)
		if dev > 0.5 {
			late05++
		}
		if dev > 1.0 {
			late1++
		}
	}
	sd := 0.0
	if n > 1 {
		sd = math.Sqrt(sqSum / float64(n-1))
	}
	sorted := make([]float64, n)
	copy(sorted, deltas)
	sort.Float64s(sorted)
	p5 := sorted[n*5/100]
	p95 := sorted[n*95/100]
	return Stats{mean, sd, mn, mx, p5, p95, late05, late1, n, deltas}
}

// PrintStats prints a summary of s to stdout.
func PrintStats(label string, s Stats, targetMs float64) {
	fmt.Printf("\n── %s ───────────────────────────────\n", label)
	fmt.Printf("  n       : %d\n", s.N)
	fmt.Printf("  target  : %.3f ms\n", targetMs)
	fmt.Printf("  mean    : %.3f ms\n", s.Mean)
	fmt.Printf("  SD      : %.3f ms\n", s.SD)
	fmt.Printf("  min/max : %.3f / %.3f ms\n", s.MinV, s.MaxV)
	fmt.Printf("  p5/p95  : %.3f / %.3f ms\n", s.P5, s.P95)
	fmt.Printf("  >0.5 ms : %d (%.1f %%)\n", s.Late05, 100*float64(s.Late05)/float64(s.N))
	fmt.Printf("  >1.0 ms : %d (%.1f %%)\n", s.Late1, 100*float64(s.Late1)/float64(s.N))
	PrintHistogram(s.Vals)
}

// PrintHistogram prints a 10-bin ASCII histogram of vals to stdout.
// Each bar shows the bin range, count, and a proportional bar of '*' characters.
func PrintHistogram(vals []float64) {
	const nBins = 10
	const barWidth = 40
	n := len(vals)
	if n == 0 {
		return
	}
	mn, mx := vals[0], vals[0]
	for _, v := range vals {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	binW := (mx - mn) / nBins
	if binW == 0 {
		binW = 1
	}
	counts := make([]int, nBins)
	for _, v := range vals {
		b := int((v - mn) / binW)
		if b >= nBins {
			b = nBins - 1
		}
		counts[b]++
	}
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	fmt.Printf("  histogram (%d bins):\n", nBins)
	for i := 0; i < nBins; i++ {
		lo := mn + float64(i)*binW
		hi := lo + binW
		bar := ""
		if maxCount > 0 {
			stars := counts[i] * barWidth / maxCount
			for j := 0; j < stars; j++ {
				bar += "*"
			}
		}
		fmt.Printf("  [%7.3f, %7.3f) ms : %5d  %s\n", lo, hi, counts[i], bar)
	}
}
