// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Replication of Experiment 1 from:
//   Planton et al. (2021). A theory of memory for binary sequences:
//   Evidence for a mental compression algorithm in humans.
//   PLoS Computational Biology, 17(1), e1008598.
//
// Run: go run . [-d] [-s <subject_id>]
//
// The experiment has two parts:
//  1. Complexity rating: judge each sequence on a 1–9 scale (30 trials).
//  2. Violation detection: press SPACE whenever the sequence is altered (10 sessions).

package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Timing constants ──────────────────────────────────────────────────────────

const (
	toneDurMs  = 50  // tone on-time (ms)
	toneRampMs = 5   // linear ramp at onset/offset (ms)
	toneISIMs  = 200 // inter-stimulus interval after each tone (ms)
	seqITIMs   = 600 // inter-trial interval between sequence repetitions (ms)

	// Valid response window in the violation-detection task (ms after deviant onset)
	respMinMs = 200
	respMaxMs = 2500
)

// ── Frequency definitions (Hz) ────────────────────────────────────────────────
// Each complex tone is the sum of four sine waves.

var (
	freqLow   = []float64{494, 740, 988, 1480}   // musical notes B, F#, B, F#
	freqHigh  = []float64{622, 932, 1245, 1865}  // musical notes D#, Bb, D#, Bb
	freqCLow  = []float64{415, 622, 831, 1245}   // super-deviant C (lower)
	freqCHigh = []float64{740, 1109, 1480, 2217} // super-deviant C (higher)
)

// Tone-slot indices into the [4]*stimuli.Tone array.
const (
	idxLow   = 0
	idxHigh  = 1
	idxCLow  = 2
	idxCHigh = 3
)

// ── Sequence definitions ──────────────────────────────────────────────────────

type seqDef struct {
	id            int
	items         [16]byte // 0 = item A, 1 = item B
	lotComplexity int
}

// Ten 16-item sequences from Planton et al. (2021), Experiment 1, Fig 2.
// All sequences contain exactly 8 As and 8 Bs.
//
// Sequences 1–4 are the four canonical (AnBn)× patterns (LoT complexity 6).
// Sequence 5 is the example from Fig 1 (LoT complexity 12; AABBABABAABBABAB).
// Sequences 6–10 are PLACEHOLDERS – verify and replace with values from the
// paper's supplementary materials before use in a real study.
var seqDefs = []seqDef{
	{0, [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1}, 6},  // A8B8
	{1, [16]byte{0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 1, 1, 1, 1}, 6},  // (A4B4)×2
	{2, [16]byte{0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1}, 6},  // (A2B2)×4
	{3, [16]byte{0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1}, 6},  // (AB)×8
	{4, [16]byte{0, 0, 1, 1, 0, 1, 0, 1, 0, 0, 1, 1, 0, 1, 0, 1}, 12}, // AABBABABAABBABAB (Fig 1)
	// TODO: replace placeholders below with the actual sequences from the supplement
	{5, [16]byte{0, 1, 0, 1, 0, 0, 1, 1, 1, 0, 1, 0, 0, 1, 1, 0}, 13},
	{6, [16]byte{0, 0, 0, 0, 1, 1, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1}, 14},
	{7, [16]byte{0, 0, 0, 1, 0, 1, 1, 1, 0, 0, 0, 1, 0, 1, 1, 1}, 15},
	{8, [16]byte{0, 0, 1, 0, 1, 0, 1, 0, 1, 1, 1, 0, 1, 0, 1, 0}, 17},
	{9, [16]byte{0, 1, 0, 0, 0, 1, 1, 1, 1, 1, 0, 1, 0, 0, 0, 1}, 23},
}

// Deviant positions (0-indexed) in the second half of the sequence.
// Correspond to 1-indexed positions 9, 11, 13, 15.
var deviantPositions = []int{8, 10, 12, 14}

// Pre-computed onset of each tone in ms from the sequence start.
// Onset[i] = i × (toneDurMs + toneISIMs).
var toneOnsetMs [16]int

func init() {
	for i := range toneOnsetMs {
		toneOnsetMs[i] = i * (toneDurMs + toneISIMs)
	}
}

// ── Trial type ────────────────────────────────────────────────────────────────

type trialKind int

const (
	kindStandard     trialKind = 0
	kindSeqDeviant   trialKind = 1
	kindSuperDeviant trialKind = 2
)

