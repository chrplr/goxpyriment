// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// Gabor Contrast Detection Threshold — QUEST adaptive staircase
//
// Estimates the contrast detection threshold for a Gabor patch using the
// QUEST procedure (Watson & Pelli, 1983). The QUEST staircase selects the
// next trial contrast from the current Bayesian posterior over threshold,
// converging on the observer's 82 % correct point (d′ ≈ 1 for 2AFC).
//
// Paradigm: 2-Interval Forced Choice (2IFC)
//   Each trial presents two temporal intervals, marked by a highlighted box.
//   Only one interval (chosen at random) contains the Gabor patch; the other
//   shows only the fixation cross. The participant presses 1 or 2.
//
// Stimulus:  45° Gabor patch, σ = 30 px, λ = 20 px/cycle, size = 200 px.
// Intensity: log₁₀(Michelson contrast), tracked from −3.0 to 0.0.
// Trials:    40 (configurable via -n flag).
//
// Usage:
//
//	go run main.go [-s <id>] [-n <trials>] [-guess <log_contrast>] [-d]
//
// Flags:
//
//	-s      int     Subject ID (default 0).
//	-n      int     Number of QUEST trials (default 40).
//	-guess  float   Initial log₁₀(contrast) guess, e.g. −1.5 (default −1.5).
//	-d              Developer mode: windowed 1024×768.

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/staircase"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ─── Gabor parameters (fixed across all trials) ─────────────────────────────

const (
	gaborSize   = 200  // total patch size in pixels (square)
	gaborSigma  = 30.0 // Gaussian envelope SD in pixels
	gaborLambda = 20.0 // spatial wavelength in pixels (0.05 cycles/pixel)
	gaborTheta  = 45.0 // orientation in degrees from horizontal
)

// Background gray matching the Gabor's mid-luminance point (127.5 ≈ 128).
var bgGray = control.RGB(128, 128, 128)

// ─── Visual helpers ──────────────────────────────────────────────────────────

// drawIntervalScreen clears the screen and renders the 2IFC frame.
//
//	active: 0 = neither box highlighted, 1 = box 1, 2 = box 2.
//	info:   status line shown at the bottom of the screen.
func drawIntervalScreen(exp *control.Experiment, active int, info string) error {
	exp.Screen.Clear()

	fix := stimuli.NewFixCross(30, 3, control.DarkGray)
	if err := fix.Draw(exp.Screen); err != nil {
		return err
	}

	col1, col2 := control.LightGray, control.LightGray
	if active == 1 {
		col1 = control.White
	}
	if active == 2 {
		col2 = control.White
	}
	box1 := stimuli.NewRectangle(-230, 0, 180, 180, col1)
	box2 := stimuli.NewRectangle(230, 0, 180, 180, col2)
	for _, b := range []*stimuli.Rectangle{box1, box2} {
		if err := b.Draw(exp.Screen); err != nil {
			return err
		}
	}

	lbl1 := stimuli.NewTextLine("1", -230, 0, control.DarkGray)
	lbl2 := stimuli.NewTextLine("2", 230, 0, control.DarkGray)
	for _, l := range []*stimuli.TextLine{lbl1, lbl2} {
		if err := l.Draw(exp.Screen); err != nil {
			return err
		}
	}

	infoLine := stimuli.NewTextLine(info, 0, 310, control.DarkGray)
	if err := infoLine.Draw(exp.Screen); err != nil {
		return err
	}

	exp.Screen.Update()
	return nil
}

// drawIntervalWithGabor renders the interval screen with the Gabor patch drawn
// on top of the fixation cross.
func drawIntervalWithGabor(exp *control.Experiment, active int, info string, gabor *stimuli.GaborPatch) error {
	exp.Screen.Clear()

	// Draw boxes and labels (same as drawIntervalScreen, but without Update).
	fix := stimuli.NewFixCross(30, 3, control.DarkGray)
	if err := fix.Draw(exp.Screen); err != nil {
		return err
	}

	col1, col2 := control.LightGray, control.LightGray
	if active == 1 {
		col1 = control.White
	}
	if active == 2 {
		col2 = control.White
	}
	box1 := stimuli.NewRectangle(-230, 0, 180, 180, col1)
	box2 := stimuli.NewRectangle(230, 0, 180, 180, col2)
	for _, b := range []*stimuli.Rectangle{box1, box2} {
		if err := b.Draw(exp.Screen); err != nil {
			return err
		}
	}

	lbl1 := stimuli.NewTextLine("1", -230, 0, control.DarkGray)
	lbl2 := stimuli.NewTextLine("2", 230, 0, control.DarkGray)
	for _, l := range []*stimuli.TextLine{lbl1, lbl2} {
		if err := l.Draw(exp.Screen); err != nil {
			return err
		}
	}

	if err := gabor.Draw(exp.Screen); err != nil {
		return err
	}

	infoLine := stimuli.NewTextLine(info, 0, 310, control.DarkGray)
	if err := infoLine.Draw(exp.Screen); err != nil {
		return err
	}

	exp.Screen.Update()
	return nil
}

