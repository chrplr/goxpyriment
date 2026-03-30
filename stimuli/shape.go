// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Shape is a filled polygon defined by Points (relative to the shape center) and a single color.
//
// Embeds BaseVisual for position management and lifecycle no-ops.
type Shape struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload
	Points     []sdl.FPoint
	Color      sdl.Color
}

// NewShape creates a new polygon shape. Points are relative to the shape's center.
func NewShape(points []sdl.FPoint, color sdl.Color) *Shape {
	return &Shape{
		Points: points,
		Color:  color,
		// BaseVisual.Position defaults to (0, 0)
	}
}

func (s *Shape) Draw(screen *apparatus.Screen) error {
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
			Color:    sdl.FColor{R: float32(s.Color.R) / 255, G: float32(s.Color.G) / 255, B: float32(s.Color.B) / 255, A: float32(s.Color.A) / 255},
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

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (s *Shape) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(s, screen, clear, update)
}

// Preload, Unload, GetPosition, SetPosition are all provided by BaseVisual.
