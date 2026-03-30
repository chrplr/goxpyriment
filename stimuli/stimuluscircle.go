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

// StimulusCircle represents a collection of stimuli arranged in a circle.
//
// Embeds BaseVisual for position management and lifecycle no-ops.
// Overrides SetPosition to translate all child stimuli when the circle is moved.
type StimulusCircle struct {
	BaseVisual // Position, GetPosition, SetPosition (overridden below), Preload, Unload
	Radius     float32
	Stimuli    []VisualStimulus
}

// NewStimulusCircle creates a new StimulusCircle.
func NewStimulusCircle(radius float32, stimuli []VisualStimulus) *StimulusCircle {
	return &StimulusCircle{
		Radius:  radius,
		Stimuli: stimuli,
		// BaseVisual.Position defaults to (0, 0)
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
func (sc *StimulusCircle) Draw(screen *apparatus.Screen) error {
	for _, stim := range sc.Stimuli {
		if err := stim.Draw(screen); err != nil {
			return err
		}
	}
	return nil
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (sc *StimulusCircle) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(sc, screen, clear, update)
}

// GetPosition is provided by BaseVisual.

// SetPosition overrides BaseVisual.SetPosition to translate all child stimuli
// by the same delta, keeping them in their relative positions on the circle.
func (sc *StimulusCircle) SetPosition(pos sdl.FPoint) {
	dx := pos.X - sc.Position.X
	dy := pos.Y - sc.Position.Y
	sc.Position = pos
	for _, stim := range sc.Stimuli {
		p := stim.GetPosition()
		stim.SetPosition(sdl.FPoint{X: p.X + dx, Y: p.Y + dy})
	}
}

// Preload, Unload are provided by BaseVisual (no-ops).
