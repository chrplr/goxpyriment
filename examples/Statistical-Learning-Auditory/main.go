// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Statistical Learning of Tone Sequences — Saffran, Johnson, Aslin & Newport (1999).
//
// Implements all three experiments from the paper:
//
//	Experiment 1 — Adults: Word vs Non-word (2AFC)
//	Experiment 2 — Adults: Word vs Part-word (2AFC)
//	Experiment 3 — Infants: Head-turn preferential listening (HPP)
//
// All experiments use pure sine tones from the chromatic scale (middle-C octave),
// each 330 ms, presented in a continuous stream with no silent gap between tones.
// The only boundary cues are transitional probabilities (TPs) between successive
// tones: high within a 3-tone "word" (the statistical unit), low across word
// boundaries.
//
// Exposure (Experiments 1 & 2): three 7-minute sessions with break screens
// between them, totalling 21 minutes of listening.
//
// Exposure (Experiment 3): a single 3-minute stream.
//
// Test (Experiments 1 & 2): 36 two-alternative forced-choice (2AFC) trials.
// Each trial presents two 3-tone sequences in succession (750 ms silent gap);
// the participant presses 1 or 2 to indicate which sounded more familiar.
//
// Test (Experiment 3): 12 head-turn preferential listening (HPP) trials.
// The experimenter controls trial stages via the keyboard and toggles the
// infant's looking state; looking time is accumulated automatically and the
// trial terminates after 15 s of total looking or 2 s of continuous look-away.
//
// Reference:
//
//	Saffran, J. R., Johnson, E. K., Aslin, R. N., & Newport, E. L. (1999).
//	Statistical learning of tone sequences by human infants and adults.
//	Cognition, 70(1), 27–52.  https://doi.org/10.1016/S0010-0277(98)00075-4
//
// Usage:
//
//	go run .
package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Timing constants ─────────────────────────────────────────────────────────

const (
	toneDurMS  = 330 // duration of each tone (ms)
	toneRampMS = 10  // linear ramp to suppress onset/offset clicks (ms)
	toneAmp    = float32(0.45)

	// Exposure
	sessionDurMinAdult = 7 // minutes per session (Experiments 1 & 2)
	nSessions          = 3 // number of sessions  (Experiments 1 & 2)
	sessionDurMinInf   = 3 // minutes (Experiment 3)

	// 2AFC test (Experiments 1 & 2)
	testPauseMS = 750  // silence between the two items within a trial
	testITIMS   = 5000 // blank interval between trials
	nTest2AFC   = 36   // trials

	// HPP (Experiment 3)
	hppGapMS         = 500    // silence between test-item repetitions during a trial
	maxLookingMS     = 15_000 // max accumulated looking time per trial (ms)
	lookAwayThreshMS = 2_000  // continuous look-away threshold (ms)
	nHPPTrials       = 12
)

// ── Tone frequencies (chromatic scale, middle-C octave) ──────────────────────

var toneFreq = map[string]float64{
	"C":  261.63,
	"C#": 277.18,
	"D":  293.66,
	"D#": 311.13,
	"E":  329.63,
	"F":  349.23,
	"F#": 369.99,
	"G":  392.00,
	"G#": 415.30,
	"A":  440.00,
	"A#": 466.16,
	"B":  493.88,
}

// ── Language definitions ──────────────────────────────────────────────────────

// Experiments 1 & 2: two 6-word languages sharing the same 11-tone alphabet.
// Language Two's words serve as non-words for Language One (TP = 0), and vice
// versa.
var (
	exp12Lang1 = [][]string{
		{"A", "D", "B"},
		{"D", "F", "E"},
		{"G", "G#", "A"},
		{"F", "C", "F#"},
		{"D#", "E", "D"},
		{"C", "C#", "D"},
	}
	exp12Lang2 = [][]string{
		{"A", "C#", "E"},
		{"F#", "G#", "E"},
		{"G", "C", "D#"},
		{"C#", "B", "A"},
		{"C#", "F", "D"},
		{"G#", "B", "A"},
	}
)

