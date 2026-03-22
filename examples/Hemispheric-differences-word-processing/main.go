// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// Hemispheric asymmetries in the time course of recognition memory
// (Federmeier & Benjamin, 2005, Psychonomic Bulletin & Review, 12(6), 993–998).
//
// This example implements a simplified version of their continuous recognition
// paradigm:
//   - Study: words are presented briefly in the left or right visual field
//     (LVF/RVF), to bias initial processing to the right or left hemisphere.
//   - Test: after a delay (lag), words are presented centrally and participants
//     make an old/new recognition judgment.
//
// For simplicity, this example:
//   - Uses a smaller number of items than the original paper.
//   - Implements fixed lags as temporal delays between study and test,
//     rather than as a fixed number of intervening items in a long stream.
//   - Still logs visual field of study, lag condition, and recognition
//     performance, so that hemispheric × lag effects can be analyzed.

package main

import (
	"log"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	StudyDurationMs = 200  // word duration at study
	StudyISI        = 2300 // ISI after study word (ms)

	ShortLagMs = 2000  // approximate short lag
	LongLagMs  = 10000 // approximate long lag

	OffsetDegX = 200.0 // horizontal offset in pixels for LVF/RVF words

	OldKey = control.K_F // "old"/seen
	NewKey = control.K_J // "new"/unseen
)

type StudyTestPair struct {
	Word   string
	VF     string // "LVF" or "RVF"
	LagMs  int
	LagTag string // e.g., "short", "long"
}

func main() {
	exp := control.NewExperimentFromFlags("Hemispheric-Differences-Word-Processing", control.Black, control.White, 32)
	defer exp.End()

	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"trial_index",
		"phase", // "old" (studied) or "new"
		"word",
		"vf",      // LVF, RVF, or "NA" for new-only items
		"lag_tag", // "short" or "long"
		"lag_ms",  // intended lag between study and test (ms)
		"key",
		"rt",
		"correct",
	})

	fixation := stimuli.NewFixCross(30, 3, control.DefaultTextColor)

	// Simple word pool (in the paper: 567 high-imageability nouns).
	wordPool := []string{
		"TABLE", "RIVER", "HOUSE", "APPLE", "MOUNTAIN", "WINDOW", "GARDEN", "SCHOOL",
		"ANIMAL", "PICTURE", "FLOWER", "MARKET", "BOTTLE", "PENCIL", "CANDLE", "MIRROR",
		"FINGER", "MUSIC", "CLOUD", "OCEAN", "FOREST", "ENGINE", "BASKET", "LETTER",
		"BUTTON", "KITCHEN", "PILLOW", "CIRCLE", "SILVER", "GUITAR", "DOOR", "BRIDGE",
		"PLATES", "ISLAND", "LADDER", "POCKET", "CUPBOARD", "TICKET", "GARDEN", "DESERT",
	}

	if len(wordPool) < 30 {
		log.Fatalf("word pool too small for this example")
	}

	// Build studied pairs: words that will be seen later at test ("old" items).
	// We create an equal number of LVF and RVF items, split between short and long lags.
	var pairs []StudyTestPair
	shuffled := make([]string, len(wordPool))
	copy(shuffled, wordPool)
	design.ShuffleList(shuffled)

	// Use first 24 words for old items (12 LVF, 12 RVF, half short, half long).
	oldWords := shuffled[:24]
	for i, w := range oldWords {
		var vf string
		if i%2 == 0 {
			vf = "LVF"
		} else {
			vf = "RVF"
		}
		var lagMs int
		var lagTag string
		if i%4 < 2 {
			lagMs = ShortLagMs
			lagTag = "short"
		} else {
			lagMs = LongLagMs
			lagTag = "long"
		}
		pairs = append(pairs, StudyTestPair{
			Word:   w,
			VF:     vf,
			LagMs:  lagMs,
			LagTag: lagTag,
		})
	}

	// Remaining words will serve as new-only test items.
	newWords := shuffled[24:36] // 12 new items

	// Shuffle the studied pairs so VF/lag conditions are intermixed.
	design.ShuffleList(pairs)

	// Instructions
	instrText := "Lateralized Recognition Memory\n\n" +
		"First, you will see words briefly flashed to the LEFT or RIGHT of fixation.\n" +
		"Your task is to keep your eyes on the central cross and try to remember the words.\n\n" +
		"Later, words will appear at the CENTER.\n" +
		"Press F if you have seen the word before in this experiment (OLD).\n" +
		"Press J if the word is NEW.\n\n" +
		"Try to respond as quickly and accurately as you can.\n\n" +
		"Press SPACE to start."

	instructions := stimuli.NewTextBox(instrText, 900, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
	if err := instructions.Present(exp.Screen, true, true); err != nil {
		log.Fatal(err)
	}
	for {
		key, err := exp.Keyboard.Wait()
		if err != nil && !control.IsEndLoop(err) {
			log.Fatal(err)
		}
		if key == control.K_SPACE || control.IsEndLoop(err) {
			break
		}
	}

	trialIndex := 0

	// Run each old item as a study–test pair with the specified VF and lag.
	for _, p := range pairs {
		trialIndex++
		if err := runStudy(exp, fixation, p); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatalf("study error: %v", err)
		}

		clock.Wait(p.LagMs)

		key, rt, correct, err := runTest(exp, p.Word, true)
		if err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatalf("test error: %v", err)
		}

		exp.Data.Add(
			trialIndex,
			"old",
			p.Word,
			p.VF,
			p.LagTag,
			p.LagMs,
			key,
			rt,
			correct,
		)

		// Short ITI after test.
		if err := exp.Blank(1000); err != nil {
			log.Fatal(err)
		}
	}

	// Now present new-only central words.
	for _, w := range newWords {
		trialIndex++
		key, rt, correct, err := runTest(exp, w, false)
		if err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatalf("new test error: %v", err)
		}
		exp.Data.Add(
			trialIndex,
			"new",
			w,
			"NA",
			"NA",
			0,
			key,
			rt,
			correct,
		)

		if err := exp.Blank(1000); err != nil {
			log.Fatal(err)
		}
	}
}

