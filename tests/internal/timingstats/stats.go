// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package timingstats provides frame-interval statistics helpers shared by
// test_tearing and Timing-Tests.
package timingstats

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"

	"github.com/chrplr/goxpyriment/results"
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
	FprintStats(os.Stdout, label, s, targetMs)
}

// FprintStats renders the summary into w. Pass a report.Tee to have the same
// bytes reach both the terminal and the data file.
func FprintStats(w io.Writer, label string, s Stats, targetMs float64) {
	fmt.Fprintf(w, "\n── %s ───────────────────────────────\n", label)
	fmt.Fprintf(w, "  n       : %d\n", s.N)
	fmt.Fprintf(w, "  target  : %.3f ms\n", targetMs)
	fmt.Fprintf(w, "  mean    : %.3f ms\n", s.Mean)
	fmt.Fprintf(w, "  SD      : %.3f ms\n", s.SD)
	fmt.Fprintf(w, "  min/max : %.3f / %.3f ms\n", s.MinV, s.MaxV)
	fmt.Fprintf(w, "  p5/p95  : %.3f / %.3f ms\n", s.P5, s.P95)
	fmt.Fprintf(w, "  >0.5 ms : %d (%.1f %%)\n", s.Late05, 100*float64(s.Late05)/float64(s.N))
	fmt.Fprintf(w, "  >1.0 ms : %d (%.1f %%)\n", s.Late1, 100*float64(s.Late1)/float64(s.N))
	FprintHistogram(w, s.Vals)
}

// Save writes a Stats summary into a data file, as one comment line, and the
// raw values as one row each under the given column name.
//
// It exists because PrintStats writes to stdout and nowhere else. A test whose
// results scroll past in a terminal is a test whose results are gone the moment
// someone runs it on a machine you are not sitting at — and several tests here
// were doing exactly that, leaving a 0-byte CSV beside a full -info.txt.
//
// Declare the columns with AddDataVariableNames before calling this; it writes
// rows, not the header.
func Save(df *results.DataFile, label string, s Stats, targetMs float64) {
	df.WriteComment(fmt.Sprintf(
		"%s n=%d target_ms=%.4f mean_ms=%.4f sd_ms=%.4f min_ms=%.4f max_ms=%.4f p5_ms=%.4f p95_ms=%.4f late0.5=%d late1.0=%d",
		label, s.N, targetMs, s.Mean, s.SD, s.MinV, s.MaxV, s.P5, s.P95, s.Late05, s.Late1))
	for i, v := range s.Vals {
		df.Add(i, fmt.Sprintf("%.6f", v))
	}
}

// PrintHistogram prints a 10-bin ASCII histogram of vals to stdout.
// Each bar shows the bin range, count, and a proportional bar of '*' characters.
func PrintHistogram(vals []float64) { FprintHistogram(os.Stdout, vals) }

// FprintHistogram renders the histogram into w.
func FprintHistogram(w io.Writer, vals []float64) {
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
	fmt.Fprintf(w, "  histogram (%d bins):\n", nBins)
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
		fmt.Fprintf(w, "  [%7.3f, %7.3f) ms : %5d  %s\n", lo, hi, counts[i], bar)
	}
}
