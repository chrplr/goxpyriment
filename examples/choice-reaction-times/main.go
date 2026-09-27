// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Choice reaction time task: a red or green square appears at the centre of
// the screen and the participant presses F (left index) or J (right index)
// according to a colour→key mapping that alternates from block to block.
//
// Timing is onset-to-onset: each square appears SOA ms after the previous
// one, whatever the response time. The square disappears at the response, or
// at the response deadline if there is none. Within every block, the 16 SOA
// values (evenly spaced over 1750–2250 ms, mean 2000 ms) are each paired once
// with red and once with green, so the SOA distribution preceding a red square
// is identical to the one preceding a green square.
package main

import (
	"fmt"
	"math"
	"runtime"
	"runtime/debug"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	NBlocks        = 8
	NSOAs          = 16 // distinct SOA values; each is used once per colour
	TrialsPerBlock = 2 * NSOAs
	SOAMin         = 1750 // ms
	SOAMax         = 2250 // ms
	DeadlineMS     = 1500 // response deadline; DeadlineMS+FeedbackMS must be < SOAMin
	FeedbackMS     = 200  // duration of the feedback frame
	SquareSize     = 200  // pixels
	FrameWidth     = 6    // pixels, thickness of the feedback frame
	CrossSize      = 30   // pixels
	CrossWidth     = 4    // pixels
)

// newFrame returns the four bars of an outline hugging the square's edge.
func newFrame(color control.Color) []stimuli.VisualStimulus {
	const off = (SquareSize + FrameWidth) / 2 // centre of each bar
	const long = SquareSize + 2*FrameWidth
	return []stimuli.VisualStimulus{
		stimuli.NewRectangle(0, off, long, FrameWidth, color),
		stimuli.NewRectangle(0, -off, long, FrameWidth, color),
		stimuli.NewRectangle(-off, 0, FrameWidth, long, color),
		stimuli.NewRectangle(off, 0, FrameWidth, long, color),
	}
}

// mapping gives the key assigned to each colour.
type mapping struct {
	name     string
	redKey   control.Keycode
	greenKey control.Keycode
}

var (
	mappingA = mapping{"red-F_green-J", control.K_F, control.K_J}
	mappingB = mapping{"red-J_green-F", control.K_J, control.K_F}
)

type trial struct {
	color string // "red" or "green"
	soa   int    // planned onset-to-onset interval preceding this trial, ms
}

func keyName(k control.Keycode) string {
	switch k {
	case control.K_F:
		return "F"
	case control.K_J:
		return "J"
	}
	return "none"
}

// soaValues returns NSOAs values evenly spaced over [SOAMin, SOAMax].
func soaValues() []int {
	v := make([]int, NSOAs)
	for i := range v {
		v[i] = SOAMin + (i*(SOAMax-SOAMin)+(NSOAs-1)/2)/(NSOAs-1)
	}
	return v
}

// blockTrials pairs every SOA value once with each colour, then shuffles.
func blockTrials(soas []int) []trial {
	trials := make([]trial, 0, TrialsPerBlock)
	for _, s := range soas {
		trials = append(trials, trial{"red", s}, trial{"green", s})
	}
	design.ShuffleList(trials)
	return trials
}

func instructions(block int, m mapping) string {
	left, right := "RED", "GREEN"
	if m.redKey == control.K_J {
		left, right = "GREEN", "RED"
	}
	return fmt.Sprintf("Block %d of %d\n\n"+
		"%s square: press F (left index finger)\n"+
		"%s square: press J (right index finger)\n\n"+
		"Press SPACE to start.", block, NBlocks, left, right)
}

const introText = "Welcome!\n\n" +
	"On each trial, a red or a green square appears at the centre of the screen.\n" +
	"Your task is to press a key according to its colour:\n" +
	"F with your left index finger, or J with your right index finger.\n\n" +
	"The experiment has 8 blocks of 32 trials.\n" +
	"IMPORTANT: the colour-key mapping switches from one block to the next.\n" +
	"Read the instructions at the start of each block carefully.\n\n" +
	"After each response, a frame briefly appears where the square was:\n" +
	"WHITE if your response was correct, BLACK if it was wrong or too slow.\n\n" +
	"Keep your eyes on the central cross and respond\n" +
	"as quickly and as accurately as possible.\n\n" +
	"Press SPACE to continue."

// keyStats accumulates, for one response key, the number of trials on which
// it was the correct key and the RTs of the correct responses.
type keyStats struct {
	trials int
	rts    []float64 // ms, correct responses only
}

// summary returns the hit rate (%), mean RT and its standard error (ms).
func (k *keyStats) summary() (hitRate, mean, se float64) {
	n := float64(len(k.rts))
	if k.trials > 0 {
		hitRate = 100 * n / float64(k.trials)
	}
	if n == 0 {
		return hitRate, math.NaN(), math.NaN()
	}
	for _, x := range k.rts {
		mean += x
	}
	mean /= n
	if n < 2 {
		return hitRate, mean, math.NaN()
	}
	var ss float64
	for _, x := range k.rts {
		ss += (x - mean) * (x - mean)
	}
	se = math.Sqrt(ss/(n-1)) / math.Sqrt(n)
	return hitRate, mean, se
}

