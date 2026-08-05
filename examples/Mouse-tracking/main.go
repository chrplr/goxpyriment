// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Mouse-tracking experiment based on Spivey et al. (2005):
// "Continuous Attraction Toward Phonological Competitors"
//
// Participants see two labeled image boxes (upper-left, upper-right) and a
// written target word (bottom-center, appearing 500 ms after image onset).
// They must click the matching image while starting to move immediately on
// start-box click. Trajectories are sampled at ~36 Hz.
//
// Two conditions:
//   - cohort:  distractor name shares onset with target (candle / candy)
//   - control: distractor is phonologically unrelated (candle / jacket)
//
// Output columns: trial, condition, target, left_item, right_item,
// target_side, response, correct, rt_ms, trajectory
//
// Usage:
//
//	go run main.go [-w] [-d N] [-s <subjectID>]
package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Layout constants (center-relative, logical 1024 × 768) ─────────────────

const (
	logicalW = int32(1024)
	logicalH = int32(768)

	imageW = float32(200)
	imageH = float32(160)

	imageLeftX  = float32(-340)
	imageRightX = float32(340)
	imageTopY   = float32(240)

	startBoxW = float32(240)
	startBoxH = float32(60)
	startBoxY = float32(-290)

	stimAsyncMS = 500  // image-onset to word-onset delay (ms)
	maxTrialMS  = 8000 // per-trial timeout (ms)
	itiMS       = 1000 // inter-trial interval (ms)

	sampleIntervalMS = 28 // ~36 Hz trajectory sampling
)

// ── Colors ───────────────────────────────────────────────────────────────────

var (
	imageBoxColor  = control.RGB(60, 90, 140)
	startBoxColor  = control.RGB(30, 140, 60)
	wordColor      = control.White
	imageTextColor = control.White
)

// ── Trial definition ─────────────────────────────────────────────────────────

type trialDef struct {
	target    string
	leftItem  string
	rightItem string
	condition string // "cohort" or "control"
}

func (t trialDef) targetSide() string {
	if t.leftItem == t.target {
		return "left"
	}
	return "right"
}

// cohortPairs: (target, phonological-cohort distractor)
var cohortPairs = [][2]string{
	{"candle", "candy"},
	{"bear", "beard"},
	{"cat", "carrot"},
	{"fork", "forest"},
}

// controlPairs: (target, phonologically-unrelated distractor)
var controlPairs = [][2]string{
	{"candle", "jacket"},
	{"bear", "lamp"},
	{"cat", "table"},
	{"fork", "pencil"},
}

func buildTrials() []trialDef {
	var trials []trialDef

	appendPairs := func(pairs [][2]string, cond string) {
		for _, pair := range pairs {
			target, distractor := pair[0], pair[1]
			// Counterbalance target side across two reps
			trials = append(trials,
				trialDef{target, target, distractor, cond},
				trialDef{target, distractor, target, cond},
			)
		}
	}
	appendPairs(cohortPairs, "cohort")
	appendPairs(controlPairs, "control")

	design.ShuffleList(trials)
	return trials
}

// ── Trajectory ───────────────────────────────────────────────────────────────

type tPoint struct {
	X, Y float32
	T    int64 // ms from image onset
}

func encodeTrajectory(pts []tPoint) string {
	var sb strings.Builder
	for i, p := range pts {
		if i > 0 {
			sb.WriteByte(';')
		}
		fmt.Fprintf(&sb, "%.1f,%.1f,%d", p.X, p.Y, p.T)
	}
	return sb.String()
}

// ── Hit testing (center-relative coords) ────────────────────────────────────

func inRect(x, y, cx, cy, w, h float32) bool {
	return x >= cx-w/2 && x <= cx+w/2 && y >= cy-h/2 && y <= cy+h/2
}

// ── Rendering helper ──────────────────────────────────────────────────────────

// drawScene clears the screen and draws the provided list of drawables.
func drawScene(screen *apparatus.Screen, items ...stimuli.VisualStimulus) error {
	if err := screen.Clear(); err != nil {
		return err
	}
	for _, s := range items {
		if err := s.Draw(screen); err != nil {
			return err
		}
	}
	return screen.Update()
}

// ── Single trial ─────────────────────────────────────────────────────────────

type trialResult struct {
	response   string
	correct    bool
	rtMS       int64
	trajectory string
	timeout    bool
}

