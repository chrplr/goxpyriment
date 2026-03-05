// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// Shape represents a polygon stimulus.
type Shape struct {
	Points   []sdl.FPoint
	Position sdl.FPoint
	Color    sdl.Color
}

// NewShape creates a new polygon shape. Points are relative to the shape's center.
func NewShape(points []sdl.FPoint, color sdl.Color) *Shape {
	return &Shape{
		Points:   points,
		Color:    color,
		Position: sdl.FPoint{X: 0, Y: 0},
	}
}

func (s *Shape) Draw(screen *io.Screen) error {
	if len(s.Points) < 3 {
		return nil // Not a polygon
	}

	cX, cY := screen.CenterToSDL(s.Position.X, s.Position.Y)
	
	// Create vertices for RenderGeometry
	// For a simple filled polygon, we treat it as a fan from the first point (simplistic)
	// A more robust implementation would use a triangulation algorithm.
	vertices := make([]sdl.Vertex, len(s.Points))
	for i, p := range s.Points {
		vertices[i] = sdl.Vertex{
			Position: sdl.FPoint{X: cX + p.X, Y: cY - p.Y},
			Color:    sdl.FColor{R: float32(s.Color.R)/255, G: float32(s.Color.G)/255, B: float32(s.Color.B)/255, A: float32(s.Color.A)/255},
		}
	}

	// Create indices for a triangle fan
	indices := make([]int32, (len(s.Points)-2)*3)
	for i := 0; i < len(s.Points)-2; i++ {
		indices[i*3] = 0
		indices[i*3+1] = int32(i + 1)
		indices[i*3+2] = int32(i + 2)
	}

	return screen.Renderer.RenderGeometry(nil, vertices, indices)
}

func (s *Shape) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := s.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (s *Shape) Preload() error { return nil }
func (s *Shape) Unload() error  { return nil }

func (s *Shape) GetPosition() sdl.FPoint { return s.Position }
func (s *Shape) SetPosition(pos sdl.FPoint) { s.Position = pos }
