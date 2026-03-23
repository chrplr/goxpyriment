// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// Package geometry provides point/distance and coordinate conversion helpers
// (Euclidean distance, Cartesian–polar conversion, degree–radian).
package geometry

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
)

// GetDistance returns the Euclidean distance between two points.
func GetDistance(p1, p2 sdl.FPoint) float32 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

// CartesianToPolar converts (x, y) to (radius, angle_in_degrees).
func CartesianToPolar(x, y float32) (float32, float32) {
	r := float32(math.Sqrt(float64(x*x + y*y)))
	a := float32(math.Atan2(float64(y), float64(x)) * 180 / math.Pi)
	return r, a
}

// PolarToCartesian converts (radius, angle_in_degrees) to (x, y).
func PolarToCartesian(r, a float32) (float32, float32) {
	rad := float64(a) * math.Pi / 180.0
	x := r * float32(math.Cos(rad))
	y := r * float32(math.Sin(rad))
	return x, y
}

// DegreeToRadian converts degrees to radians.
func DegreeToRadian(deg float32) float64 {
	return float64(deg) * math.Pi / 180.0
}
