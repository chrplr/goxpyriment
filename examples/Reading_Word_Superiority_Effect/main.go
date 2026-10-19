// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Reading_Word_Superiority_Effect — Reicher (1969), letter identification in
// words, anagrams and single letters.
//
// A four-letter word (WORD), an anagram of it that keeps the critical letter in
// place (ORWD), or the critical letter alone (___D) is flashed for a few display
// frames. It is replaced at once by a pattern mask of overlapping Xs and Os,
// with two alternative letters above and below the critical position (D and K).
// Both alternatives complete a word in the word condition (WORD / WORK) and a
// non-word in the anagram condition, so knowing the word cannot tell the two
// apart. The word superiority effect is the higher accuracy for words than for
// single letters and anagrams.
//
// The session has three phases:
//
//  1. practice — 6 trials at a long duration, with feedback;
//  2. staircase — single letters with the list's letter pairs, 1-up/3-down on
//     the number of frames, which sets the duration for the main task
//     (skipped with -frames N);
//  3. main — every item in the three materials, shuffled, no feedback.
//
// Usage:
//
//	go run ./examples/Reading_Word_Superiority_Effect -w -s 999
//	go run ./examples/Reading_Word_Superiority_Effect -w -s 999 -lang fr -frames 3
package main

import (
	_ "embed"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/staircase"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const (
	nSlots = 4 // letters per string

	keyTop    = control.K_UP
	keyBottom = control.K_DOWN

	// Practice trials are shown long enough to be seen clearly.
	practiceMS = 150
	feedbackMS = 600

	// Staircase on single letters: 1-up/3-down converges near 79 % correct,
	// between the 60 % and 90 % points Reicher used to bracket his durations.
	staircaseStartMS   = 100
	staircaseMaxMS     = 300
	staircaseReversals = 10
	staircaseMaxTrials = 100
	staircaseLastN     = 6 // reversals averaged for the threshold

	fixSize      = 20
	fixLineWidth = 3

	feedbackRadius = 8

	blank = '_' // empty slot in the "displayed" column
)

var materials = []string{"word", "anagram", "letter"}

// ── Flags ─────────────────────────────────────────────────────────────────────
//
// Declared at package level so they are registered before
// control.NewExperimentFromFlags calls flag.Parse().

var (
	langArg    = flag.String("lang", "en", "Stimulus and instruction language: en or fr")
	framesArg  = flag.Int("frames", 0, "Stimulus duration in display frames; 0 = set it with the single-letter staircase")
	stimFile   = flag.String("stim", "", "Stimulus CSV (word,pos,alt,anagram) to use instead of the embedded list")
	fontSize   = flag.Float64("fontsize", 64, "Point size of the letters and the mask")
	fixMS      = flag.Int("fix", 500, "Fixation cross duration, ms")
	itiMS      = flag.Int("iti", 700, "Blank inter-trial interval, ms")
	breakEvery = flag.Int("break", 48, "Main-task trials between rest breaks (0 = no breaks)")
)

// ── Stimulus list ─────────────────────────────────────────────────────────────

//go:embed stimuli_en.csv
var stimuliEN string

//go:embed stimuli_fr.csv
var stimuliFR string

// Practice items, kept out of the stimulus lists so practice does not show any
// experimental item.
var practiceItems = map[string][]item{
	"en": {{word: "FISH", pos: 1, alt: 'D', anagram: "FSHI"}, {word: "MOON", pos: 4, alt: 'D', anagram: "OMON"}},
	"fr": {{word: "CHAT", pos: 3, alt: 'U', anagram: "HCAT"}, {word: "LAIT", pos: 4, alt: 'D', anagram: "ILAT"}},
}

// item is one row of the stimulus list: a word, the 1-based position of its
// critical letter, the alternative letter that makes another word there, and
// an anagram of the word that keeps the critical letter in place.
type item struct {
	word    string
	pos     int
	alt     byte
	anagram string
}

