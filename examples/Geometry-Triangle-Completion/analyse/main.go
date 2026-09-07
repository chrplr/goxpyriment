// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// analyse computes the scaling exponent of a localization session.
//
// Hart, Mahadevan & Dillon (2022) model visual triangle completion as a
// correlated random walk with two competing processes: local smooth motion, and
// a global correction that pulls the walk back onto the direction the given
// corner angles imply. The balance between them is summarised by the SCALING
// EXPONENT alpha, the power by which the spread of a participant's estimates
// grows with triangle side length:
//
//	log(SD_y) = alpha · log(side length) + c
//
// alpha ≈ 0.5 means the local noise is strongly corrected, preserving the
// scale-invariant angle information of Euclidean geometry; alpha ≈ 1.0 means the
// noise accumulates uncorrected. The paper's children produced a median alpha of
// 0.83 (95% CI [0.80, 0.86], range [0.56, 1.14]).
//
// Usage:
//
//	go run ./examples/Geometry-Triangle-Completion/analyse \
//	    goxpy_data/Geometry-Triangle-Completion_sub-001_date-...-localization.csv
//
//	-o summary.csv   also write the per-configuration table as CSV
//
// Several files may be given; each is summarised separately.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// group collects every trial of one triangle configuration.
type group struct {
	configID  string
	sideUnits float64
	errorsY   []float64 // y_true − y_clicked, screen px (positive = fell short)
	clickedY  []float64
	rts       []int64
}

func (g group) meanError() float64 { return mean(g.errorsY) }
func (g group) sdY() float64       { return sd(g.clickedY) }
func (g group) medianRT() float64 {
	v := make([]float64, len(g.rts))
	for i, r := range g.rts {
		v[i] = float64(r)
	}
	sort.Float64s(v)
	if len(v) == 0 {
		return math.NaN()
	}
	return v[len(v)/2]
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

// sd is the sample standard deviation (n−1), which is what the paper's
// per-participant SD of estimates is.
func sd(v []float64) float64 {
	if len(v) < 2 {
		return math.NaN()
	}
	m := mean(v)
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)-1))
}

// fit is an ordinary least-squares regression of y on x.
type fit struct {
	slope, intercept float64
	slopeSE          float64
	r2               float64
	n                int
}

func regress(x, y []float64) (fit, error) {
	n := len(x)
	if n < 3 {
		return fit{}, fmt.Errorf("need at least 3 usable configurations, have %d", n)
	}
	mx, my := mean(x), mean(y)
	var sxx, sxy float64
	for i := range x {
		sxx += (x[i] - mx) * (x[i] - mx)
		sxy += (x[i] - mx) * (y[i] - my)
	}
	if sxx == 0 {
		return fit{}, fmt.Errorf("all side lengths are identical — no slope to fit")
	}
	b := sxy / sxx
	a := my - b*mx

	var ssRes, ssTot float64
	for i := range x {
		r := y[i] - (a + b*x[i])
		ssRes += r * r
		ssTot += (y[i] - my) * (y[i] - my)
	}
	se := math.NaN()
	if n > 2 {
		se = math.Sqrt(ssRes / float64(n-2) / sxx)
	}
	r2 := math.NaN()
	if ssTot > 0 {
		r2 = 1 - ssRes/ssTot
	}
	return fit{slope: b, intercept: a, slopeSE: se, r2: r2, n: n}, nil
}

func main() {
	out := flag.String("o", "", "also write the per-configuration table to this CSV")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: analyse [-o summary.csv] <...-localization.csv> [more files]")
		os.Exit(2)
	}

	var rows [][]string
	rows = append(rows, []string{"file", "subject_id", "config_id", "side_length_units",
		"n", "mean_error_y_px", "sd_y_px", "median_rt_ms"})

	failed := false
	for _, path := range flag.Args() {
		subject, groups, err := load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		rows = append(rows, report(path, subject, groups)...)
	}

	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "creating %s: %v\n", *out, err)
			os.Exit(1)
		}
		defer f.Close()
		w := csv.NewWriter(f)
		if err := w.WriteAll(rows); err != nil {
			fmt.Fprintf(os.Stderr, "writing %s: %v\n", *out, err)
			os.Exit(1)
		}
		fmt.Printf("\nwrote %s (%d rows)\n", *out, len(rows)-1)
	}
	if failed {
		os.Exit(1)
	}
}