func (k trialKind) String() string {
	switch k {
	case kindStandard:
		return "standard"
	case kindSeqDeviant:
		return "seq_deviant"
	case kindSuperDeviant:
		return "super_deviant"
	default:
		return "unknown"
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// buildSounds returns a []SoundStreamElement for one sequence presentation,
// including a trailing silent element for the ITI so that late responses
// share the same timestamp base as the sequence itself.
//
// aIsLow: true → A=low-pitch, B=high-pitch; false → reversed.
// deviantIdx: -1 = no deviant; 0–15 = 0-indexed position of the deviant tone.
func buildSounds(seq seqDef, tones [4]*stimuli.Tone, aIsLow bool, deviantIdx int, kind trialKind) []stimuli.SoundStreamElement {
	sounds := make([]stimuli.AudioPlayable, 16)
	onsetMs := make([]int, 16)
	durationMs := make([]int, 16)

	for i := range seq.items {
		item := seq.items[i]
		onsetMs[i] = i * (toneDurMs + toneISIMs)
		durationMs[i] = toneDurMs

		var t *stimuli.Tone
		if deviantIdx == i && kind == kindSuperDeviant {
			if rand.Intn(2) == 0 {
				t = tones[idxCLow]
			} else {
				t = tones[idxCHigh]
			}
		} else {
			if deviantIdx == i && kind == kindSeqDeviant {
				item = 1 - item // flip A ↔ B
			}
			if aIsLow {
				if item == 0 {
					t = tones[idxLow]
				} else {
					t = tones[idxHigh]
				}
			} else {
				if item == 0 {
					t = tones[idxHigh]
				} else {
					t = tones[idxLow]
				}
			}
		}
		sounds[i] = t
	}

	elements, _ := stimuli.MakeSoundStream(sounds, onsetMs, durationMs)

	// Silent ITI element: late responses are captured in the same stream call.
	elements = append(elements, stimuli.SoundStreamElement{
		DurationOff: time.Duration(seqITIMs) * time.Millisecond,
	})
	return elements
}

// showText displays a text box and returns immediately (no wait).
func showText(exp *control.Experiment, msg string) error {
	txt := stimuli.NewTextBox(msg, 900, control.Origin(), control.White)
	return exp.Show(txt)
}

// showAndWaitSpace displays a message and waits for the SPACE key.
func showAndWaitSpace(exp *control.Experiment, msg string) error {
	if err := showText(exp, msg); err != nil {
		return err
	}
	return exp.Keyboard.WaitKey(control.K_SPACE)
}

// firstSpaceInWindow returns the RT in ms if SPACE was pressed within
// [minMs, maxMs] ms after deviantOnsetMs, otherwise -1.
func firstSpaceInWindow(events []stimuli.UserEvent, deviantOnsetMs, minMs, maxMs int) int64 {
	origin := time.Duration(deviantOnsetMs) * time.Millisecond
	for _, ev := range events {
		if ev.Event.Type == sdl.EVENT_KEY_DOWN &&
			ev.Event.KeyboardEvent().Key == sdl.K_SPACE {
			delta := (ev.Timestamp - origin).Milliseconds()
			if delta >= int64(minMs) && delta <= int64(maxMs) {
				return delta
			}
		}
	}
	return -1
}

// anySpace returns true if SPACE was pressed at any point in events.
func anySpace(events []stimuli.UserEvent) bool {
	for _, ev := range events {
		if ev.Event.Type == sdl.EVENT_KEY_DOWN &&
			ev.Event.KeyboardEvent().Key == sdl.K_SPACE {
			return true
		}
	}
	return false
}

// playFixation shows a fixation cross, then plays the element sequence.
func playFixation(exp *control.Experiment) error {
	return showText(exp, "+")
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	exp := control.NewExperimentFromFlags("Binary Sequences (Planton 2021)", control.Black, control.White, 32)
	defer exp.End()

	// Pre-synthesise and preload all four complex tones.
	allFreqs := [][]float64{freqLow, freqHigh, freqCLow, freqCHigh}
	var tones [4]*stimuli.Tone
	for i, freqs := range allFreqs {
		t := stimuli.NewComplexTone(freqs, toneDurMs, toneRampMs, 0.5)
		if err := t.PreloadDevice(exp.AudioDevice); err != nil {
			log.Fatalf("failed to preload tone %d: %v", i, err)
		}
		tones[i] = t
	}

	// Data file columns (subject ID is prepended automatically).
	exp.AddDataVariableNames([]string{
		"part", "session", "trial", "seq_id", "lot_complexity",
		"a_is_low", "deviant_present", "deviant_pos", "deviant_type",
		"rating", "hit", "fa", "rt_ms",
	})

	err := exp.Run(func() error {

		// ── Welcome ───────────────────────────────────────────────────────────
		if err := showAndWaitSpace(exp,
			"Welcome to the Binary Sequences Experiment.\n\n"+
				"This experiment has two parts:\n"+
				"  Part 1 — Complexity Rating (30 trials)\n"+
				"  Part 2 — Violation Detection (10 sessions)\n\n"+
				"Press SPACE to read the instructions for Part 1."); err != nil {
			return err
		}

		// ── Part 1: Complexity Rating ─────────────────────────────────────────
		if err := showAndWaitSpace(exp,
			"PART 1: COMPLEXITY RATING\n\n"+
				"You will hear sequences of two different beeps.\n"+
				"After each sequence, rate its complexity:\n"+
				"  1 = very simple     9 = very complex\n\n"+
				"Press SPACE to hear two brief examples, then start."); err != nil {
			return err
		}

		// Play the two example sequences mentioned in the paper.
		examples := []struct {
			label string
			items [16]byte
		}{
			{"rather simple (AABBAABBAABBAABB)", [16]byte{0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1}},
			{"rather complex (ABAAABABAABBBABB+B)", [16]byte{0, 1, 0, 0, 0, 1, 0, 1, 0, 0, 1, 1, 1, 0, 1, 1}},
		}
		for _, ex := range examples {
			if err := showAndWaitSpace(exp,
				fmt.Sprintf("Example — %s\n\nPress SPACE to listen.", ex.label)); err != nil {
				return err
			}
			if err := playFixation(exp); err != nil {
				return err
			}
			seq := seqDef{items: ex.items}
			elems := buildSounds(seq, tones, rand.Intn(2) == 0, -1, kindStandard)
			evs, _, err := stimuli.PlayStreamOfSounds(elems)
			if err != nil {
				return err
			}
			_ = evs
		}

		if err := showAndWaitSpace(exp,
			"Now the real trials begin.\n\n"+
				"After each sequence, press 1–9 to rate its complexity.\n"+
				"Press SPACE to start."); err != nil {
			return err
		}

		// Build 30-trial list: 10 sequences × 3 repetitions, shuffled.
		ratingTrials := make([]int, 30)
		for i := range ratingTrials {
			ratingTrials[i] = i / 3
		}
		rand.Shuffle(len(ratingTrials), func(i, j int) {
			ratingTrials[i], ratingTrials[j] = ratingTrials[j], ratingTrials[i]
		})

		ratingKeys := []sdl.Keycode{
			sdl.K_1, sdl.K_2, sdl.K_3, sdl.K_4, sdl.K_5,
			sdl.K_6, sdl.K_7, sdl.K_8, sdl.K_9,
		}

		for trial, seqIdx := range ratingTrials {
			seq := seqDefs[seqIdx]
			aIsLow := rand.Intn(2) == 0

			if err := playFixation(exp); err != nil {
				return err
			}
			elems := buildSounds(seq, tones, aIsLow, -1, kindStandard)
			evs, _, err := stimuli.PlayStreamOfSounds(elems)
			if err != nil {
				return err
			}
			_ = evs

			// Rating prompt
			if err := showText(exp, "Complexity?  1 (very simple)  ·····  9 (very complex)"); err != nil {
				return err
			}
			t0 := time.Now()
			key, err := exp.Keyboard.WaitKeys(ratingKeys, 10_000)
			if err != nil {
				return err
			}
			rtMs := time.Since(t0).Milliseconds()
			rating := 0
			if key >= sdl.K_1 && key <= sdl.K_9 {
				rating = int(key-sdl.K_1) + 1
			}

			aIsLowInt := 0
			if aIsLow {
				aIsLowInt = 1
			}
			exp.Data.Add(
				"rating", "-", trial+1, seq.id, seq.lotComplexity,
				aIsLowInt, 0, -1, "none",
				rating, "-", "-", rtMs,
			)

			if err := exp.Blank(500); err != nil {
				return err
			}
		}

		// ── Part 2: Violation Detection ───────────────────────────────────────
		if err := showAndWaitSpace(exp,
			"PART 2: VIOLATION DETECTION\n\n"+
				"You will hear a sequence repeated several times.\n"+
				"First, listen and memorise the sequence.\n"+
				"Then PRESS SPACE as quickly as possible whenever you hear a change.\n\n"+
				"There will be 10 short sessions (one sequence each).\n\n"+
				"Press SPACE to start."); err != nil {
			return err
		}

		sessionOrder := rand.Perm(len(seqDefs))

		for sessionNum, seqIdx := range sessionOrder {
			seq := seqDefs[seqIdx]
			aIsLow := rand.Intn(2) == 0

			if err := showAndWaitSpace(exp,
				fmt.Sprintf("Session %d of %d\n\n"+
					"Listen carefully to the sequence and try to memorise it.\n\n"+
					"Press SPACE to begin.",
					sessionNum+1, len(seqDefs))); err != nil {
				return err
			}

			// ── Block 1: Habituation (8 repetitions, no scoring) ─────────────
			if err := showText(exp, "LISTEN AND MEMORISE\n\n+"); err != nil {
				return err
			}
			for rep := 0; rep < 8; rep++ {
				elems := buildSounds(seq, tones, aIsLow, -1, kindStandard)
				evs, _, err := stimuli.PlayStreamOfSounds(elems)
				if err != nil {
					return err
				}
				_ = evs
			}

			// ── Blocks 2 & 3: Testing ─────────────────────────────────────────
			for block := 0; block < 2; block++ {
				var intro string
				if block == 0 {
					intro = "Now PRESS SPACE whenever you detect a change in the sequence.\n" +
						"Respond as quickly as possible — you don't need to wait for the sequence to end.\n\n" +
						"Press SPACE to start."
				} else {
					intro = "Brief pause.\n\nPress SPACE to continue."
				}
				if err := showAndWaitSpace(exp, intro); err != nil {
					return err
				}

				// 18 trials per block: 9 standard, 6 seq-deviant, 3 super-deviant.
				blockTrials := make([]trialKind, 0, 18)
				for i := 0; i < 9; i++ {
					blockTrials = append(blockTrials, kindStandard)
				}
				for i := 0; i < 6; i++ {
					blockTrials = append(blockTrials, kindSeqDeviant)
				}
				for i := 0; i < 3; i++ {
					blockTrials = append(blockTrials, kindSuperDeviant)
				}
				rand.Shuffle(len(blockTrials), func(i, j int) {
					blockTrials[i], blockTrials[j] = blockTrials[j], blockTrials[i]
				})

				if err := playFixation(exp); err != nil {
					return err
				}

				for trialNum, kind := range blockTrials {
					deviantIdx := -1
					if kind != kindStandard {
						deviantIdx = deviantPositions[rand.Intn(len(deviantPositions))]
					}

					elems := buildSounds(seq, tones, aIsLow, deviantIdx, kind)
					events, _, err := stimuli.PlayStreamOfSounds(elems)
					if err != nil {
						return err
					}

					// Score
					hit, fa := 0, 0
					rtMs := int64(-1)
					if kind != kindStandard {
						rt := firstSpaceInWindow(events, toneOnsetMs[deviantIdx], respMinMs, respMaxMs)
						if rt >= 0 {
							hit = 1
							rtMs = rt
						}
					} else {
						if anySpace(events) {
							fa = 1
						}
					}

					deviantPos1idx := -1
					if deviantIdx >= 0 {
						deviantPos1idx = deviantIdx + 1 // 1-indexed
					}
					deviantPresent := 0
					if kind != kindStandard {
						deviantPresent = 1
					}
					aIsLowInt := 0
					if aIsLow {
						aIsLowInt = 1
					}
					exp.Data.Add(
						"detection", sessionNum+1, trialNum+1, seq.id, seq.lotComplexity,
						aIsLowInt, deviantPresent, deviantPos1idx, kind.String(),
						"-", hit, fa, rtMs,
					)
				}
			}
		}

		return showText(exp,
			"The experiment is complete. Thank you!\n\nPress SPACE to exit.")
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
