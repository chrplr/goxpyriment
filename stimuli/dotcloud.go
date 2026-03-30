// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"math"
	"math/rand"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// DotCloud represents a circular cloud of dots.
//
// Embeds BaseVisual for position management and lifecycle no-ops.
// Overrides SetPosition to translate all child dots when the cloud is moved.
type DotCloud struct {
	BaseVisual      // Position, GetPosition, SetPosition (overridden below), Preload, Unload
	Radius          float32
	BackgroundColor sdl.Color
	DotColor        sdl.Color
	Dots            []*Circle
}

// NewDotCloud creates a new DotCloud.
func NewDotCloud(radius float32, bgColor, dotColor sdl.Color) *DotCloud {
	return &DotCloud{
		Radius: radius,
		// BaseVisual.Position defaults to (0, 0)
		BackgroundColor: bgColor,
		DotColor:        dotColor,
		Dots:            make([]*Circle, 0),
	}
}

// Make generates the dots randomly within the cloud.
func (dc *DotCloud) Make(nDots int, dotRadius float32, gap float32) bool {
	dc.Dots = make([]*Circle, 0, nDots)

	for i := 0; i < nDots; i++ {
		reps := 0
		for {
			dot := NewCircle(dotRadius, dc.DotColor)

			// Random position within square bounding box of radius
			x := rand.Float32()*(2*dc.Radius-2*dotRadius) - (dc.Radius - dotRadius)
			y := rand.Float32()*(2*dc.Radius-2*dotRadius) - (dc.Radius - dotRadius)
			dot.Position = sdl.FPoint{X: dc.Position.X + x, Y: dc.Position.Y + y}

			// Check if inside the cloud radius
			if dot.InsideCircle(dc.Radius, dc.Position) {
				// Check overlap with existing dots
				overlapping := false
				for _, other := range dc.Dots {
					dx := dot.Position.X - other.Position.X
					dy := dot.Position.Y - other.Position.Y
					dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
					if dist < dot.Radius+other.Radius+gap {
						overlapping = true
						break
					}
				}

				if !overlapping {
					dc.Dots = append(dc.Dots, dot)
					break
				}
			}

			reps++
			if reps > 10000 {
				return false
			}
		}
	}
	return true
}

// Draw draws all the dots and the background if specified.
func (dc *DotCloud) Draw(screen *apparatus.Screen) error {
	// If background color is not transparent (A > 0), draw it
	if dc.BackgroundColor.A > 0 {
		bgCircle := NewCircle(dc.Radius, dc.BackgroundColor)
		bgCircle.Position = dc.Position
		if err := bgCircle.Draw(screen); err != nil {
			return err
		}
	}

	for _, dot := range dc.Dots {
		if err := dot.Draw(screen); err != nil {
			return err
		}
	}
	return nil
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (dc *DotCloud) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(dc, screen, clear, update)
}

// GetPosition is provided by BaseVisual.

// SetPosition overrides BaseVisual.SetPosition to translate all child dots
// by the same delta, keeping them in their relative positions within the cloud.
func (dc *DotCloud) SetPosition(pos sdl.FPoint) {
	dx := pos.X - dc.Position.X
	dy := pos.Y - dc.Position.Y
	dc.Position = pos
	for _, dot := range dc.Dots {
		p := dot.GetPosition()
		dot.SetPosition(sdl.FPoint{X: p.X + dx, Y: p.Y + dy})
	}
}

// Preload, Unload are provided by BaseVisual (no-ops).
