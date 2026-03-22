package geometry

import (
	"math"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

func TestGetDistance(t *testing.T) {
	p1 := sdl.FPoint{X: 0, Y: 0}
	p2 := sdl.FPoint{X: 3, Y: 4}
	expected := float32(5.0)
	got := GetDistance(p1, p2)
	if got != expected {
		t.Errorf("Expected distance 5.0, got %f", got)
	}
}

func TestCartesianToPolar(t *testing.T) {
	x, y := float32(10), float32(0)
	r, a := CartesianToPolar(x, y)
	if r != 10 {
		t.Errorf("Expected radius 10, got %f", r)
	}
	if a != 0 {
		t.Errorf("Expected angle 0, got %f", a)
	}

	x2, y2 := float32(0), float32(10)
	r2, a2 := CartesianToPolar(x2, y2)
	if r2 != 10 {
		t.Errorf("Expected radius 10, got %f", r2)
	}
	if a2 != 90 {
		t.Errorf("Expected angle 90, got %f", a2)
	}
}

func TestPolarToCartesian(t *testing.T) {
	r, a := float32(10), float32(0)
	x, y := PolarToCartesian(r, a)
	if x != 10 || y != 0 {
		t.Errorf("Expected (10, 0), got (%f, %f)", x, y)
	}

	r2, a2 := float32(10), float32(90)
	x2, y2 := PolarToCartesian(r2, a2)
	// Use a small epsilon for float comparison due to precision
	if math.Abs(float64(x2)) > 1e-6 || math.Abs(float64(y2-10)) > 1e-6 {
		t.Errorf("Expected (0, 10), got (%f, %f)", x2, y2)
	}
}

func TestDegreeToRadian(t *testing.T) {
	deg := float32(180)
	expected := math.Pi
	got := DegreeToRadian(deg)
	if math.Abs(got-expected) > 1e-6 {
		t.Errorf("Expected %f radians, got %f", expected, got)
	}
}
