// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Finger-Tracking experiment based on Dotan & Dehaene (2013):
// "How do we convert a number into a finger trajectory?" (Cognition, 129).
//
// The participant drags a finger (or the mouse, with the left button held) from
// a small rectangle at the bottom-centre of the screen up to a horizontal,
// unmarked number line at the top, aiming at the screen position that
// corresponds to a target number between 0 and 40. The whole pointer trajectory
// is sampled and recorded.
//
// Trial flow (per §2.1 of the paper):
//  1. The number line (labelled 0 and 40 at its ends) and a start rectangle are
//     shown. Pressing/holding the button inside the start rectangle starts the
//     trial and shows a fixation cross above the middle of the line.
//  2. The participant slides upward. When the pointer reaches 70 px from the
//     bottom of the screen the target number replaces the fixation cross
//     (this is the movement onset). The pointer keeps moving up to the line.
//  3. When the pointer crosses the number line the target disappears, a click
//     sound plays, and a green downward arrow marks where the line was crossed.
//
// Measures recorded per trial: endpoint (on the 0-40 scale), endpoint bias
// (endpoint - target), endpoint error (|bias|), the time the pointer reached the
// threshold, the target onset (the flip that first showed the number, one frame
// after the threshold), and movement time (target onset to line crossing). The
// full trajectory is stored as one CSV column of "x,y,t;..." triples, sampled
// once per display frame. All times are ms from the start press, on the SDL
// clock (the press's event timestamp, flip timestamps). Offline resampling to a
// fixed 100 Hz (cubic spline, as in the paper) is left to analysis.
//
// This v1 records data only: the paper's trial-validity rules (no backward /
// sideways motion, minimum / average velocity limits) and failed-trial
// re-presentation are NOT enforced.
//
// On a touchscreen, SDL synthesises the same held-button mouse events from a
// single finger drag, so the same code path serves both mouse and touch.
//
// Usage:
//
//	go run main.go [-w] [-d N] [-s <subjectID>] [-reps N]
package main

