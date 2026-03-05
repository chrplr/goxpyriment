// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/chrplr/goxpyriment/io"
	"math"
	"math/rand"
	"github.com/Zyko0/go-sdl3/sdl"
)

// DotCloud represents a circular cloud of dots.
type DotCloud struct {
	Radius          float32
	Position        sdl.FPoint
	BackgroundColor sdl.Color
	DotColor        sdl.Color
	Dots            []*Circle
}

// NewDotCloud creates a new DotCloud.
func NewDotCloud(radius float32, bgColor, dotColor sdl.Color) *DotCloud {
	return &DotCloud{
		Radius:          radius,
		Position:        sdl.FPoint{X: 0, Y: 0},
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
func (dc *DotCloud) Draw(screen *io.Screen) error {
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

// Present clears and draws the cloud.
func (dc *DotCloud) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := dc.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (dc *DotCloud) GetPosition() sdl.FPoint {
	return dc.Position
}

func (dc *DotCloud) SetPosition(pos sdl.FPoint) {
	dx := pos.X - dc.Position.X
	dy := pos.Y - dc.Position.Y
	dc.Position = pos
	for _, dot := range dc.Dots {
		p := dot.GetPosition()
		dot.SetPosition(sdl.FPoint{X: p.X + dx, Y: p.Y + dy})
	}
}

func (dc *DotCloud) Preload() error { return nil }
func (dc *DotCloud) Unload() error  { return nil }
