// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Visual Search — Treisman & Gelade (1980) paradigm
//
// Replicates the classic feature-vs-conjunction search design:
//
//	Feature search:    target = Red T, distractors = Blue T
//	Conjunction search: target = Red T, distractors = Blue T + Red L
//
// Procedure per trial:
//
//	1000 ms blank ITI
//	500 ms fixation cross
//	Search array displayed until response
//	Response: J = target present, F = target absent
//	500 ms feedback ("Correct" / "Incorrect" / "Too slow!")
//
// Design: 2 (task) × 2 (target presence) × 3 (set size: 4, 12, 24) × 20 reps
// = 240 trials total, presented in randomised order.
//
// Usage:
//
//	go run main.go [-w] [-d N] [-s <subjectID>]
package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Experiment parameters ────────────────────────────────────────────────────

const (
	nRepsPerCell = 20   // repetitions per (task × presence × set_size) cell
	fixationMs   = 500  // fixation cross duration (ms)
	feedbackMs   = 500  // feedback duration (ms)
	itiMs        = 1000 // inter-trial interval (ms)
	tooSlowMs    = 2000 // RT threshold for "Too slow!" feedback (ms)
	stimFontSize = 36   // point size for search-array letters
	keyPresent   = control.K_J
	keyAbsent    = control.K_F
)

var setSizes = []int{4, 12, 24}

// ── Grid layout (6 cols × 4 rows = 24 cells, center-relative) ───────────────

const (
	gridCols     = 6
	gridRows     = 4
	gridSpacingX = float32(150)
	gridSpacingY = float32(140)
	jitterPx     = float32(20)
)

// ── Trial definition ─────────────────────────────────────────────────────────

type trialDef struct {
	task          string // "feature" or "conjunction"
	setSize       int
	targetPresent bool
}

func buildTrials() []trialDef {
	var trials []trialDef
	for _, task := range []string{"feature", "conjunction"} {
		for _, sz := range setSizes {
			for _, present := range []bool{true, false} {
				for i := 0; i < nRepsPerCell; i++ {
					trials = append(trials, trialDef{task, sz, present})
				}
			}
		}
	}
	design.ShuffleList(trials)
	return trials
}

// ── Grid position sampling ───────────────────────────────────────────────────

// gridPositions returns n center-relative positions sampled (without
// replacement) from a freshly jittered 6×4 grid.
type pos = [2]float32