// showFeedback briefly flashes the screen green (correct) or red (wrong).
func showFeedback(exp *control.Experiment, correct bool) error {
	color := control.Red
	if correct {
		color = control.Green
	}
	fb := stimuli.NewRectangle(0, 0, 500, 140, color)
	if err := exp.Show(fb); err != nil {
		return err
	}
	clock.Wait(300)
	return nil
}

// ─── Single 2IFC trial ───────────────────────────────────────────────────────

// runTrial presents one 2IFC trial at the given log₁₀(contrast) and returns
// whether the participant correctly identified the signal interval.
func runTrial(exp *control.Experiment, logContrast float64, totalTrials, trialNum int) (bool, error) {
	contrast := math.Pow(10, logContrast)

	// Randomly assign signal to interval 1 or 2.
	signalInterval := 1 + rand.Intn(2)

	info := fmt.Sprintf("trial %d/%d  |  log-contrast %.2f  (%.1f %%)",
		trialNum, totalTrials, logContrast, contrast*100)

	// Build Gabor at this trial's contrast; preload before timing begins.
	gabor := stimuli.NewGaborPatch(gaborSigma, gaborTheta, gaborLambda, 0, 0, 1, bgGray, gaborSize)
	gabor.Contrast = contrast
	defer gabor.Unload()
	if err := stimuli.PreloadVisualOnScreen(exp.Screen, gabor); err != nil {
		return false, err
	}

	// Pre-trial fixation (500 ms).
	if err := drawIntervalScreen(exp, 0, info); err != nil {
		return false, err
	}
	clock.Wait(500)

	// ── Interval 1 ──────────────────────────────────────────────────────────
	var err error
	if signalInterval == 1 {
		err = drawIntervalWithGabor(exp, 1, info, gabor)
	} else {
		err = drawIntervalScreen(exp, 1, info)
	}
	if err != nil {
		return false, err
	}
	clock.Wait(150)

	// Blank within interval 1 (short persistence period).
	if err := drawIntervalScreen(exp, 0, info); err != nil {
		return false, err
	}
	clock.Wait(50)

	// ── Inter-stimulus interval (400 ms) ─────────────────────────────────────
	clock.Wait(400)

	// ── Interval 2 ──────────────────────────────────────────────────────────
	if signalInterval == 2 {
		err = drawIntervalWithGabor(exp, 2, info, gabor)
	} else {
		err = drawIntervalScreen(exp, 2, info)
	}
	if err != nil {
		return false, err
	}
	clock.Wait(150)

	// Blank within interval 2.
	if err := drawIntervalScreen(exp, 0, info); err != nil {
		return false, err
	}
	clock.Wait(50)

	// ── Response ─────────────────────────────────────────────────────────────
	exp.Screen.Clear()
	exp.Keyboard.Clear() // discard stale keys before the response prompt appears
	prompt := stimuli.NewTextBox(
		"Which interval contained the pattern?\n\nPress  1  or  2.",
		600, control.Point(0, 0), control.DarkGray)
	if err := prompt.Present(exp.Screen, false, true); err != nil {
		return false, err
	}
	if err := prompt.Unload(); err != nil {
		return false, err
	}

	responseKeys := []control.Keycode{control.K_1, control.K_KP_1, control.K_2, control.K_KP_2}
	k, _, err := exp.Keyboard.WaitKeysRT(responseKeys, -1)
	if err != nil {
		return false, err
	}
	var response int
	if k == control.K_1 || k == control.K_KP_1 {
		response = 1
	} else {
		response = 2
	}

	correct := response == signalInterval
	if err := showFeedback(exp, correct); err != nil {
		return false, err
	}
	return correct, nil
}

