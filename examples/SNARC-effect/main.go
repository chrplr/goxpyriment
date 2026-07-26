// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// SNARC-effect experiment — replication of Dehaene et al. (1993) Experiment 1.
//
// Participants perform a parity judgment task (odd or even?) on digits 0–9.
// The key manipulation is the response-mapping: in one block, even→left key and
// odd→right key; in the other block the mapping is reversed. Counterbalancing
// (which block comes first) is determined by subject ID parity (even ID → A first).
//
// Reference: Dehaene, S., Bossini, S., & Giraux, P. (1993). The mental
// representation of parity and number magnitude. Journal of Experimental
// Psychology: General, 122(3), 371–396.
package main

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Response keys: F (left index finger) and L (right index finger).
const (
	LeftKey  = control.K_F
	RightKey = control.K_L
)

// Frame dimensions (pixels). Approximates the 22 mm × 32 mm frame described in
// Dehaene et al. (1993) at roughly 100 DPI / 50 cm viewing distance.
const (
	frameW     = 88  // pixels, ≈ 22 mm at 100 DPI
	frameH     = 128 // pixels, ≈ 32 mm at 100 DPI
	lineWidth  = float32(2)
	NTraining  = 12
	NExpTrials = 90 // 9 presentations × 10 digits
)

// blockMapping describes the response-key assignment for a single block.
type blockMapping struct {
	name     string
	evenKey  control.Keycode // key to press for even digits
	oddKey   control.Keycode // key to press for odd digits
	evenSide string
	oddSide  string
}

// generateTrainingSeq returns n random digits 0–9 with no two consecutive repeats.
func generateTrainingSeq(n int) []int {
	seq := make([]int, n)
	seq[0] = rand.Intn(10)
	for i := 1; i < n; i++ {
		d := rand.Intn(10)
		for d == seq[i-1] {
			d = rand.Intn(10)
		}
		seq[i] = d
	}
	return seq
}

