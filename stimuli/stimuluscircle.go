// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/chrplr/goxpyriment/io"
	"math"
	"math/rand"
	"github.com/Zyko0/go-sdl3/sdl"
)

// StimulusCircle represents a collection of stimuli arranged in a circle.
type StimulusCircle struct {
	Radius   float32
	Stimuli  []VisualStimulus
	Position sdl.FPoint
}

// NewStimulusCircle creates a new StimulusCircle.
func NewStimulusCircle(radius float32, stimuli []VisualStimulus) *StimulusCircle {
	return &StimulusCircle{
		Radius:  radius,
		Stimuli: stimuli,
		Position: sdl.FPoint{X: 0, Y: 0},
	}
}

// Make arranges the stimuli in a circle.
func (sc *StimulusCircle) Make(shuffle bool, jitter bool) {
	n := len(sc.Stimuli)
	if n == 0 {
		return
	}
	
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	
	if shuffle {
		rand.Shuffle(n, func(i, j int) {
			indices[i], indices[j] = indices[j], indices[i]
		})
	}
	
	step := 2 * math.Pi / float64(n)
	offset := 0.0
	if jitter {
		offset = rand.Float64() * step
	}
	
	for i, idx := range indices {
		angle := offset + float64(i)*step - math.Pi/2
		x := sc.Radius * float32(math.Cos(angle))
		y := sc.Radius * float32(math.Sin(angle))
		
		sc.Stimuli[idx].SetPosition(sdl.FPoint{X: sc.Position.X + x, Y: sc.Position.Y + y})
	}
}

// Draw draws all stimuli in the circle.
func (sc *StimulusCircle) Draw(screen *io.Screen) error {
	for _, stim := range sc.Stimuli {
		if err := stim.Draw(screen); err != nil {
			return err
		}
	}
	return nil
}

// Present clears and draws the circle.
func (sc *StimulusCircle) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := sc.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (sc *StimulusCircle) GetPosition() sdl.FPoint {
	return sc.Position
}

func (sc *StimulusCircle) SetPosition(pos sdl.FPoint) {
	// When we move the circle, we should move all its stimuli too
	dx := pos.X - sc.Position.X
	dy := pos.Y - sc.Position.Y
	sc.Position = pos
	for _, stim := range sc.Stimuli {
		p := stim.GetPosition()
		stim.SetPosition(sdl.FPoint{X: p.X + dx, Y: p.Y + dy})
	}
}

func (sc *StimulusCircle) Preload() error { return nil }
func (sc *StimulusCircle) Unload() error  { return nil }
