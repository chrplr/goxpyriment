// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math"

	"github.com/chrplr/goxpyriment/control"
)

// The 11 reference quadrilaterals of Sablé-Meyer et al. (2021), Table S1 of
// the SI Appendix. Coordinates are in arbitrary units with the bottom-left
// vertex at the origin and y pointing up; the polygon is BL → TL → TR → BR.
// The average of the six pairwise vertex distances is 1.434 for every shape
// (the authors' size-matching criterion; checked in shapes_test.go).
//
// The order is the predicted-regularity order of Fig. 1A.
var referenceShapes = []struct {
	name       string
	tl, tr, br control.FPoint
}{
	{"square", control.Point(0.000, 1.260), control.Point(1.260, 1.260), control.Point(1.260, 0)},
	{"rectangle", control.Point(0.000, 1.000), control.Point(1.500, 1.000), control.Point(1.500, 0)},
	{"rhombus", control.Point(-0.908, 0.931), control.Point(0.392, 0.931), control.Point(1.300, 0)},
	{"parallelogram", control.Point(-0.517, 0.896), control.Point(0.983, 0.896), control.Point(1.500, 0)},
	{"right-kite", control.Point(0.529, 1.404), control.Point(1.500, 1.038), control.Point(1.500, 0)},
	{"iso-trapezoid", control.Point(0.365, 1.362), control.Point(1.109, 1.362), control.Point(1.500, 0)},
	{"kite", control.Point(0.766, 1.290), control.Point(1.770, 1.007), control.Point(1.500, 0)},
	{"right-hinge", control.Point(-0.296, 0.634), control.Point(1.064, 1.268), control.Point(1.500, 0)},
	{"hinge", control.Point(-0.248, 0.533), control.Point(0.980, 1.393), control.Point(1.500, 0)},
	{"trapezoid", control.Point(-0.227, 1.200), control.Point(0.724, 1.200), control.Point(1.500, 0)},
	{"irregular", control.Point(-0.450, 1.058), control.Point(0.227, 1.240), control.Point(1.500, 0)},
}

// avgPairwiseDistance is the matched mean distance between all pairs of
// vertices (Table S1, "Avg pairs").
const avgPairwiseDistance = 1.434

// deviantFraction is the displacement of the bottom-right vertex as a
// fraction of avgPairwiseDistance: 30 % in every experiment except the
// sequence one (55 %).
const deviantFraction = 0.30

// deviantNames lists the four deviants in the order used for the data file.
var deviantNames = []string{"shorter", "longer", "rot_up", "rot_down"}

// part is one filled convex polygon of a figure. Points are relative to the
// figure's own origin (y up), in shape units.
type part struct {
	points []control.FPoint
	color  control.Color
}

// figure is what goes into one slot of the display: one or more filled
// polygons. The quadrilaterals are a single white part; the training figures
// (training.go) may have several coloured parts.
type figure struct {
	name  string
	parts []part
}

// quadrilateral builds a single-part white figure from the three free
// vertices (the bottom-left one is the origin).
func quadrilateral(name string, tl, tr, br control.FPoint) figure {
	return figure{name: name, parts: []part{{
		points: []control.FPoint{control.Origin(), tl, tr, br},
		color:  control.White,
	}}}
}

// referenceFigure returns the reference quadrilateral of the given name.
func referenceFigure(name string) (figure, error) {
	for _, s := range referenceShapes {
		if s.name == name {
			return quadrilateral(name, s.tl, s.tr, s.br), nil
		}
	}
	return figure{}, fmt.Errorf("unknown shape %q", name)
}

// deviantFigure returns one of the four deviants of a reference shape. Only
// the bottom-right vertex BR moves, by a Euclidean distance d = 30 % of the
// matched average pairwise distance: ±d along the line of the bottom edge
// ("shorter", "longer"), or on the circle centred on BL through BR, by the
// angle whose chord is d ("rot_up", "rot_down"). Fig. 1A, bottom-right cell.
func deviantFigure(name, deviant string) (figure, error) {
	ref, err := referenceFigure(name)
	if err != nil {
		return figure{}, err
	}
	pts := ref.parts[0].points
	br := pts[3]
	d := deviantFraction * avgPairwiseDistance
	l := float64(br.X) // |BR|: the bottom edge lies on the x axis
	theta := 2 * math.Asin(d/(2*l))
	var nbr control.FPoint
	switch deviant {
	case "shorter":
		nbr = control.Point(br.X-float32(d), 0)
	case "longer":
		nbr = control.Point(br.X+float32(d), 0)
	case "rot_up":
		nbr = control.Point(float32(l*math.Cos(theta)), float32(l*math.Sin(theta)))
	case "rot_down":
		nbr = control.Point(float32(l*math.Cos(theta)), -float32(l*math.Sin(theta)))
	default:
		return figure{}, fmt.Errorf("unknown deviant %q", deviant)
	}
	return quadrilateral(name+"/"+deviant, pts[1], pts[2], nbr), nil
}