func runStudy(exp *control.Experiment, fixation *stimuli.FixCross, p StudyTestPair) error {
	// Fixation
	if err := exp.Show(fixation); err != nil {
		return err
	}
	clock.Wait(500)

	// Lateralized word
	x := float32(OffsetDegX)
	if p.VF == "LVF" {
		x = -OffsetDegX
	}
	wordStim := stimuli.NewTextLine(p.Word, x, 0, control.DefaultTextColor)

	if err := exp.Screen.Clear(); err != nil {
		return err
	}
	// Draw fixation and word
	if err := fixation.Draw(exp.Screen); err != nil {
		return err
	}
	if err := wordStim.Draw(exp.Screen); err != nil {
		return err
	}
	if err := exp.Screen.Update(); err != nil {
		return err
	}
	clock.Wait(StudyDurationMs)

	// ISI with fixation only
	if err := exp.Screen.Clear(); err != nil {
		return err
	}
	if err := fixation.Draw(exp.Screen); err != nil {
		return err
	}
	if err := exp.Screen.Update(); err != nil {
		return err
	}
	clock.Wait(StudyISI)

	return nil
}

func runTest(exp *control.Experiment, word string, isOld bool) (control.Keycode, int64, bool, error) {
	prompt := stimuli.NewTextLine(word, 0, -40, control.DefaultTextColor)
	instr := stimuli.NewTextLine("F = OLD   J = NEW", 0, 40, control.DefaultTextColor)

	if err := exp.Screen.Clear(); err != nil {
		return 0, 0, false, err
	}
	if err := prompt.Draw(exp.Screen); err != nil {
		return 0, 0, false, err
	}
	if err := instr.Draw(exp.Screen); err != nil {
		return 0, 0, false, err
	}
	if err := exp.Screen.Update(); err != nil {
		return 0, 0, false, err
	}

	start := clock.GetTime()
	key, err := exp.Keyboard.WaitKeys([]control.Keycode{OldKey, NewKey, control.K_ESCAPE}, -1)
	if err != nil {
		return 0, 0, false, err
	}
	rt := clock.GetTime() - start

	if key == control.K_ESCAPE {
		return key, rt, false, control.EndLoop
	}

	var correct bool
	if isOld {
		correct = (key == OldKey)
	} else {
		correct = (key == NewKey)
	}
	if !correct {
		_ = stimuli.PlayBuzzer(exp.AudioDevice)
	}
	return key, rt, correct, nil
}