func gridPositions(n int, rng *rand.Rand) []pos {
	all := make([]pos, gridCols*gridRows)
	startX := -float32(gridCols-1) / 2 * gridSpacingX
	startY := -float32(gridRows-1) / 2 * gridSpacingY
	idx := 0
	for r := 0; r < gridRows; r++ {
		for c := 0; c < gridCols; c++ {
			x := startX + float32(c)*gridSpacingX + (rng.Float32()*2-1)*jitterPx
			y := startY + float32(r)*gridSpacingY + (rng.Float32()*2-1)*jitterPx
			all[idx] = pos{x, y}
			idx++
		}
	}
	rng.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	result := make([]pos, n)
	copy(result, all[:n])
	return result
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	exp := control.NewExperimentFromFlags("Visual Search", control.Gray, control.White, 32)
	defer exp.End()

	// Larger font for search-array letters.
	stimFont, err := control.FontFromMemory(assets_embed.InconsolataFont, stimFontSize)
	if err != nil {
		exp.Fatal("font load: %v", err)
	}
	defer stimFont.Close()

	exp.AddDataVariableNames([]string{
		"trial", "task", "set_size", "target_present", "response", "rt_ms", "correct",
	})

	trials := buildTrials()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	fixCross := stimuli.NewFixCross(30, 4, control.Black)
	respKeys := []control.Keycode{keyPresent, keyAbsent}

	instrText := "Visual Search Task\n\n" +
		"You will see arrays of letters on the screen.\n\n" +
		"Your task is to decide whether a RED T is present.\n\n" +
		"  Press  J  if the RED T is PRESENT.\n" +
		"  Press  F  if the RED T is ABSENT.\n\n" +
		"Respond as quickly and accurately as possible.\n\n" +
		fmt.Sprintf("There will be %d trials in total.\n\n", len(trials)) +
		"Press SPACE to begin."

	err = exp.Run(func() error {
		exp.ShowInstructions(instrText)

		for i, t := range trials {
			// ── Inter-trial interval ─────────────────────────────────────────
			if err := exp.Blank(itiMs); err != nil {
				return err
			}

			// ── Fixation cross ───────────────────────────────────────────────
			if err := exp.ShowTimed(fixCross, fixationMs); err != nil {
				return err
			}

			// ── Build search array ───────────────────────────────────────────
			type item struct {
				letter string
				color  control.Color
			}

			positions := gridPositions(t.setSize, rng)
			items := make([]item, t.setSize)

			if t.task == "feature" {
				// Target: Red T. Distractors: Blue T.
				targetIdx := -1
				if t.targetPresent {
					targetIdx = rng.Intn(t.setSize)
				}
				for j := range items {
					if j == targetIdx {
						items[j] = item{"T", control.Red}
					} else {
						items[j] = item{"T", control.Blue}
					}
				}
			} else {
				// Conjunction. Target: Red T. Distractors: Blue T + Red L.
				targetIdx := -1
				if t.targetPresent {
					targetIdx = rng.Intn(t.setSize)
				}
				nDistractors := t.setSize
				if t.targetPresent {
					nDistractors--
				}
				// Split distractors roughly evenly between Blue T and Red L.
				nBlueT := nDistractors / 2
				nRedL := nDistractors - nBlueT
				pool := make([]item, 0, nDistractors)
				for k := 0; k < nBlueT; k++ {
					pool = append(pool, item{"T", control.Blue})
				}
				for k := 0; k < nRedL; k++ {
					pool = append(pool, item{"L", control.Red})
				}
				rng.Shuffle(len(pool), func(a, b int) { pool[a], pool[b] = pool[b], pool[a] })
				di := 0
				for j := range items {
					if j == targetIdx {
						items[j] = item{"T", control.Red}
					} else {
						items[j] = pool[di]
						di++
					}
				}
			}

			// ── Draw search array ────────────────────────────────────────────
			// Clear stale keypresses before showing the stimulus.
			exp.Keyboard.Clear()

			screen := exp.Screen
			if err := screen.Clear(); err != nil {
				return err
			}
			textStims := make([]*stimuli.TextLine, t.setSize)
			for j, it := range items {
				tl := stimuli.NewTextLine(it.letter, positions[j][0], positions[j][1], it.color)
				tl.Font = stimFont
				textStims[j] = tl
				if err := tl.Draw(screen); err != nil {
					return err
				}
			}
			onsetNS, err := screen.FlipTS()
			if err != nil {
				return err
			}

			// ── Wait for response ────────────────────────────────────────────
			key, eventTS, respErr := exp.Keyboard.GetKeyEventTS(respKeys, -1)
			if respErr != nil {
				return respErr
			}
			rt := int64(eventTS-onsetNS) / 1_000_000

			// Free GPU textures immediately after response.
			for _, tl := range textStims {
				_ = tl.Unload()
			}

			// ── Score response ───────────────────────────────────────────────
			var response string
			if key == keyPresent {
				response = "present"
			} else {
				response = "absent"
			}
			correct := (t.targetPresent && key == keyPresent) || (!t.targetPresent && key == keyAbsent)

			exp.Data.Add(i+1, t.task, t.setSize, t.targetPresent, response, rt, correct)
			fmt.Printf("Trial %3d  task=%-11s  sz=%2d  present=%-5v  resp=%-7s  rt=%4d ms  correct=%v\n",
				i+1, t.task, t.setSize, t.targetPresent, response, rt, correct)

			// ── Feedback ─────────────────────────────────────────────────────
			var fbText string
			switch {
			case rt > tooSlowMs:
				fbText = "Too slow!"
			case correct:
				fbText = "Correct!"
			default:
				fbText = "Incorrect"
			}
			if err := exp.ShowTimed(stimuli.NewTextLine(fbText, 0, 0, control.Black), feedbackMs); err != nil {
				return err
			}
		}

		// ── End screen ───────────────────────────────────────────────────────
		return exp.ShowEndMessage("Experiment complete. Thank you!\n\nPress any key to exit.")
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