// ─── main ─────────────────────────────────────────────────────────────────────

func main() {
	nTrials := flag.Int("n", 40, "Number of QUEST trials")
	initGuess := flag.Float64("guess", -1.5, "Initial log₁₀(contrast) guess (e.g. -1.5 ≈ 3 %)")

	exp := control.NewExperimentFromFlags("Contrast Detection (QUEST)", bgGray, control.DarkGray, 24)
	defer exp.End()

	if err := exp.SetLogicalSize(1024, 768); err != nil {
		log.Printf("warning: set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"trial", "log_contrast", "linear_contrast_pct",
		"signal_interval", "response", "correct",
		"quest_threshold", "quest_sd",
	})

	runErr := exp.Run(func() error {
		// ── Instructions ─────────────────────────────────────────────────────
		instrText := fmt.Sprintf(
			"Gabor Contrast Detection\n\n"+
				"On each trial you will see two intervals (labelled 1 and 2).\n"+
				"Only ONE interval contains a tilted grating pattern — the other is blank.\n\n"+
				"Press  1  if you saw the pattern in interval 1.\n"+
				"Press  2  if you saw the pattern in interval 2.\n\n"+
				"The pattern will sometimes be very faint. Do your best.\n"+
				"Feedback will be shown after every trial.\n\n"+
				"This session will run %d trials.\n\n"+
				"Press SPACE to begin.", *nTrials)

		instr := stimuli.NewTextBox(instrText, 800, control.Point(0, 0), control.DarkGray)
		if err := exp.Show(instr); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}
		if err := instr.Unload(); err != nil {
			return err
		}

		// ── QUEST staircase ──────────────────────────────────────────────────
		sc := staircase.NewQuest(staircase.QuestConfig{
			TGuess:         *initGuess,
			TGuessSd:       1.5,  // wide prior: ±1.5 log-units covers the plausible range
			PThreshold:     0.82, // 2AFC d′ ≈ 1 threshold criterion
			Beta:           3.5,  // typical Weibull slope for contrast detection
			Delta:          0.01, // 1 % lapse rate
			Gamma:          0.5,  // 2AFC lower asymptote
			IntensityMin:   -3.0, // log₁₀(0.001) = 0.1 % contrast
			IntensityMax:   0.0,  // log₁₀(1.0)   = 100 % contrast
			IntensityStep:  0.01, // 0.01 log-unit resolution
			MaxTrials:      *nTrials,
			EstimateMethod: "mean",
		})

		// ── Trial loop ───────────────────────────────────────────────────────
		for trialNum := 1; !sc.Done(); trialNum++ {
			logContrast := sc.Intensity()

			correct, err := runTrial(exp, logContrast, *nTrials, trialNum)
			if err != nil {
				return err
			}
			sc.Update(correct)

			// Log trial data immediately.
			contrast := math.Pow(10, logContrast)
			history := sc.History()
			last := history[len(history)-1]
			exp.Data.Add(
				trialNum,
				fmt.Sprintf("%.4f", logContrast),
				fmt.Sprintf("%.2f", contrast*100),
				fmt.Sprintf("%d", (trialNum%2)+1), // signal interval (logged for reference)
				"?",                               // response logged as part of correct
				last.Correct,
				fmt.Sprintf("%.4f", sc.Threshold()),
				fmt.Sprintf("%.4f", sc.SD()),
			)
		}

		// ── Results ──────────────────────────────────────────────────────────
		threshold := sc.Threshold()
		sd := sc.SD()
		linearPct := math.Pow(10, threshold) * 100

		summaryText := fmt.Sprintf(
			"Estimated contrast detection threshold\n\n"+
				"log₁₀(contrast) = %.3f  (±%.3f)\n"+
				"Linear contrast  = %.2f %%\n\n"+
				"Based on %d trials.\n\n"+
				"Press SPACE to exit.", threshold, sd, linearPct, *nTrials)

		summary := stimuli.NewTextBox(summaryText, 700, control.Point(0, 0), control.DarkGray)
		if err := exp.Show(summary); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("experiment error: %v", runErr)
	}
}