// load reads a localization CSV into one group per configuration.
func load(path string) (string, []group, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		return "", nil, fmt.Errorf("reading the header: %w", err)
	}
	col := map[string]int{}
	for i, h := range head {
		col[strings.TrimSpace(h)] = i
	}
	for _, need := range []string{"config_id", "side_length_units", "clicked_y", "error_y", "rt_ms"} {
		if _, ok := col[need]; !ok {
			return "", nil, fmt.Errorf("column %q is missing — is this a -localization.csv?", need)
		}
	}

	subject := ""
	byID := map[string]*group{}
	var order []string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}
		if i, ok := col["subject_id"]; ok && subject == "" && i < len(rec) {
			subject = rec[i]
		}
		id := rec[col["config_id"]]
		g, ok := byID[id]
		if !ok {
			side, err := strconv.ParseFloat(rec[col["side_length_units"]], 64)
			if err != nil {
				return "", nil, fmt.Errorf("config %s: bad side_length_units: %w", id, err)
			}
			g = &group{configID: id, sideUnits: side}
			byID[id] = g
			order = append(order, id)
		}
		cy, err1 := strconv.ParseFloat(rec[col["clicked_y"]], 64)
		ey, err2 := strconv.ParseFloat(rec[col["error_y"]], 64)
		if err1 != nil || err2 != nil {
			continue // skip an unparseable row rather than abandoning the file
		}
		g.clickedY = append(g.clickedY, cy)
		g.errorsY = append(g.errorsY, ey)
		if rt, err := strconv.ParseInt(rec[col["rt_ms"]], 10, 64); err == nil {
			g.rts = append(g.rts, rt)
		}
		_ = order
	}

	groups := make([]group, 0, len(byID))
	for _, id := range order {
		groups = append(groups, *byID[id])
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].sideUnits < groups[j].sideUnits })
	return subject, groups, nil
}

// report prints the per-configuration table and the scaling exponent.
func report(path, subject string, groups []group) [][]string {
	fmt.Printf("\n== %s", path)
	if subject != "" {
		fmt.Printf("   (subject %s)", subject)
	}
	fmt.Println()
	fmt.Println("  config   side (u)     n   mean error_y (px)   SD_y (px)   median RT (ms)")

	var rows [][]string
	var logSide, logSD []float64
	for _, g := range groups {
		fmt.Printf("  %-7s %9.5f %5d %19.1f %11.1f %16.0f\n",
			g.configID, g.sideUnits, len(g.clickedY), g.meanError(), g.sdY(), g.medianRT())
		rows = append(rows, []string{path, subject, g.configID,
			strconv.FormatFloat(g.sideUnits, 'g', -1, 64),
			strconv.Itoa(len(g.clickedY)),
			fmt.Sprintf("%.4f", g.meanError()),
			fmt.Sprintf("%.4f", g.sdY()),
			fmt.Sprintf("%.0f", g.medianRT()),
		})
		// A configuration with fewer than two clicks has no SD to regress.
		if s := g.sdY(); !math.IsNaN(s) && s > 0 && g.sideUnits > 0 {
			logSide = append(logSide, math.Log(g.sideUnits))
			logSD = append(logSD, math.Log(s))
		}
	}

	f, err := regress(logSide, logSD)
	if err != nil {
		fmt.Printf("\n  scaling exponent: not computed — %v\n", err)
		return rows
	}
	fmt.Printf("\n  scaling exponent alpha = %.3f  (SE %.3f, R2 %.3f, %d configurations)\n",
		f.slope, f.slopeSE, f.r2, f.n)
	fmt.Printf("  %s\n", interpret(f.slope))
	return rows
}

func interpret(alpha float64) string {
	switch {
	case alpha < 0.65:
		return "close to 0.5: the local noise is strongly corrected, preserving scale-invariant angles"
	case alpha > 0.9:
		return "close to 1.0: the local noise accumulates largely uncorrected"
	default:
		return "between 0.5 and 1.0: partial correction — the paper's children had a median of 0.83"
	}
}
