// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// Package stimuli provides visual and auditory stimulus types (fixation cross,
// text, pictures, video, shapes, sound, etc.) that implement the Stimulus and
// VisualStimulus interfaces for use with the experiment Screen.
//
// # Architecture
//
// Stimulus types follow a layered design:
//
//   - Interfaces: Stimulus and VisualStimulus (this file) define the contracts.
//   - BaseVisual (base.go): embeddable struct providing default Position,
//     GetPosition/SetPosition, and no-op Preload/Unload implementations.
//   - PresentDrawable (base.go): free function implementing the standard
//     clear → draw → update Present() cycle shared by most visual stimuli.
//
// # Implementing a new stimulus
//
// For a typical visual stimulus, embed BaseVisual, implement Draw, and
// delegate Present to PresentDrawable:
//
//	type MyStim struct {
//	    stimuli.BaseVisual   // Position, lifecycle no-ops
//	    // ... your fields
//	}
//
//	func (m *MyStim) Draw(screen *apparatus.Screen) error { /* render */ }
//
//	func (m *MyStim) Present(screen *apparatus.Screen, clear, update bool) error {
//	    return stimuli.PresentDrawable(m, screen, clear, update)
//	}
//
// If your stimulus creates GPU textures, override Unload to destroy them.
// If your stimulus is a container (holds children), override SetPosition
// to translate child positions too (see DotCloud, StimulusCircle).
//
// # Preload pattern
//
// Most stimuli use a "lazy-init" pattern: the public Preload() is a no-op,
// and actual GPU resource creation happens on the first Draw() call via a
// private preload(screen) method. This avoids requiring a Screen reference
// at construction time. For eager preloading (e.g., before a time-critical
// trial), use PreloadVisualOnScreen or PreloadAllVisual.
package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Stimulus is the interface for all visual and auditory stimuli.
//
// Present draws (and optionally clears/updates) the stimulus on screen.
// Preload prepares resources ahead of time (no-op for most types).
// Unload releases GPU or audio resources; safe to call multiple times.
type Stimulus interface {
	Present(screen *apparatus.Screen, clear, update bool) error
	Preload() error
	Unload() error
}

// VisualStimulus represents a visual stimulus that can be drawn on a Screen.
//
// Draw renders the stimulus onto the current render target without clearing
// or flipping. GetPosition/SetPosition manage the center-based position.
//
// Most implementations embed BaseVisual (see base.go) for the default
// position and lifecycle methods, and only need to implement Draw.
type VisualStimulus interface {
	Stimulus
	Draw(screen *apparatus.Screen) error
	GetPosition() sdl.FPoint
	SetPosition(pos sdl.FPoint)
}

// ---------------------------------------------------------------------------
// Audio stimulus conventions
// ---------------------------------------------------------------------------
//
// Sound and Tone implement the Stimulus interface (Present/Preload/Unload)
// but NOT VisualStimulus, because they have no position or Draw method.
//
// Their Present methods ignore the screen/clear/update parameters and simply
// play the audio. Resource preparation uses a device-specific method:
//
//	sound.PreloadDevice(audioDeviceID)  // loads WAV, creates SDL audio stream
//	tone.PreloadDevice(audioDeviceID)   // generates PCM data, creates stream
//
// This separate PreloadDevice method is needed because audio resources are
// bound to a specific SDL audio device, unlike visual resources that share
// a single renderer.

// PreloadVisualOnScreen attempts to preload GPU resources for a visual
// stimulus on the given screen without presenting it. For known stimulus
// types it calls their internal preload routines; for others it falls back
// to calling Draw, which will lazily allocate textures without updating the
// screen contents.
func PreloadVisualOnScreen(screen *apparatus.Screen, v VisualStimulus) error {
	switch s := v.(type) {
	case *TextLine:
		f := s.Font
		if f == nil {
			f = screen.DefaultFont
		}
		if f == nil {
			return nil
		}
		return s.preload(screen, f)
	case *TextBox:
		f := s.Font
		if f == nil {
			f = screen.DefaultFont
		}
		if f == nil {
			return nil
		}
		return s.preload(screen, f)
	case *Picture:
		return s.preload(screen)
	case *VisualMask:
		return s.preload(screen)
	case *RDS:
		return s.preload(screen)
	default:
		// Fallback: let the stimulus lazily allocate its resources via Draw.
		return v.Draw(screen)
	}
}

// PreloadAllVisual preloads a slice of visual stimuli on the given screen.
func PreloadAllVisual(screen *apparatus.Screen, visuals []VisualStimulus) error {
	for _, v := range visuals {
		if err := PreloadVisualOnScreen(screen, v); err != nil {
			return err
		}
	}
	return nil
}