// validate checks everything about an item that can be checked mechanically.
// Whether the words are words and the anagrams are not is up to the list.
func (it item) validate() error {
	isUpper := func(s string) bool {
		for i := 0; i < len(s); i++ {
			if s[i] < 'A' || s[i] > 'Z' {
				return false
			}
		}
		return len(s) == nSlots
	}
	if !isUpper(it.word) || !isUpper(it.anagram) {
		return fmt.Errorf("word %q and anagram %q must be %d letters A-Z", it.word, it.anagram, nSlots)
	}
	if it.pos < 1 || it.pos > nSlots {
		return fmt.Errorf("pos %d out of range 1..%d", it.pos, nSlots)
	}
	if it.alt < 'A' || it.alt > 'Z' || it.alt == it.word[it.pos-1] {
		return fmt.Errorf("alt %q must be a letter A-Z other than %q", it.alt, it.word[it.pos-1])
	}
	if it.anagram == it.word {
		return fmt.Errorf("anagram %q is the word itself", it.anagram)
	}
	if it.anagram[it.pos-1] != it.word[it.pos-1] {
		return fmt.Errorf("anagram %q does not keep %q at position %d", it.anagram, it.word[it.pos-1], it.pos)
	}
	a, w := []byte(it.anagram), []byte(it.word)
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	sort.Slice(w, func(i, j int) bool { return w[i] < w[j] })
	if string(a) != string(w) {
		return fmt.Errorf("%q is not an anagram of %q", it.anagram, it.word)
	}
	return nil
}

// loadItems reads the stimulus list for lang, or the file at path when given.
// The first row is a header; a row that fails validation is an error, not a
// skipped row, since a silently shortened list unbalances the design.
func loadItems(path, lang string) ([]item, string, error) {
	var r io.Reader
	var source string
	switch {
	case path != "":
		f, err := os.Open(path)
		if err != nil {
			return nil, "", fmt.Errorf("opening stimulus file: %w", err)
		}
		defer f.Close()
		r, source = f, path
	case lang == "en":
		r, source = strings.NewReader(stimuliEN), "embedded stimuli_en.csv"
	case lang == "fr":
		r, source = strings.NewReader(stimuliFR), "embedded stimuli_fr.csv"
	default:
		return nil, "", fmt.Errorf("-lang must be en or fr, got %q", lang)
	}

	records, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, "", fmt.Errorf("reading %s: %w", source, err)
	}
	var items []item
	for i, rec := range records {
		if i == 0 {
			continue // header
		}
		if len(rec) < 4 {
			return nil, "", fmt.Errorf("%s row %d: want 4 fields word,pos,alt,anagram, got %d", source, i+1, len(rec))
		}
		pos, err := strconv.Atoi(strings.TrimSpace(rec[1]))
		if err != nil {
			return nil, "", fmt.Errorf("%s row %d: bad pos %q", source, i+1, rec[1])
		}
		alt := strings.ToUpper(strings.TrimSpace(rec[2]))
		if len(alt) != 1 {
			return nil, "", fmt.Errorf("%s row %d: alt %q must be one letter", source, i+1, rec[2])
		}
		it := item{
			word:    strings.ToUpper(strings.TrimSpace(rec[0])),
			pos:     pos,
			alt:     alt[0],
			anagram: strings.ToUpper(strings.TrimSpace(rec[3])),
		}
		if err := it.validate(); err != nil {
			return nil, "", fmt.Errorf("%s row %d: %w", source, i+1, err)
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		return nil, "", fmt.Errorf("%s: stimulus list is empty", source)
	}
	return items, source, nil
}

// ── Trials ────────────────────────────────────────────────────────────────────

type trial struct {
	phase       string // practice, staircase or main
	material    string // word, anagram or letter
	item        string // the base word the letter pair comes from
	displayed   string // the 4 slots as shown; '_' is an empty slot
	pos         int    // 1-based critical position
	target      byte   // the letter that was shown at pos
	foil        byte   // the other alternative
	targetOnTop bool
}

