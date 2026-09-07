// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math/rand"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Question space ────────────────────────────────────────────────────────────
//
// Eight question types = 4 transformations of the two visible corners × 2
// properties of the missing corner asked about (paper §2.3).

type transformation int

const (
	angleIncrease transformation = iota
	angleDecrease
	distanceIncrease
	distanceDecrease
)

func (t transformation) phrase() string {
	return [...]string{
		"increase the angle size of the bottom two corners",
		"decrease the angle size of the bottom two corners",
		"increase the distance between the bottom two corners",
		"decrease the distance between the bottom two corners",
	}[t]
}

func (t transformation) name() string {
	return [...]string{"angle_increase", "angle_decrease", "distance_increase", "distance_decrease"}[t]
}

// kind is the change applied: to the base angles, or to the base length.
func (t transformation) kind() string {
	if t == angleIncrease || t == angleDecrease {
		return "angle"
	}
	return "distance"
}

// direction is whether the change makes the quantity bigger or smaller.
func (t transformation) direction() string {
	if t == angleIncrease || t == distanceIncrease {
		return "increase"
	}
	return "decrease"
}

type dimension int

const (
	dimPosition dimension = iota // where the missing corner is
	dimAngle                     // how big the missing corner's angle is
)

func (d dimension) name() string {
	if d == dimPosition {
		return "position"
	}
	return "angle"
}

func (d dimension) question() string {
	if d == dimPosition {
		return "will the top corner move up, move down, or stay in the same place?"
	}
	return "will the angle size of the top corner get bigger, get smaller, or stay the same size?"
}

func (d dimension) options() []string {
	if d == dimPosition {
		return []string{"move up", "move down", "stay in the same place"}
	}
	return []string{"get bigger", "get smaller", "stay the same size"}
}

// correctOption is the answer Euclidean geometry gives.
//
// With base angles L and R, the apex height is h = b·tanL·tanR/(tanL+tanR) and
// the apex angle is 180−L−R. So growing both base angles raises the apex and
// shrinks its angle; lengthening the base raises the apex proportionally and
// leaves its angle untouched (the triangle is merely scaled).
//
// Every cell here matches the modal adult response in the paper's Fig. 5.
func correctOption(t transformation, d dimension) string {
	if d == dimPosition {
		if t.direction() == "increase" {
			return "move up"
		}
		return "move down"
	}
	switch t {
	case angleIncrease:
		return "get smaller"
	case angleDecrease:
		return "get bigger"
	default: // a distance change scales the triangle; the angles are invariant
		return "stay the same size"
	}
}

// ── Trial list ────────────────────────────────────────────────────────────────

type reasoningTrial struct {
	block  int
	trans  transformation
	dim    dimension
	figure tri
}

func (r reasoningTrial) questionType() string {
	return fmt.Sprintf("%s_%s", r.dim.name(), r.trans.name())
}

func (r reasoningTrial) text() string {
	return fmt.Sprintf("Take this partial triangle here. What if I %s, %s",
		r.trans.phrase(), r.dim.question())
}

// buildReasoningTrials produces `blocks` blocks of the same eight questions,
// shuffled within each block and each paired with a randomly chosen triangle
// from Table 1. Neither the question type nor the triangle may repeat on
// consecutive trials, including across a block boundary (paper §2.3).
func buildReasoningTrials(blocks int, rng *rand.Rand) []reasoningTrial {
	base := make([]reasoningTrial, 0, 8)
	for _, t := range []transformation{angleIncrease, angleDecrease, distanceIncrease, distanceDecrease} {
		for _, d := range []dimension{dimPosition, dimAngle} {
			base = append(base, reasoningTrial{trans: t, dim: d})
		}
	}

	var out []reasoningTrial
	for b := 0; b < blocks; b++ {
		// Reshuffle the block until its first question differs from the last
		// question of the previous block; within a block a permutation of eight
		// distinct questions never repeats one anyway.
		var block []reasoningTrial
		for attempt := 0; ; attempt++ {
			block = append([]reasoningTrial(nil), base...)
			rng.Shuffle(len(block), func(i, j int) { block[i], block[j] = block[j], block[i] })
			if len(out) == 0 || block[0].questionType() != out[len(out)-1].questionType() || attempt > 50 {
				break
			}
		}
		for i := range block {
			block[i].block = b + 1
		}
		out = append(out, block...)
	}

	// Assign triangles, never repeating one on consecutive trials.
	prev := ""
	for i := range out {
		for {
			pick := reasoningTriangles[rng.Intn(len(reasoningTriangles))]
			if pick.id != prev {
				out[i].figure = pick.tri
				prev = pick.id
				break
			}
		}
	}
	return out
}