func main() {
	// Mid-grey background keeps the luminance change at square onset small.
	exp := control.NewExperimentFromFlags("Choice Reaction Times", control.Gray, control.Black, 32)
	defer exp.End()

	exp.AddDataVariableNames([]string{"block", "trial", "mapping", "color", "soa_planned", "soa_actual",
		"correct_key", "response", "rt", "correct"})

	red := stimuli.NewRectangle(0, 0, SquareSize, SquareSize, control.Red)
	// Pure red (255,0,0) and this green have the luminance of the mid-grey
	// background under the nominal sRGB curve (Y ≈ 0.21); pure green would be
	// about three times brighter. Not photometrically verified.
	green := stimuli.NewRectangle(0, 0, SquareSize, SquareSize, control.RGB(0, 149, 0))
	cross := stimuli.NewFixCross(CrossSize, CrossWidth, control.Black)
	responseKeys := []control.Keycode{control.K_F, control.K_J}

	whiteFrame := newFrame(control.White) // correct response
	blackFrame := newFrame(control.Black) // wrong key or no response

	// present draws stims under the permanent fixation cross and flips,
	// returning the flip timestamp.
	present := func(stims ...stimuli.VisualStimulus) (uint64, error) {
		if err := exp.Screen.Clear(); err != nil {
			return 0, err
		}
		for _, st := range stims {
			if err := st.Draw(exp.Screen); err != nil {
				return 0, err
			}
		}
		if err := cross.Draw(exp.Screen); err != nil {
			return 0, err
		}
		return exp.Screen.FlipTS()
	}
	soas := soaValues()

	// Counterbalance the starting mapping by subject ID; mappings alternate.
	first, second := mappingA, mappingB
	if exp.SubjectID%2 == 0 {
		first, second = mappingB, mappingA
	}

	// Aim each onset half a frame early: the flip lands on the next vblank,
	// so the actual onset is centred on the planned one.
	halfFrameNS := uint64(exp.Screen.FrameDuration().Nanoseconds() / 2)

	// waitUntil keeps the screen and event queue alive (and honours ESC)
	// until the SDL clock reaches t.
	waitUntil := func(t uint64) {
		for control.TicksNS()+halfFrameNS < t {
			exp.Wait(1)
		}
	}

	stats := map[control.Keycode]*keyStats{control.K_F: {}, control.K_J: {}}

	err := exp.Run(func() error {
		if err := exp.ShowInstructions(introText); err != nil {
			return err
		}
		for b := 0; b < NBlocks; b++ {
			m := first
			if b%2 == 1 {
				m = second
			}
			if err := exp.ShowInstructions(instructions(b+1, m)); err != nil {
				return err
			}

			// The fixation-only screen after the instructions is the reference
			// onset for the first trial's SOA.
			prevOnset, err := present()
			if err != nil {
				return err
			}

			nCorrect := 0
			debug.SetGCPercent(-1)
			for i, t := range blockTrials(soas) {
				var stim stimuli.VisualStimulus = red
				correctKey := m.redKey
				if t.color == "green" {
					stim, correctKey = green, m.greenKey
				}

				waitUntil(prevOnset + uint64(t.soa)*1_000_000)
				exp.Keyboard.Clear() // discard anticipations
				onset, err := present(stim)
				if err != nil {
					debug.SetGCPercent(100)
					return err
				}
				key, keyTS, err := exp.Keyboard.GetKeyEventTS(responseKeys, DeadlineMS)
				if err != nil {
					debug.SetGCPercent(100)
					return err
				}

				// Feedback: the square is replaced by a frame at its edge.
				correct := key == correctKey
				frame := blackFrame
				if correct {
					nCorrect++
					frame = whiteFrame
				}
				if _, err := present(frame...); err != nil {
					debug.SetGCPercent(100)
					return err
				}
				exp.Wait(FeedbackMS)
				if _, err := present(); err != nil {
					debug.SetGCPercent(100)
					return err
				}

				soaActual := math.Round(float64(onset-prevOnset)/1e5) / 10 // ms, 0.1 ms resolution
				prevOnset = onset
				var rt any = "NA"
				if key != 0 {
					rt = int64(keyTS-onset) / 1_000_000
				}
				ks := stats[correctKey]
				ks.trials++
				if correct {
					ks.rts = append(ks.rts, float64(keyTS-onset)/1e6)
				}
				exp.Data.Add(b+1, i+1, m.name, t.color, t.soa, soaActual,
					keyName(correctKey), keyName(key), rt, correct)
			}
			debug.SetGCPercent(100)
			runtime.GC()

			exp.Wait(1000)
			msg := fmt.Sprintf("End of block %d of %d\n\nCorrect responses: %d / %d\n\n",
				b+1, NBlocks, nCorrect, TrialsPerBlock)
			if b < NBlocks-1 {
				msg += "Take a short break.\n\nPress SPACE to continue."
			} else {
				msg += "Press SPACE to see your results."
			}
			if err := exp.ShowInstructions(msg); err != nil {
				return err
			}
		}

		msg := "The experiment is over. Thank you!\n\nCorrect responses:\n\n"
		for _, k := range []struct {
			label string
			key   control.Keycode
		}{{"Left key (F)", control.K_F}, {"Right key (J)", control.K_J}} {
			hit, mean, se := stats[k.key].summary()
			msg += fmt.Sprintf("%s:  hit rate %.1f%%,  mean RT %.0f ms (SE %.1f ms)\n", k.label, hit, mean, se)
		}
		msg += "\nPress SPACE to exit."
		if err := exp.ShowInstructions(msg); err != nil {
			return err
		}
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