import (
	"flag"
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

// ── Layout constants (center-relative, logical 1024 × 768, +Y up) ──────────────
// Values are pixel-exact to the paper's iPad geometry (§2.1.1). Top edge y=+384,
// bottom edge y=-384.

const (
	logicalW = int32(1024)
	logicalH = int32(768)

	lineY      = float32(304) // 80 px below the top edge (384 - 80)
	lineHalf   = float32(422) // half of the 844 px line length
	lineThick  = float32(2)   // 2 px thick
	lineLeftX  = -lineHalf    // x mapped to number 0
	lineRightX = lineHalf     // x mapped to number 40
	lineRange  = float32(40)  // numbers span 0..40
	labelY     = lineY - 24   // end labels just below the line

	targetY = float32(340) // fixation cross / target number, just above the line

	onsetThresholdY = float32(-314) // 70 px from the bottom (-384 + 70): movement onset

	startRectW = float32(60)
	startRectH = float32(40)
	startRectY = float32(-360) // dark-grey start rectangle, bottom-centre

	maxTrialMS = 8000 // per-trial timeout from the start press (ms)
	itiMS      = 800  // inter-trial interval (ms)
	feedbackMS = 700  // how long the feedback arrow stays on (ms)
)

// The trajectory is sampled once per display frame — 60 Hz on a 60 Hz display.
// The loop flips every iteration and Update holds to the frame boundary, so it
// cannot sample faster; an 8 ms interval it once had, labelled "~125 Hz", was
// never reached. The paper resamples to 100 Hz offline, which here means
// interpolating between frame-rate samples.

// ── Colors ─────────────────────────────────────────────────────────────────────

var (
	lineColor     = control.White
	labelColor    = control.RGB(180, 180, 180) // light grey end labels
	startColor    = control.RGB(80, 80, 80)    // dark grey start rectangle
	targetColor   = control.White
	fixColor      = control.White
	feedbackColor = control.RGB(0, 200, 0) // green feedback arrow
)

// ── Trajectory ───────────────────────────────────────────────────────────────

type tPoint struct {
	X, Y float32
	T    int64 // ms from the start press
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

// ── Helpers (center-relative coords) ───────────────────────────────────────────

func inRect(x, y, cx, cy, w, h float32) bool {
	return x >= cx-w/2 && x <= cx+w/2 && y >= cy-h/2 && y <= cy+h/2
}

// endpointFromX maps a crossing x-coordinate to the 0-40 number-line scale.
// Values outside 0..40 are kept (the pointer may cross beyond an end).
func endpointFromX(x float32) float32 {
	return (x - lineLeftX) / (lineRightX - lineLeftX) * lineRange
}

// feedbackArrow builds a green downward-pointing triangle whose tip touches the
// number line at horizontal position crossX (~7.7 mm tall).
func feedbackArrow(crossX float32) *stimuli.Shape {
	const halfW = float32(15)
	const height = float32(34)
	// Points relative to the shape centre (+Y up): two top corners and a bottom apex.
	arrow := stimuli.NewShape([]control.FPoint{
		{X: -halfW, Y: +height / 2},
		{X: +halfW, Y: +height / 2},
		{X: 0, Y: -height / 2},
	}, feedbackColor)
	// Place so the apex (tip) sits on the line and the body is above it.
	arrow.SetPosition(control.Point(crossX, lineY+height/2))
	return arrow
}

// drawScene clears the screen, draws the provided drawables and flips. It
// returns the flip's timestamp (SDL clock, ns), the frame's onset.
func drawScene(screen *apparatus.Screen, items ...stimuli.VisualStimulus) (uint64, error) {
	if err := screen.Clear(); err != nil {
		return 0, err
	}
	for _, s := range items {
		if err := s.Draw(screen); err != nil {
			return 0, err
		}
	}
	return screen.FlipTS()
}

// ── Single trial ─────────────────────────────────────────────────────────────

// trialResult holds one trial's measures. Times are ms from the start press,
// all on the SDL clock; -1 means the event did not happen.
type trialResult struct {
	endpoint      float32
	bias          float32
	errAbs        float32
	thresholdMS   int64 // the pointer reached the movement-onset threshold
	targetOnsetMS int64 // the flip that first showed the target number
	movementMS    int64 // target onset to line crossing
	completed     bool
	trajectory    string
}

// applyTargetFont, if set, applies the large target font to a text stimulus.
// It is a closure so this file imports only `control` (not the ttf package).
var applyTargetFont func(*stimuli.TextLine)

func runTrial(exp *control.Experiment, target int) (trialResult, error) {
	// Persistent scene stimuli
	numberLine := stimuli.NewRectangle(0, lineY, 2*lineHalf, lineThick, lineColor)
	label0 := stimuli.NewTextLine("0", lineLeftX, labelY, labelColor)
	label40 := stimuli.NewTextLine("40", lineRightX, labelY, labelColor)
	startRect := stimuli.NewRectangle(0, startRectY, startRectW, startRectH, startColor)

	fixCross := stimuli.NewFixCross(40, 4, fixColor)
	fixCross.SetPosition(control.Point(0, targetY))

	targetText := stimuli.NewTextLine(fmt.Sprintf("%d", target), 0, targetY, targetColor)
	if applyTargetFont != nil {
		applyTargetFont(targetText)
	}

	// ── Phase 1: wait for a button press inside the start rectangle ───────────
	//
	// The trial clock starts at the press itself: the button-down event's SDL
	// timestamp. It used to start when the polling loop noticed the button was
	// down, 0 to ~one frame + 5 ms after the press, which shifted every time in
	// the trial by a different amount on each trial. When no button-down event
	// arrived in the cycle that saw the press — the button was already held, or
	// was pressed outside the box and slid in — the trial starts at that poll.
	var pressNS uint64
	for {
		if _, err := drawScene(exp.Screen, numberLine, label0, label40, startRect); err != nil {
			return trialResult{}, err
		}
		state := exp.PollEvents(nil)
		if state.QuitRequested {
			return trialResult{}, control.EndLoop
		}
		polledNS := control.TicksNS()
		if exp.Mouse.IsPressed(control.BUTTON_LEFT) {
			mx, my := exp.Screen.MousePosition()
			if inRect(mx, my, 0, startRectY, startRectW, startRectH) {
				pressNS = polledNS
				if state.LastMouseButton == control.BUTTON_LEFT {
					pressNS = state.LastMouseTimestamp
				}
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	msSincePress := func(ts uint64) int64 { return int64(ts-pressNS) / 1_000_000 }

	// ── Phase 2: drag up while the button is held; record the trajectory ──────
	var traj []tPoint
	res := trialResult{thresholdMS: -1, targetOnsetMS: -1, movementMS: -1}

	for {
		// Pump the events first, so the position read next is as fresh as the
		// clock read beside it. No sleep in this loop: the flip below paces it,
		// and a sleep would only delay each sample from its frame's start.
		state := exp.PollEvents(nil)
		if state.QuitRequested {
			return trialResult{}, control.EndLoop
		}
		nowMS := msSincePress(control.TicksNS())
		mx, my := exp.Screen.MousePosition()
		traj = append(traj, tPoint{X: mx, Y: my, T: nowMS})

		// Movement onset: pointer reaches the 70-px-from-bottom threshold.
		// The target replaces the fixation cross on the next flip.
		if res.thresholdMS < 0 && my >= onsetThresholdY {
			res.thresholdMS = nowMS
		}

		// Crossing the number line ends the trial.
		if my >= lineY {
			res.endpoint = endpointFromX(mx)
			res.bias = res.endpoint - float32(target)
			res.errAbs = res.bias
			if res.errAbs < 0 {
				res.errAbs = -res.errAbs
			}
			// -1 if the pointer passed the threshold and the line within one
			// sample, so the target was never on screen.
			if res.targetOnsetMS >= 0 {
				res.movementMS = nowMS - res.targetOnsetMS
			}
			res.completed = true
			res.trajectory = encodeTrajectory(traj)
			return res, nil
		}

		// Build the scene: number line + labels, then cross or target.
		items := []stimuli.VisualStimulus{numberLine, label0, label40}
		showsTarget := res.thresholdMS >= 0
		if showsTarget {
			items = append(items, targetText)
		} else {
			items = append(items, fixCross)
		}
		flipNS, err := drawScene(exp.Screen, items...)
		if err != nil {
			return trialResult{}, err
		}
		if showsTarget && res.targetOnsetMS < 0 {
			res.targetOnsetMS = msSincePress(flipNS)
		}

		// Button released before crossing, or timeout: incomplete (no re-presentation in v1).
		if !exp.Mouse.IsPressed(control.BUTTON_LEFT) || nowMS > maxTrialMS {
			res.completed = false
			res.trajectory = encodeTrajectory(traj)
			return res, nil
		}
	}
}

// showFeedback plays the acknowledgement click and shows the green arrow.
func showFeedback(exp *control.Experiment, crossX float32) error {
	numberLine := stimuli.NewRectangle(0, lineY, 2*lineHalf, lineThick, lineColor)
	label0 := stimuli.NewTextLine("0", lineLeftX, labelY, labelColor)
	label40 := stimuli.NewTextLine("40", lineRightX, labelY, labelColor)
	arrow := feedbackArrow(crossX)

	exp.Audio.PlayCorrect() // brief acknowledgement "click"

	start := clock.GetTime()
	for clock.GetTime()-start < feedbackMS {
		if _, err := drawScene(exp.Screen, numberLine, label0, label40, arrow); err != nil {
			return err
		}
		state := exp.PollEvents(nil)
		if state.QuitRequested {
			return control.EndLoop
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	reps := flag.Int("reps", 2, "repetitions of each target number 0..40 (paper uses 10 → 410 trials)")

	exp := control.NewExperimentFromFlags("Finger Tracking", control.Black, control.White, 24)
	defer exp.End()

	// Runs on a touchscreen, but also with a mouse — where the participant needs
	// to see what they are dragging. Initialize() hides the cursor by default.
	if err := exp.ShowCursor(); err != nil {
		log.Printf("Warning: could not show cursor: %v", err)
	}

	if err := exp.SetLogicalSize(logicalW, logicalH); err != nil {
		log.Printf("Warning: could not set logical size: %v", err)
	}

	// Larger font for the target number (~10 mm, Inconsolata; Arial bold not bundled).
	targetFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 48)
	if err != nil {
		log.Printf("Warning: could not load target font: %v", err)
	} else {
		defer targetFont.Close()
		applyTargetFont = func(t *stimuli.TextLine) { t.Font = targetFont }
	}

	exp.AddDataVariableNames([]string{
		"trial", "target", "endpoint", "endpoint_bias", "endpoint_error",
		"threshold_ms", "target_onset_ms", "movement_time_ms", "completed", "trajectory",
	})

	// Build 0..40 repeated `reps` times, then a single global random order.
	var targets []int
	for r := 0; r < *reps; r++ {
		for n := 0; n <= 40; n++ {
			targets = append(targets, n)
		}
	}
	design.ShuffleList(targets)

	instrText := fmt.Sprintf(
		"FINGER-TRACKING EXPERIMENT\n\n"+
			"A horizontal number line runs across the top of the screen,\n"+
			"labelled 0 at the left end and 40 at the right end.\n\n"+
			"Press and HOLD inside the grey box at the bottom to start a trial.\n"+
			"Keep the button held and slide straight up. A number will appear.\n"+
			"Move to the spot on the line where that number belongs, and cross it.\n\n"+
			"Move smoothly and do not go back down.\n\n"+
			"There will be %d trials.\n\n"+
			"Press SPACE to begin.",
		len(targets),
	)

	err = exp.Run(func() error {
		if err := exp.ShowInstructions(instrText); err != nil {
			return err
		}

		for i, target := range targets {
			exp.Blank(itiMS)

			result, err := runTrial(exp, target)
			if err != nil {
				return err
			}

			if result.completed {
				crossX := result.endpoint/lineRange*(lineRightX-lineLeftX) + lineLeftX
				if err := showFeedback(exp, crossX); err != nil {
					return err
				}
				exp.Data.Add(i+1, target, fmt.Sprintf("%.3f", result.endpoint),
					fmt.Sprintf("%.3f", result.bias), fmt.Sprintf("%.3f", result.errAbs),
					result.thresholdMS, result.targetOnsetMS, result.movementMS, true, result.trajectory)
				fmt.Printf("Trial %3d  target=%2d  endpoint=%.2f  bias=%+.2f  MT=%dms\n",
					i+1, target, result.endpoint, result.bias, result.movementMS)
			} else {
				exp.Data.Add(i+1, target, "", "", "", result.thresholdMS, result.targetOnsetMS, -1, false, result.trajectory)
				fmt.Printf("Trial %3d  target=%2d  incomplete\n", i+1, target)
			}
		}

		exp.ShowEndMessage("Experiment complete!\n\nThank you for your participation.\n\nPress any key to exit.")
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
