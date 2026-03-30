// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Posner & Mitchell (1967) chronometric classification task.
//
// Pairs of letters from {A, B, C, E, a, b, c, e} are shown side by side.
// The participant presses "same" or "different" as quickly as possible.
// The definition of "same" depends on the instruction level selected in the
// setup dialog:
//
//	Level 1 – Physical identity: same only if both letters are identical in
//	           form and case (e.g. AA, bb).  ~468 ms in the original paper.
//
//	Level 2 – Name identity: same if both letters share the same name
//	           regardless of case (e.g. Aa, BB).  ~550 ms for name matches.
//
//	Level 3 – Rule identity: same if both are vowels {A,E,a,e} or both are
//	           consonants {B,C,b,c}.  ~700–900 ms for rule matches.
//
// The systematic increase in RT across levels is the central result: it
// reflects the depth of processing (physical → name → conceptual) required
// to classify each pair.
//
// Trial deck (96 trials, close to the paper's 88-card deck):
//   - Physically identical pairs (AA, bb …)         × 3 reps = 24
//   - Name-same, different case (Aa, Bb …)           × 3 reps = 24
//   - Rule-same, different name (AE, BC, Ae, bC …)  × 1 rep  = 16
//   - Rule-different (one vowel + one consonant)     × 1 rep  = 32
//
// Reference:
//
//	Posner, M. I., & Mitchell, R. F. (1967). Chronometric analysis of
//	classification. Psychological Review, 74(5), 392–409.
//	https://doi.org/10.1037/h0024808
//
// Usage:
//
//	go run .
package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Timing constants (ms).
// The original used a 10 s ITI for manual card-handling; 1 s suits a
// computerised session without sacrificing the paradigm's integrity.
const (
	itiMS      = 1000
	maxRTms    = 5000
	feedbackMS = 1200

	letterOffset = 150 // px from screen centre to each letter (logical coords)
)

// Letter set and vowel classification used throughout all experiments.
var (
	allLetters = []string{"A", "B", "C", "E", "a", "b", "c", "e"}
	vowelSet   = map[string]bool{"A": true, "E": true, "a": true, "e": true}
)

// ── Letter-pair logic ─────────────────────────────────────────────────────────

type letterPair struct{ left, right string }

func (p letterPair) physicalSame() bool { return p.left == p.right }
func (p letterPair) nameSame() bool {
	return strings.ToUpper(p.left) == strings.ToUpper(p.right)
}
func (p letterPair) ruleSame() bool { return vowelSet[p.left] == vowelSet[p.right] }

// correctSame reports whether "same" is the correct response at the given level.
func (p letterPair) correctSame(level int) bool {
	switch level {
	case 1:
		return p.physicalSame()
	case 2:
		return p.nameSame()
	case 3:
		return p.ruleSame()
	}
	return false
}

// category returns the match type of the pair:
// "physical" ⊂ "name" ⊂ "rule", then "different".
func (p letterPair) category() string {
	if p.physicalSame() {
		return "physical"
	}
	if p.nameSame() {
		return "name"
	}
	if p.ruleSame() {
		return "rule"
	}
	return "different"
}

// ── Trial deck ────────────────────────────────────────────────────────────────

