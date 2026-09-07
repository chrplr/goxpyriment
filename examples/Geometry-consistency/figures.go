// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math"

	"github.com/chrplr/goxpyriment/control"
)

// ── Figures ───────────────────────────────────────────────────────────────────
//
// Every figure is defined in NORMALIZED UNITS with the implied area printed in
// the paper's Figures 2a, 4 and 6. A single pixels-per-unit factor scales them
// to the screen, so all the area ratios the design depends on hold by
// construction, at any display size.
//
// Vertices use the goxpyriment center-based convention (+Y up) and are recentred
// on the CENTROID of the vertices. That centroid is the point placed at the
// jittered location and the point the figure rotates and mirrors about; the
// paper does not specify a reference point, so this is our choice.

type figure struct {
	id     string
	pts    []control.FPoint // normalized units, centred on the centroid
	closed bool
	area   float64 // implied area (for open figures: the implied triangle)
	maxR   float64 // max |vertex − centroid|, used to auto-fit the panel
}

// centre recentres pts on their centroid and fills in maxR.
func (f figure) centre() figure {
	var sx, sy float64
	for _, p := range f.pts {
		sx += float64(p.X)
		sy += float64(p.Y)
	}
	n := float64(len(f.pts))
	cx, cy := sx/n, sy/n
	for i := range f.pts {
		f.pts[i].X -= float32(cx)
		f.pts[i].Y -= float32(cy)
		if r := math.Hypot(float64(f.pts[i].X), float64(f.pts[i].Y)); r > f.maxR {
			f.maxR = r
		}
	}
	return f
}

// closedTriangle builds a closed triangle with the given interior angles (in
// degrees) scaled to the given area.
//
// Side lengths are DERIVED from the angles by the law of sines rather than
// copied from the two-decimal values printed in the paper: that keeps the angles
// exact and the area ratio exact. It reproduces the paper's 15°–45°–120°
// triangle to the printed precision and lands within ~3 % on the 45°–60°–75°
// one, whose printed sides (1.00 / 1.06 / 0.77) actually describe a
// 43.8°–64.0°–72.3° triangle.
func closedTriangle(id string, angA, angB, angC, area float64) figure {
	a := deg2rad(angA)
	b := deg2rad(angB)
	c := deg2rad(angC)

	// Sides opposite each angle, for a circumdiameter of 1.
	lenA, lenB, lenC := math.Sin(a), math.Sin(b), math.Sin(c)
	unitArea := lenA * lenB * lenC / 2
	k := math.Sqrt(area / unitArea)
	lenB, lenC = lenB*k, lenC*k

	// Vertex A at the origin, side c (= AB) along +x, vertex C at angle A.
	pts := []control.FPoint{
		{X: 0, Y: 0},
		{X: float32(lenC), Y: 0},
		{X: float32(lenB * math.Cos(a)), Y: float32(lenB * math.Sin(a))},
	}
	return figure{id: id, pts: pts, closed: true, area: area}.centre()
}

// openFigure builds an open two-line figure: two segments of length len1 and
// len2 meeting at a vertex with the given angle. The implied area is the area of
// the triangle that closes the two open endpoints — the paper's definition
// (§2.5.1), which is how area was matched across the streams.
func openFigure(id string, angleDeg, len1, len2 float64) figure {
	a := deg2rad(angleDeg)
	pts := []control.FPoint{
		{X: float32(len1), Y: 0}, // end of arm 1
		{X: 0, Y: 0},             // the vertex
		{X: float32(len2 * math.Cos(a)), Y: float32(len2 * math.Sin(a))}, // end of arm 2
	}
	area := 0.5 * len1 * len2 * math.Sin(a)
	return figure{id: id, pts: pts, closed: false, area: area}.centre()
}

// scaled returns a uniformly scaled copy — the area-only change partner.
func (f figure) scaled(k float64, id string) figure {
	pts := make([]control.FPoint, len(f.pts))
	for i, p := range f.pts {
		pts[i] = control.FPoint{X: p.X * float32(k), Y: p.Y * float32(k)}
	}
	return figure{
		id:     id,
		pts:    pts,
		closed: f.closed,
		area:   f.area * k * k,
		maxR:   f.maxR * k,
	}
}

func deg2rad(d float64) float64 { return d * math.Pi / 180 }

// ── Conditions ────────────────────────────────────────────────────────────────

type condition struct {
	name  string
	label string

	// The shape-and-area change pair. Their implied areas stand in a ~1:2 ratio;
	// the stream alternates between them.
	small, large figure

	// Scale applied to the CONTEXT figure to make its area-only partner. Zero
	// means "derive from the areas", which equates the implied-area change across
	// the two streams — what every condition but 2B requires.
	scaleFromSmall, scaleFromLarge float64

	rotationDeg float64 // ±30 for limited rotation, 360 for unlimited
	allowFlip   bool    // 50 % left-right mirroring per presentation
}

// conditionNames is the display order (also the dialog's option order).
var conditionNames = []string{"1A", "1B", "2A", "2B", "3A", "3B", "3C", "3D"}

