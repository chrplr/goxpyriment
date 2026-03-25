// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// Number Double-Digits Comparison — replication of Dehaene et al. (1990),
// Experiments 1 and 2.
//
// A target two-digit number is displayed and the participant presses a key
// to indicate whether it is larger or smaller than the fixed standard (55 or 65).
//
// Usage:
//
//	# Experiment 1 (standard = 55):
//	go run . -exp 1 -d -s 1
//
//	# Experiment 2 (standard = 65), LR response mapping:
//	go run . -exp 2 -group LR -d -s 2
//
//	# Experiment 2, LL response mapping (left hand = larger):
//	go run . -exp 2 -group LL -d -s 3
//
// Response keys:
//
//	F (left)  = "smaller than standard"   (group LR, and experiment 1)
//	J (right) = "larger than standard"
//
//	F (left)  = "larger than standard"    (group LL, experiment 2 only)
//	J (right) = "smaller than standard"

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── timing constants ──────────────────────────────────────────────────────────

const (
	stimDurMs    = 2000 // stimulus display duration (ms)
	isiMs        = 2000 // inter-stimulus interval blank (ms)
	trainingSize = 10   // training-block trials
)

// ── trial ─────────────────────────────────────────────────────────────────────

type trial struct {
	target int
}

// ── trial lists ───────────────────────────────────────────────────────────────

// buildExp1Trials returns the Experiment 1 (standard = 55) trial list:
// targets 11–99 ∖ {55}; numbers in [41,69] appear 4×, others 2×.
func buildExp1Trials() []trial {
	var trials []trial
	for n := 11; n <= 99; n++ {
		if n == 55 {
			continue
		}
		reps := 2
		if n >= 41 && n <= 69 {
			reps = 4
		}
		for i := 0; i < reps; i++ {
			trials = append(trials, trial{n})
		}
	}
	return trials
}

// buildExp2Trials returns the Experiment 2 (standard = 65) trial list:
// targets 31–99 ∖ {65}, each appearing 4×.
func buildExp2Trials() []trial {
	var trials []trial
	for n := 31; n <= 99; n++ {
		if n == 65 {
			continue
		}
		for i := 0; i < 4; i++ {
			trials = append(trials, trial{n})
		}
	}
	return trials
}

// ── constrained shuffle ───────────────────────────────────────────────────────

// constrainedShuffle pseudorandomizes trials so that:
//  1. The same target number does not appear twice in a row.
//  2. No more than 3 consecutive trials require the same response direction
//     (larger or smaller than standard).
//
// Up to maxAttempts random shuffles are tried; the first valid permutation is
// returned. If no valid permutation is found, the best partial result is
// returned and a warning is printed.
func constrainedShuffle(trials []trial, standard int, rng *rand.Rand, maxAttempts int) []trial {
	result := make([]trial, len(trials))
	copy(result, trials)

	for i := 0; i < maxAttempts; i++ {
		rng.Shuffle(len(result), func(a, b int) { result[a], result[b] = result[b], result[a] })
		if isValid(result, standard) {
			return result
		}
	}
	log.Printf("warning: could not find a fully constrained shuffle in %d attempts; using best random order", maxAttempts)
	return result
}

