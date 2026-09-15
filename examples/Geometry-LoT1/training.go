// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import "github.com/chrplr/goxpyriment/control"

// Training stimuli (SI Appendix, "Adults, experiment 2", point 4).
//
// Block A used the ten picture pairs of the baboons' "Train 5" stage (Fig. 3B:
// coloured discs, watermelon/apple, letters, stars, gratings, …) and block B
// the three easy polygon pairs of "generalization 2" (Fig. 3A). The bitmaps
// were never published, and their only role is to teach the
// click-the-odd-one-out rule, so block A uses procedural vector stand-ins
// that keep the flavour of each original pair (colour, texture, orientation,
// outline vs filled). Being polygons, they get the same rotation and scaling
// as the quadrilaterals for free.
//
// Every part must be convex (stimuli.Shape fills a triangle fan), so concave
// figures — stars, the chevron, the letters — are assembled from several
// convex parts.

// pair is one training pair; either member can be the intruder.
type pair struct{ a, b figure }

const (
	discR   = 0.75 // circumradius of the discs and regular polygons, shape units
	barHalf = 0.14 // half-thickness of the bars used for letters and lines
)

func disc(name string, color control.Color) figure {
	return figure{name: name, parts: []part{{regularPolygon(32, discR, 0), color}}}
}

// halfDisc is the upper half of a disc (flat edge at the bottom).
func halfDisc(name string, color control.Color) figure {
	pts := regularPolygon(32, discR, 0)[:17] // angles 0 … 180°
	return figure{name: name, parts: []part{{pts, color}}}
}

// quarterDisc is the upper-right quarter of a disc.
func quarterDisc(name string, color control.Color) figure {
	pts := append([]control.FPoint{control.Origin()}, regularPolygon(32, discR, 0)[:9]...)
	return figure{name: name, parts: []part{{pts, color}}}
}

// bar is an axis-aligned rectangle from (x0,y0) to (x1,y1).
func bar(x0, y0, x1, y1 float32, color control.Color) part {
	return part{[]control.FPoint{
		control.Point(x0, y0), control.Point(x0, y1), control.Point(x1, y1), control.Point(x1, y0),
	}, color}
}

// star is an n-pointed star as a central n-gon plus n triangles.
func star(name string, n int, outer, inner float32, color control.Color) figure {
	o := regularPolygon(n, outer, 90)
	in := regularPolygon(n, inner, 90+180/float64(n))
	parts := []part{{in, color}}
	for i := 0; i < n; i++ {
		prev := in[(i+n-1)%n]
		parts = append(parts, part{[]control.FPoint{prev, o[i], in[i]}, color})
	}
	return figure{name: name, parts: parts}
}

// stripes is three parallel bars, horizontal or vertical.
func stripes(name string, vertical bool, color control.Color) figure {
	var parts []part
	for i := -1; i <= 1; i++ {
		c := float32(i) * 0.5
		if vertical {
			parts = append(parts, bar(c-barHalf, -discR, c+barHalf, discR, color))
		} else {
			parts = append(parts, bar(-discR, c-barHalf, discR, c+barHalf, color))
		}
	}
	return figure{name: name, parts: parts}
}

