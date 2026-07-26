// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package stimuli

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
)

// Rectangle is a filled rectangle with center at (x,y) and size (w,h) in center-based coordinates.
//
// Embeds BaseVisual for position management and lifecycle no-ops; delegates
// Present to PresentDrawable — only Draw contains stimulus-specific logic.
type Rectangle struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload
	Rect       sdl.FRect
	Color      sdl.Color
}

// NewRectangle creates a rectangle centered at (x, y) with width w, height h, and the given color.
func NewRectangle(x, y, w, h float32, color sdl.Color) *Rectangle {
	return &Rectangle{
		BaseVisual: BaseVisual{Position: sdl.FPoint{X: x, Y: y}},
		Rect:       sdl.FRect{W: w, H: h},
		Color:      color,
	}
}

func (r *Rectangle) Draw(screen *apparatus.Screen) error {
	if err := screen.Renderer.SetDrawColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A); err != nil {
		return fmt.Errorf("stimuli.Rectangle.Draw: setting draw color: %w", err)
	}

	// Centering the rectangle at the target position
	destRect := screen.CenteredRect(r.Position, r.Rect.W, r.Rect.H)
	return screen.Renderer.RenderFillRect(destRect)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (r *Rectangle) Present(screen *apparatus.Screen, clear, update bool) error {
	return PresentDrawable(r, screen, clear, update)
}

// Preload, Unload, GetPosition, SetPosition are all provided by BaseVisual.
