// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Package stimuli provides visual and auditory stimulus types (fixation cross,
// text, pictures, video, shapes, sound, etc.) that implement the Stimulus and
// VisualStimulus interfaces for use with the experiment Screen.
package stimuli

import (
	"github.com/chrplr/goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// Stimulus is the interface for all visual and auditory stimuli.
type Stimulus interface {
	Present(screen *io.Screen, clear, update bool) error
	Preload() error
	Unload() error
}

// VisualStimulus represents a visual stimulus that can be drawn on a Screen.
type VisualStimulus interface {
	Stimulus
	Draw(screen *io.Screen) error
	GetPosition() sdl.FPoint
	SetPosition(pos sdl.FPoint)
}

// PreloadVisualOnScreen attempts to preload GPU resources for a visual
// stimulus on the given screen without presenting it. For known stimulus
// types it calls their internal preload routines; for others it falls back
// to calling Draw, which will lazily allocate textures without updating the
// screen contents.
func PreloadVisualOnScreen(screen *io.Screen, v VisualStimulus) error {
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
	default:
		// Fallback: let the stimulus lazily allocate its resources via Draw.
		return v.Draw(screen)
	}
}

// PreloadAllVisual preloads a slice of visual stimuli on the given screen.
func PreloadAllVisual(screen *io.Screen, visuals []VisualStimulus) error {
	for _, v := range visuals {
		if err := PreloadVisualOnScreen(screen, v); err != nil {
			return err
		}
	}
	return nil
}

