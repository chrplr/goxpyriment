// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Replication of Experiment 1 from:
//
//	Povel, D.-J., & Essens, P. (1985). Perception of temporal patterns.
//	Music Perception, 2(4), 411–440.
//
// Run: go run . [-d] [-s <subject_id>] [-sound tone|cymbal]
//
// The experiment uses 35 rhythmic sequences (all permutations of the interval
// set {1,1,1,1,1,2,2,3,4}, where unit = 200 ms).
//
// For each sequence:
//  1. Learning phase: the sequence plays on repeat; the subject taps along
//     and presses ENTER when ready to reproduce.
//  2. Reproduction phase: the subject taps SPACE for 4 complete periods
//     (36 taps → 35 inter-tap intervals).
//
// Metrics recorded: number of presentations, reproduction error (sum of
// |observed - expected| intervals in ms).
//
// NOTE: The sequence table was transcribed from description.md (which may
// contain errors relative to the original paper). Sequences 1 and 13 were
// corrected to be valid permutations of the interval set. Sequences 11 and 31
// are duplicates of 7 and 24 in the source; they are kept as-is but flagged.
// Verify all sequences against the original paper before use in real studies.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

//go:embed cymbal22050.wav
var cymbalBytes []byte

// ── Timing constants ──────────────────────────────────────────────────────────

const (
	unitMs     = 200 // smallest inter-onset interval (ms)
	toneDurMs  = 50  // tone on-duration (ms)
	toneRampMs = 5   // linear ramp applied at onset/offset (ms)
)

// ── Sequence definitions ──────────────────────────────────────────────────────

// seqDef holds one of the 35 stimuli. intervals contains the 9 inter-onset
// intervals in units of unitMs. The period = sum(intervals) * unitMs = 3200 ms.
type seqDef struct {
	id        int
	category  int
	intervals [9]int
}

// All 35 sequences from Povel & Essens (1985), Table 1.
// Numbers represent multiples of 200 ms (unit=1 → 200 ms, 2 → 400 ms, etc.).
// Each valid sequence is a permutation of {1,1,1,1,1,2,2,3,4} (sum = 16 units).
var sequences = []seqDef{
	// ── Category 1 (strongest clock induction) ──────────────────────────────
	{1, 1, [9]int{1, 1, 1, 2, 1, 2, 3, 4, 1}}, // corrected (original sum=18)
	{2, 1, [9]int{1, 1, 2, 1, 2, 1, 3, 1, 4}},
	{3, 1, [9]int{1, 1, 2, 2, 1, 1, 3, 1, 4}},
	{4, 1, [9]int{2, 1, 1, 2, 1, 3, 1, 4, 1}},
	{5, 1, [9]int{2, 2, 1, 1, 1, 3, 1, 4, 1}},

	// ── Category 2 ──────────────────────────────────────────────────────────
	{6, 2, [9]int{1, 3, 2, 1, 2, 1, 1, 4, 1}},
	{7, 2, [9]int{1, 1, 2, 1, 1, 2, 1, 4, 3}},
	{8, 2, [9]int{1, 1, 2, 1, 2, 3, 1, 4, 1}},
	{9, 2, [9]int{3, 1, 1, 1, 1, 2, 2, 4, 1}},
	{10, 2, [9]int{3, 2, 1, 1, 2, 1, 1, 4, 1}},

	// ── Category 3 ──────────────────────────────────────────────────────────
	{11, 3, [9]int{1, 1, 2, 1, 1, 2, 1, 4, 3}}, // same as seq 7 in source; verify
	{12, 3, [9]int{1, 1, 1, 2, 1, 3, 2, 4, 1}},
	{13, 3, [9]int{2, 1, 1, 1, 1, 2, 3, 4, 1}}, // corrected (source had 10 values)
	{14, 3, [9]int{2, 1, 1, 2, 1, 1, 3, 4, 1}},
	{15, 3, [9]int{1, 3, 1, 2, 2, 1, 1, 4, 1}},

	// ── Category 4 ──────────────────────────────────────────────────────────
	{16, 4, [9]int{1, 1, 3, 1, 1, 2, 2, 4, 1}},
	{17, 4, [9]int{2, 1, 1, 1, 2, 1, 1, 4, 3}},
	{18, 4, [9]int{1, 2, 1, 1, 1, 2, 1, 4, 3}},
	{19, 4, [9]int{1, 2, 1, 1, 1, 3, 2, 4, 1}},
	{20, 4, [9]int{1, 3, 1, 2, 1, 1, 1, 4, 2}},

	// ── Category 5 ──────────────────────────────────────────────────────────
	{21, 5, [9]int{1, 3, 1, 1, 1, 2, 1, 4, 2}},
	{22, 5, [9]int{1, 1, 1, 1, 2, 1, 2, 3, 4}},
	{23, 5, [9]int{1, 1, 1, 2, 3, 1, 1, 2, 4}},
	{24, 5, [9]int{1, 1, 3, 1, 2, 1, 1, 2, 4}},
	{25, 5, [9]int{1, 2, 1, 3, 2, 1, 1, 1, 4}},

	// ── Category 6 ──────────────────────────────────────────────────────────
	{26, 6, [9]int{3, 1, 2, 1, 1, 2, 1, 1, 4}},
	{27, 6, [9]int{1, 1, 1, 2, 2, 3, 1, 1, 4}},
	{28, 6, [9]int{2, 1, 1, 1, 2, 3, 1, 1, 4}},
	{29, 6, [9]int{2, 3, 1, 1, 1, 2, 1, 1, 4}},
	{30, 6, [9]int{1, 1, 2, 1, 2, 3, 1, 1, 4}},

	// ── Category 7 (weakest clock induction) ────────────────────────────────
	{31, 7, [9]int{1, 1, 3, 1, 2, 1, 1, 2, 4}}, // same as seq 24 in source; verify
	{32, 7, [9]int{1, 1, 1, 2, 1, 1, 3, 2, 4}},
	{33, 7, [9]int{1, 1, 1, 3, 1, 2, 1, 2, 4}},
	{34, 7, [9]int{2, 1, 1, 1, 1, 3, 1, 2, 4}},
	{35, 7, [9]int{1, 2, 3, 1, 1, 1, 1, 2, 4}},
}

