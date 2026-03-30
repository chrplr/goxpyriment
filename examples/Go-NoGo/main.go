// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Stop-Signal Task — Experiment 1 from Logan, Cowan & Davis (1984).
//
// Participants perform simple and choice reaction-time tasks.
// On 20 % of trials a 900 Hz stop-signal tone is played at one of four
// fixed delays after letter onset; the participant must try to withhold
// their response when they hear it.
//
// Simple RT (delays 50 / 100 / 150 / 200 ms):
//
//	Press SPACE for every letter (E, F, H, L).
//
// Choice RT (delays 100 / 200 / 300 / 400 ms):
//
//	Press F or J depending on which letter appeared (mapping set in dialog).
//
// Session: 8 blocks × 80 trials (4 simple + 4 choice, order counterbalanced
// by subject-ID parity).  Each block has 64 go-trials and 16 stop-signal
// trials (4 delays × 4 letters × 1 rep).
//
// Key dependent variables (computable from the saved CSV):
//
//	P(inhibit) as a function of stop-signal delay
//	Mean RT on no-signal (go) trials
//	Mean RT on signal-respond trials (stop signal present but responded)
//	Estimated stop-signal RT (SSRT) via the horse-race model
//
// Reference:
//
//	Logan, G. D., Cowan, W. B., & Davis, K. A. (1984). On the ability to
//	inhibit simple and choice reaction time responses: A model and a method.
//	Journal of Experimental Psychology: Human Perception and Performance,
//	10(2), 276–291.  https://doi.org/10.1037/0096-1523.10.2.276
//
// Usage:
//
//	go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Timing (ms), matching the paper exactly.
const (
	fixMS  = 500  // fixation warning interval
	maxRTms = 1000 // response window from letter onset (letter shown for 500 ms in
	//               the original; we keep it visible until response or 1000 ms)
	itiMS  = 2500 // blank inter-trial interval after letter offset

	nBlocks        = 8  // 4 simple + 4 choice
	trialsPerBlock = 80
	nGoPerBlock    = 64 // 80 % go trials  (16 per letter)
	nStopPerBlock  = 16 // 20 % stop trials (4 delays × 4 letters)

	stopToneFreq  = 900.0 // Hz
	stopToneDurMS = 500   // ms
	stopToneAmp   = 0.3   // comfortable level
)

var (
	taskLetters  = []string{"E", "F", "H", "L"}
	simpleDelays = []int{50, 100, 150, 200}  // ms — simple RT
	choiceDelays = []int{100, 200, 300, 400} // ms — choice RT
)

// ── Choice-task letter-to-key mapping ────────────────────────────────────────

type choiceMap struct {
	fGroup [2]string // letters assigned to the F key
	jGroup [2]string // letters assigned to the J key
}

func (m choiceMap) keyFor(letter string) control.Keycode {
	if letter == m.fGroup[0] || letter == m.fGroup[1] {
		return control.K_F
	}
	return control.K_J
}

func (m choiceMap) label() string {
	return fmt.Sprintf("%s, %s  →  F       %s, %s  →  J",
		m.fGroup[0], m.fGroup[1], m.jGroup[0], m.jGroup[1])
}

// All 6 balanced partitions of {E,F,H,L} into two pairs.
var allMappings = []choiceMap{
	{[2]string{"E", "F"}, [2]string{"H", "L"}},
	{[2]string{"E", "H"}, [2]string{"F", "L"}},
	{[2]string{"E", "L"}, [2]string{"F", "H"}},
	{[2]string{"H", "L"}, [2]string{"E", "F"}},
	{[2]string{"F", "L"}, [2]string{"E", "H"}},
	{[2]string{"F", "H"}, [2]string{"E", "L"}},
}

// ── Trial ─────────────────────────────────────────────────────────────────────

type trial struct {
	letter    string
	isStop    bool
	stopDelay int // ms; 0 on go trials
}

