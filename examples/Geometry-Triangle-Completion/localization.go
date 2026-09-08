// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"math/rand"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const localizationPrompt = "Can you click where the top corner is?"

// ── Trial list ────────────────────────────────────────────────────────────────

// buildLocalizationTrials returns reps repetitions of each of the seven
// configurations (49 trials in the paper), pseudo-randomised so that the same
// configuration is never presented twice in a row (paper §2.4).
func buildLocalizationTrials(reps int, rng *rand.Rand) []tri {
	var pool []tri
	for _, c := range localizationTriangles {
		for r := 0; r < reps; r++ {
			pool = append(pool, c.tri)
		}
	}

	// Reshuffle until no configuration repeats consecutively. With 7 equally
	// frequent values a valid permutation is common, so this converges quickly;
	// the bound keeps a pathological case (e.g. reps=1 with one config) finite.
	for attempt := 0; attempt < 1000; attempt++ {
		rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		ok := true
		for i := 1; i < len(pool); i++ {
			if pool[i].id == pool[i-1].id {
				ok = false
				break
			}
		}
		if ok {
			return pool
		}
	}
	return pool
}

// ── Presentation ──────────────────────────────────────────────────────────────

type localizationResponse struct {
	clickX, clickY float32 // centre-based, +Y up
	rtMs           int64
	onsetNS        uint64
}

// runLocalizationTrial shows one fragmented triangle, waits for a click, then
// requires a click on the "Next" button before returning — the paper advanced
// trials with a separate button (Fig. 1b) rather than on the response click, so
// a stray double-click cannot skip a trial.
//
// The button is labelled in words rather than with the paper's arrow glyph:
// the embedded Inconsolata has no U+2192, so "->" drew an empty box.
//
// The click position comes from Screen.MousePosition (centre-based, +Y up,
// corrected for the logical size), and the reaction time from the SDL hardware
// event timestamp measured against the stimulus flip — one clock throughout.
func runLocalizationTrial(exp *control.Experiment, cfg config, t tri) (localizationResponse, error) {
	prompt := stimuli.NewTextLine(localizationPrompt, 0, float32(logicalHeight)/2-70, inkBlack)
	defer prompt.Unload()
	next := newButton("Next", "", float32(logicalWidth)/2-140, float32(logicalHeight)/2-70, 160, 66)
	defer next.unload()

	fragments := t.fragments(cfg.baseY, cfg.fragmentFrac, cfg.strokePx, inkBlack)

	var resp localizationResponse
	answered := false
	// The response mark is drawn only after the click, and carries no feedback
	// about accuracy — it just shows where the click landed.
	mark := stimuli.NewCircle(7, control.RGB(190, 40, 40))

	draw := func() error {
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := prompt.Draw(exp.Screen); err != nil {
			return err
		}
		if err := drawFragments(exp, fragments); err != nil {
			return err
		}
		if answered {
			mark.Position = control.FPoint{X: resp.clickX, Y: resp.clickY}
			if err := mark.Draw(exp.Screen); err != nil {
				return err
			}
			if err := next.draw(exp); err != nil {
				return err
			}
		}
		return nil
	}

	if err := draw(); err != nil {
		return resp, err
	}
	onset, err := exp.Screen.FlipTS()
	if err != nil {
		return resp, err
	}
	resp.onsetNS = onset

	for {
		mx, my := exp.Screen.MousePosition()
		next.hovered = answered && next.contains(mx, my)

		if err := draw(); err != nil {
			return resp, err
		}
		if _, err := exp.Screen.FlipTS(); err != nil {
			return resp, err
		}

		st := exp.PollEvents(nil)
		if st.QuitRequested {
			return resp, control.EndLoop
		}
		if st.LastMouseButton == 0 || st.LastMouseTimestamp == 0 {
			continue
		}

		if !answered {
			// Ignore a click that lands where Next will appear, so the
			// participant cannot answer and advance with one press.
			if next.contains(mx, my) {
				continue
			}
			resp.clickX, resp.clickY = mx, my
			resp.rtMs = msSince(onset, st.LastMouseTimestamp)
			answered = true
			continue
		}
		if next.hovered {
			return resp, nil
		}
	}
}

// runPracticeTrial is the single practice trial with the sample triangle, run
// before the test trials so the participant has produced one click before the
// data start (paper §2.4). Its response is not recorded.
func runPracticeTrial(exp *control.Experiment, cfg config) error {
	_, err := runLocalizationTrial(exp, cfg, sampleTriangle)
	return err
}