// buildDeck generates 96 trials matching the proportions of the 88-card deck
// described in the paper (physical-same and name-same pairs appear ×3).
func buildDeck() []letterPair {
	var deck []letterPair
	for _, l1 := range allLetters {
		for _, l2 := range allLetters {
			p := letterPair{l1, l2}
			reps := 1
			if p.physicalSame() || p.nameSame() {
				reps = 3
			}
			for i := 0; i < reps; i++ {
				deck = append(deck, p)
			}
		}
	}
	return deck
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	// ── Setup dialog ─────────────────────────────────────────────────────────
	fields := []control.InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		{
			Name:    "level",
			Label:   "Instruction level",
			Type:    control.FieldSelect,
			Options: []string{"Level 1 — Physical", "Level 2 — Name", "Level 3 — Rule"},
		},
		{
			Name:    "same_key",
			Label:   "'Same' response key",
			Type:    control.FieldSelect,
			Options: []string{"F = Same,  J = Different", "J = Same,  F = Different"},
		},
		control.FullscreenField,
	}

	info, err := control.GetParticipantInfo(
		"Posner & Mitchell (1967) — Classification Task", fields)
	if errors.Is(err, control.ErrCancelled) {
		log.Fatal("Setup cancelled.")
	}
	if err != nil {
		log.Fatalf("GetParticipantInfo: %v", err)
	}

	// Parse level
	var level int
	switch info["level"] {
	case "Level 1 — Physical":
		level = 1
	case "Level 2 — Name":
		level = 2
	default:
		level = 3
	}

	// Parse key assignment
	var sameKey, diffKey control.Keycode
	var sameLabel, diffLabel string
	if info["same_key"] == "F = Same,  J = Different" {
		sameKey, diffKey = control.K_F, control.K_J
		sameLabel, diffLabel = "F", "J"
	} else {
		sameKey, diffKey = control.K_J, control.K_F
		sameLabel, diffLabel = "J", "F"
	}

	// ── Initialise experiment ─────────────────────────────────────────────────
	fullscreen := info["fullscreen"] == "true"
	winW, winH := 0, 0
	if !fullscreen {
		winW, winH = 1024, 768
	}
	exp := control.NewExperiment("Posner-Mitchell-1967", winW, winH, fullscreen,
		control.White, control.Black, 32)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("Initialize: %v", err)
	}
	defer exp.End()
	exp.Info = info

	if err := exp.SetLogicalSize(1920, 1080); err != nil {
		log.Printf("Warning: SetLogicalSize: %v", err)
	}

	// ── Fonts ─────────────────────────────────────────────────────────────────
	letterFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 120)
	if err != nil {
		log.Fatalf("letter font: %v", err)
	}
	defer letterFont.Close()

	feedbackFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 40)
	if err != nil {
		log.Fatalf("feedback font: %v", err)
	}
	defer feedbackFont.Close()

	// ── Stimuli ───────────────────────────────────────────────────────────────
	// Pre-create one TextLine per letter × position to avoid per-trial allocation.
	leftStim  := make(map[string]*stimuli.TextLine, len(allLetters))
	rightStim := make(map[string]*stimuli.TextLine, len(allLetters))
	for _, l := range allLetters {
		ls := stimuli.NewTextLine(l, -letterOffset, 0, control.Black)
		ls.Font = letterFont
		leftStim[l] = ls

		rs := stimuli.NewTextLine(l, letterOffset, 0, control.Black)
		rs.Font = letterFont
		rightStim[l] = rs
	}

	// ── Design ────────────────────────────────────────────────────────────────
	trials := buildDeck()
	design.ShuffleList(trials)

	exp.AddDataVariableNames([]string{
		"trial", "left", "right", "category",
		"expected", "response", "rt_ms", "correct",
	})

	// ── Instructions ─────────────────────────────────────────────────────────
	var levelDesc string
	switch level {
	case 1:
		levelDesc = fmt.Sprintf(
			"LEVEL 1 — PHYSICAL IDENTITY\n\n"+
				"Press '%s' (SAME) only if the two letters are\n"+
				"identical in shape and case.\n"+
				"Examples of SAME:      A A     b b     C C\n"+
				"Examples of DIFFERENT: A a     A B     a B\n\n"+
				"Press '%s' (DIFFERENT) for everything else.",
			sameLabel, diffLabel)
	case 2:
		levelDesc = fmt.Sprintf(
			"LEVEL 2 — NAME IDENTITY\n\n"+
				"Press '%s' (SAME) if both letters have the same name,\n"+
				"regardless of upper or lower case.\n"+
				"Examples of SAME:      A A     A a     b B\n"+
				"Examples of DIFFERENT: A B     A b     a C\n\n"+
				"Press '%s' (DIFFERENT) if the letters have different names.",
			sameLabel, diffLabel)
	case 3:
		levelDesc = fmt.Sprintf(
			"LEVEL 3 — RULE IDENTITY\n\n"+
				"Vowels: A  E  a  e         Consonants: B  C  b  c\n\n"+
				"Press '%s' (SAME) if both letters are vowels or both\n"+
				"are consonants.\n"+
				"Examples of SAME:      A E     a e     B C     A a     b b\n"+
				"Examples of DIFFERENT: A B     E c     a C\n\n"+
				"Press '%s' (DIFFERENT) if one is a vowel and the other\n"+
				"a consonant.",
			sameLabel, diffLabel)
	}

	instrText := levelDesc + "\n\n" +
		"Respond as quickly as possible while keeping errors to a minimum.\n" +
		"Your reaction time and correctness will be shown after each trial.\n\n" +
		fmt.Sprintf("There are %d trials in this session.\n\n", len(trials)) +
		"Press SPACE to start."

	// ── Experiment loop ───────────────────────────────────────────────────────
	runErr := exp.Run(func() error {
		exp.ShowInstructions(instrText)

		for i, t := range trials {
			// ITI: blank white screen
			_ = exp.Screen.Clear()
			_ = exp.Screen.Update()
			exp.Wait(itiMS)

			// Display letter pair; capture VSYNC-aligned onset timestamp
			_ = exp.Screen.Clear()
			_ = leftStim[t.left].Draw(exp.Screen)
			_ = rightStim[t.right].Draw(exp.Screen)
			onsetNS, flipErr := exp.Screen.FlipNS()
			if flipErr != nil {
				return flipErr
			}

			// Wait for "same" or "different" (letters remain on screen)
			key, eventTS, kErr := exp.Keyboard.WaitKeysEventRT(
				[]control.Keycode{sameKey, diffKey},
				maxRTms,
			)
			if kErr != nil {
				return kErr
			}

			// Compute RT and correctness
			var rtMS int64
			if eventTS != 0 {
				rtMS = int64(eventTS-onsetNS) / 1_000_000
			}
			wantSame := t.correctSame(level)
			expected := "different"
			if wantSame {
				expected = "same"
			}
			var response string
			var correct bool
			switch key {
			case sameKey:
				response = "same"
				correct = wantSame
			case diffKey:
				response = "different"
				correct = !wantSame
			default:
				response = "timeout"
			}

			exp.Data.Add(i+1, t.left, t.right, t.category(),
				expected, response, rtMS, correct)
			fmt.Printf("Trial %3d  %s–%s  [%-9s]  exp=%-9s  resp=%-9s  rt=%4d ms  ok=%v\n",
				i+1, t.left, t.right, t.category(), expected, response, rtMS, correct)

			// Feedback: correctness + RT (per paper: shown after every trial)
			var fbText string
			var fbColor control.Color
			switch {
			case response == "timeout":
				fbText = "No response"
				fbColor = control.Red
			case correct:
				fbText = fmt.Sprintf("Correct     RT: %d ms", rtMS)
				fbColor = control.RGB(0, 140, 0)
			default:
				fbText = fmt.Sprintf("Wrong       RT: %d ms", rtMS)
				fbColor = control.Red
			}
			fb := stimuli.NewTextLine(fbText, 0, 0, fbColor)
			fb.Font = feedbackFont
			_ = exp.Screen.Clear()
			_ = fb.Draw(exp.Screen)
			_ = exp.Screen.Update()
			exp.Wait(feedbackMS)
		}

		_ = exp.Data.Save()
		exp.ShowInstructions("Session complete.\n\nThank you!\n\nPress SPACE to exit.")
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("experiment error: %v", runErr)
	}
}
