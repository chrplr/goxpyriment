// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"goxpyriment/io"
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
