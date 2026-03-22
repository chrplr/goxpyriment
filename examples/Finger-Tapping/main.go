// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Finger-Tapping replicates the patterned finger-tapping paradigm of
// Povel & Collard (1982). Subjects memorise a finger sequence, then reproduce
// it 6 times in a row as fast as possible. Only error-free runs are recorded.
//
// Keys:  1 = index   2 = middle   3 = ring   4 = little
// (use the top-row digit keys, or remap fingerKeys / responseKeys below)
package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ---------------------------------------------------------------------------
// Key mapping — change here if you want different response keys.
// ---------------------------------------------------------------------------

// fingerKey maps finger number (1–4) to its keyboard key.
//
//	1 = index  → '1'   2 = middle → '2'
//	3 = ring   → '3'   4 = little → '4'
var fingerKey = [5]control.Keycode{0, control.K_1, control.K_2, control.K_3, control.K_4}

var keyFinger = map[control.Keycode]int{
	control.K_1: 1,
	control.K_2: 2,
	control.K_3: 3,
	control.K_4: 4,
}

var responseKeys = []control.Keycode{control.K_1, control.K_2, control.K_3, control.K_4}

// ---------------------------------------------------------------------------
// Stimulus patterns
// ---------------------------------------------------------------------------

// Pattern holds a named finger-tap sequence.
type Pattern struct {
	Name     string // e.g. "A1"
	Set      string // e.g. "A"
	Sequence []int  // finger numbers: 1=index, 2=middle, 3=ring, 4=little
}

// experimentalPatterns are the 12 sequences from Povel & Collard (1982), Table 1.
// Confirmed from the Results section:
//
//	Set A: cyclic permutations of (3 2 1 2 3 4) — M(2T_{-1}(3)) / M(212) / M(2T(1))
//	Set B: cyclic permutations of (1 2 3 2 3 4) — two-chunk structure
//	Set C: cyclic permutations of (1 2 3 3 2 1) — repeat-structure
//	Set D: (2 4 3 4 2 1) and cyclic variants — no structure
var experimentalPatterns = []Pattern{
	{"A1", "A", []int{3, 2, 1, 2, 3, 4}},
	{"A2", "A", []int{2, 1, 2, 3, 4, 3}},
	{"A3", "A", []int{1, 2, 3, 4, 3, 2}},
	{"B1", "B", []int{1, 2, 3, 2, 3, 4}},
	{"B2", "B", []int{2, 3, 4, 1, 2, 3}},
	{"B3", "B", []int{2, 3, 2, 3, 4, 1}},
	{"C1", "C", []int{1, 2, 3, 3, 2, 1}},
	{"C2", "C", []int{3, 3, 2, 1, 1, 2}},
	{"C3", "C", []int{2, 3, 3, 2, 1, 1}},
	{"D1", "D", []int{2, 4, 3, 4, 2, 1}},
	{"D2", "D", []int{1, 2, 4, 3, 4, 2}},
	{"D3", "D", []int{3, 4, 2, 1, 2, 4}},
}

// practicePatterns are 10 simpler sequences used before the actual experiment.
// The paper mentions "10 practise patterns" without specifying them.
var practicePatterns = []Pattern{
	{"P01", "P", []int{1, 2, 3, 4}},
	{"P02", "P", []int{4, 3, 2, 1}},
	{"P03", "P", []int{1, 2, 1, 2}},
	{"P04", "P", []int{3, 4, 3, 4}},
	{"P05", "P", []int{1, 3, 2, 4}},
	{"P06", "P", []int{2, 4, 1, 3}},
	{"P07", "P", []int{1, 2, 3, 2}},
	{"P08", "P", []int{2, 3, 4, 3}},
	{"P09", "P", []int{1, 4, 2, 3}},
	{"P10", "P", []int{3, 2, 4, 1}},
}

const nReps = 6 // complete sequence repetitions required per trial

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// seqString formats a sequence as a large spaced string, e.g. "3   2   1   2   3   4".
func seqString(seq []int) string {
	parts := make([]string, len(seq))
	for i, f := range seq {
		parts[i] = fmt.Sprintf("%d", f)
	}
	return strings.Join(parts, "   ")
}

// ---------------------------------------------------------------------------
// Trial logic
// ---------------------------------------------------------------------------

// tapRecord holds timing data for one tap.
type tapRecord struct {
	rep, tap, expected, pressed int
	tGoMs, itiMs                int64
}

