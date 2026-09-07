// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"log"
	"math"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Canvas and units ──────────────────────────────────────────────────────────
//
// The paper presented the tasks on a 65" screen at 1920 × 1006 px and defined
// one length unit as 1920 × 0.85 = 1632 px (Table 1 note). Reproducing that
// logical canvas exactly keeps every pixel value below comparable with the
// published data, whatever the real display resolution is.

const (
	logicalWidth  = 1920
	logicalHeight = 1006
	unitPx        = 1632.0
)

// ── Triangles ─────────────────────────────────────────────────────────────────

// tri is a triangle specified the way the paper specifies it: a horizontal base
// of a given length with two given base angles. The apex is derived.
//
// All lengths are in normalized units; multiply by unitPx for pixels.
type tri struct {
	id                string
	baseUnits         float64
	leftDeg, rightDeg float64
}

func rad(d float64) float64 { return d * math.Pi / 180 }

// heightUnits is the perpendicular distance from the base to the apex:
// b·tanL·tanR / (tanL + tanR).
func (t tri) heightUnits() float64 {
	tl, tr := math.Tan(rad(t.leftDeg)), math.Tan(rad(t.rightDeg))
	return t.baseUnits * tl * tr / (tl + tr)
}

// areaUnits is the enclosed area. The paper's "triangle size" column is this
// value × 100.
func (t tri) areaUnits() float64 { return 0.5 * t.baseUnits * t.heightUnits() }

// apexAngleDeg is the angle at the missing corner.
func (t tri) apexAngleDeg() float64 { return 180 - t.leftDeg - t.rightDeg }

// legUnits returns the lengths of the two sides rising from the base corners.
func (t tri) legUnits() (left, right float64) {
	h := t.heightUnits()
	return h / math.Sin(rad(t.leftDeg)), h / math.Sin(rad(t.rightDeg))
}

// corners returns the two base corners and the apex in centre-based screen
// pixels (+Y up), with the base horizontal, centred on x = 0, at height baseY.
func (t tri) corners(baseY float32) (bl, br, apex control.FPoint) {
	b := t.baseUnits * unitPx
	h := t.heightUnits() * unitPx
	bl = control.FPoint{X: float32(-b / 2), Y: baseY}
	br = control.FPoint{X: float32(b / 2), Y: baseY}
	// Horizontal distance from the left corner to the foot of the altitude.
	apex = control.FPoint{
		X: float32(-b/2 + h/math.Tan(rad(t.leftDeg))),
		Y: baseY + float32(h),
	}
	return bl, br, apex
}

// fragments returns the two visible corner marks: at each base corner, one arm
// along the base and one along the leg, each a fraction frac of that whole side.
//
// Because the arms are a fraction of the sides rather than a fixed length, the
// fragment is a similar figure at every triangle size — the display carries no
// size cue that the triangle itself does not carry, which is what these tasks
// probe.
func (t tri) fragments(baseY float32, frac float64, stroke float32, colour control.Color) []*stimuli.PolyLine {
	bl, br, apex := t.corners(baseY)
	legL, legR := t.legUnits()

	// A corner mark runs: end of the base arm → the corner → end of the leg arm.
	mark := func(corner, alongBase, towardApex control.FPoint, baseLen, legLen float64) *stimuli.PolyLine {
		pts := []control.FPoint{
			lerpTo(corner, alongBase, frac*baseLen*unitPx),
			corner,
			lerpTo(corner, towardApex, frac*legLen*unitPx),
		}
		return stimuli.NewPolyLine(pts, false, stroke, colour)
	}

	return []*stimuli.PolyLine{
		mark(bl, br, apex, t.baseUnits, legL),
		mark(br, bl, apex, t.baseUnits, legR),
	}
}

// complete returns the whole triangle as a closed outline — used only by the
// demonstration phase, never during a test trial.
func (t tri) complete(baseY float32, stroke float32, colour control.Color) *stimuli.PolyLine {
	bl, br, apex := t.corners(baseY)
	return stimuli.NewPolyLine([]control.FPoint{bl, br, apex}, true, stroke, colour)
}

// lerpTo returns the point at distance d from a, along the direction a → b.
func lerpTo(a, b control.FPoint, d float64) control.FPoint {
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	l := math.Hypot(dx, dy)
	if l == 0 {
		return a
	}
	return control.FPoint{X: a.X + float32(dx/l*d), Y: a.Y + float32(dy/l*d)}
}

// ── Stimulus tables ───────────────────────────────────────────────────────────

// reasoningTriangles is Table 1 of the paper: the eight scalene fragments of the
// reasoning task. paperSize is the printed "triangle size" column, kept here so
// the startup self-check can confirm the geometry against the publication.
var reasoningTriangles = []struct {
	tri
	paperSize float64
}{
	{tri{"R1", 0.44, 32, 48}, 3.87},
	{tri{"R2", 0.66, 40, 32}, 7.80},
	{tri{"R3", 0.77, 56, 32}, 13.03},
	{tri{"R4", 0.55, 32, 40}, 5.41},
	{tri{"R5", 0.77, 48, 56}, 18.82},
	{tri{"R6", 0.66, 48, 40}, 10.41},
	{tri{"R7", 0.44, 40, 56}, 5.19},
	{tri{"R8", 0.55, 56, 48}, 9.60},
}

// localizationTriangles is Table 2: the seven isosceles configurations of the
// localization task. Their seven distinct leg lengths are the x-axis of the
// scaling-exponent regression.
var localizationTriangles = []struct {
	tri
	paperSize float64
}{
	{tri{"L1", 0.90, 36, 36}, 14.70},
	{tri{"L2", 0.40, 36, 36}, 2.90},
	{tri{"L3", 0.10, 36, 36}, 0.18},
	{tri{"L4", 0.04, 36, 36}, 0.03},
	{tri{"L5", 0.40, 45, 45}, 4.00},
	{tri{"L6", 0.10, 45, 45}, 0.25},
	{tri{"L7", 0.04, 45, 45}, 0.04},
}

// sampleTriangle is the practice/demonstration figure: 30° base angles and a
// base length of 0.7 units (paper §2.3). It never appears in a test trial.
var sampleTriangle = tri{"sample", 0.70, 30, 30}

// checkTables verifies every table row against the area printed in the paper,
// so a typo in the numbers above is caught at startup rather than in the data.
func checkTables() {
	// The paper prints these sizes rounded to two decimals, so the check has to
	// tolerate the rounding: the largest legitimate deviation across the fifteen
	// rows is 0.0125 (L1, printed as 14.7). A 0.02 absolute / 0.5 % relative
	// band accepts every correct row while still catching a transposed digit.
	verify := func(what string, id string, got, want float64) {
		tol := math.Max(0.02, 0.005*want)
		if math.Abs(got-want) > tol {
			log.Fatalf("%s %s: computed size %.4f but the paper prints %.2f — check the table",
				what, id, got, want)
		}
	}
	for _, r := range reasoningTriangles {
		verify("reasoning triangle", r.id, r.areaUnits()*100, r.paperSize)
	}
	seen := map[string]string{}
	for _, l := range localizationTriangles {
		verify("localization triangle", l.id, l.areaUnits()*100, l.paperSize)
		// The paper's seven configurations must have seven DISTINCT side
		// lengths; the regression that yields the scaling exponent needs them.
		leg, _ := l.legUnits()
		key := fmt.Sprintf("%.6f", leg)
		if other, dup := seen[key]; dup {
			log.Fatalf("localization triangles %s and %s share a leg length of %s units — "+
				"the scaling exponent needs seven distinct side lengths", other, l.id, key)
		}
		seen[key] = l.id
	}
}