// makeTrial builds the trial for one item in one material. useAlt selects which
// member of the pair is shown: false shows the word's own letter (WORD, target
// D), true shows the alternative (WORK, target K). Both alternatives are always
// offered, so which one is the target is balanced, not fixed by the list.
func makeTrial(it item, material string, useAlt, targetOnTop bool, phase string) trial {
	target, foil := it.word[it.pos-1], it.alt
	if useAlt {
		target, foil = foil, target
	}
	var s []byte
	switch material {
	case "word":
		s = []byte(it.word)
	case "anagram":
		s = []byte(it.anagram)
	default:
		s = []byte(strings.Repeat(string(blank), nSlots))
	}
	s[it.pos-1] = target
	return trial{
		phase: phase, material: material, item: it.word, displayed: string(s),
		pos: it.pos, target: target, foil: foil, targetOnTop: targetOnTop,
	}
}

// balanced returns n booleans, half true, in random order (the odd one out is
// a coin flip).
func balanced(n int) []bool {
	b := make([]bool, n)
	for i := 0; i < n/2; i++ {
		b[i] = true
	}
	if n%2 == 1 {
		b[n-1] = design.CoinFlip(0.5)
	}
	design.ShuffleList(b)
	return b
}

// buildMainTrials crosses every item with the three materials. Within each
// material x critical position, which member of the pair is the target and
// whether the target is the top alternative are each balanced 50/50.
func buildMainTrials(items []item) []trial {
	var trials []trial
	for _, m := range materials {
		for p := 1; p <= nSlots; p++ {
			var group []item
			for _, it := range items {
				if it.pos == p {
					group = append(group, it)
				}
			}
			useAlt, onTop := balanced(len(group)), balanced(len(group))
			for i, it := range group {
				trials = append(trials, makeTrial(it, m, useAlt[i], onTop[i], "main"))
			}
		}
	}
	design.ShuffleList(trials)
	return trials
}

// buildPracticeTrials shows each practice item once in each material.
func buildPracticeTrials(items []item) []trial {
	var trials []trial
	for _, it := range items {
		for _, m := range materials {
			trials = append(trials, makeTrial(it, m, design.CoinFlip(0.5), design.CoinFlip(0.5), "practice"))
		}
	}
	design.ShuffleList(trials)
	return trials
}