// runTrial shows the sequence, waits for the subject to practise and signal
// ready, then collects nReps error-free repetitions.
//
// If the subject makes an error the error tone is played and the whole trial
// is restarted (pattern shown again). The function only returns when the
// full set of error-free reps has been collected.
//
// Data is appended to exp only when phase == "experiment".
func runTrial(
	exp *control.Experiment,
	p Pattern, phase string,
	readyTone, goTone, stopTone, errorTone *stimuli.Tone,
) error {
	seqLabel := fmt.Sprintf("Pattern  %s\n\n%s", p.Name, seqString(p.Sequence))
	fingerLabel := "1 = index   2 = middle   3 = ring   4 = little"

	for { // loop until error-free run
		// 1. Show sequence + practise prompt; wait for SPACE.
		practiceMsg := seqLabel + "\n\n" + fingerLabel +
			"\n\n\nPractise until you know the sequence from memory.\nPress SPACE when ready."
		if err := exp.ShowInstructions(practiceMsg); err != nil {
			return err
		}

		// 2. "Get ready" screen + ready tone.
		readyStim := stimuli.NewTextLine("Get ready…", 0, 0, control.White)
		if err := exp.Show(readyStim); err != nil {
			return err
		}
		if err := readyTone.Play(); err != nil {
			log.Printf("ready tone: %v", err)
		}
		clock.Wait(1000)

		// 3. Blank screen + go tone — subject taps from memory.
		if err := exp.Screen.ClearAndUpdate(); err != nil {
			return err
		}
		if err := goTone.Play(); err != nil {
			log.Printf("go tone: %v", err)
		}

		// 4. Collect nReps repetitions.
		trialClock := clock.NewClock()
		var records []tapRecord
		errorOccurred := false
		var tPrevMs int64

	outer:
		for rep := 0; rep < nReps; rep++ {
			for step, expected := range p.Sequence {
				key, _, err := exp.Keyboard.WaitKeysRT(responseKeys, 5000)
				if err != nil {
					return err // ESC or quit
				}
				tNowMs := trialClock.NowMillis()

				var itiMs int64
				if rep == 0 && step == 0 {
					itiMs = tNowMs // first tap: RT from go signal
				} else {
					itiMs = tNowMs - tPrevMs
				}
				tPrevMs = tNowMs

				pressed := 0
				if key != 0 {
					pressed = keyFinger[key]
				}

				records = append(records, tapRecord{
					rep:      rep + 1,
					tap:      step + 1,
					expected: expected,
					pressed:  pressed,
					tGoMs:    tNowMs,
					itiMs:    itiMs,
				})

				if pressed != expected {
					errorOccurred = true
					break outer
				}
			}
		}

		if errorOccurred {
			_ = errorTone.Play()
			clock.Wait(600)
			continue // show pattern again
		}

		// 5. Success.
		_ = stopTone.Play()
		clock.Wait(300)

		// 6. Save data (experiment phase only).
		if phase == "experiment" {
			for _, r := range records {
				exp.Data.Add(
					p.Name, p.Set, phase,
					r.rep, r.tap, r.expected, r.pressed,
					r.tGoMs, r.itiMs,
				)
			}
		}

		fmt.Printf("[%s] %s: %d taps OK\n", phase, p.Name, len(records))
		return nil
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	exp := control.NewExperimentFromFlags(
		"Patterned Finger Tapping",
		control.Black, control.White, 48,
	)
	defer exp.End()

	exp.AddDataVariableNames([]string{
		"pattern", "set", "phase",
		"rep", "tap", "finger_expected", "finger_pressed",
		"t_from_go_ms", "iti_ms",
	})

	// Prepare tones.
	readyTone := stimuli.NewTone(440, 200, 0.5) // A4  — "ready"
	goTone := stimuli.NewTone(880, 100, 0.6)    // A5  — "go"
	stopTone := stimuli.NewTone(660, 400, 0.4)  // E5  — "stop"
	errorTone := stimuli.NewTone(220, 350, 0.5) // A3  — "error"

	for _, t := range []*stimuli.Tone{readyTone, goTone, stopTone, errorTone} {
		if err := t.PreloadDevice(exp.AudioDevice); err != nil {
			log.Printf("Warning: tone preload failed: %v", err)
		}
	}

	err := exp.Run(func() error {
		// --- Instructions ---
		instrText := "PATTERNED FINGER TAPPING\n\n" +
			"In each trial a sequence of digits will appear on screen.\n" +
			"Each digit indicates a finger of your dominant hand:\n\n" +
			"   1 = index   2 = middle   3 = ring   4 = little\n\n" +
			"Memorise the sequence, then tap it 6 times in a row as fast as possible.\n" +
			"You will hear a 'Ready' tone, then a 'Go' tone — start tapping on 'Go'.\n" +
			"A low buzz means an error; the sequence will be shown again.\n\n" +
			"You will first complete 10 practice trials.\n\n" +
			"Press SPACE to begin."
		if err := exp.ShowInstructions(instrText); err != nil {
			return err
		}

		// --- Practice block ---
		for _, p := range practicePatterns {
			if err := runTrial(exp, p, "practice", readyTone, goTone, stopTone, errorTone); err != nil {
				return err
			}
		}

		// --- Transition ---
		transText := "Practice complete.\n\nThe actual experiment will now begin.\n\n" +
			"Press SPACE to continue."
		if err := exp.ShowInstructions(transText); err != nil {
			return err
		}

		// --- Experimental block (randomised order) ---
		expPatterns := make([]Pattern, len(experimentalPatterns))
		copy(expPatterns, experimentalPatterns)
		rand.Shuffle(len(expPatterns), func(i, j int) {
			expPatterns[i], expPatterns[j] = expPatterns[j], expPatterns[i]
		})

		for _, p := range expPatterns {
			if err := runTrial(exp, p, "experiment", readyTone, goTone, stopTone, errorTone); err != nil {
				return err
			}
		}

		// --- End ---
		if err := exp.ShowInstructions("Thank you — the experiment is complete!\n\nPress SPACE to exit."); err != nil {
			return err
		}
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
