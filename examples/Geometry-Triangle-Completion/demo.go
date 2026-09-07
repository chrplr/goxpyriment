// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Demonstration phase (paper §2.3) ──────────────────────────────────────────
//
// Before the reasoning task the participant is shown the sample fragmented
// triangle — first its two base corners, then the complete triangle, then the
// base corners again — and then what each of the four possible changes to those
// corners looks like, one button per change.
//
// The paper's experimenter demonstrated the changes by hand; here each button
// animates the sample triangle smoothly through its transformation, which is
// what a 7-year-old actually needs to see. The display stays reachable during
// the test: any trial screen offers "show sample changes".

const (
	demoCycleMs    = 1500 // one sweep of a transformation
	introHoldMs    = 1400 // how long each step of the intro sequence is held
	demoAngleSwing = 12.0 // degrees the base angles move in the demo
	demoBaseSwing  = 0.22 // units the base length moves in the demo
)

// morph returns the sample triangle part-way through a transformation.
// phase runs 0 → 1 → 0 so the animation breathes rather than jumping back.
func morph(t transformation, phase float64) tri {
	s := sampleTriangle
	switch t {
	case angleIncrease:
		s.leftDeg += demoAngleSwing * phase
		s.rightDeg += demoAngleSwing * phase
	case angleDecrease:
		s.leftDeg -= demoAngleSwing * phase
		s.rightDeg -= demoAngleSwing * phase
	case distanceIncrease:
		s.baseUnits += demoBaseSwing * phase
	case distanceDecrease:
		s.baseUnits -= demoBaseSwing * phase
	}
	return s
}

// runIntro plays the fixed opening sequence: corners → whole triangle → corners.
func runIntro(exp *control.Experiment, cfg config) error {
	frags := sampleTriangle.fragments(cfg.baseY, cfg.fragmentFrac, cfg.strokePx, inkBlack)
	whole := sampleTriangle.complete(cfg.baseY, cfg.strokePx, inkBlack)

	steps := []struct {
		text string
		full bool
	}{
		{"This is a partial triangle. Two corners are showing.", false},
		{"Here is the whole triangle — this is the corner we cannot see.", true},
		{"And here are the two corners again.", false},
	}
	for _, st := range steps {
		// A fresh TextLine per step: the GPU texture is built on first Draw from
		// the text as it stood then, so mutating Text in place would not repaint.
		caption := stimuli.NewTextLine(st.text, 0, float32(logicalHeight)/2-80, inkBlack)
		defer caption.Unload()

		deadline := time.Now().Add(introHoldMs * time.Millisecond)
		for time.Now().Before(deadline) {
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := caption.Draw(exp.Screen); err != nil {
				return err
			}
			if st.full {
				if err := whole.Draw(exp.Screen); err != nil {
					return err
				}
			} else if err := drawFragments(exp, frags); err != nil {
				return err
			}
			if err := exp.Screen.Flip(); err != nil {
				return err
			}
			if exp.PollEvents(nil).QuitRequested {
				return control.EndLoop
			}
		}
	}
	return nil
}

// runDemo shows the four transformation buttons and animates whichever one is
// selected. It returns when the participant clicks "continue" or presses SPACE.
func runDemo(exp *control.Experiment, cfg config) error {
	labels := []struct {
		text  string
		trans transformation
	}{
		{"corners get bigger", angleIncrease},
		{"corners get smaller", angleDecrease},
		{"corners move apart", distanceIncrease},
		{"corners move together", distanceDecrease},
	}

	top := float32(logicalHeight)/2 - 150
	var buttons []*button
	for i, l := range labels {
		b := newButton(l.text, "", -640+float32(i)*430, top, 400, 60)
		buttons = append(buttons, b)
		defer b.unload()
	}
	done := newButton("continue", "space", 0, top-90, 300, 60)
	defer done.unload()

	title := stimuli.NewTextLine("Click a button to see what that change looks like.",
		0, float32(logicalHeight)/2-70, inkBlack)
	defer title.Unload()

	active := -1
	start := time.Now()

	for {
		mx, my := exp.Screen.MousePosition()
		hit := updateHover(buttons, mx, my)
		done.hovered = done.contains(mx, my)

		// A full cycle goes 0 → 1 → 0, so the shape returns to its starting
		// state rather than snapping back.
		shape := sampleTriangle
		if active >= 0 {
			ph := math.Mod(time.Since(start).Seconds()*1000, demoCycleMs) / demoCycleMs
			shape = morph(labels[active].trans, 0.5-0.5*math.Cos(2*math.Pi*ph))
		}
		frags := shape.fragments(cfg.baseY, cfg.fragmentFrac, cfg.strokePx, inkBlack)

		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := title.Draw(exp.Screen); err != nil {
			return err
		}
		for _, b := range buttons {
			if err := b.draw(exp); err != nil {
				return err
			}
		}
		if err := done.draw(exp); err != nil {
			return err
		}
		if err := drawFragments(exp, frags); err != nil {
			return err
		}
		if err := exp.Screen.Flip(); err != nil {
			return err
		}

		st := exp.PollEvents(nil)
		if st.QuitRequested {
			return control.EndLoop
		}
		if st.LastKey == control.K_SPACE && st.LastKeyTimestamp != 0 {
			return nil
		}
		if st.LastMouseButton != 0 && st.LastMouseTimestamp != 0 {
			if done.hovered {
				return nil
			}
			if hit >= 0 {
				active = hit
				start = time.Now()
			}
		}
	}
}
