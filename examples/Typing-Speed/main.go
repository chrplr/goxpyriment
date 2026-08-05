// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Typing-Speed — a typing-performance experiment.
//
// On each trial a target sentence is shown and the participant copies it into
// a framed input box below (with a blinking cursor). Every keystroke onset is
// recorded with a hardware-precision timestamp: the time since the trial's
// target appeared, the interval since the previous key, and one of three
// categories — "correct" (the typed character matches the target character at
// the current position), "incorrect" (it does not), or "movement" (an editing
// or control key: Backspace, arrows, Delete, Home, End, Enter).
//
// A trial ends only when the typed text is an EXACT copy of the target and the
// participant presses ENTER; a non-matching ENTER is recorded but ignored.
// Editing is linear: characters are appended at the end and Backspace deletes
// the last one (arrow/Delete keys are logged as movements but do not edit).
//
// At the end, per-keystroke statistics are shown and written to the info file:
// percent correct keystrokes (movements ignored) and typing speed in characters
// per second (cps) for correct keystrokes — mean, P10, P50 and P90.
//
// Data: one CSV row per keystroke (see AddVariableNames below).
//
// Flags (in addition to the standard -w / -d / -s):
//
//	-file <path>  a text file with one target sentence per line (overrides the
//	              built-in list; blank lines are skipped)
//	-n <int>      number of trials to run (0 = all available; default 0)
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// defaultSentences is the built-in target list, used when -file is not given.
// Short pangrams keep the copy fitting on one line at the default font size.
var defaultSentences = []string{
	"The quick brown fox jumps over the lazy dog.",
	"Pack my box with five dozen liquor jugs.",
	"How vexingly quick daft zebras jump!",
	"Sphinx of black quartz, judge my vow.",
	"The five boxing wizards jump quickly.",
	"Jackdaws love my big sphinx of quartz.",
	"Waltz, bad nymph, for quick jigs vex.",
	"Bright vixens jump; dozy fowl quack.",
}

const instructions = "TYPING PERFORMANCE\n\n" +
	"On each trial a sentence appears at the center of the screen.\n" +
	"Type an exact copy of it into the box below, then press ENTER.\n\n" +
	"ENTER only advances when your copy matches the sentence exactly.\n" +
	"Use BACKSPACE to correct mistakes. Every keystroke is timed.\n\n" +
	"Press SPACE to begin. (ESC quits at any time.)"

// keystroke is one recorded key event within a trial.
type keystroke struct {
	trial      int
	index      int    // 1-based position within the trial
	onsetMS    int64  // ms from the trial's target onset
	intervalMS int64  // ms since the previous keystroke in this trial
	input      string // the character typed, or the key name for movement keys
	category   string // "correct", "incorrect", or "movement"
	position   int    // cursor index at which the key was applied
	expected   string // target character expected at that position ("" if beyond)
}

