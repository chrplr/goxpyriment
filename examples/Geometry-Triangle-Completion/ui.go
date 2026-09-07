// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Palette (after Fig. 1 of the paper: black figures on a pale green field) ──

var (
	bgGreen    = control.RGB(198, 226, 205)
	inkBlack   = control.Black
	buttonFill = control.RGB(178, 208, 186)
	buttonHot  = control.RGB(150, 190, 162) // hovered
	buttonEdge = control.RGB(90, 120, 98)
)

// ── button ────────────────────────────────────────────────────────────────────

// button is a labelled clickable rectangle. Hit-testing is done in the
// centre-based coordinate space that Screen.MousePosition already returns, so
// there is no window→renderer conversion to get wrong and nothing to recompute
// if the logical size changes.
//
// stimuli.ChoiceGrid is not usable for these screens: it clears the display and
// runs its own blocking event loop, so it cannot show the triangle at the same
// time as the options.
type button struct {
	label      string
	cx, cy     float32
	w, h       float32
	hint       string // optional keyboard shortcut shown at the left
	hovered    bool
	labelStim  *stimuli.TextLine
	hintStim   *stimuli.TextLine
	background *stimuli.Rectangle
	border     *stimuli.PolyLine
}

func newButton(label, hint string, cx, cy, w, h float32) *button {
	b := &button{label: label, cx: cx, cy: cy, w: w, h: h, hint: hint}
	b.background = stimuli.NewRectangle(cx, cy, w, h, buttonFill)
	b.border = stimuli.NewPolyLine([]control.FPoint{
		{X: cx - w/2, Y: cy - h/2}, {X: cx + w/2, Y: cy - h/2},
		{X: cx + w/2, Y: cy + h/2}, {X: cx - w/2, Y: cy + h/2},
	}, true, 2, buttonEdge)
	b.labelStim = stimuli.NewTextLine(label, cx, cy, inkBlack)
	if hint != "" {
		b.hintStim = stimuli.NewTextLine(hint, cx-w/2+22, cy, buttonEdge)
	}
	return b
}

func (b *button) contains(x, y float32) bool {
	return x >= b.cx-b.w/2 && x <= b.cx+b.w/2 && y >= b.cy-b.h/2 && y <= b.cy+b.h/2
}

func (b *button) draw(exp *control.Experiment) error {
	b.background.Color = buttonFill
	if b.hovered {
		b.background.Color = buttonHot
	}
	if err := b.background.Draw(exp.Screen); err != nil {
		return err
	}
	if err := b.border.Draw(exp.Screen); err != nil {
		return err
	}
	if b.hintStim != nil {
		if err := b.hintStim.Draw(exp.Screen); err != nil {
			return err
		}
	}
	return b.labelStim.Draw(exp.Screen)
}

func (b *button) unload() {
	_ = b.labelStim.Unload()
	if b.hintStim != nil {
		_ = b.hintStim.Unload()
	}
}

// updateHover refreshes every button's hover state from the cursor position and
// returns the index under the cursor, or -1.
func updateHover(buttons []*button, mx, my float32) int {
	hit := -1
	for i, b := range buttons {
		b.hovered = b.contains(mx, my)
		if b.hovered {
			hit = i
		}
	}
	return hit
}

// drawFragments renders a triangle's visible corner marks.
func drawFragments(exp *control.Experiment, fs []*stimuli.PolyLine) error {
	for _, f := range fs {
		if err := f.Draw(exp.Screen); err != nil {
			return err
		}
	}
	return nil
}