func buildConditions() map[string]condition {
	// Experiment 1 — closed triangles. The shape-change pair is the smaller
	// 15°-45°-120° triangle and the larger 45°-60°-75° one (paper, Figure 2a).
	tri := func() (figure, figure) {
		return closedTriangle("tri_15-45-120_small", 15, 45, 120, 0.185),
			closedTriangle("tri_45-60-75_large", 45, 60, 75, 0.37)
	}
	tri1Small, tri1Large := tri()
	tri2Small, tri2Large := tri()

	// Experiment 3A and 3D use the same figures.
	ang3 := func() (figure, figure) {
		return openFigure("open_151.00deg_1x1.75", 151.00, 1, 1.75),
			openFigure("open_75.50deg_1x1.75", 75.50, 1, 1.75)
	}
	a3Small, a3Large := ang3()
	d3Small, d3Large := ang3()

	list := []condition{
		{
			name: "1A", label: "Triangles, ±0–30° rotation, no flips",
			small: tri1Small, large: tri1Large,
			rotationDeg: 30, allowFlip: false,
		},
		{
			name: "1B", label: "Triangles, 0–359° rotation, left/right flips",
			small: tri2Small, large: tri2Large,
			rotationDeg: 360, allowFlip: true,
		},
		{
			name: "2A", label: "Relative length, obtuse (106.77°), 1:1.5 vs 1:3",
			small:       openFigure("open_106.77deg_1x1.5", 106.77, 1, 1.5),
			large:       openFigure("open_106.77deg_1x3", 106.77, 1, 3),
			rotationDeg: 360, allowFlip: true,
		},
		{
			// The only condition whose area-only partner is NOT area-matched: it
			// is scaled to equate the 1.5-unit TOTAL LINE LENGTH change instead
			// (paper §2.6.1). 1×1.5 (total 2.5) → ×1.6 gives 1.60×2.40 (total
			// 4.0); 1×3 (total 4.0) → ×0.625 gives 0.625×1.875 (total 2.5). The
			// paper prints those rounded as 1.60×2.40 and 0.63×1.88.
			name: "2B", label: "Relative length, acute (53.39°), total length equated",
			small:          openFigure("open_53.39deg_1x1.5", 53.39, 1, 1.5),
			large:          openFigure("open_53.39deg_1x3", 53.39, 1, 3),
			scaleFromSmall: 1.6, scaleFromLarge: 0.625,
			rotationDeg: 360, allowFlip: true,
		},
		{
			name: "3A", label: "Angle, 75.50° vs 151.00° (2-fold), 0–359° rotation",
			small: a3Small, large: a3Large,
			rotationDeg: 360, allowFlip: true,
		},
		{
			name: "3B", label: "Angle, 26.57° vs 116.57° (4.39-fold), 0–359° rotation",
			small:       openFigure("open_26.57deg_1x2.25", 26.57, 1, 2.25),
			large:       openFigure("open_116.57deg_1x2.25", 116.57, 1, 2.25),
			rotationDeg: 360, allowFlip: true,
		},
		{
			name: "3C", label: "Angle, 20.00° vs 137.50° (6.88-fold), 0–359° rotation",
			small:       openFigure("open_20.00deg_1x2.25", 20.00, 1, 2.25),
			large:       openFigure("open_137.50deg_1x2.25", 137.50, 1, 2.25),
			rotationDeg: 360, allowFlip: true,
		},
		{
			name: "3D", label: "Angle, 75.50° vs 151.00° (2-fold), ±0–30° rotation, no flips",
			small: d3Small, large: d3Large,
			rotationDeg: 30, allowFlip: false,
		},
	}

	conds := make(map[string]condition, len(list))
	for _, c := range list {
		if c.scaleFromSmall == 0 {
			// Equate the implied-area change across the two streams.
			c.scaleFromSmall = math.Sqrt(c.large.area / c.small.area)
			c.scaleFromLarge = 1 / c.scaleFromSmall
		}
		conds[c.name] = c
	}
	return conds
}

// areaRatio is the implied-area ratio of the shape-change pair. The design calls
// for 2; the angle conditions land a little off because the paper fixes the
// angles (and their ratio) rather than the areas.
func (c condition) areaRatio() float64 { return c.large.area / c.small.area }

// describe is one line per condition for the session metadata file.
func (c condition) describe() string {
	return fmt.Sprintf(
		"condition %s (%s): small=%s area=%.4f, large=%s area=%.4f, area ratio=%.4f, "+
			"area-only scale from small=%.4f from large=%.4f, rotation=%s, flips=%v",
		c.name, c.label,
		c.small.id, c.small.area, c.large.id, c.large.area, c.areaRatio(),
		c.scaleFromSmall, c.scaleFromLarge, c.rotationRange(), c.allowFlip)
}

// rotationRange describes the orientation variation the way the paper does.
func (c condition) rotationRange() string {
	if c.rotationDeg >= 360 {
		return "0-359°"
	}
	return fmt.Sprintf("±0-%.0f°", c.rotationDeg)
}