// trainingPicturePairs are the block-A stand-ins, one per original pair.
func trainingPicturePairs() []pair {
	orange := control.RGB(255, 140, 0)
	cyan := control.RGB(0, 200, 220)
	green := control.RGB(60, 200, 60)
	purple := control.RGB(170, 90, 220)
	w := palette.shape
	return []pair{
		// orange vs cyan blurred discs
		{disc("orange-disc", orange), disc("cyan-disc", cyan)},
		// apple vs watermelon slice
		{disc("red-disc", control.Red), halfDisc("green-half-disc", green)},
		// letters A vs K → T vs L
		{figure{"letter-T", []part{bar(-discR, discR-2*barHalf, discR, discR, w), bar(-barHalf, -discR, barHalf, discR-2*barHalf, w)}},
			figure{"letter-L", []part{bar(-discR, -discR, -discR+2*barHalf, discR, w), bar(-discR+2*barHalf, -discR, discR, -discR+2*barHalf, w)}}},
		// two 4-point stars → 4- vs 5-point star
		{star("star-4", 4, discR, 0.3, w), star("star-5", 5, discR, 0.4, w)},
		// horizontal vs vertical grating
		{stripes("h-stripes", false, w), stripes("v-stripes", true, w)},
		// two half-disc shapes → half vs quarter disc
		{halfDisc("half-disc", w), quarterDisc("quarter-disc", w)},
		// filled vs outline Y → Y vs arrow
		{figure{"letter-Y", []part{
			bar(-barHalf, -discR, barHalf, 0, w),
			{[]control.FPoint{control.Point(-barHalf, 0), control.Point(-discR, discR-barHalf), control.Point(-discR+2*barHalf, discR), control.Point(barHalf, 0)}, w},
			{[]control.FPoint{control.Point(barHalf, 0), control.Point(discR, discR-barHalf), control.Point(discR-2*barHalf, discR), control.Point(-barHalf, 0)}, w},
		}},
			figure{"arrow", []part{
				bar(-barHalf, -discR, barHalf, 0.1, w),
				{[]control.FPoint{control.Point(-0.55, 0), control.Point(0, discR), control.Point(0.55, 0)}, w},
			}}},
		// checkerboard vs dots → ring vs disc (the hole is background-coloured)
		{figure{"ring", []part{{regularPolygon(32, discR, 0), w}, {regularPolygon(32, discR*0.55, 0), palette.background}}}, disc("disc", w)},
		// gradient disc vs grey polygon → crescent vs pentagon
		{figure{"crescent", []part{{regularPolygon(32, discR, 0), purple}, {offset(regularPolygon(32, discR*0.85, 0), 0.4, 0.15), palette.background}}},
			figure{"pentagon", []part{{regularPolygon(5, discR, 90), purple}}}},
		// dotted vs solid line
		{figure{"solid-line", []part{bar(-barHalf, -discR, barHalf, discR, w)}},
			figure{"dashed-line", []part{
				bar(-barHalf, -discR, barHalf, -discR+0.4, w),
				bar(-barHalf, -0.2, barHalf, 0.2, w),
				bar(-barHalf, discR-0.4, barHalf, discR, w),
			}}},
	}
}

// trainingPolygonPairs are the three "generalization 2" pairs of Fig. 3A
// (block B): arrow-head vs pentagon, square vs triangle, triangle vs pentagon.
func trainingPolygonPairs() []pair {
	w := palette.shape
	pentagon := figure{"pentagon", []part{{regularPolygon(5, discR, 90), w}}}
	triangle := figure{"triangle", []part{{[]control.FPoint{
		control.Point(-0.6, 0.7), control.Point(0.8, 0), control.Point(-0.6, -0.7)}, w}}}
	square := figure{"square", []part{bar(-0.6, -0.6, 0.6, 0.6, w)}}
	// Concave chevron: two convex halves split along its axis of symmetry.
	chevron := figure{"chevron", []part{
		{[]control.FPoint{control.Point(-0.8, 0.7), control.Point(0.7, 0), control.Point(-0.3, 0)}, w},
		{[]control.FPoint{control.Point(-0.3, 0), control.Point(0.7, 0), control.Point(-0.8, -0.7)}, w},
	}}
	return []pair{{chevron, pentagon}, {square, triangle}, {triangle, pentagon}}
}

// offset translates a point list.
func offset(pts []control.FPoint, dx, dy float32) []control.FPoint {
	out := make([]control.FPoint, len(pts))
	for i, p := range pts {
		out[i] = control.Point(p.X+dx, p.Y+dy)
	}
	return out
}
