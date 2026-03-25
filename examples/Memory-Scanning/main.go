// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Sternberg memory search implements the two experiments from:
// Sternberg, S. (1966). High-speed scanning in human memory. Science, 153, 652-654.
// https://www.sas.upenn.edu/~saul/hss.html
//
// Experiment 1 (varied set): A series of 1–6 digits is shown one at a time; after a delay,
// a test digit appears. Subject responds YES (in set) or NO (not in set). RT increases
// linearly with set size (~38 ms per item), suggesting serial exhaustive scanning.
//
// Experiment 2 (fixed set): Same task but the positive set is fixed for a block (s=1, 2, or 4).
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"

	"github.com/Zyko0/go-sdl3/ttf"
)

const (
	YesKey = control.K_F // F = digit was in the memorized set
	NoKey  = control.K_J // J = digit was not in the set

	// Experiment 1 timing (Sternberg 1966)
	DigitDurationMs  = 1200 // each digit in memory set
	DelayBeforeProbe = 2000 // delay after last digit, before warning
	WarningDuration  = 500  // warning/fixation before probe
	FeedbackDuration = 800  // show correct/incorrect
	ITIExp1          = 1000 // inter-trial interval

	// Experiment 2 timing
	ISIExp2 = 3700 // 3.7 s between response and next trial (paper)
)

var digits = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

// trialExp1 represents one trial of Experiment 1 (varied set).
type trialExp1 struct {
	SetSize   int   // 1..6
	MemorySet []int // digits to memorize (order shown)
	Probe     int   // test digit
	Positive  bool  // true if probe is in set (correct response = Yes)
}

// trialExp2 represents one trial of Experiment 2 (fixed set).
type trialExp2 struct {
	PositiveSet []int // fixed for the block
	Probe       int
	Positive    bool
}

func main() {
	expNum := flag.Int("exp", 1, "Which experiment: 1 (varied set), 2 (fixed set), or 0 (both)")
	exp := control.NewExperimentFromFlags("Sternberg Memory Search", control.Black, control.White, 32)
	defer exp.End()

	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("Warning: set logical size: %v", err)
	}

	// Large font for digits (like parity_decision)
	var bigFont *ttf.Font
	if f, err := control.FontFromMemory(assets_embed.InconsolataFont, 64); err == nil {
		bigFont = f
		defer bigFont.Close()
	}

	fixation := stimuli.NewFixCross(50, 4, control.DefaultTextColor)
	blank := stimuli.NewBlankScreen(control.Black)

	// Build digit stimuli (0-9) with large font
	digitStim := make(map[int]*stimuli.TextLine)
	for _, d := range digits {
		s := stimuli.NewTextLine(fmt.Sprintf("%d", d), 0, 0, control.DefaultTextColor)
		if bigFont != nil {
			s.Font = bigFont
		}
		digitStim[d] = s
	}

	runInstructions := func(title, body string) error {
		txt := stimuli.NewTextBox(title+"\n\n"+body, 900, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
		if err := exp.Show(txt); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}
		return nil
	}

	// Wait for F or J; return key, rt (ms), error.
	// Keyboard.Clear() must be called by the caller BEFORE showing the probe,
	// not here — otherwise fast responses made just after stimulus onset are lost.
	waitYesNo := func(onsetNS uint64) (control.Keycode, int64, error) {
		key, eventTS, err := exp.Keyboard.WaitKeysEventRT([]control.Keycode{YesKey, NoKey}, -1)
		return key, int64(eventTS-onsetNS) / 1_000_000, err
	}

	showFeedback := func(correct bool) error {
		var msg string
		var c control.Color
		if correct {
			msg = "Correct"
			c = control.Green
		} else {
			msg = "Incorrect"
			c = control.Red
			// Play an error buzzer on incorrect responses during both training and main trials.
			_ = stimuli.PlayBuzzer(exp.AudioDevice)
		}
		txt := stimuli.NewTextLine(msg, 0, 0, c)
		if bigFont != nil {
			txt.Font = bigFont
		}
		if err := exp.Show(txt); err != nil {
			return err
		}
		return waitInterruption(exp, FeedbackDuration)
	}

	exp.AddDataVariableNames([]string{"experiment", "block", "set_size", "trial", "probe", "positive", "key", "rt", "correct"})

	if *expNum == 1 || *expNum == 0 {
		trainingExp1 := buildTrainingTrialsExp1()
		trialsExp1 := buildTrialsExp1()
		if err := runInstructions(
			"Sternberg Experiment 1: Varied Set",
			"You will see a series of digits (1 to 6 digits), one at a time. Memorize them.\n\nAfter a short delay, a single test digit will appear. Press F if it was IN the set you saw, J if it was NOT in the set. Respond as quickly and accurately as you can.\n\nPress SPACE to start.",
		); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatal(err)
		}
		// Training block (10 trials, no data logging).
		if err := runExp1(exp, trainingExp1, digitStim, fixation, blank, waitYesNo, showFeedback, false); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatal(err)
		}
		// Training finished screen.
		trainDone := stimuli.NewTextBox(
			"Training finished.\n\nPress a key to go on to the main experiment.",
			900,
			control.FPoint{X: 0, Y: 0},
			control.DefaultTextColor,
		)
		if err := exp.Show(trainDone); err != nil {
			log.Fatal(err)
		}
		if _, err := exp.Keyboard.Wait(); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatal(err)
		}

		// Main experimental block (data logged).
		if err := runExp1(exp, trialsExp1, digitStim, fixation, blank, waitYesNo, showFeedback, true); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatal(err)
		}
		if *expNum == 0 {
			if err := runInstructions("Break", "End of Experiment 1.\n\nPress SPACE to continue to Experiment 2."); err != nil {
				if control.IsEndLoop(err) {
					return
				}
				log.Fatal(err)
			}
		}
	}

	if *expNum == 2 || *expNum == 0 {
		trainingExp2 := buildTrainingTrialsExp2()
		blocksExp2 := buildBlocksExp2()
		if err := runInstructions(
			"Sternberg Experiment 2: Fixed Set",
			"In each block you will be told a small set of digits to remember. Then you will see a series of test digits. Press F if the digit is IN your memorized set, J if it is NOT. The set stays the same for the whole block.\n\nPress SPACE to start.",
		); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatal(err)
		}
		// Training block (10 trials, no data logging).
		if err := runExp2(exp, [][]trialExp2{trainingExp2}, digitStim, fixation, blank, waitYesNo, showFeedback, false); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatal(err)
		}
		// Training finished screen.
		trainDone := stimuli.NewTextBox(
			"Training finished.\n\nPress a key to go on to the main experiment.",
			900,
			control.FPoint{X: 0, Y: 0},
			control.DefaultTextColor,
		)
		if err := exp.Show(trainDone); err != nil {
			log.Fatal(err)
		}
		if _, err := exp.Keyboard.Wait(); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatal(err)
		}

		// Main experimental blocks (data logged).
		if err := runExp2(exp, blocksExp2, digitStim, fixation, blank, waitYesNo, showFeedback, true); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatal(err)
		}
	}

	// End screen
	if err := runInstructions("Finished", "Thank you. The experiment is complete."); err != nil && !control.IsEndLoop(err) {
		log.Fatal(err)
	}
}

