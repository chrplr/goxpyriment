// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/chrplr/goxpyriment/apparatus"
)

// DrawFunc turns a function that draws a whole scene into a VisualStimulus,
// so a composite display — several pictures, labels, a highlight — can go
// through exp.Show / exp.ShowTS like any single stimulus:
//
//	scene := stimuli.DrawFunc(func(s *apparatus.Screen) error {
//		if err := left.Draw(s); err != nil {
//			return err
//		}
//		return right.Draw(s)
//	})
//	onset, err := exp.ShowTS(scene)
//
// The function must be able to run more than once: ShowTS redraws the scene
// for every frame it holds (see control.Experiment.ShowTS and the onset
// marker). It draws without clearing; Present clears first. A DrawFunc has no
// position of its own (SetPosition is ignored) and owns no resources (Preload
// and Unload do nothing): the stimuli it draws keep theirs.
type DrawFunc func(screen *apparatus.Screen) error

// Draw calls the function.
func (f DrawFunc) Draw(screen *apparatus.Screen) error { return f(screen) }

// Present clears the screen if asked, draws the scene and flips if asked.
func (f DrawFunc) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(f, screen, clear, update)
}

// Preload does nothing: the stimuli the function draws own their resources.
func (f DrawFunc) Preload() error { return nil }

// Unload does nothing, for the same reason.
func (f DrawFunc) Unload() error { return nil }

// GetPosition returns the origin: a scene has no position of its own.
func (f DrawFunc) GetPosition() sdl.FPoint { return sdl.FPoint{} }

// SetPosition is ignored.
func (f DrawFunc) SetPosition(sdl.FPoint) {}
