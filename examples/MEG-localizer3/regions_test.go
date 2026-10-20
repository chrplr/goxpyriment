// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math"
	"testing"
)

// Geometry regression tests. Two properties matter more than the rest:
//
//   - The dartboard's mean luminance must equal the mid-grey background,
//     because that is what makes alpha modulation equivalent to a contrast
//     ramp. If the pattern were not balanced, fading it out would also shift
//     the mean luminance, and the "linear contrast fade" would be a luminance
//     transient the data file does not record.
//   - +Y must be up. Mirroring the map about the horizontal meridian produces
//     a perfectly plausible-looking stimulus and silently mislabels every
//     region in the lower half of the field.

const testSize = 512

func testRingPx() [NRings + 1]float64 {
	var ringPx [NRings + 1]float64
	for i, d := range RingRadiiDeg {
		ringPx[i] = math.Tan(d*math.Pi/180) * 1000 // arbitrary scale
	}
	k := (float64(testSize) / 2) / ringPx[NRings]
	for i := range ringPx {
		ringPx[i] *= k
	}
	return ringPx
}

// composite renders all regions into one greyscale buffer plus a coverage
// buffer, the way the GPU composites them over the background.
func composite(t *testing.T) (lum []float64, cover []float64) {
	t.Helper()
	lum = make([]float64, testSize*testSize)
	cover = make([]float64, testSize*testSize)
	for _, ri := range RasterizeRegions(testSize, testRingPx()) {
		for y := 0; y < ri.H; y++ {
			for x := 0; x < ri.W; x++ {
				o := (y*ri.W + x) * 4
				a := float64(ri.Pix[o+3]) / 255
				if a == 0 {
					continue
				}
				idx := (ri.OffsetY+y)*testSize + (ri.OffsetX + x)
				lum[idx] += a * float64(ri.Pix[o]) / 255
				cover[idx] += a
			}
		}
	}
	return lum, cover
}

func TestDartboardIsLuminanceBalanced(t *testing.T) {
	lum, cover := composite(t)
	var sumLum, sumCover float64
	for i := range cover {
		sumLum += lum[i]
		sumCover += cover[i]
	}
	if sumCover == 0 {
		t.Fatal("nothing was rasterised")
	}
	mean := sumLum / sumCover
	// 0.5 is mid-grey. A few tenths of a percent of slack covers the
	// antialiasing at the disc's outer edge.
	if math.Abs(mean-0.5) > 0.005 {
		t.Errorf("mean luminance of the pattern = %.4f, want 0.5; "+
			"alpha modulation is only a contrast fade when the pattern's mean "+
			"equals the background", mean)
	}
}

// TestRegionsDoNotOverlap checks that no pixel is claimed by two regions.
// Overlap would double-blend at the seams and, more importantly, would mean a
// pixel's response is attributed to two sequences at once.
func TestRegionsDoNotOverlap(t *testing.T) {
	_, cover := composite(t)
	worst := 0.0
	for _, c := range cover {
		if c > worst {
			worst = c
		}
	}
	if worst > 1.02 {
		t.Errorf("maximum total coverage = %.3f, want <= 1; regions overlap", worst)
	}
}

// TestYAxisIsUp pins the field convention: sector 1 spans 45-90 degrees, which
// is the upper-right quadrant, so in SDL pixel coordinates (+Y down) its
// bounding box must sit above the centre.
func TestYAxisIsUp(t *testing.T) {
	imgs := RasterizeRegions(testSize, testRingPx())
	centre := testSize / 2
	for _, ri := range imgs {
		if ri.Ring != NRings-1 {
			continue // outermost annulus: the least ambiguous
		}
		upper := ri.Sector <= 3 // sectors 0-3 span 0-180 deg = upper field
		mid := ri.OffsetY + ri.H/2
		if upper && mid > centre {
			t.Errorf("region %d (sector %d, upper field) is centred below the "+
				"midline at y=%d; the vertical axis is flipped",
				ri.Index, ri.Sector, mid)
		}
		if !upper && mid < centre {
			t.Errorf("region %d (sector %d, lower field) is centred above the "+
				"midline at y=%d; the vertical axis is flipped",
				ri.Index, ri.Sector, mid)
		}
	}
}

// TestChecksAlternateAcrossMeridians verifies that the per-region 4x4 checks
// are phased from global indices, so the 24 regions form one continuous
// dartboard rather than 24 independently-phased patches.
func TestChecksAlternateAcrossMeridians(t *testing.T) {
	lum, cover := composite(t)
	c := float64(testSize) / 2
	at := func(x, y int) (float64, bool) {
		i := y*testSize + x
		if cover[i] < 0.99 {
			return 0, false // antialiased or outside: not a clean sample
		}
		return lum[i] / cover[i], true
	}
	pure := func(v float64) bool { return v < 0.02 || v > 0.98 }
	ringPx := testRingPx()
	checked := 0
	// The offset from the meridian must stay well inside one angular check
	// (360/(NSectors*ChecksAngular) = 11.25 degrees), so it has to scale with
	// eccentricity: a fixed offset crosses into the neighbouring check near
	// the fovea. 3 degrees is comfortably inside, and needs r >= ~40 px to be
	// at least two pixels.
	const probeRad = 3 * math.Pi / 180
	for r := math.Max(ringPx[0]+6, 40); r < ringPx[NRings]-6; r += 5 {
		dy := int(math.Round(r * math.Tan(probeRad)))
		if dy < 2 {
			continue
		}
		a, okA := at(int(c+r), int(c)-dy)
		b, okB := at(int(c+r), int(c)+dy)
		// Only compare pixels that are purely black or purely white; a sample
		// straddling a radial band edge is antialiased even at full coverage.
		if !okA || !okB || !pure(a) || !pure(b) {
			continue
		}
		checked++
		if math.Abs(a-b) < 0.9 {
			t.Errorf("at r=%.0f the checks do not alternate across the "+
				"horizontal meridian (%.2f vs %.2f)", r, a, b)
		}
	}
	if checked < 10 {
		t.Fatalf("only %d clean samples; the test is not exercising anything", checked)
	}
}