// buildTrialsExp1 creates 144 main trials for Experiment 1.
// For each set size 1..6: 24 trials, 12 positive and 12 negative (equal frequency).
func buildTrialsExp1() []trialExp1 {
	var trials []trialExp1
	for setSize := 1; setSize <= 6; setSize++ {
		for pos := 0; pos < 12; pos++ {
			trials = append(trials, genTrialExp1(setSize, true))
		}
		for neg := 0; neg < 12; neg++ {
			trials = append(trials, genTrialExp1(setSize, false))
		}
	}
	design.ShuffleList(trials)
	return trials
}

// buildTrainingTrialsExp1 creates 10 training trials with random set size and membership.
func buildTrainingTrialsExp1() []trialExp1 {
	trials := make([]trialExp1, 10)
	for i := range trials {
		setSize := rand.Intn(6) + 1
		positive := rand.Float64() < 0.5
		trials[i] = genTrialExp1(setSize, positive)
	}
	return trials
}

func genTrialExp1(setSize int, positive bool) trialExp1 {
	perm := design.RandIntSequence(0, 9)
	memorySet := perm[:setSize]
	var probe int
	if positive {
		probe = design.RandElement(memorySet)
	} else {
		// pick one not in memorySet
		excl := make(map[int]bool)
		for _, d := range memorySet {
			excl[d] = true
		}
		var cand []int
		for _, d := range digits {
			if !excl[d] {
				cand = append(cand, d)
			}
		}
		probe = design.RandElement(cand)
	}
	return trialExp1{SetSize: setSize, MemorySet: memorySet, Probe: probe, Positive: positive}
}

// buildBlocksExp2 creates 3 blocks (set size 1, 2, 4) with non-overlapping positive sets.
// Each block has 120 main trials (positive/negative mixed).
func buildBlocksExp2() [][]trialExp2 {
	perm := design.RandIntSequence(0, 9)
	// Non-overlapping sets: e.g. [3], [1,7], [0,4,8] for s=1,2,4
	s1 := []int{perm[0]}
	s2 := []int{perm[1], perm[2]}
	s4 := []int{perm[3], perm[4], perm[5], perm[6]}
	blocks := [][]int{s1, s2, s4}
	var out [][]trialExp2
	for _, posSet := range blocks {
		var block []trialExp2
		for i := 0; i < 120; i++ {
			positive := rand.Float64() < 0.5
			var probe int
			if positive {
				probe = design.RandElement(posSet)
			} else {
				excl := make(map[int]bool)
				for _, d := range posSet {
					excl[d] = true
				}
				var cand []int
				for _, d := range digits {
					if !excl[d] {
						cand = append(cand, d)
					}
				}
				probe = design.RandElement(cand)
			}
			block = append(block, trialExp2{PositiveSet: posSet, Probe: probe, Positive: positive})
		}
		out = append(out, block)
	}
	return out
}