// Experiment 3: two 4-word languages for the infant study.
// Within-word TP = 1.0; part-word first-bigram TP ≈ 0.33.
var (
	exp3Lang1 = [][]string{
		{"A", "F", "B"},
		{"F#", "A#", "D"},
		{"E", "G", "D#"},
		{"C", "G#", "C#"},
	}
	exp3Lang2 = [][]string{
		{"D#", "C", "G#"},
		{"C#", "E", "G"},
		{"F", "B", "F#"},
		{"A#", "D", "A"},
	}
)

// ── Part-word generation ──────────────────────────────────────────────────────

// buildPartWords returns one part-word per trained word.
// Each part-word is: last tone of words[i] + first two tones of words[(i+1)%n].
// Such sequences span word boundaries and carry lower transitional probability
// than within-word bigrams.
func buildPartWords(words [][]string) [][]string {
	n := len(words)
	parts := make([][]string, n)
	for i, w := range words {
		j := (i + 1) % n
		parts[i] = []string{w[2], words[j][0], words[j][1]}
	}
	return parts
}

// ── Tone preloading ───────────────────────────────────────────────────────────

// preloadToneSet creates and preloads (on the audio device) one *stimuli.Tone
// per distinct note name used across all provided word sets.
func preloadToneSet(exp *control.Experiment, wordSets ...[][]string) (map[string]*stimuli.Tone, error) {
	seen := map[string]bool{}
	for _, ws := range wordSets {
		for _, w := range ws {
			for _, note := range w {
				seen[note] = true
			}
		}
	}
	tones := make(map[string]*stimuli.Tone, len(seen))
	for note := range seen {
		freq, ok := toneFreq[note]
		if !ok {
			return nil, fmt.Errorf("unknown note: %q", note)
		}
		t := stimuli.NewComplexTone([]float64{freq}, toneDurMS, toneRampMS, toneAmp)
		if err := t.PreloadDevice(exp.AudioDevice); err != nil {
			return nil, fmt.Errorf("preload %q: %w", note, err)
		}
		tones[note] = t
	}
	return tones, nil
}

// ── Stream generation ─────────────────────────────────────────────────────────

// generateStream returns a flat slice of note names long enough to fill
// targetDurMS milliseconds.  Words are selected at random with the constraint
// that the same word never repeats back-to-back.
func generateStream(words [][]string, targetDurMS int64) []string {
	var stream []string
	nTones := int(targetDurMS / toneDurMS)
	lastWord := -1
	for len(stream) < nTones {
		wi := rand.Intn(len(words))
		for wi == lastWord {
			wi = rand.Intn(len(words))
		}
		lastWord = wi
		stream = append(stream, words[wi]...)
	}
	return stream[:nTones]
}

// ── Low-level helpers ─────────────────────────────────────────────────────────

// waitMS waits for the given number of milliseconds, returning an error if ESC
// is pressed or the experiment loop ends.
func waitMS(exp *control.Experiment, ms int) error {
	_, _, err := exp.Keyboard.WaitKeysEventRT(nil, ms)
	if err != nil {
		return err
	}
	return nil
}

// playSequence plays all notes in a 3-tone sequence with toneDurMS between each
// onset.  Returns an error if ESC is pressed.
func playSequence(exp *control.Experiment, notes []string, tones map[string]*stimuli.Tone) error {
	for _, note := range notes {
		if err := tones[note].Play(); err != nil {
			return err
		}
		_, _, err := exp.Keyboard.WaitKeysEventRT(nil, toneDurMS)
		if err != nil {
			return err
		}
	}
	return nil
}