// ── Presentation ──────────────────────────────────────────────────────────────

type reasoningResponse struct {
	selected string
	rtMs     int64
	onsetNS  uint64
}

// runReasoningTask presents every trial and returns once all are answered.
func runReasoningTask(exp *control.Experiment, cfg config, trials []reasoningTrial,
	record func(reasoningTrial, int, reasoningResponse)) error {

	for i, t := range trials {
		resp, err := askReasoningTrial(exp, cfg, t)
		if err != nil {
			return err
		}
		record(t, i+1, resp)
	}
	return nil
}

// askReasoningTrial draws one question screen and waits for a response, either a
// click on one of the three options or the matching number key. The paper has an
// experimenter enter the child's spoken answer, so both routes are provided.
//
// The "show sample changes" button re-enters the demonstration at any time
// (paper §2.3: "they could revisit the sample-changes display at any point").
func askReasoningTrial(exp *control.Experiment, cfg config, t reasoningTrial) (reasoningResponse, error) {
	opts := t.dim.options()

	question := stimuli.NewTextBox(t.text(), 1500,
		control.FPoint{X: 0, Y: float32(logicalHeight)/2 - 90}, inkBlack)
	defer question.Unload()

	var buttons []*button
	for i, o := range opts {
		b := newButton(o, fmt.Sprintf("%d", i+1), -430, float32(logicalHeight)/2-215-float32(i)*72, 620, 60)
		buttons = append(buttons, b)
		defer b.unload()
	}
	sample := newButton("show sample changes", "", 640, float32(logicalHeight)/2-215, 460, 60)
	defer sample.unload()

	fragments := t.figure.fragments(cfg.baseY, cfg.fragmentFrac, cfg.strokePx, inkBlack)

	draw := func() error {
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := question.Draw(exp.Screen); err != nil {
			return err
		}
		for _, b := range buttons {
			if err := b.draw(exp); err != nil {
				return err
			}
		}
		if err := sample.draw(exp); err != nil {
			return err
		}
		return drawFragments(exp, fragments)
	}

	if err := draw(); err != nil {
		return reasoningResponse{}, err
	}
	onset, err := exp.Screen.FlipTS()
	if err != nil {
		return reasoningResponse{}, err
	}

	for {
		mx, my := exp.Screen.MousePosition()
		hit := updateHover(buttons, mx, my)
		sample.hovered = sample.contains(mx, my)

		if err := draw(); err != nil {
			return reasoningResponse{}, err
		}
		if _, err := exp.Screen.FlipTS(); err != nil {
			return reasoningResponse{}, err
		}

		st := exp.PollEvents(nil)
		if st.QuitRequested {
			return reasoningResponse{}, control.EndLoop
		}

		// Keyboard route: 1 / 2 / 3 pick an option, S revisits the demo.
		if st.LastKey != 0 && st.LastKeyTimestamp != 0 {
			if idx := digitIndex(st.LastKey, len(opts)); idx >= 0 {
				return reasoningResponse{opts[idx], msSince(onset, st.LastKeyTimestamp), onset}, nil
			}
			if st.LastKey == control.K_S {
				if err := runDemo(exp, cfg); err != nil {
					return reasoningResponse{}, err
				}
				if err := draw(); err != nil {
					return reasoningResponse{}, err
				}
				if _, err := exp.Screen.FlipTS(); err != nil {
					return reasoningResponse{}, err
				}
			}
		}

		// Mouse route.
		if st.LastMouseButton != 0 && st.LastMouseTimestamp != 0 {
			if hit >= 0 {
				return reasoningResponse{opts[hit], msSince(onset, st.LastMouseTimestamp), onset}, nil
			}
			if sample.hovered {
				if err := runDemo(exp, cfg); err != nil {
					return reasoningResponse{}, err
				}
			}
		}
	}
}

// digitIndex maps K_1…K_9 to a 0-based option index, or -1.
func digitIndex(k control.Keycode, n int) int {
	for i := 0; i < n && i < 9; i++ {
		if k == control.Keycode(int(control.K_1)+i) {
			return i
		}
	}
	return -1
}

// msSince converts two SDL nanosecond timestamps to a millisecond interval.
func msSince(onsetNS, eventNS uint64) int64 {
	if eventNS <= onsetNS {
		return 0
	}
	return int64(eventNS-onsetNS) / 1_000_000
}