func main() {
	filePath := flag.String("file", "", "text file with one target sentence per line (overrides the built-in list)")
	nTrials := flag.Int("n", 0, "number of trials to run (0 = all available)")

	exp := control.NewExperimentFromFlags("Typing-Speed", control.Black, control.White, 28)
	defer exp.End()

	targets, err := loadTargets(*filePath)
	if err != nil {
		exp.Fatal("loading targets: %v", err)
	}
	if *nTrials > 0 && *nTrials < len(targets) {
		targets = targets[:*nTrials]
	}

	// One CSV row per keystroke (subject_id is prepended automatically).
	exp.Data.AddVariableNames([]string{
		"trial", "keystroke", "onset_ms", "interval_ms",
		"input", "category", "position", "expected",
	})

	var allKeys []keystroke

	err = exp.Run(func() error {
		if err := exp.ShowInstructions(instructions); err != nil {
			return err
		}

		for ti, target := range targets {
			exp.Blank(700) // inter-trial pause
			ks, err := runTrial(exp, ti+1, len(targets), target)

			// Persist whatever was collected, even on an aborted trial.
			for _, k := range ks {
				exp.Data.Add(k.trial, k.index, k.onsetMS, k.intervalMS,
					k.input, k.category, k.position, k.expected)
			}
			allKeys = append(allKeys, ks...)

			if err != nil {
				if control.IsEndLoop(err) {
					break // ESC / window close — still show the summary
				}
				return err
			}
		}

		if err := showStatistics(exp, allKeys); err != nil {
			return err
		}
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}

// runTrial presents one target sentence and records every keystroke the
// participant makes while copying it. It returns when the typed text exactly
// matches the target and ENTER is pressed (nil error), or on ESC / window
// close (control.EndLoop), with the keystrokes collected so far in either case.
func runTrial(exp *control.Experiment, trialNum, totalTrials int, target string) ([]keystroke, error) {
	screen := exp.Screen
	targetRunes := []rune(target)

	if err := screen.Window.StartTextInput(); err != nil {
		return nil, fmt.Errorf("runTrial: starting text input: %w", err)
	}
	defer screen.Window.StopTextInput()

	exp.Keyboard.Clear() // discard stale presses from the previous trial

	var (
		records      []keystroke
		typed        []rune
		keyIndex     int
		prevOnsetMS  int64
		showMismatch bool
	)

	// --- Layout, derived from the monospace line height ----------------
	// Coordinates are centre-based with +Y pointing UP, so items higher on the
	// screen have larger Y. Top to bottom: prompt, counter, target, cursor, box.
	th := float32(screen.DefaultFont.Height())
	gap := th * 0.5          // half a line
	boxH := th + 16.0        // input-box height
	pairH := th + gap + boxH // target + gap + box, centred vertically on y=0
	targetY := pairH/2 - th/2
	boxCenterY := pairH/2 - th - gap - boxH/2
	typedY := boxCenterY
	counterY := targetY + th + gap // one line above the target
	promptY := counterY + th
	cursorY := boxCenterY // inside the box, on the typing line
	cursorH := th
	hintY := boxCenterY - boxH/2 - th

	boxW := float32(screen.Width) * 0.85
	if boxW < 400 {
		boxW = 400
	}

	// --- Persistent stimuli, built once and reused every frame ---------
	counterLine := stimuli.NewTextLine(fmt.Sprintf("Trial %d / %d", trialNum, totalTrials), 0, counterY, control.Gray)
	defer counterLine.Unload()
	promptLine := stimuli.NewTextLine("Copy the sentence, then press ENTER", 0, promptY, control.DarkGray)
	defer promptLine.Unload()
	targetLine := stimuli.NewTextLine(target, 0, targetY, control.White)
	defer targetLine.Unload()
	mismatchLine := stimuli.NewTextLine("Not an exact copy yet — keep correcting, then press ENTER.", 0, hintY, control.Red)
	defer mismatchLine.Unload()
	frame := stimuli.NewRectangle(0, boxCenterY, boxW, boxH, control.Gray)
	inner := stimuli.NewRectangle(0, boxCenterY, boxW-4, boxH-4, control.RGB(30, 30, 30))

	// Column geometry: the default font is monospace, so every glyph has the
	// same advance. Preloading the target yields its pixel width, from which we
	// derive the per-character width and the block's left edge; the cursor and
	// the typed text are then positioned column-by-column beneath the target, so
	// the target never scrolls horizontally.
	if err := stimuli.PreloadVisualOnScreen(screen, targetLine); err != nil {
		return records, fmt.Errorf("runTrial: preloading target: %w", err)
	}
	nChars := len(targetRunes)
	if nChars == 0 {
		nChars = 1
	}
	charW := targetLine.Width / float32(nChars)
	leftEdge := -targetLine.Width / 2

	// The typed text is a separate line rebuilt only when it changes (not every
	// frame), so the blinking cursor — a plain rectangle — does not churn GPU
	// textures during the busy render loop.
	var entryLine *stimuli.TextLine
	lastEntry := "\x00" // sentinel: force the first build
	defer func() {
		if entryLine != nil {
			entryLine.Unload()
		}
	}()
	rebuildEntry := func() {
		cur := string(typed)
		if cur == lastEntry {
			return
		}
		lastEntry = cur
		if entryLine != nil {
			entryLine.Unload()
			entryLine = nil
		}
		if cur == "" {
			return
		}
		// Left-align the typed text under the target's columns.
		xTyped := leftEdge + float32(len(typed))*charW/2
		entryLine = stimuli.NewTextLine(cur, xTyped, typedY, control.White)
	}

	// redraw paints one frame (without flipping): prompt, counter, target,
	// input box, typed text, and — at the current column, just below the target
	// character to copy — the blinking cursor.
	redraw := func(cursorVisible bool) error {
		screen.Clear()
		for _, d := range []stimuli.VisualStimulus{promptLine, counterLine, targetLine, frame, inner} {
			if err := d.Draw(screen); err != nil {
				return err
			}
		}
		if entryLine != nil {
			if err := entryLine.Draw(screen); err != nil {
				return err
			}
		}
		if cursorVisible {
			cx := leftEdge + (float32(len(typed))+0.5)*charW
			if err := stimuli.NewRectangle(cx, cursorY, 3, cursorH, control.White).Draw(screen); err != nil {
				return err
			}
		}
		if showMismatch {
			if err := mismatchLine.Draw(screen); err != nil {
				return err
			}
		}
		return nil
	}

	// Draw the initial frame; its flip is the trial-onset reference.
	rebuildEntry()
	if err := redraw(true); err != nil {
		return records, fmt.Errorf("runTrial: %w", err)
	}
	trialOnsetNS, err := screen.FlipTS()
	if err != nil {
		return records, fmt.Errorf("runTrial: %w", err)
	}

	record := func(ts uint64, input, category string, position int, expected string) {
		var onsetMS int64
		if ts > trialOnsetNS {
			onsetMS = int64((ts - trialOnsetNS) / 1_000_000)
		}
		keyIndex++
		records = append(records, keystroke{
			trial:      trialNum,
			index:      keyIndex,
			onsetMS:    onsetMS,
			intervalMS: onsetMS - prevOnsetMS,
			input:      input,
			category:   category,
			position:   position,
			expected:   expected,
		})
		prevOnsetMS = onsetMS
	}

	const blinkPeriodNS = 500_000_000 // 2 Hz cursor blink

	for {
		var ev control.Event
		for control.PollEvent(&ev) {
			switch ev.Type {
			case control.EVENT_QUIT:
				return records, control.EndLoop

			case control.EVENT_KEY_DOWN:
				ke := ev.KeyboardEvent()
				if ke.Repeat {
					continue // count genuine onsets only, not auto-repeat
				}
				switch ke.Key {
				case control.K_ESCAPE:
					return records, control.EndLoop

				case control.K_RETURN, control.K_KP_ENTER:
					record(ke.Timestamp, "RETURN", "movement", len(typed), "")
					if string(typed) == target {
						return records, nil // exact copy — trial complete
					}
					showMismatch = true

				case control.K_BACKSPACE:
					record(ke.Timestamp, "BACKSPACE", "movement", len(typed), "")
					if len(typed) > 0 {
						typed = typed[:len(typed)-1]
					}
					showMismatch = false

				case control.K_LEFT, control.K_RIGHT, control.K_UP, control.K_DOWN,
					control.K_HOME, control.K_END, control.K_DELETE:
					// Logged as movements; no effect in the linear editing model.
					record(ke.Timestamp, ke.Key.KeyName(), "movement", len(typed), "")

					// Printable keys are handled by EVENT_TEXT_INPUT below.
				}

			case control.EVENT_TEXT_INPUT:
				ti := ev.TextInputEvent()
				for _, r := range ti.Text {
					pos := len(typed)
					expected := ""
					correct := false
					if pos < len(targetRunes) {
						expected = string(targetRunes[pos])
						correct = r == targetRunes[pos]
					}
					category := "incorrect"
					if correct {
						category = "correct"
					}
					record(ti.Timestamp, string(r), category, pos, expected)
					typed = append(typed, r)
				}
				showMismatch = false
			}
		}

		rebuildEntry()
		elapsedNS := control.TicksNS() - trialOnsetNS
		cursorVisible := (elapsedNS/blinkPeriodNS)%2 == 0
		if err := redraw(cursorVisible); err != nil {
			return records, fmt.Errorf("runTrial: %w", err)
		}
		if err := screen.Flip(); err != nil {
			return records, fmt.Errorf("runTrial: %w", err)
		}
	}
}

// showStatistics computes and displays the typing-performance summary, and
// writes it to the info file under a --TYPING PERFORMANCE section.
func showStatistics(exp *control.Experiment, keys []keystroke) error {
	var correct, incorrect, movement int
	var cps []float64 // instantaneous speed for correct inter-key intervals

	for _, k := range keys {
		switch k.category {
		case "correct":
			correct++
			// index > 1 ensures a real previous keystroke in the same trial;
			// the first key's interval is time-to-first-keystroke, not a rate.
			if k.index > 1 && k.intervalMS > 0 {
				cps = append(cps, 1000.0/float64(k.intervalMS))
			}
		case "incorrect":
			incorrect++
		case "movement":
			movement++
		}
	}

	totalChars := correct + incorrect
	pctCorrect := 0.0
	if totalChars > 0 {
		pctCorrect = 100 * float64(correct) / float64(totalChars)
	}

	sort.Float64s(cps)
	meanCPS := mean(cps)
	p10 := percentile(cps, 10)
	p50 := percentile(cps, 50)
	p90 := percentile(cps, 90)

	// Write the summary to the companion info file.
	d := exp.Data
	d.WriteComment("--TYPING PERFORMANCE")
	d.WriteComment(fmt.Sprintf("t total_keystrokes: %d", len(keys)))
	d.WriteComment(fmt.Sprintf("t correct_chars: %d", correct))
	d.WriteComment(fmt.Sprintf("t incorrect_chars: %d", incorrect))
	d.WriteComment(fmt.Sprintf("t movement_keys: %d", movement))
	d.WriteComment(fmt.Sprintf("t percent_correct: %.1f", pctCorrect))
	d.WriteComment(fmt.Sprintf("t cps_mean: %.2f", meanCPS))
	d.WriteComment(fmt.Sprintf("t cps_p10: %.2f", p10))
	d.WriteComment(fmt.Sprintf("t cps_p50: %.2f", p50))
	d.WriteComment(fmt.Sprintf("t cps_p90: %.2f", p90))
	d.Save()

	summary := fmt.Sprintf(
		"TYPING PERFORMANCE\n\n"+
			"Keystrokes: %d   (correct %d, incorrect %d, movements %d)\n"+
			"Correct keystrokes: %.1f %%\n\n"+
			"Speed for correct keystrokes (characters/second):\n"+
			"  mean %.2f    P10 %.2f    P50 %.2f    P90 %.2f\n\n"+
			"Press any key to finish.",
		len(keys), correct, incorrect, movement,
		pctCorrect, meanCPS, p10, p50, p90)

	// Also echo to the terminal for convenience.
	fmt.Println(strings.ReplaceAll(summary, "\n\nPress any key to finish.", ""))

	return exp.ShowEndMessage(summary)
}

// loadTargets returns the target sentences: from a file (one per line, blank
// lines skipped) when path is non-empty, otherwise the built-in list.
func loadTargets(path string) ([]string, error) {
	if path == "" {
		return defaultSentences, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no sentences found in %s", path)
	}
	return out, nil
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// percentile returns the linear-interpolated p-th percentile of an
// already-sorted slice (p in [0, 100]).
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := p / 100 * float64(n-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (rank-float64(lo))*(sorted[hi]-sorted[lo])
}