// generateExpSeq returns 90 digits (each of 0–9 appearing exactly 9 times) with no
// two consecutive digits the same.  The sequence approximates — but does not strictly
// enforce — the Dehaene et al. transition constraint (every ordered pair appearing
// exactly once), which mathematically requires 91 elements (an Eulerian path on K_10).
func generateExpSeq() []int {
	// Build the pool: each digit appears 9 times.
	pool := make([]int, 0, NExpTrials)
	for d := 0; d < 10; d++ {
		for i := 0; i < 9; i++ {
			pool = append(pool, d)
		}
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	// Fix consecutive duplicates: for each position where pool[i] == pool[i-1],
	// find a later position j where pool[j] != pool[i] and swap.
	for i := 1; i < len(pool); i++ {
		if pool[i] == pool[i-1] {
			// Search for a swap candidate.
			swapped := false
			start := rand.Intn(len(pool)-i) + i + 1
			for offset := 0; offset < len(pool)-i-1; offset++ {
				j := i + 1 + (start-i-1+offset)%(len(pool)-i-1)
				if pool[j] != pool[i-1] && (i+1 >= len(pool) || pool[j] != pool[i+1]) {
					pool[i], pool[j] = pool[j], pool[i]
					swapped = true
					break
				}
			}
			if !swapped {
				// Fallback: just find any later element that differs from pool[i-1].
				for j := i + 1; j < len(pool); j++ {
					if pool[j] != pool[i-1] {
						pool[i], pool[j] = pool[j], pool[i]
						break
					}
				}
			}
		}
	}
	return pool
}

// drawFrame renders a rectangular outline (no fill) centered on the screen.
func drawFrame(exp *control.Experiment, color control.Color) {
	hw := float32(frameW) / 2
	hh := float32(frameH) / 2

	lines := []*stimuli.Line{
		stimuli.NewLine(control.Point(-hw, -hh), control.Point(hw, -hh), color, lineWidth), // top
		stimuli.NewLine(control.Point(hw, -hh), control.Point(hw, hh), color, lineWidth),   // right
		stimuli.NewLine(control.Point(hw, hh), control.Point(-hw, hh), color, lineWidth),   // bottom
		stimuli.NewLine(control.Point(-hw, hh), control.Point(-hw, -hh), color, lineWidth), // left
	}
	for _, l := range lines {
		_ = l.Draw(exp.Screen)
	}
}

// runBlock executes one block of the SNARC experiment (training + experimental trials).
// It returns true if the experiment should continue (no quit), false otherwise.
func runBlock(exp *control.Experiment, m blockMapping, bigFont interface{ Close() }, digitStims []*stimuli.TextLine) {
	instrText := fmt.Sprintf(
		"Block: %s\n\n"+
			"You will see a digit (0–9) appear on the screen.\n"+
			"Decide as QUICKLY as possible whether it is even or odd.\n\n"+
			"  If the digit is EVEN → press F (left hand)\n"+
			"  If the digit is ODD  → press L (right hand)\n\n"+
			"(Even = %s key,  Odd = %s key)\n\n"+
			"There will be %d training trials followed by %d experimental trials.\n\n"+
			"Press SPACE to begin.",
		m.name, m.evenSide, m.oddSide, NTraining, NExpTrials,
	)
	// Override instruction for reversed mapping.
	if m.evenKey == RightKey {
		instrText = fmt.Sprintf(
			"Block: %s\n\n"+
				"You will see a digit (0–9) appear on the screen.\n"+
				"Decide as QUICKLY as possible whether it is even or odd.\n\n"+
				"  If the digit is EVEN → press L (right hand)\n"+
				"  If the digit is ODD  → press F (left hand)\n\n"+
				"(Even = %s key,  Odd = %s key)\n\n"+
				"There will be %d training trials followed by %d experimental trials.\n\n"+
				"Press SPACE to begin.",
			m.name, m.evenSide, m.oddSide, NTraining, NExpTrials,
		)
	}
	exp.ShowInstructions(instrText)

	// Build trial lists.
	trainSeq := generateTrainingSeq(NTraining)
	expSeq := generateExpSeq()

	allTrials := struct {
		seqs    [][]int
		labels  []string
		records []bool // whether to record in data file
	}{
		seqs:    [][]int{trainSeq, expSeq},
		labels:  []string{"training", "experimental"},
		records: []bool{false, true},
	}

	responseKeys := []control.Keycode{LeftKey, RightKey}

	for phaseIdx, seq := range allTrials.seqs {
		phaseLabel := allTrials.labels[phaseIdx]
		record := allTrials.records[phaseIdx]

		for trialIdx, digit := range seq {
			// ITI: blank screen for 1500 ms (also serves as pre-trial gap).
			exp.Blank(1500)

			// Fixation frame: 300 ms.
			_ = exp.Screen.Clear()
			drawFrame(exp, control.White)
			_ = exp.Screen.Update()
			exp.Wait(300)

			// Flush stale key presses before stimulus onset.
			exp.Keyboard.Clear()

			// Show frame + digit; record onset timestamp.
			digitStim := digitStims[digit]
			_ = exp.Screen.Clear()
			drawFrame(exp, control.White)
			_ = digitStim.Draw(exp.Screen)
			onset, _ := exp.Screen.FlipTS()

			// Wait for response (max 1300 ms).
			key, keyTS, _ := exp.Keyboard.GetKeyEventTS(responseKeys, 1300)

			var rtMs int64
			var correct bool
			if key != 0 {
				rtMs = int64(keyTS-onset) / 1_000_000
				isEven := digit%2 == 0
				correct = (isEven && key == m.evenKey) || (!isEven && key == m.oddKey)
			}
			// key == 0 means timeout; correct stays false, rtMs stays 0.

			if record {
				exp.Data.Add(m.name, phaseLabel, trialIdx+1, digit, key, rtMs, correct)
			}

			fmt.Printf("[%s/%s] Trial %3d: digit=%d  key=%d  RT=%4d ms  correct=%v\n",
				m.name, phaseLabel, trialIdx+1, digit, key, rtMs, correct)

			if !correct && key != 0 {
				exp.Audio.PlayBuzzer()
			}
		}

		if phaseIdx == 0 {
			// Brief pause between training and experimental phase.
			exp.ShowInstructions(
				fmt.Sprintf("Training complete.\n\nThe main experiment (%d trials) will now begin.\n\nPress SPACE to continue.", NExpTrials),
			)
		}
	}

	exp.ShowInstructions("Block complete.\n\nTake a short break.\n\nPress SPACE when you are ready to continue.")
}

func main() {
	exp := control.NewExperimentFromFlags("SNARC Effect", control.Black, control.White, 32)
	defer exp.End()

	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	// Larger font for digit stimuli (≈ 10 mm × 17 mm target size).
	bigFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 64)
	if err != nil {
		log.Printf("Warning: failed to load big font: %v", err)
	} else {
		defer bigFont.Close()
	}

	// Pre-create one TextLine per digit.
	digitStims := make([]*stimuli.TextLine, 10)
	for d := 0; d < 10; d++ {
		stim := stimuli.NewTextLine(fmt.Sprintf("%d", d), 0, 0, control.White)
		if bigFont != nil {
			stim.Font = bigFont
		}
		digitStims[d] = stim
	}

	exp.AddDataVariableNames([]string{"block", "phase", "trial", "digit", "key", "rt_ms", "correct"})

	// Counterbalancing: even subject ID → Block A (even=left, odd=right) first;
	// odd subject ID → Block B (even=right, odd=left) first.
	blockA := blockMapping{
		name: "A", evenKey: LeftKey, oddKey: RightKey,
		evenSide: "left (F)", oddSide: "right (L)",
	}
	blockB := blockMapping{
		name: "B", evenKey: RightKey, oddKey: LeftKey,
		evenSide: "right (L)", oddSide: "left (F)",
	}

	var blocks []blockMapping
	if exp.SubjectID%2 == 0 {
		blocks = []blockMapping{blockA, blockB}
	} else {
		blocks = []blockMapping{blockB, blockA}
	}

	err = exp.Run(func() error {
		exp.ShowInstructions(
			"SNARC Effect Experiment\n\n" +
				"In this experiment you will see single digits (0–9) on the screen.\n" +
				"Your task is to decide, as quickly and accurately as possible,\n" +
				"whether each digit is ODD or EVEN.\n\n" +
				"You will complete two blocks with different key assignments.\n" +
				"The assignment will be explained before each block.\n\n" +
				"Press SPACE to begin.",
		)

		for _, block := range blocks {
			runBlock(exp, block, bigFont, digitStims)
		}

		_ = exp.Data.Save()

		exp.ShowInstructions("The experiment is over.\n\nThank you for your participation!\n\nPress SPACE to exit.")
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