// polygonArea returns the signed area (positive for counter-clockwise).
func polygonArea(p []control.FPoint) float64 {
	var a float64
	for i := range p {
		j := (i + 1) % len(p)
		a += float64(p[i].X)*float64(p[j].Y) - float64(p[j].X)*float64(p[i].Y)
	}
	return a / 2
}

// polygonCentroid returns the area centroid of a simple polygon.
func polygonCentroid(p []control.FPoint) control.FPoint {
	a := polygonArea(p)
	var cx, cy float64
	for i := range p {
		j := (i + 1) % len(p)
		c := float64(p[i].X)*float64(p[j].Y) - float64(p[j].X)*float64(p[i].Y)
		cx += (float64(p[i].X) + float64(p[j].X)) * c
		cy += (float64(p[i].Y) + float64(p[j].Y)) * c
	}
	return control.Point(float32(cx/(6*a)), float32(cy/(6*a)))
}

// isConvex reports whether a polygon turns the same way at every vertex.
func isConvex(p []control.FPoint) bool {
	pos, neg := false, false
	for i := range p {
		a, b, c := p[i], p[(i+1)%len(p)], p[(i+2)%len(p)]
		cross := (b.X-a.X)*(c.Y-b.Y) - (b.Y-a.Y)*(c.X-b.X)
		if cross > 0 {
			pos = true
		} else if cross < 0 {
			neg = true
		}
	}
	return !(pos && neg)
}

// centre returns the point that must sit at the slot centre: the area
// centroid for a single-part figure (the paper centres each quadrilateral on
// its centre of mass — recomputed for every deviant, otherwise the outlier
// would betray itself by an offset), the bounding-box centre for composite
// training figures whose parts overlap.
func (f figure) centre() control.FPoint {
	if len(f.parts) == 1 {
		return polygonCentroid(f.parts[0].points)
	}
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, pt := range f.parts {
		for _, p := range pt.points {
			minX, maxX = min(minX, p.X), max(maxX, p.X)
			minY, maxY = min(minY, p.Y), max(maxY, p.Y)
		}
	}
	return control.Point((minX+maxX)/2, (minY+maxY)/2)
}

// transform returns the figure's parts in pixels relative to the slot centre:
// translate the centre to the origin, multiply by unitPx·scale, rotate by
// rotDeg counter-clockwise.
func (f figure) transform(unitPx, scale float32, rotDeg float64) []part {
	c := f.centre()
	cos, sin := math.Cos(rotDeg*math.Pi/180), math.Sin(rotDeg*math.Pi/180)
	out := make([]part, len(f.parts))
	for i, pt := range f.parts {
		pts := make([]control.FPoint, len(pt.points))
		for j, p := range pt.points {
			x := float64((p.X - c.X) * unitPx * scale)
			y := float64((p.Y - c.Y) * unitPx * scale)
			pts[j] = control.Point(float32(x*cos-y*sin), float32(x*sin+y*cos))
		}
		out[i] = part{points: pts, color: pt.color}
	}
	return out
}

// pointInPolygon is the even-odd rule test.
func pointInPolygon(x, y float32, p []control.FPoint) bool {
	inside := false
	for i, j := 0, len(p)-1; i < len(p); j, i = i, i+1 {
		if (p[i].Y > y) != (p[j].Y > y) &&
			x < (p[j].X-p[i].X)*(y-p[i].Y)/(p[j].Y-p[i].Y)+p[i].X {
			inside = !inside
		}
	}
	return inside
}

// regularPolygon returns an n-gon of the given circumradius centred on the
// origin, first vertex at angle phaseDeg.
func regularPolygon(n int, radius float32, phaseDeg float64) []control.FPoint {
	pts := make([]control.FPoint, n)
	for i := range pts {
		a := phaseDeg*math.Pi/180 + 2*math.Pi*float64(i)/float64(n)
		pts[i] = control.Point(radius*float32(math.Cos(a)), radius*float32(math.Sin(a)))
	}
	return pts
}