// staircaseTrial is a single-letter trial built from a random item of the list:
// its critical letter pair, at its critical position, either member shown.
//
// Reicher's threshold sessions paired each letter with a random foil instead.
// A random foil is usually easy to tell from the target (Q vs L), while the
// list's pairs are often similar (A/O, I/U), so a staircase on random foils
// settles on a duration at which the main-task letter condition is near
// chance. Using the list's pairs calibrates the discrimination that condition
// actually tests.
func staircaseTrial(items []item) trial {
	return makeTrial(design.RandElement(items), "letter", design.CoinFlip(0.5), design.CoinFlip(0.5), "staircase")
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// placed is one glyph at one position on a frame.
type placed struct {
	glyph *stimuli.TextLine
	at    control.FPoint
}

// scene is a composite VisualStimulus drawing a list of glyphs, so a whole
// display can be presented with ShowFrames/ShowTS. Each glyph is a preloaded
// single-letter TextLine that is moved to its position before drawing, so a
// letter is drawn identically wherever it appears.
type scene struct {
	stimuli.BaseVisual
	items []placed
}

func (s *scene) Draw(screen *apparatus.Screen) error {
	for _, p := range s.items {
		p.glyph.Position = p.at
		if err := p.glyph.Draw(screen); err != nil {
			return fmt.Errorf("scene.Draw: %w", err)
		}
	}
	return nil
}

func (s *scene) Present(screen *apparatus.Screen, clear, update bool) error {
	return stimuli.PresentDrawable(s, screen, clear, update)
}

// layout holds the preloaded glyphs and the geometry shared by every display.
type layout struct {
	glyphs  map[byte]*stimuli.TextLine // 'A'..'Z'
	centres [nSlots]float32            // x of each slot
	dy      float32                    // vertical offset of the alternatives
}

// newLayout preloads one TextLine per letter and derives the slot centres from
// the monospace advance, so a string drawn slot by slot occupies exactly the
// positions it would as a single TextLine.
func newLayout(screen *apparatus.Screen, fontSize float32, color control.Color) (*layout, error) {
	font, err := control.FontFromMemory(assets_embed.InconsolataFont, fontSize)
	if err != nil {
		return nil, fmt.Errorf("loading stimulus font: %w", err)
	}
	l := &layout{glyphs: make(map[byte]*stimuli.TextLine, 26)}
	for c := byte('A'); c <= 'Z'; c++ {
		t := stimuli.NewTextLine(string(c), 0, 0, color)
		t.Font = font
		if err := stimuli.PreloadVisualOnScreen(screen, t); err != nil {
			return nil, fmt.Errorf("preloading letter %q: %w", c, err)
		}
		l.glyphs[c] = t
	}
	row := stimuli.NewTextLine(strings.Repeat("X", nSlots), 0, 0, color)
	row.Font = font
	if err := stimuli.PreloadVisualOnScreen(screen, row); err != nil {
		return nil, fmt.Errorf("measuring letter row: %w", err)
	}
	defer row.Unload()
	advance := row.Width / nSlots
	for i := range l.centres {
		l.centres[i] = (float32(i) - float32(nSlots-1)/2) * advance
	}
	// TextLine.Height is the font's line box, a little taller than a capital,
	// so one line height leaves a clear gap between alternatives and mask.
	l.dy = l.glyphs['X'].Height
	return l, nil
}

// stimulusScene draws the letters of t.displayed in their slots.
func (l *layout) stimulusScene(t trial) *scene {
	s := &scene{}
	for i := 0; i < nSlots; i++ {
		if c := t.displayed[i]; c != blank {
			s.items = append(s.items, placed{l.glyphs[c], control.Point(l.centres[i], 0)})
		}
	}
	return s
}

// responseScene draws the mask — an X and an O overlaid in every slot, after
// Reicher's "overlapping Xs and Os" — with the two alternatives directly above
// and below the critical slot (+Y is up).
func (l *layout) responseScene(t trial) *scene {
	s := &scene{}
	for i := 0; i < nSlots; i++ {
		s.items = append(s.items,
			placed{l.glyphs['X'], control.Point(l.centres[i], 0)},
			placed{l.glyphs['O'], control.Point(l.centres[i], 0)})
	}
	top, bottom := t.target, t.foil
	if !t.targetOnTop {
		top, bottom = bottom, top
	}
	x := l.centres[t.pos-1]
	s.items = append(s.items,
		placed{l.glyphs[top], control.Point(x, l.dy)},
		placed{l.glyphs[bottom], control.Point(x, -l.dy)})
	return s
}

func (l *layout) unload() {
	for _, g := range l.glyphs {
		_ = g.Unload()
	}
}

// ── Timing helpers ────────────────────────────────────────────────────────────

// framesFor converts a duration in ms to the nearest whole number of display
// refreshes, never fewer than one.
func framesFor(ms int, frameDur time.Duration) int {
	if frameDur <= 0 {
		return 1
	}
	d := time.Duration(ms) * time.Millisecond
	n := int((d + frameDur/2) / frameDur)
	if n < 1 {
		n = 1
	}
	return n
}

// refreshRate returns the display refresh rate in Hz and where it came from.
// Screen.RefreshRate reports 0 when VSync cannot be queried (the normal case
// in the browser), so fall back to inverting the frame duration.
func refreshRate(exp *control.Experiment, frameDur time.Duration) (float64, string) {
	if hz := exp.Screen.RefreshRate(); hz > 0 {
		return float64(hz), "display mode via VSync"
	}
	if frameDur > 0 {
		return float64(time.Second) / float64(frameDur), "derived from frame duration (VSync not queryable)"
	}
	return 0, "unavailable"
}

// ── Session statistics ────────────────────────────────────────────────────────

type tally struct{ n, hits int }

func (t tally) String() string {
	if t.n == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%5.1f%% correct (%d/%d)", 100*float64(t.hits)/float64(t.n), t.hits, t.n)
}

// ── Instructions ──────────────────────────────────────────────────────────────

type texts struct {
	instructions, practice, staircase, main, pause, end string
}

var messages = map[string]texts{
	"en": {
		instructions: "In each trial a string of letters, or a single letter, is flashed very briefly " +
			"and then covered by a pattern of Xs and Os.\n\n" +
			"Two letters then appear, one above and one below the place where one of the letters was. " +
			"Only one of them was shown.\n\n" +
			"Press the UP arrow if it was the TOP letter.\n" +
			"Press the DOWN arrow if it was the BOTTOM letter.\n\n" +
			"Keep your eyes on the cross at the start of each trial. " +
			"If you are not sure, guess. Accuracy matters, not speed.\n\n" +
			"Press the SPACE bar to continue.",
		practice: "First, %d practice trials, shown slowly.\n\n" +
			"After each answer a dot appears: green if you were right, red if you were wrong.\n\n" +
			"Press the SPACE bar to begin.",
		staircase: "Next, single letters only. The flash will get shorter or longer " +
			"depending on your answers, so some trials will be hard. Guess when you must.\n\n" +
			"There is no feedback from here on.\n\n" +
			"Press the SPACE bar to begin.",
		main:  "Now the main part: letter strings and single letters, %d trials.\n\nPress the SPACE bar to begin.",
		pause: "Short break (%d of %d trials done).\n\nPress the SPACE bar to continue.",
		end:   "Thank you!\n\nWords:    %s\nAnagrams: %s\nLetters:  %s\n\nResults saved in %s\n\nPress any key to exit.",
	},
	"fr": {
		instructions: "À chaque essai, une suite de lettres, ou une lettre seule, est flashée très brièvement " +
			"puis recouverte par un motif de X et de O.\n\n" +
			"Deux lettres apparaissent alors, l'une au-dessus et l'autre au-dessous de l'emplacement " +
			"de l'une des lettres. Une seule des deux était présente.\n\n" +
			"Appuyez sur la flèche HAUT si c'était la lettre du HAUT.\n" +
			"Appuyez sur la flèche BAS si c'était la lettre du BAS.\n\n" +
			"Fixez la croix au début de chaque essai. " +
			"En cas de doute, devinez. C'est la précision qui compte, pas la vitesse.\n\n" +
			"Appuyez sur ESPACE pour continuer.",
		practice: "D'abord, %d essais d'entraînement, présentés lentement.\n\n" +
			"Après chaque réponse, un point apparaît : vert si vous avez raison, rouge sinon.\n\n" +
			"Appuyez sur ESPACE pour commencer.",
		staircase: "Ensuite, des lettres seules. La durée du flash va raccourcir ou s'allonger " +
			"selon vos réponses : certains essais seront difficiles. Devinez si nécessaire.\n\n" +
			"Il n'y a plus de retour sur vos réponses à partir d'ici.\n\n" +
			"Appuyez sur ESPACE pour commencer.",
		main:  "Voici la partie principale : suites de lettres et lettres seules, %d essais.\n\nAppuyez sur ESPACE pour commencer.",
		pause: "Courte pause (%d essais sur %d).\n\nAppuyez sur ESPACE pour continuer.",
		end:   "Merci !\n\nMots :        %s\nAnagrammes :  %s\nLettres :     %s\n\nRésultats enregistrés dans %s\n\nAppuyez sur une touche pour quitter.",
	},
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	exp := control.NewExperimentFromFlags("Word-Superiority-Effect", control.Black, control.White, 32)
	defer exp.End()

	msg, ok := messages[*langArg]
	if !ok {
		exp.Fatal("-lang must be en or fr, got %q", *langArg)
	}
	items, source, err := loadItems(*stimFile, *langArg)
	if err != nil {
		exp.Fatal("%v", err)
	}
	if *framesArg < 0 {
		exp.Fatal("-frames must be >= 0, got %d", *framesArg)
	}

	frameDur := exp.Screen.FrameDuration()
	frameMS := frameDur.Seconds() * 1000
	refreshHz, refreshSrc := refreshRate(exp, frameDur)
	practiceFrames := framesFor(practiceMS, frameDur)
	fmt.Printf("Word superiority: %.2f Hz refresh, %s (%.3f ms/frame)\n", refreshHz, refreshSrc, frameMS)
	fmt.Printf("  %d items from %s\n", len(items), source)

	exp.AddDataVariableNames([]string{
		"phase", "trial", "material", "item", "displayed", "crit_pos",
		"target", "foil", "top_letter", "frames", "stim_dur_ms",
		"response", "chosen", "correct", "rt_ms",
	})

	practice := buildPracticeTrials(practiceItems[*langArg])
	trials := buildMainTrials(items)

	exp.Data.WriteComment("--SESSION PARAMETERS")
	for _, line := range []string{
		fmt.Sprintf("language: %s", *langArg),
		fmt.Sprintf("stimulus_list: %s (%d items)", source, len(items)),
		fmt.Sprintf("refresh_hz: %.3f", refreshHz),
		fmt.Sprintf("refresh_hz_source: %s", refreshSrc),
		fmt.Sprintf("frame_duration_ms: %.4f", frameMS),
		fmt.Sprintf("practice_frames: %d (%.2f ms)", practiceFrames, float64(practiceFrames)*frameMS),
		fmt.Sprintf("font: Inconsolata %.0f pt, white on black", *fontSize),
		"mask: X and O overlaid in each of the 4 slots, shown with the alternatives until the response",
		fmt.Sprintf("fixation_ms: %d", *fixMS),
		fmt.Sprintf("iti_ms: %d", *itiMS),
		fmt.Sprintf("n_main_trials: %d", len(trials)),
	} {
		exp.Data.WriteComment(line)
	}

	stats := map[string]*tally{}
	for _, m := range materials {
		stats[m] = &tally{}
	}
	respKeys := []control.Keycode{keyTop, keyBottom}

	err = exp.Run(func() error {
		lay, err := newLayout(exp.Screen, float32(*fontSize), control.White)
		if err != nil {
			return err
		}
		defer lay.unload()

		fixation := stimuli.NewFixCross(fixSize, fixLineWidth, control.White)
		greenDot := stimuli.NewCircle(feedbackRadius, control.Green)
		redDot := stimuli.NewCircle(feedbackRadius, control.Red)

		trialNum := 0

		// runTrial presents one trial at the given number of frames, records
		// it, and returns whether the response was correct.
		runTrial := func(t trial, frames int) (bool, error) {
			stim, resp := lay.stimulusScene(t), lay.responseScene(t)

			if err := exp.Blank(*itiMS); err != nil {
				return false, err
			}
			if err := exp.ShowTimed(fixation, *fixMS); err != nil {
				return false, err
			}

			// The stimulus is followed by the mask on the very next frame.
			// Nothing in between allocates, so GC is held off for both.
			old := debug.SetGCPercent(-1)
			onset, err := exp.ShowFrames(stim, frames)
			var maskOnset uint64
			if err == nil {
				maskOnset, err = exp.ShowTS(resp)
			}
			debug.SetGCPercent(old)
			if err != nil {
				return false, err
			}

			// A key pressed before the alternatives appeared cannot be a
			// choice between them, so it is discarded.
			exp.Keyboard.Clear()
			key, eventTS, err := exp.Keyboard.GetKeyEventTS(respKeys, -1)
			if err != nil {
				return false, err
			}

			top, bottom := t.target, t.foil
			if !t.targetOnTop {
				top, bottom = bottom, top
			}
			chosen, response := top, "up"
			if key == keyBottom {
				chosen, response = bottom, "down"
			}
			correct := chosen == t.target
			stimDurMS := float64(maskOnset-onset) / 1e6
			rt := int64(eventTS-maskOnset) / 1_000_000

			trialNum++
			exp.Data.Add(t.phase, trialNum, t.material, t.item, t.displayed, t.pos,
				string(t.target), string(t.foil), string(top), frames,
				fmt.Sprintf("%.2f", stimDurMS), response, string(chosen), correct, rt)
			fmt.Printf("%-9s %3d  %-7s %s  %c/%c  frames=%d stim=%.2f ms  %-4s correct=%-5t rt=%d ms\n",
				t.phase, trialNum, t.material, t.displayed, t.target, t.foil,
				frames, stimDurMS, response, correct, rt)
			return correct, nil
		}

		if err := exp.ShowInstructions(msg.instructions); err != nil {
			return err
		}

		// ── Practice ──
		if err := exp.ShowInstructions(fmt.Sprintf(msg.practice, len(practice))); err != nil {
			return err
		}
		for _, t := range practice {
			correct, err := runTrial(t, practiceFrames)
			if err != nil {
				return err
			}
			dot := redDot
			if correct {
				dot = greenDot
			}
			if err := exp.ShowTimed(dot, feedbackMS); err != nil {
				return err
			}
		}

		// ── Staircase ──
		frames := *framesArg
		if frames == 0 {
			if err := exp.ShowInstructions(msg.staircase); err != nil {
				return err
			}
			sc := staircase.NewUpDown(staircase.UpDownConfig{
				StartIntensity:         float64(framesFor(staircaseStartMS, frameDur)),
				MinIntensity:           1,
				MaxIntensity:           float64(framesFor(staircaseMaxMS, frameDur)),
				StepUp:                 1,
				StepDown:               1,
				NCorrectDown:           3,
				MaxReversals:           staircaseReversals,
				MaxTrials:              staircaseMaxTrials,
				NReversalsForThreshold: staircaseLastN,
			})
			for !sc.Done() {
				correct, err := runTrial(staircaseTrial(items), int(math.Round(sc.Intensity())))
				if err != nil {
					return err
				}
				sc.Update(correct)
			}
			threshold := sc.Threshold()
			frames = int(math.Round(threshold))
			if frames < 1 {
				frames = 1
			}
			exp.Data.WriteComment(fmt.Sprintf("staircase: 1-up/3-down on single letters with the list's letter pairs, %d trials, %d reversals %v",
				len(sc.History()), sc.NReversals(), sc.Reversals()))
			exp.Data.WriteComment(fmt.Sprintf("staircase_threshold_frames: %.2f (mean of last %d reversals)", threshold, staircaseLastN))
			fmt.Printf("Staircase threshold: %.2f frames -> %d frames (%.2f ms)\n", threshold, frames, float64(frames)*frameMS)
		} else {
			exp.Data.WriteComment("staircase: skipped (-frames given)")
		}
		exp.Data.WriteComment(fmt.Sprintf("main_frames: %d (%.2f ms)", frames, float64(frames)*frameMS))
		if frames == 1 {
			warn := "WARNING: main-task duration is 1 frame, the shortest possible; single letters may be near ceiling"
			exp.Data.WriteComment(warn)
			fmt.Println(warn)
		}

		// ── Main task ──
		if err := exp.ShowInstructions(fmt.Sprintf(msg.main, len(trials))); err != nil {
			return err
		}
		for i, t := range trials {
			if *breakEvery > 0 && i > 0 && i%*breakEvery == 0 {
				if err := exp.ShowInstructions(fmt.Sprintf(msg.pause, i, len(trials))); err != nil {
					return err
				}
			}
			correct, err := runTrial(t, frames)
			if err != nil {
				return err
			}
			stats[t.material].n++
			if correct {
				stats[t.material].hits++
			}
		}

		exp.Data.WriteComment("--RESULTS (main task)")
		for _, m := range materials {
			line := fmt.Sprintf("accuracy_%s: %s", m, stats[m])
			exp.Data.WriteComment(line)
			fmt.Println(line)
		}
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
	_ = exp.ShowEndMessage(fmt.Sprintf(msg.end, stats["word"], stats["anagram"], stats["letter"], exp.Data.FullPath))
}