func runTrial(exp *control.Experiment, t trialDef) (trialResult, error) {
	// Persistent stimuli for this trial
	leftBox := stimuli.NewRectangle(imageLeftX, imageTopY, imageW, imageH, imageBoxColor)
	leftLabel := stimuli.NewTextLine(t.leftItem, imageLeftX, imageTopY, imageTextColor)
	rightBox := stimuli.NewRectangle(imageRightX, imageTopY, imageW, imageH, imageBoxColor)
	rightLabel := stimuli.NewTextLine(t.rightItem, imageRightX, imageTopY, imageTextColor)

	startBox := stimuli.NewRectangle(0, startBoxY, startBoxW, startBoxH, startBoxColor)
	startBoxLabel := stimuli.NewTextLine("CLICK HERE TO START", 0, startBoxY, control.White)

	wordStim := stimuli.NewTextLine(t.target, 0, startBoxY, wordColor)

	// ── Phase 1: wait for click inside start box ──────────────────────────

	for {
		if err := drawScene(exp.Screen, startBox, startBoxLabel); err != nil {
			return trialResult{}, err
		}
		state := exp.PollEvents(nil)
		if state.QuitRequested {
			return trialResult{}, control.EndLoop
		}
		if state.LastMouseButton == control.BUTTON_LEFT {
			mx, my := exp.Screen.MousePosition()
			if inRect(mx, my, 0, startBoxY, startBoxW, startBoxH) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
	}

	// ── Phase 2: image onset + trajectory recording ───────────────────────

	imageOnset := clock.GetTime()
	var traj []tPoint
	var lastSampleMS int64

	wordShown := false

	for {
		nowMS := clock.GetTime() - imageOnset

		// Sample mouse position at ~36 Hz
		if nowMS-lastSampleMS >= sampleIntervalMS {
			mx, my := exp.Screen.MousePosition()
			traj = append(traj, tPoint{X: mx, Y: my, T: nowMS})
			lastSampleMS = nowMS
		}

		// Update display
		items := []stimuli.VisualStimulus{leftBox, leftLabel, rightBox, rightLabel}
		if wordShown || nowMS >= stimAsyncMS {
			wordShown = true
			items = append(items, wordStim)
		}
		if err := drawScene(exp.Screen, items...); err != nil {
			return trialResult{}, err
		}

		// Poll events
		state := exp.PollEvents(nil)
		if state.QuitRequested {
			return trialResult{}, control.EndLoop
		}
		if state.LastMouseButton == control.BUTTON_LEFT {
			mx, my := exp.Screen.MousePosition()
			switch {
			case inRect(mx, my, imageLeftX, imageTopY, imageW, imageH):
				return trialResult{
					response:   t.leftItem,
					correct:    t.leftItem == t.target,
					rtMS:       nowMS,
					trajectory: encodeTrajectory(traj),
				}, nil
			case inRect(mx, my, imageRightX, imageTopY, imageW, imageH):
				return trialResult{
					response:   t.rightItem,
					correct:    t.rightItem == t.target,
					rtMS:       nowMS,
					trajectory: encodeTrajectory(traj),
				}, nil
			}
		}

		if nowMS > maxTrialMS {
			return trialResult{timeout: true, trajectory: encodeTrajectory(traj)}, nil
		}

		time.Sleep(5 * time.Millisecond)
	}
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	exp := control.NewExperimentFromFlags("Mouse Tracking", control.Black, control.White, 36)
	defer exp.End()

	// The whole paradigm is the participant's pointer trajectory, so the cursor
	// must be visible; Initialize() hides it by default.
	if err := exp.ShowCursor(); err != nil {
		log.Printf("Warning: could not show cursor: %v", err)
	}

	if err := exp.SetLogicalSize(logicalW, logicalH); err != nil {
		log.Printf("Warning: could not set logical size: %v", err)
	}

	instrFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 24)
	if err != nil {
		log.Printf("Warning: could not load instruction font: %v", err)
	} else {
		defer instrFont.Close()
	}

	exp.AddDataVariableNames([]string{
		"trial", "condition", "target", "left_item", "right_item",
		"target_side", "response", "correct", "rt_ms", "trajectory",
	})

	trials := buildTrials()

	instrText := fmt.Sprintf(
		"MOUSE-TRACKING EXPERIMENT\n\n"+
			"Two labeled boxes will appear at the top of the screen.\n"+
			"Half a second later, a WORD will appear at the bottom.\n\n"+
			"Click the box that matches the word.\n\n"+
			"Important: start moving the mouse UPWARD as soon as\n"+
			"you click the green start box — before the word appears.\n\n"+
			"There will be %d trials.\n\n"+
			"Press SPACE to begin.",
		len(trials),
	)

	err = exp.Run(func() error {
		tb := stimuli.NewTextBox(instrText, int32(float32(exp.Screen.Width)*0.80), control.FPoint{}, exp.ForegroundColor)
		if instrFont != nil {
			tb.Font = instrFont
		}
		exp.Show(tb)
		exp.Keyboard.WaitKey(control.K_SPACE)

		for i, t := range trials {
			exp.Blank(itiMS)

			result, err := runTrial(exp, t)
			if err != nil {
				return err
			}

			if result.timeout {
				exp.ShowTimed(stimuli.NewTextLine("Too slow! Please respond faster.", 0, 0, control.Red), 1500)
				exp.Data.Add(i+1, t.condition, t.target, t.leftItem, t.rightItem,
					t.targetSide(), "", false, -1, result.trajectory)
				fmt.Printf("Trial %2d [%s] %s | timeout\n", i+1, t.condition, t.target)
				continue
			}

			if !result.correct {
				exp.Audio.PlayBuzzer()
			}

			exp.Data.Add(i+1, t.condition, t.target, t.leftItem, t.rightItem,
				t.targetSide(), result.response, result.correct, result.rtMS, result.trajectory)

			fmt.Printf("Trial %2d [%s] %s → %s  correct=%v  RT=%d ms\n",
				i+1, t.condition, t.target, result.response, result.correct, result.rtMS)
		}

		exp.ShowEndMessage("Experiment complete!\n\nThank you for your participation.\n\nPress any key to exit.")
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