// generateBlock returns 80 shuffled trials.
// Go trials: 16 per letter (64 total).
// Stop trials: 1 per (letter × delay) combination (16 total).
func generateBlock(delays []int) []trial {
	var trials []trial
	goPerLetter := nGoPerBlock / len(taskLetters)
	for _, l := range taskLetters {
		for i := 0; i < goPerLetter; i++ {
			trials = append(trials, trial{letter: l})
		}
	}
	for _, l := range taskLetters {
		for _, d := range delays {
			trials = append(trials, trial{letter: l, isStop: true, stopDelay: d})
		}
	}
	design.ShuffleList(trials)
	return trials
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	// ── Setup dialog ─────────────────────────────────────────────────────────
	mappingOpts := make([]string, len(allMappings))
	for i, m := range allMappings {
		mappingOpts[i] = m.label()
	}

	fields := []control.InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		{
			Name:    "choice_mapping",
			Label:   "Choice task — letter-to-key mapping",
			Type:    control.FieldSelect,
			Options: mappingOpts,
		},
		control.FullscreenField,
	}

	info, err := control.GetParticipantInfo(
		"Stop-Signal Task — Logan, Cowan & Davis (1984)", fields)
	if errors.Is(err, control.ErrCancelled) {
		log.Fatal("Setup cancelled.")
	}
	if err != nil {
		log.Fatalf("GetParticipantInfo: %v", err)
	}

	// Resolve choice mapping
	cmap := allMappings[0]
	for i, m := range allMappings {
		if m.label() == info["choice_mapping"] {
			cmap = allMappings[i]
			break
		}
	}

	// Block order: odd subject ID → simple first; even → choice first.
	subjectID, _ := strconv.Atoi(info["subject_id"])
	simpleFirst := subjectID%2 == 1

	// ── Experiment initialisation ─────────────────────────────────────────────
	fullscreen := info["fullscreen"] == "true"
	winW, winH := 0, 0
	if !fullscreen {
		winW, winH = 1024, 768
	}
	exp := control.NewExperiment("Stop-Signal-Logan1984", winW, winH, fullscreen,
		control.Black, control.White, 32)
	if initErr := exp.Initialize(); initErr != nil {
		log.Fatalf("Initialize: %v", initErr)
	}
	defer exp.End()
	exp.Info = info

	if err := exp.SetLogicalSize(1920, 1080); err != nil {
		log.Printf("Warning: SetLogicalSize: %v", err)
	}

	// ── Stop-signal tone ──────────────────────────────────────────────────────
	stopTone := stimuli.NewTone(stopToneFreq, stopToneDurMS, stopToneAmp)
	if toneErr := stopTone.PreloadDevice(exp.AudioDevice); toneErr != nil {
		log.Printf("Warning: could not preload stop tone: %v", toneErr)
	}

	// ── Visual stimuli ────────────────────────────────────────────────────────
	letterFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 100)
	if err != nil {
		log.Fatalf("letter font: %v", err)
	}
	defer letterFont.Close()

	letterStims := make(map[string]*stimuli.TextLine, len(taskLetters))
	for _, l := range taskLetters {
		s := stimuli.NewTextLine(l, 0, 0, control.White)
		s.Font = letterFont
		letterStims[l] = s
	}

	fixation := stimuli.NewFixCross(16, 2, control.White)

	// ── Data columns ──────────────────────────────────────────────────────────
	exp.AddDataVariableNames([]string{
		"trial", "block", "task",
		"letter", "stop_signal", "stop_delay_ms",
		"response_key", "rt_ms", "correct", "inhibited",
	})

	// ── Instruction strings ───────────────────────────────────────────────────
	simpleInstr :=
		"SIMPLE REACTION TIME\n\n" +
			"A letter (E, F, H, or L) will appear in the centre of the screen.\n" +
			"Press SPACE as quickly as possible for every letter.\n\n" +
			"STOP SIGNAL\n" +
			"On some trials you will hear a high-pitched tone shortly after\n" +
			"the letter appears.  When you hear it, try to STOP yourself\n" +
			"from pressing SPACE.  You will not always succeed — that is\n" +
			"expected and perfectly normal.\n\n" +
			"Respond as fast as possible on all other trials.\n\n" +
			"Place your right index finger on SPACE, then press SPACE to start."

	choiceInstr := fmt.Sprintf(
		"CHOICE REACTION TIME\n\n"+
			"A letter will appear in the centre of the screen.\n"+
			"Press the key that matches the letter:\n\n"+
			"    F  →  %s   %s\n"+
			"    J  →  %s   %s\n\n"+
			"STOP SIGNAL\n"+
			"On some trials you will hear a high-pitched tone shortly after\n"+
			"the letter appears.  When you hear it, try to STOP yourself\n"+
			"from pressing.  You will not always succeed — that is normal.\n\n"+
			"Respond as fast as possible on all other trials.\n\n"+
			"Place your index finger on F and middle finger on J, then\n"+
			"press SPACE to start.",
		cmap.fGroup[0], cmap.fGroup[1],
		cmap.jGroup[0], cmap.jGroup[1])

	// ── Build block order ─────────────────────────────────────────────────────
	type blockSpec struct {
		task   string
		delays []int
	}
	var blockOrder []blockSpec
	for i := 0; i < 4; i++ {
		if simpleFirst {
			blockOrder = append(blockOrder, blockSpec{"simple", simpleDelays})
		} else {
			blockOrder = append(blockOrder, blockSpec{"choice", choiceDelays})
		}
	}
	for i := 0; i < 4; i++ {
		if simpleFirst {
			blockOrder = append(blockOrder, blockSpec{"choice", choiceDelays})
		} else {
			blockOrder = append(blockOrder, blockSpec{"simple", simpleDelays})
		}
	}

	// ── Experiment loop ───────────────────────────────────────────────────────
	runErr := exp.Run(func() error {
		exp.ShowInstructions(
			"Welcome to the Stop-Signal Task\n\n" +
				"You will perform two types of letter-identification tasks.\n" +
				"On some trials a TONE will sound — try to stop your response.\n\n" +
				"Press SPACE to read the instructions for the first task.")

		trialNum := 0
		prevTask := ""

		for blockIdx, spec := range blockOrder {
			// Show full instructions on the first block of each task type;
			// show a brief break screen for subsequent blocks of the same task.
			if spec.task != prevTask {
				if spec.task == "simple" {
					exp.ShowInstructions(simpleInstr)
				} else {
					exp.ShowInstructions(choiceInstr)
				}
				prevTask = spec.task
			} else {
				exp.ShowInstructions(fmt.Sprintf(
					"Block %d of %d  —  %s RT\n\n"+
						"Same task, same rules.\n\n"+
						"Press SPACE to continue.",
					blockIdx+1, nBlocks, spec.task))
			}

			trials := generateBlock(spec.delays)

			for _, t := range trials {
				trialNum++

				// ── Fixation (500 ms warning dot) ─────────────────────────────
				_ = exp.Screen.Clear()
				_ = fixation.Draw(exp.Screen)
				_ = exp.Screen.Update()
				exp.Wait(fixMS)

				// ── Letter onset ──────────────────────────────────────────────
				_ = exp.Screen.Clear()
				_ = letterStims[t.letter].Draw(exp.Screen)
				onsetNS, flipErr := exp.Screen.FlipNS()
				if flipErr != nil {
					return flipErr
				}

				// ── Stop-signal goroutine ─────────────────────────────────────
				// A cancellable goroutine fires the tone at the specified delay.
				// Cancelled immediately when the response window closes so the
				// tone never bleeds into the next trial.
				ctx, cancel := context.WithCancel(context.Background())
				if t.isStop {
					d := time.Duration(t.stopDelay) * time.Millisecond
					go func() {
						select {
						case <-time.After(d):
							_ = stopTone.Play()
						case <-ctx.Done():
						}
					}()
				}

				// ── Response window ───────────────────────────────────────────
				var responseKeys []control.Keycode
				if spec.task == "simple" {
					responseKeys = []control.Keycode{control.K_SPACE}
				} else {
					responseKeys = []control.Keycode{control.K_F, control.K_J}
				}
				key, eventTS, kErr := exp.Keyboard.WaitKeysEventRT(responseKeys, maxRTms)
				cancel()
				if kErr != nil {
					return kErr
				}

				// ── Blank screen ──────────────────────────────────────────────
				_ = exp.Screen.Clear()
				_ = exp.Screen.Update()

				// ── Derive RT, correctness, inhibition ────────────────────────
				var rtMS int64
				if eventTS != 0 {
					rtMS = int64(eventTS-onsetNS) / 1_000_000
				}

				inhibited := t.isStop && key == 0

				var correct bool
				respStr := "none"
				if key != 0 {
					respStr = fmt.Sprintf("%d", key)
					if spec.task == "simple" {
						correct = true // any response = correct for simple RT
					} else {
						correct = key == cmap.keyFor(t.letter)
					}
				} else if t.isStop {
					correct = true // successful inhibition counts as correct
				}

				exp.Data.Add(
					trialNum, blockIdx+1, spec.task,
					t.letter, t.isStop, t.stopDelay,
					respStr, rtMS, correct, inhibited,
				)
				fmt.Printf(
					"B%d T%3d [%-6s] %s  stop=%-5v d=%3dms  rt=%4dms  ok=%v inh=%v\n",
					blockIdx+1, trialNum, spec.task, t.letter,
					t.isStop, t.stopDelay, rtMS, correct, inhibited,
				)

				// ── ITI (2500 ms blank) ───────────────────────────────────────
				exp.Wait(itiMS)
			}
		}

		_ = exp.Data.Save()
		exp.ShowInstructions(
			"Experiment complete!\n\n" +
				"Thank you for your participation.\n\n" +
				"Press SPACE to exit.")
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("experiment error: %v", runErr)
	}
}