// buildTrainingTrialsExp2 creates 10 training trials for Experiment 2,
// using random positive sets of size 1, 2, or 4 and mixed positive/negative probes.
func buildTrainingTrialsExp2() []trialExp2 {
	trials := make([]trialExp2, 10)
	for i := range trials {
		// Choose set size 1, 2, or 4.
		var setSize int
		switch rand.Intn(3) {
		case 0:
			setSize = 1
		case 1:
			setSize = 2
		default:
			setSize = 4
		}
		perm := design.RandIntSequence(0, 9)
		posSet := perm[:setSize]
		positive := rand.Float64() < 0.5
		var probe int
		if positive {
			probe = design.RandElement(posSet)
		} else {
			excl := make(map[int]bool)
			for _, d := range posSet {
				excl[d] = true
			}
			var cand []int
			for _, d := range digits {
				if !excl[d] {
					cand = append(cand, d)
				}
			}
			probe = design.RandElement(cand)
		}
		trials[i] = trialExp2{PositiveSet: posSet, Probe: probe, Positive: positive}
	}
	return trials
}

func waitInterruption(exp *control.Experiment, timeout int) error {
	start := clock.GetTime()
	for {
		key, _, err := exp.HandleEvents()
		if err != nil {
			return err
		}
		if key == control.K_ESCAPE {
			return control.EndLoop
		}
		if int(clock.GetTime()-start) >= timeout {
			return nil
		}
		clock.Wait(1)
	}
}

func runExp1(exp *control.Experiment, trials []trialExp1, digitStim map[int]*stimuli.TextLine, fixation *stimuli.FixCross, blank *stimuli.BlankScreen, waitYesNo func(uint64) (control.Keycode, int64, error), showFeedback func(bool) error, logData bool) error {
	return exp.Run(func() error {
		for i, t := range trials {
			// Blank
			if err := exp.Show(blank); err != nil {
				return err
			}
			if err := waitInterruption(exp, ITIExp1); err != nil {
				return err
			}

			// Present memory set: one digit at a time, 1.2 s each
			for _, d := range t.MemorySet {
				if err := exp.Show(digitStim[d]); err != nil {
					return err
				}
				if err := waitInterruption(exp, DigitDurationMs); err != nil {
					return err
				}
			}

			// Delay 2 s then warning
			if err := exp.Show(blank); err != nil {
				return err
			}
			if err := waitInterruption(exp, DelayBeforeProbe); err != nil {
				return err
			}
			if err := exp.Show(fixation); err != nil {
				return err
			}
			if err := waitInterruption(exp, WarningDuration); err != nil {
				return err
			}

			// Probe
			exp.Keyboard.Clear() // discard any stale keys before probe onset
			onsetNS, err := exp.ShowNS(digitStim[t.Probe])
			if err != nil {
				return err
			}
			key, rt, err := waitYesNo(onsetNS)
			if err != nil {
				return err
			}
			yes := key == YesKey
			correct := yes == t.Positive
			if err := showFeedback(correct); err != nil {
				return err
			}
			if logData {
				exp.Data.Add(1, 0, t.SetSize, i, t.Probe, t.Positive, key, rt, correct)
			}
		}
		return control.EndLoop
	})
}

func runExp2(exp *control.Experiment, blocks [][]trialExp2, digitStim map[int]*stimuli.TextLine, fixation *stimuli.FixCross, blank *stimuli.BlankScreen, waitYesNo func(uint64) (control.Keycode, int64, error), showFeedback func(bool) error, logData bool) error {
	return exp.Run(func() error {
		for blockIdx, block := range blocks {
			setSize := len(block[0].PositiveSet)
			posSet := block[0].PositiveSet
			// Announce positive set
			announce := fmt.Sprintf("Memorize this set of %d digit(s): ", setSize)
			for i, d := range posSet {
				if i > 0 {
					announce += "  "
				}
				announce += fmt.Sprintf("%d", d)
			}
			announce += "\n\nYou will see test digits. Press F if in set, J if not.\n\nPress SPACE to start this block."
			announceBox := stimuli.NewTextBox(announce, 700, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
			if err := exp.Show(announceBox); err != nil {
				return err
			}
			if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
				return err
			}

			for trialIdx, t := range block {
				// Warning
				if err := exp.Show(fixation); err != nil {
					return err
				}
				if err := waitInterruption(exp, WarningDuration); err != nil {
					return err
				}

				// Test digit
				exp.Keyboard.Clear() // discard any stale keys before probe onset
				onsetNS, err := exp.ShowNS(digitStim[t.Probe])
				if err != nil {
					return err
				}
				key, rt, err := waitYesNo(onsetNS)
				if err != nil {
					return err
				}
				yes := key == YesKey
				correct := yes == t.Positive
				if err := showFeedback(correct); err != nil {
					return err
				}
				if logData {
					exp.Data.Add(2, blockIdx, setSize, trialIdx, t.Probe, t.Positive, key, rt, correct)
				}

				// 3.7 s until next trial (paper)
				if err := waitInterruption(exp, ISIExp2); err != nil {
					return err
				}
			}
		}
		return control.EndLoop
	})
}