func isValid(trials []trial, standard int) bool {
	streak := 1
	for i := 1; i < len(trials); i++ {
		// Constraint 1: no same number twice in a row.
		if trials[i].target == trials[i-1].target {
			return false
		}
		// Constraint 2: no more than 3 consecutive same direction.
		if (trials[i].target > standard) == (trials[i-1].target > standard) {
			streak++
			if streak > 3 {
				return false
			}
		} else {
			streak = 1
		}
	}
	return true
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	expNum := flag.Int("exp", 1, "experiment number: 1 (standard=55) or 2 (standard=65)")
	groupFlag := flag.String("group", "LR", "response mapping for exp 2: LR (right=larger) or LL (left=larger)")
	// -d and -s are parsed by NewExperimentFromFlags
	exp := control.NewExperimentFromFlags("Number Comparison (Two Digits)", control.Gray, control.Black, 32)
	defer exp.End()

	// ── validate flags ────────────────────────────────────────────────────────

	var standard int
	switch *expNum {
	case 1:
		standard = 55
	case 2:
		standard = 65
	default:
		log.Fatalf("-exp must be 1 or 2, got %d", *expNum)
	}

	group := *groupFlag
	if *expNum == 2 {
		switch group {
		case "LR", "LL":
		default:
			log.Fatalf("-group must be LR or LL for exp 2, got %q", group)
		}
	} else {
		group = "LR" // experiment 1 uses right=larger
	}

	// leftLarger = true means F (left) = "larger than standard"
	leftLarger := group == "LL"

	// ── layout ────────────────────────────────────────────────────────────────

	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("warning: SetLogicalSize: %v", err)
	}

	// Large font for the target number.
	bigFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 120)
	if err != nil {
		log.Fatalf("load big font: %v", err)
	}
	defer bigFont.Close()

	// Medium font for the standard label and response reminder.
	medFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 36)
	if err != nil {
		log.Fatalf("load medium font: %v", err)
	}
	defer medFont.Close()

	// ── static stimuli ────────────────────────────────────────────────────────

	// Standard label shown permanently at the top.
	standardLabel := stimuli.NewTextLine(
		fmt.Sprintf("Standard: %d", standard), 0, 350, control.Black)
	standardLabel.Font = medFont

	// Response reminder shown permanently at the bottom.
	var reminderText string
	if leftLarger {
		reminderText = fmt.Sprintf("F = LARGER than %d        J = SMALLER than %d", standard, standard)
	} else {
		reminderText = fmt.Sprintf("F = SMALLER than %d        J = LARGER than %d", standard, standard)
	}
	reminderStim := stimuli.NewTextLine(reminderText, 0, -400, control.Black)
	reminderStim.Font = medFont

	// Target number stimulus — text is updated per trial.
	targetStim := stimuli.NewTextLine("", 0, 0, control.Black)
	targetStim.Font = bigFont

	// ── data columns ──────────────────────────────────────────────────────────

	exp.AddDataVariableNames([]string{
		"exp", "group", "standard", "block", "is_training",
		"target", "distance", "response", "rt_ms", "correct",
	})

	// ── instructions ─────────────────────────────────────────────────────────

	var largerKey, smallerKey string
	if leftLarger {
		largerKey, smallerKey = "F", "J"
	} else {
		largerKey, smallerKey = "J", "F"
	}
	practiceInstr := fmt.Sprintf(
		`Number Comparison — Experiment %d

A two-digit number will appear on screen.
Press %s if it is LARGER than %d.
Press %s if it is SMALLER than %d.

Respond as quickly and accurately as possible.
The number will disappear after 2 seconds.

You will first complete a short practice block.
Press SPACE to begin.`, *expNum, largerKey, standard, smallerKey, standard)

	startInstr := fmt.Sprintf(
		`Practice complete.

The experiment will now begin (%d trials).
Press SPACE to start.`, func() int {
		if *expNum == 1 {
			return len(buildExp1Trials())
		}
		return len(buildExp2Trials())
	}())

	// ── build trial lists ─────────────────────────────────────────────────────

	rng := rand.New(rand.NewSource(int64(exp.SubjectID)*1234567 + 17))

	var allTrials []trial
	if *expNum == 1 {
		allTrials = buildExp1Trials()
	} else {
		allTrials = buildExp2Trials()
	}

	// Training block: 10 trials sampled from allTrials without replacement.
	trainTrials := make([]trial, len(allTrials))
	copy(trainTrials, allTrials)
	rng.Shuffle(len(trainTrials), func(i, j int) { trainTrials[i], trainTrials[j] = trainTrials[j], trainTrials[i] })
	trainTrials = trainTrials[:trainingSize]

	expTrials := constrainedShuffle(allTrials, standard, rng, 2000)

	// ── response keys ─────────────────────────────────────────────────────────

	respKeys := []control.Keycode{control.K_F, control.K_J}

	isCorrect := func(target int, key control.Keycode) bool {
		larger := target > standard
		if leftLarger {
			return (key == control.K_F && larger) || (key == control.K_J && !larger)
		}
		return (key == control.K_J && larger) || (key == control.K_F && !larger)
	}

	keyLabel := func(k control.Keycode) string {
		switch k {
		case control.K_F:
			return "F"
		case control.K_J:
			return "J"
		default:
			return "timeout"
		}
	}

	// ── run a single block ────────────────────────────────────────────────────

	runBlock := func(blockTrials []trial, blockNum int, isTraining bool) error {
		for _, t := range blockTrials {
			// ISI blank with fixation
			if err := exp.Blank(isiMs); err != nil {
				return err
			}

			// Update target text (triggers texture re-render on next Draw).
			targetStim.Text = fmt.Sprintf("%d", t.target)
			targetStim.Unload()

			// Draw: standard label + response reminder + target number.
			exp.Screen.Clear()
			if err := standardLabel.Draw(exp.Screen); err != nil {
				return err
			}
			if err := reminderStim.Draw(exp.Screen); err != nil {
				return err
			}
			if err := targetStim.Draw(exp.Screen); err != nil {
				return err
			}
			onsetNS, err := exp.Screen.FlipNS()
			if err != nil {
				return err
			}

			// Wait for a keypress up to stimDurMs.
			key, eventTS, respErr := exp.Keyboard.WaitKeysEventRT(respKeys, stimDurMs)
			if control.IsEndLoop(respErr) {
				return control.EndLoop
			}

			var rtMs int64
			if key != 0 && eventTS >= onsetNS {
				rtMs = int64(eventTS-onsetNS) / 1_000_000
			}

			distance := t.target - standard
			if distance < 0 {
				distance = -distance
			}

			exp.Data.Add(
				*expNum, group, standard, blockNum, isTraining,
				t.target, distance, keyLabel(key), rtMs,
				key != 0 && isCorrect(t.target, key),
			)

			// If response came early, blank for remaining stimulus duration
			// so total trial time is always stimDurMs + isiMs = 4 s.
			if key != 0 && rtMs < stimDurMs {
				remaining := int(stimDurMs) - int(rtMs)
				if err := exp.Blank(remaining); err != nil {
					if control.IsEndLoop(err) {
						return control.EndLoop
					}
				}
			}
		}
		return nil
	}

	// ── experiment run loop ───────────────────────────────────────────────────

	runErr := exp.Run(func() error {
		if err := exp.ShowInstructions(practiceInstr); err != nil {
			return err
		}

		// Training block
		shuffledTrain := make([]trial, len(trainTrials))
		copy(shuffledTrain, trainTrials)
		rng.Shuffle(len(shuffledTrain), func(i, j int) { shuffledTrain[i], shuffledTrain[j] = shuffledTrain[j], shuffledTrain[i] })
		if err := runBlock(shuffledTrain, 0, true); err != nil {
			return err
		}

		// Main block
		if err := exp.ShowInstructions(startInstr); err != nil {
			return err
		}
		if err := runBlock(expTrials, 1, false); err != nil {
			return err
		}

		// End screen
		endStim := stimuli.NewTextBox(
			"The experiment is complete. Thank you!\n\nPress any key to exit.",
			800, control.FPoint{}, control.Black)
		if err := exp.Show(endStim); err != nil {
			return err
		}
		_, err := exp.Keyboard.Wait()
		if err != nil {
			return err
		}
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatal(runErr)
	}
}