// blinkUntilSpace blinks a stimulus at 2 Hz until the observer presses SPACE.
func blinkUntilSpace(exp *control.Experiment, stim stimuli.VisualStimulus) error {
	blinkOn := true
	blinkPeriod := 500 * time.Millisecond
	nextToggle := time.Now().Add(blinkPeriod)
	for {
		now := time.Now()
		if now.After(nextToggle) {
			blinkOn = !blinkOn
			nextToggle = now.Add(blinkPeriod)
			_ = exp.Screen.Clear()
			if blinkOn {
				_ = stim.Draw(exp.Screen)
			}
			_ = exp.Screen.Update()
		}
		key, kErr := exp.Keyboard.Check()
		if kErr != nil {
			return kErr
		}
		if key == control.K_SPACE {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// ── Exposure phase (adults) ───────────────────────────────────────────────────

func runAdultExposure(exp *control.Experiment, words [][]string, tones map[string]*stimuli.Tone) error {
	sessionDurMS := int64(sessionDurMinAdult * 60 * 1000)
	fixation := stimuli.NewFixCross(16, 2, control.White)

	for sess := 1; sess <= nSessions; sess++ {
		var msg string
		if sess == 1 {
			msg = fmt.Sprintf(
				"LISTENING PHASE\n\n"+
					"You will hear a continuous sequence of tones.\n"+
					"Listen carefully — there will be a memory test afterwards.\n\n"+
					"Session %d of %d  (%d minutes)\n\n"+
					"Press SPACE to begin.", sess, nSessions, sessionDurMinAdult)
		} else {
			msg = fmt.Sprintf(
				"Break\n\nSession %d of %d.\n\nPress SPACE to continue.",
				sess, nSessions)
		}
		exp.ShowInstructions(msg)

		_ = exp.Screen.Clear()
		_ = fixation.Draw(exp.Screen)
		_ = exp.Screen.Update()

		stream := generateStream(words, sessionDurMS)
		for _, note := range stream {
			if err := tones[note].Play(); err != nil {
				return err
			}
			_, _, err := exp.Keyboard.WaitKeysEventRT(nil, toneDurMS)
			if err != nil {
				return err
			}
		}

		_ = exp.Screen.Clear()
		_ = exp.Screen.Update()
	}
	return nil
}

// ── Test phase (adults, 2AFC) ─────────────────────────────────────────────────

type testPair struct {
	item1   []string // first sequence played
	item2   []string // second sequence played
	wordIs  int      // 1 or 2: which item is the trained word
}

// buildTestPairs2AFC creates 36 test pairs by crossing each of 6 trained words
// with each of 6 other items (non-words or part-words), with word order
// counterbalanced randomly.
func buildTestPairs2AFC(words, others [][]string) []testPair {
	var pairs []testPair
	for _, w := range words {
		for _, o := range others {
			wc := append([]string{}, w...)
			oc := append([]string{}, o...)
			if rand.Intn(2) == 0 {
				pairs = append(pairs, testPair{item1: wc, item2: oc, wordIs: 1})
			} else {
				pairs = append(pairs, testPair{item1: oc, item2: wc, wordIs: 2})
			}
		}
	}
	design.ShuffleList(pairs)
	return pairs
}

func run2AFCTest(exp *control.Experiment, pairs []testPair, tones map[string]*stimuli.Tone) error {
	instrFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 36)
	if err != nil {
		return err
	}
	defer instrFont.Close()

	exp.ShowInstructions(
		"TEST PHASE\n\n" +
			"You will now hear pairs of tone sequences.\n\n" +
			"Press  1  if the FIRST sequence sounded more familiar.\n" +
			"Press  2  if the SECOND sequence sounded more familiar.\n\n" +
			"There are no right or wrong answers — trust your intuition.\n\n" +
			"Press SPACE to start.")

	exp.AddDataVariableNames([]string{
		"trial", "word_position", "response", "correct",
	})

	for i, p := range pairs {
		trialNum := i + 1
		_ = exp.Screen.Clear()
		_ = exp.Screen.Update()
		if err := waitMS(exp, 500); err != nil {
			return err
		}

		// --- Item 1 ---
		lbl1 := stimuli.NewTextLine("Sequence  1", 0, 0, control.White)
		lbl1.Font = instrFont
		_ = exp.Screen.Clear()
		_ = lbl1.Draw(exp.Screen)
		_ = exp.Screen.Update()
		if err := playSequence(exp, p.item1, tones); err != nil {
			return err
		}

		// Inter-item pause
		_ = exp.Screen.Clear()
		_ = exp.Screen.Update()
		if err := waitMS(exp, testPauseMS); err != nil {
			return err
		}

		// --- Item 2 ---
		lbl2 := stimuli.NewTextLine("Sequence  2", 0, 0, control.White)
		lbl2.Font = instrFont
		_ = exp.Screen.Clear()
		_ = lbl2.Draw(exp.Screen)
		_ = exp.Screen.Update()
		if err := playSequence(exp, p.item2, tones); err != nil {
			return err
		}

		// --- Response ---
		prompt := stimuli.NewTextLine("Which sounded more familiar?   1   or   2", 0, 0, control.White)
		prompt.Font = instrFont
		_ = exp.Screen.Clear()
		_ = prompt.Draw(exp.Screen)
		_ = exp.Screen.Update()

		key, _, kErr := exp.Keyboard.WaitKeysEventRT(
			[]control.Keycode{control.K_1, control.K_2}, -1)
		if kErr != nil {
			return kErr
		}

		resp := 2
		if key == control.K_1 {
			resp = 1
		}
		correct := resp == p.wordIs

		exp.Data.Add(trialNum, p.wordIs, resp, correct)
		fmt.Printf("T%3d  word=%d  resp=%d  ok=%v\n", trialNum, p.wordIs, resp, correct)

		// ITI
		_ = exp.Screen.Clear()
		_ = exp.Screen.Update()
		if err := waitMS(exp, testITIMS); err != nil {
			return err
		}
	}
	return nil
}

// ── Exposure phase (infants) ──────────────────────────────────────────────────

func runInfantExposure(exp *control.Experiment, words [][]string, tones map[string]*stimuli.Tone) error {
	sessionDurMS := int64(sessionDurMinInf * 60 * 1000)
	fixation := stimuli.NewFixCross(16, 2, control.White)

	exp.ShowInstructions(
		"EXPOSURE PHASE\n\n" +
			"The infant will now hear a 3-minute tone stream.\n" +
			"No response is required.\n\n" +
			"Press SPACE to begin.")

	_ = exp.Screen.Clear()
	_ = fixation.Draw(exp.Screen)
	_ = exp.Screen.Update()

	stream := generateStream(words, sessionDurMS)
	for _, note := range stream {
		if err := tones[note].Play(); err != nil {
			return err
		}
		_, _, err := exp.Keyboard.WaitKeysEventRT(nil, toneDurMS)
		if err != nil {
			return err
		}
	}

	_ = exp.Screen.Clear()
	_ = exp.Screen.Update()
	return nil
}

// ── HPP test phase (infants) ──────────────────────────────────────────────────

type hppTrial struct {
	item   []string
	isWord bool
}

// buildHPPTrials creates nHPPTrials trials from the first 2 words and first 2
// part-words (3 repetitions each), pseudo-randomised.
func buildHPPTrials(words, parts [][]string) []hppTrial {
	var trials []hppTrial
	for rep := 0; rep < 3; rep++ {
		for _, w := range words[:2] {
			trials = append(trials, hppTrial{item: append([]string{}, w...), isWord: true})
		}
		for _, p := range parts[:2] {
			trials = append(trials, hppTrial{item: append([]string{}, p...), isWord: false})
		}
	}
	design.ShuffleList(trials)
	return trials
}

// runHPPTest runs the HPP test phase.
//
// Observer key bindings:
//
//	SPACE — advance stage / toggle infant looking state
//	ESC   — abort
//
// Trial stages:
//  1. Central light blinks → press SPACE when infant fixates centre.
//  2. Side light blinks    → press SPACE when infant turns (~30°) to the side.
//  3. Sound plays on loop  → SPACE toggles "looking" / "not-looking".
//     Trial ends automatically after 15 s total looking or 2 s look-away.
func runHPPTest(exp *control.Experiment, trials []hppTrial, tones map[string]*stimuli.Tone) error {
	exp.ShowInstructions(
		"TEST PHASE — HEAD-TURN PREFERENTIAL LISTENING\n\n" +
			"Observer controls:\n\n" +
			"  Stage 1 — Central light blinks. Press SPACE when infant fixates.\n" +
			"  Stage 2 — Side light blinks.    Press SPACE when infant turns (30°).\n" +
			"  Stage 3 — Sound plays.           Press SPACE to toggle looking / not-looking.\n\n" +
			"The trial ends after 15 s of looking or 2 s of continuous look-away.\n\n" +
			"Press SPACE to start.")

	exp.AddDataVariableNames([]string{
		"trial", "item_type", "looking_time_ms",
	})

	// Create blink stimuli; positions set via SetPosition.
	centreDot := stimuli.NewCircle(20, control.Yellow)
	leftDot := stimuli.NewCircle(30, control.White)
	rightDot := stimuli.NewCircle(30, control.White)
	leftDot.SetPosition(control.FPoint{X: -400, Y: 0})
	rightDot.SetPosition(control.FPoint{X: 400, Y: 0})

	// Sequence cycle length in milliseconds
	seqCycleMS := int64(len(trials[0].item)*toneDurMS) + hppGapMS

	for trialNum, t := range trials {
		fmt.Printf("Trial %2d  item=%v  word=%v\n", trialNum+1, t.item, t.isWord)

		// ── Stage 1: central blink until fixation ────────────────────────
		if err := blinkUntilSpace(exp, centreDot); err != nil {
			return err
		}

		// ── Stage 2: side blink until head turn ──────────────────────────
		var sideDot *stimuli.Circle
		if rand.Intn(2) == 0 {
			sideDot = leftDot
		} else {
			sideDot = rightDot
		}
		if err := blinkUntilSpace(exp, sideDot); err != nil {
			return err
		}

		// Show the side light steadily while sound plays
		_ = exp.Screen.Clear()
		_ = sideDot.Draw(exp.Screen)
		_ = exp.Screen.Update()

		// ── Stage 3: sound loop + looking-time tracking ───────────────────
		looking := true
		lookingTotalMS := int64(0)
		lookAwayMS := int64(0)
		lastUpdate := time.Now()
		trialStart := time.Now()
		lastNoteIdx := -1
		lastCycle := int64(-1)

		for lookingTotalMS < maxLookingMS && lookAwayMS < lookAwayThreshMS {
			now := time.Now()
			dtMS := now.Sub(lastUpdate).Milliseconds()
			lastUpdate = now

			if looking {
				lookingTotalMS += dtMS
				lookAwayMS = 0
			} else {
				lookAwayMS += dtMS
			}

			// Trigger tones at the right moments (non-blocking)
			elapsedMS := now.Sub(trialStart).Milliseconds()
			cycle := elapsedMS / seqCycleMS
			posMS := elapsedMS % seqCycleMS
			noteIdx := int(posMS / toneDurMS)
			if noteIdx >= len(t.item) {
				noteIdx = -1 // in gap
			}
			if cycle != lastCycle || noteIdx != lastNoteIdx {
				if noteIdx >= 0 {
					_ = tones[t.item[noteIdx]].Play()
				}
				lastCycle = cycle
				lastNoteIdx = noteIdx
			}

			// Check observer input
			key, kErr := exp.Keyboard.Check()
			if kErr != nil {
				return kErr
			}
			if key == control.K_SPACE {
				looking = !looking
			}

			time.Sleep(5 * time.Millisecond)
		}

		itemType := "word"
		if !t.isWord {
			itemType = "part-word"
		}
		exp.Data.Add(trialNum+1, itemType, lookingTotalMS)
		fmt.Printf("  looking=%.3f s\n", float64(lookingTotalMS)/1000.0)

		_ = exp.Screen.Clear()
		_ = exp.Screen.Update()
		if err := waitMS(exp, 1000); err != nil {
			return err
		}
	}
	return nil
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	fields := []control.InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		{
			Name:  "experiment",
			Label: "Experiment",
			Type:  control.FieldSelect,
			Options: []string{
				"1 — Adults: Word vs Non-word",
				"2 — Adults: Word vs Part-word",
				"3 — Infants: Preferential listening (HPP)",
			},
		},
		{
			Name:  "language",
			Label: "Language assignment",
			Type:  control.FieldSelect,
			Options: []string{"Language 1", "Language 2"},
		},
		control.FullscreenField,
	}

	info, err := control.GetParticipantInfo(
		"Statistical Learning of Tone Sequences — Saffran et al. (1999)", fields)
	if errors.Is(err, control.ErrCancelled) {
		log.Fatal("Setup cancelled.")
	}
	if err != nil {
		log.Fatalf("GetParticipantInfo: %v", err)
	}

	expNum := 1
	switch info["experiment"] {
	case "2 — Adults: Word vs Part-word":
		expNum = 2
	case "3 — Infants: Preferential listening (HPP)":
		expNum = 3
	}
	langNum := 1
	if info["language"] == "Language 2" {
		langNum = 2
	}

	fullscreen := info["fullscreen"] == "true"
	winW, winH := 0, 0
	if !fullscreen {
		winW, winH = 1024, 768
	}
	exp := control.NewExperiment("Saffran1999", winW, winH, fullscreen,
		control.Black, control.White, 32)
	if initErr := exp.Initialize(); initErr != nil {
		log.Fatalf("Initialize: %v", initErr)
	}
	defer exp.End()
	exp.Info = info

	if err := exp.SetLogicalSize(1920, 1080); err != nil {
		log.Printf("Warning: SetLogicalSize: %v", err)
	}

	// ── Select language and prepare test items ────────────────────────────────
	var (
		trainWords [][]string
		testOthers [][]string
	)
	switch expNum {
	case 1:
		if langNum == 1 {
			trainWords, testOthers = exp12Lang1, exp12Lang2
		} else {
			trainWords, testOthers = exp12Lang2, exp12Lang1
		}
	case 2:
		if langNum == 1 {
			trainWords = exp12Lang1
			testOthers = buildPartWords(exp12Lang1)
		} else {
			trainWords = exp12Lang2
			testOthers = buildPartWords(exp12Lang2)
		}
	case 3:
		if langNum == 1 {
			trainWords = exp3Lang1
			testOthers = buildPartWords(exp3Lang1)
		} else {
			trainWords = exp3Lang2
			testOthers = buildPartWords(exp3Lang2)
		}
	}

	// ── Preload tones ─────────────────────────────────────────────────────────
	tones, tErr := preloadToneSet(exp, trainWords, testOthers)
	if tErr != nil {
		log.Fatalf("preloadToneSet: %v", tErr)
	}
	defer func() {
		for _, t := range tones {
			_ = t.Unload()
		}
	}()

	// ── Run ───────────────────────────────────────────────────────────────────
	runErr := exp.Run(func() error {
		switch expNum {
		case 1, 2:
			if err := runAdultExposure(exp, trainWords, tones); err != nil {
				return err
			}
			pairs := buildTestPairs2AFC(trainWords, testOthers)
			if err := run2AFCTest(exp, pairs, tones); err != nil {
				return err
			}
		case 3:
			if err := runInfantExposure(exp, trainWords, tones); err != nil {
				return err
			}
			hppTrials := buildHPPTrials(trainWords, testOthers)
			if err := runHPPTest(exp, hppTrials, tones); err != nil {
				return err
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
