// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package stimuli

// base.go — Shared building blocks for visual stimulus types.
//
// RATIONALE: Nearly every visual stimulus in this package needs the same
// boilerplate: a center-based Position with trivial GetPosition/SetPosition,
// no-op Preload/Unload, and a Present method that clears → draws → updates.
//
// Rather than copy-pasting this code into 15+ files, we provide:
//
//   - BaseVisual  — an embeddable struct that satisfies the position and
//     lifecycle parts of the VisualStimulus interface.
//
//   - PresentDrawable — a free function that implements the standard
//     clear → draw → update Present() contract.
//
// Together they let a new stimulus type satisfy the full VisualStimulus
// interface with just a Draw method and a one-line Present delegation.

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// ---------------------------------------------------------------------------
// BaseVisual — embeddable default implementations
// ---------------------------------------------------------------------------

// BaseVisual provides default implementations of GetPosition, SetPosition,
// Preload, and Unload for visual stimuli that store a simple center-based
// Position and have no GPU resources to manage at preload/unload time.
//
// Embedding BaseVisual eliminates the repetitive boilerplate that would
// otherwise appear in every stimulus type:
//
//	type MyStimulus struct {
//	    stimuli.BaseVisual   // provides Position, GetPosition, SetPosition, Preload, Unload
//	    // ... stimulus-specific fields
//	}
//
// Types that manage GPU textures should override Unload to release them.
// Types with composite positions (e.g. Line with Start/End, or container
// types that move children) should override GetPosition and/or SetPosition.
type BaseVisual struct {
	Position sdl.FPoint
}

// GetPosition returns the center-based position of the stimulus.
func (b *BaseVisual) GetPosition() sdl.FPoint {
	return b.Position
}

// SetPosition updates the center-based position of the stimulus.
func (b *BaseVisual) SetPosition(pos sdl.FPoint) {
	b.Position = pos
}

// Preload is a no-op for stimuli that don't need advance GPU preparation.
// Types with expensive setup (e.g., texture generation) should override this.
func (b *BaseVisual) Preload() error {
	return nil
}

// Unload is a no-op for stimuli with no GPU resources.
// Types that create SDL textures should override this to call Texture.Destroy().
func (b *BaseVisual) Unload() error {
	return nil
}

// ---------------------------------------------------------------------------
// PresentDrawable — standard Present() logic
// ---------------------------------------------------------------------------

// Drawable is the subset of VisualStimulus required by PresentDrawable.
// Any type with a Draw method satisfies it.
type Drawable interface {
	Draw(screen *apparatus.Screen) error
}

// PresentDrawable implements the standard Present() logic shared by most
// visual stimuli: optionally clear the screen, draw the stimulus, and
// optionally flip the backbuffer.
//
// Individual stimulus types delegate their Present method to this function:
//
//	func (r *Rectangle) Present(screen *apparatus.Screen, clear, update bool) error {
//	    return stimuli.PresentDrawable(r, screen, clear, update)
//	}
//
// Stimuli with non-standard Present behaviour (e.g., BlankScreen ignores
// the clear flag, audio stimuli ignore the screen entirely) should NOT use
// this helper and should implement Present directly.
func PresentDrawable(d Drawable, screen *apparatus.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := d.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}
