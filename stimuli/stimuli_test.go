package stimuli

import (
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

func TestBaseVisual(t *testing.T) {
	bv := &BaseVisual{Position: sdl.FPoint{X: 10, Y: 20}}
	
	pos := bv.GetPosition()
	if pos.X != 10 || pos.Y != 20 {
		t.Errorf("Expected (10, 20), got (%f, %f)", pos.X, pos.Y)
	}

	bv.SetPosition(sdl.FPoint{X: -5, Y: 100})
	pos = bv.GetPosition()
	if pos.X != -5 || pos.Y != 100 {
		t.Errorf("Expected (-5, 100), got (%f, %f)", pos.X, pos.Y)
	}
}

func TestCircleCreation(t *testing.T) {
	color := sdl.Color{R: 255, G: 0, B: 0, A: 255}
	c := NewCircle(50, color)

	if c.Radius != 50 {
		t.Errorf("Expected radius 50, got %f", c.Radius)
	}
	if c.Color != color {
		t.Errorf("Expected color %v, got %v", color, c.Color)
	}
	if c.Position.X != 0 || c.Position.Y != 0 {
		t.Errorf("Expected default position (0, 0), got %v", c.Position)
	}
}

func TestInsideCircle(t *testing.T) {
	c := NewCircle(10, sdl.Color{})
	c.SetPosition(sdl.FPoint{X: 0, Y: 0})

	areaPos := sdl.FPoint{X: 0, Y: 0}
	if !c.InsideCircle(20, areaPos) {
		t.Error("Circle (r=10) at (0,0) should be inside area (r=20) at (0,0)")
	}

	if c.InsideCircle(5, areaPos) {
		t.Error("Circle (r=10) at (0,0) should NOT be inside area (r=5) at (0,0)")
	}

	c.SetPosition(sdl.FPoint{X: 15, Y: 0})
	if c.InsideCircle(20, areaPos) {
		t.Error("Circle (r=10) at (15,0) should NOT be inside area (r=20) at (0,0) (15+10 > 20)")
	}
}

func TestRectangleCreation(t *testing.T) {
	color := sdl.Color{R: 0, G: 255, B: 0, A: 255}
	r := NewRectangle(10, 20, 100, 200, color)

	if r.Position.X != 10 || r.Position.Y != 20 {
		t.Errorf("Expected position (10, 20), got %v", r.Position)
	}
	if r.Rect.W != 100 || r.Rect.H != 200 {
		t.Errorf("Expected size (100, 200), got %v", r.Rect)
	}
	if r.Color != color {
		t.Errorf("Expected color %v, got %v", color, r.Color)
	}
}

func TestTextLineCreation(t *testing.T) {
	color := sdl.Color{R: 0, G: 0, B: 255, A: 255}
	txt := NewTextLine("Hello", 5, 5, color)

	if txt.Text != "Hello" {
		t.Errorf("Expected text 'Hello', got %q", txt.Text)
	}
	if txt.Position.X != 5 || txt.Position.Y != 5 {
		t.Errorf("Expected position (5, 5), got %v", txt.Position)
	}
	if txt.Color != color {
		t.Errorf("Expected color %v, got %v", color, txt.Color)
	}
}