// ── Stream construction ───────────────────────────────────────────────────────

// buildStream creates a SoundStreamElement slice for one presentation of seq.
// Each element: tone plays for toneDurMs, then silence fills the rest of the IOI.
func buildStream(seq seqDef, sound stimuli.AudioPlayable) []stimuli.SoundStreamElement {
	elements := make([]stimuli.SoundStreamElement, len(seq.intervals))
	for i, unit := range seq.intervals {
		ioiMs := unit * unitMs
		offMs := ioiMs - toneDurMs
		if offMs < 0 {
			offMs = 0
		}
		elements[i] = stimuli.SoundStreamElement{
			Sound:       sound,
			DurationOn:  time.Duration(toneDurMs) * time.Millisecond,
			DurationOff: time.Duration(offMs) * time.Millisecond,
		}
	}
	return elements
}

// ── Reproduction tap collection ───────────────────────────────────────────────

// collectTaps waits for n SPACE presses and records their absolute timestamps.
// ESC or window-close returns sdl.EndLoop.
func collectTaps(n int) ([]time.Time, error) {
	var taps []time.Time
	for len(taps) < n {
		var event sdl.Event
		sdl.WaitEvent(&event)
		switch event.Type {
		case sdl.EVENT_KEY_DOWN:
			key := event.KeyboardEvent().Key
			if key == sdl.K_ESCAPE {
				return taps, sdl.EndLoop
			}
			if key == sdl.K_SPACE {
				taps = append(taps, time.Now())
			}
		case sdl.EVENT_QUIT:
			return taps, sdl.EndLoop
		}
	}
	return taps, nil
}

// ── Scoring ───────────────────────────────────────────────────────────────────

// reproductionError computes the sum of absolute differences (ms) between
// the observed inter-tap intervals and the expected cyclic pattern.
func reproductionError(taps []time.Time, seq seqDef) int64 {
	n := len(seq.intervals)
	var totalErr int64
	for k := 0; k < len(taps)-1; k++ {
		expected := int64(seq.intervals[k%n]) * int64(unitMs)
		observed := taps[k+1].Sub(taps[k]).Milliseconds()
		diff := observed - expected
		if diff < 0 {
			diff = -diff
		}
		totalErr += diff
	}
	return totalErr
}

// ── UI helpers ────────────────────────────────────────────────────────────────

func showMsg(exp *control.Experiment, msg string) error {
	txt := stimuli.NewTextBox(msg, 900, control.Origin(), control.White)
	return exp.Show(txt)
}

func showAndWait(exp *control.Experiment, msg string, key control.Keycode) error {
	if err := showMsg(exp, msg); err != nil {
		return err
	}
	return exp.Keyboard.WaitKey(key)
}

// enterPressed returns true if any UserEvent is a KEY_DOWN for RETURN/ENTER.
func enterPressed(events []stimuli.UserEvent) bool {
	for _, ev := range events {
		if ev.Event.Type == sdl.EVENT_KEY_DOWN {
			key := ev.Event.KeyboardEvent().Key
			if key == sdl.K_RETURN || key == sdl.K_KP_ENTER {
				return true
			}
		}
	}
	return false
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	// Register experiment-specific flags BEFORE NewExperimentFromFlags parses them.
	soundFlag := flag.String("sound", "tone",
		"Sound type: 'tone' (830 Hz sine) or 'cymbal' (embedded cymbal sample)")

	exp := control.NewExperimentFromFlags(
		"Perception of Temporal Patterns", control.Black, control.White, 28)
	defer exp.End()

	// ── Sound preparation ────────────────────────────────────────────────────
	var sound stimuli.AudioPlayable
	switch *soundFlag {
	case "cymbal":
		s := stimuli.NewSoundFromMemory(cymbalBytes)
		if err := s.PreloadDevice(exp.AudioDevice); err != nil {
			log.Fatalf("failed to preload cymbal: %v", err)
		}
		sound = s
	default: // "tone"
		// Approximate a square wave at 830 Hz by summing the first four odd harmonics.
		t := stimuli.NewComplexTone(
			[]float64{830, 2490, 4150, 5810}, toneDurMs, toneRampMs, 0.3)
		if err := t.PreloadDevice(exp.AudioDevice); err != nil {
			log.Fatalf("failed to preload tone: %v", err)
		}
		sound = t
	}

	// ── Data file setup ──────────────────────────────────────────────────────
	exp.AddDataVariableNames([]string{
		"seq_id", "category", "sound_type",
		"n_presentations", "repro_error_ms",
	})

	// ── Instructions ─────────────────────────────────────────────────────────
	err := showAndWait(exp,
		"Perception of Temporal Patterns\n\n"+
			"You will hear rhythmic sequences of tones.\n\n"+
			"LEARNING PHASE\n"+
			"The sequence repeats automatically.\n"+
			"Tap along with the rhythm using SPACE.\n"+
			"Press ENTER when you are ready to reproduce it.\n\n"+
			"REPRODUCTION PHASE\n"+
			"Tap SPACE once for each tone onset,\n"+
			"for 4 complete repetitions of the pattern (36 taps total).\n\n"+
			"Press SPACE to begin.",
		control.K_SPACE)
	if err != nil {
		return
	}

	// ── Trial loop ───────────────────────────────────────────────────────────
	order := rand.Perm(len(sequences))

	for _, idx := range order {
		seq := sequences[idx]
		stream := buildStream(seq, sound)

		// -- Learning phase --------------------------------------------------
		presentations := 0
		for {
			presentations++
			events, _, streamErr := stimuli.PlayStreamOfSounds(stream)
			if streamErr != nil {
				log.Printf("stream error: %v", streamErr)
				return
			}
			if enterPressed(events) {
				break
			}
		}

		// -- Reproduction phase ----------------------------------------------
		nTaps := 4 * len(seq.intervals) // 4 periods × 9 tones = 36 taps
		err = showMsg(exp,
			fmt.Sprintf("Reproduce the rhythm.\n"+
				"Tap SPACE for each beat — 4 full cycles (%d taps).\n\n"+
				"Start tapping now...", nTaps))
		if err != nil {
			return
		}

		taps, tapErr := collectTaps(nTaps)
		if tapErr != nil {
			break
		}

		repError := reproductionError(taps, seq)
		exp.Data.Add(seq.id, seq.category, *soundFlag, presentations, repError)

		fmt.Printf("Seq %2d (cat %d): %2d presentation(s), error = %d ms\n",
			seq.id, seq.category, presentations, repError)

		// Brief blank between trials
		if err = exp.Blank(500); err != nil {
			return
		}
	}

	// ── End ──────────────────────────────────────────────────────────────────
	_ = showAndWait(exp,
		"Thank you! The experiment is complete.\n\nPress SPACE to exit.",
		control.K_SPACE)
}
