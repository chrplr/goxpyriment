// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Temporal-Integration-Word-Recognition implements Experiments 1 and 2 from:
//
//	Forget, J., Buiatti, M., & Dehaene, S. (2010). Temporal Integration in
//	Visual Word Recognition. Journal of Cognitive Neuroscience, 22(5), 1054–1068.
//
// A string of letters is split into odd-positioned and even-positioned letters.
// These two components alternate on screen at a variable SOA. Depending on the
// rate, observers either fuse the components into a single word or perceive them
// as two separate words.
//
// Experiment 1 (subjective): after a fixed sequence of 3 cycles + mask, the
// participant reports how many words they perceived: 0, 1, or 2.
//
// Experiment 2 (objective): components alternate indefinitely until the
// participant makes a word / pseudoword lexical decision. RT is measured from
// the onset of the first odd component (the moment all letters have appeared
// at least once).
//
// Stimulus conditions
//
//	Exp 1 — three conditions:
//	  whole_word      merged string is a valid French word; components are nonwords
//	  component_words each component is a valid French word; merged string is a nonword
//	  nonword         neither components nor merged string are French words
//
//	Exp 2 — two lexicality levels × three word lengths (4, 6, 8 letters):
//	  word       merged string is a valid French word
//	  pseudoword merged string is a pronounceable nonword (cross-splice)
//
// Usage:
//
//	go run main.go -exp 1 [-d] [-s <id>]
//	go run main.go -exp 2 [-d] [-s <id>]
package main

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"flag"
	"fmt"
	stdlio "io"
	"log"
	"math"
	"math/rand"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Constants ─────────────────────────────────────────────────────────────────

// stimFrames is the number of VSYNC frames each component is displayed.
// It starts at 1 (≈16.7 ms at 60 Hz) and may be scaled by -timescale.
var stimFrames = 1

const (
	// Trial timing
	fixationDurationMS = 1510
	nCyclesExp1        = 3
	maskString         = "########"

	// Font
	fontSizePt = 20

	// Frame drawn around stimulus area on every flip to reduce apparent motion.
	framePad float32 = 8.0

	// Exp 1: trial counts
	nStimuliPerCondExp1 = 20 // 20 stimuli × 6 SOAs = 120 trials per condition

	// Exp 2: trial counts per (length × lexicality)
	nStimuliPerCellExp2 = 10 // 10 stimuli × 6 SOAs = 60 trials per cell

	// Response keys
	exp1Key0 = sdl.K_0
	exp1Key1 = sdl.K_1
	exp1Key2 = sdl.K_2

	exp2WordKey = sdl.K_F // right index = word
	exp2PwdKey  = sdl.K_J // left index  = pseudoword
)

// soaTable maps each SOA (ms) to its ISI duration in 60 Hz frames.
// SOA = stimFrames (1) + isiFrames; at 60 Hz 1 frame ≈ 16.67 ms.
// These are the base values; -timescale multiplies isiFrames and soaMS.
var soaTable = []struct {
	soaMS     int
	isiFrames int
}{
	{50, 2},
	{67, 3},
	{83, 4},
	{100, 5},
	{117, 6},
	{133, 7},
}

//go:embed assets/words.tsv
var wordsData []byte

// ── Word loading ──────────────────────────────────────────────────────────────

type wordEntry struct {
	upper    string // uppercased stimulus
	nLetters int
}

// loadWords reads words TSV data from r and returns the word list and a lexicon set.
func loadWords(r stdlio.Reader) ([]wordEntry, map[string]bool, error) {
	cr := csv.NewReader(r)
	cr.Comma = '\t'
	records, err := cr.ReadAll()
	if err != nil {
		return nil, nil, err
	}

	lexicon := make(map[string]bool, len(records))
	words := make([]wordEntry, 0, len(records))
	for i, rec := range records {
		if i == 0 || len(rec) < 2 {
			continue
		}
		n, _ := strconv.Atoi(rec[1])
		upper := strings.ToUpper(rec[0])
		words = append(words, wordEntry{upper, n})
		lexicon[upper] = true
	}
	return words, lexicon, nil
}

// byLength returns all words with exactly n letters.
func byLength(words []wordEntry, n int) []wordEntry {
	var out []wordEntry
	for _, w := range words {
		if w.nLetters == n {
			out = append(out, w)
		}
	}
	return out
}

// ── Component string helpers ──────────────────────────────────────────────────

// splitWord splits a word into its odd-indexed (0,2,4,…) and even-indexed
// (1,3,5,…) letters (0-based), corresponding to the paper's "odd" and "even"
// positions respectively.
//
// Example: "CAMPAGNE" → odd="CMAN", even="APGE"
func splitWord(word string) (odd, even string) {
	runes := []rune(word)
	o := make([]rune, 0, len(runes)/2+1)
	e := make([]rune, 0, len(runes)/2)
	for i, r := range runes {
		if i%2 == 0 {
			o = append(o, r)
		} else {
			e = append(e, r)
		}
	}
	return string(o), string(e)
}

// oddDisplayStr builds the display string for the odd component.
// Letters occupy positions 0,2,4,… and fill occupies 1,3,5,…
// so that the string visually aligns with the full merged word.
//
// Example with fill=' ': "CMAN" → "C M A N " (8 chars for a 8-letter merged word)
func oddDisplayStr(letters string, fill rune) string {
	runes := []rune(letters)
	buf := make([]rune, 2*len(runes))
	for i, r := range runes {
		buf[2*i] = r
		buf[2*i+1] = fill
	}
	return string(buf)
}

// evenDisplayStr builds the display string for the even component.
// fill occupies positions 0,2,4,… and letters occupy 1,3,5,…
//
// Example with fill=' ': "APGE" → " A P G E" (8 chars)
func evenDisplayStr(letters string, fill rune) string {
	runes := []rune(letters)
	buf := make([]rune, 2*len(runes))
	for i, r := range runes {
		buf[2*i] = fill
		buf[2*i+1] = r
	}
	return string(buf)
}

// interleave merges odd and even letter strings into the full merged word.
//
// Example: "CMAN" + "APGE" → "CAMPAGNE"
func interleave(oddLetters, evenLetters string) string {
	o := []rune(oddLetters)
	e := []rune(evenLetters)
	buf := make([]rune, len(o)+len(e))
	for i, r := range o {
		buf[2*i] = r
	}
	for i, r := range e {
		buf[2*i+1] = r
	}
	return string(buf)
}

// shuffleRunes returns a copy of s with its runes randomly permuted.
func shuffleRunes(s string, rng *rand.Rand) string {
	runes := []rune(s)
	rng.Shuffle(len(runes), func(i, j int) { runes[i], runes[j] = runes[j], runes[i] })
	return string(runes)
}

// scrambleNotInLexicon shuffles s until the result differs from s and is
// absent from the lexicon. Returns "" if it fails after 30 attempts.
func scrambleNotInLexicon(s string, lexicon map[string]bool, rng *rand.Rand) string {
	for i := 0; i < 30; i++ {
		c := shuffleRunes(s, rng)
		if c != s && !lexicon[c] {
			return c
		}
	}
	return ""
}

// sampleN returns n distinct elements chosen uniformly at random from words.
// If n >= len(words) it returns a shuffled copy of all words.
func sampleN(words []wordEntry, n int, rng *rand.Rand) []wordEntry {
	cp := make([]wordEntry, len(words))
	copy(cp, words)
	rng.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
	if n > len(cp) {
		n = len(cp)
	}
	return cp[:n]
}

// ── Trial ─────────────────────────────────────────────────────────────────────

type trial struct {
	// Content
	oddLetters  string // letters in the odd-position component (no spaces)
	evenLetters string // letters in the even-position component (no spaces)
	merged      string // full merged string (for logging)
	condition   string // "whole_word" | "component_words" | "nonword" | "word" | "pseudoword"
	length      int    // number of letters in the merged string
	// Timing
	soaMS     int
	isiFrames int // ISI between components in 60 Hz frames
}

func (t *trial) oddStim(color sdl.Color, fill rune) *stimuli.TextLine {
	return stimuli.NewTextLine(oddDisplayStr(t.oddLetters, fill), 0, 0, color)
}

func (t *trial) evenStim(color sdl.Color, fill rune) *stimuli.TextLine {
	return stimuli.NewTextLine(evenDisplayStr(t.evenLetters, fill), 0, 0, color)
}

// ── Trial generation ──────────────────────────────────────────────────────────

// expand creates one trial per SOA level for the given (oddLetters, evenLetters, merged, condition).
func expand(oddLetters, evenLetters, merged, condition string) []trial {
	var out []trial
	for _, soa := range soaTable {
		out = append(out, trial{
			oddLetters:  oddLetters,
			evenLetters: evenLetters,
			merged:      merged,
			condition:   condition,
			length:      len([]rune(merged)),
			soaMS:       soa.soaMS,
			isiFrames:   soa.isiFrames,
		})
	}
	return out
}

// buildWholeWordTrials uses 6- and 8-letter words; the merged string is a valid
// word while the two components are nonwords by construction.
func buildWholeWordTrials(words6, words8 []wordEntry, n int, rng *rand.Rand) []trial {
	half := n / 2
	pool := append(sampleN(words6, half, rng), sampleN(words8, n-half, rng)...)
	var out []trial
	for _, w := range pool {
		odd, even := splitWord(w.upper)
		out = append(out, expand(odd, even, w.upper, "whole_word")...)
	}
	return out
}

// buildComponentWordTrials pairs two 4-letter words; each component is a real
// word but the merged 8-letter string is a nonword (verified against lexicon).
func buildComponentWordTrials(words4 []wordEntry, n int, lexicon map[string]bool, rng *rand.Rand) []trial {
	pool := sampleN(words4, n*3, rng) // oversample to handle rejections
	var out []trial
	count := 0
	for i := 0; i+1 < len(pool) && count < n; i += 2 {
		w1 := pool[i].upper
		w2 := pool[i+1].upper
		merged := interleave(w1, w2)
		if lexicon[merged] {
			continue
		}
		out = append(out, expand(w1, w2, merged, "component_words")...)
		count++
	}
	return out
}

// buildNonwordTrials scrambles letter order so that neither component nor
// merged string is a valid word.
func buildNonwordTrials(words4 []wordEntry, n int, lexicon map[string]bool, rng *rand.Rand) []trial {
	pool := sampleN(words4, n*4, rng)
	var out []trial
	count := 0
	for i := 0; i+1 < len(pool) && count < n; i += 2 {
		w1 := scrambleNotInLexicon(pool[i].upper, lexicon, rng)
		w2 := scrambleNotInLexicon(pool[i+1].upper, lexicon, rng)
		if w1 == "" || w2 == "" {
			continue
		}
		merged := interleave(w1, w2)
		if lexicon[merged] {
			continue
		}
		out = append(out, expand(w1, w2, merged, "nonword")...)
		count++
	}
	return out
}

// buildWordTrialsExp2 selects words of each target length for Exp 2.
func buildWordTrialsExp2(words []wordEntry, n int, rng *rand.Rand) []trial {
	var out []trial
	for _, length := range []int{4, 6, 8} {
		pool := sampleN(byLength(words, length), n, rng)
		for _, w := range pool {
			odd, even := splitWord(w.upper)
			out = append(out, expand(odd, even, w.upper, "word")...)
		}
	}
	return out
}

// buildPseudowordTrialsExp2 generates pseudowords by cross-splicing pairs of
// words of the same length. The pseudoword is the merged string; its odd/even
// components are derived by splitting it.
func buildPseudowordTrialsExp2(words []wordEntry, n int, lexicon map[string]bool, rng *rand.Rand) []trial {
	var out []trial
	for _, length := range []int{4, 6, 8} {
		pool := sampleN(byLength(words, length), n*4, rng)
		half := length / 2
		count := 0
		for i := 0; i < len(pool) && count < n; i++ {
			for j := i + 1; j < len(pool) && count < n; j++ {
				r1 := []rune(pool[i].upper)
				r2 := []rune(pool[j].upper)
				if len(r1) != length || len(r2) != length {
					continue
				}
				pseudo := string(r1[:half]) + string(r2[half:])
				if lexicon[pseudo] || len([]rune(pseudo)) != length {
					continue
				}
				odd, even := splitWord(pseudo)
				out = append(out, expand(odd, even, pseudo, "pseudoword")...)
				count++
				break
			}
		}
	}
	return out
}

// ── Frame-accurate display helpers ────────────────────────────────────────────

// drawFrame draws a white rectangle outline around the stimulus area.
func drawFrame(screen *apparatus.Screen, frame *sdl.FRect) {
	if frame == nil {
		return
	}
	_ = screen.Renderer.SetDrawColor(255, 255, 255, 255)
	_ = screen.Renderer.RenderRect(frame)
}

// flipStim clears the screen, draws the frame (if non-nil), draws stim (or
// leaves blank if nil), and flips. Returns the VSYNC onset timestamp in nanoseconds.
func flipStim(screen *apparatus.Screen, stim stimuli.VisualStimulus, frame *sdl.FRect) uint64 {
	_ = screen.Clear()
	drawFrame(screen, frame)
	if stim != nil {
		_ = stim.Draw(screen)
	}
	ts, _ := screen.FlipNS()
	return ts
}

// flipBlank clears, draws the frame (if non-nil), and flips without drawing
// any stimulus. Drains the SDL event queue but does not act on events
// (caller is responsible for ESC / quit).
func flipBlank(screen *apparatus.Screen, frame *sdl.FRect) {
	_ = screen.Clear()
	drawFrame(screen, frame)
	_ = screen.Update()
}

// drainEvents discards pending SDL events and returns true if ESC or quit was seen.
func drainEvents() bool {
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
		if ev.Type == sdl.EVENT_QUIT {
			return true
		}
		if ev.Type == sdl.EVENT_KEY_DOWN && ev.KeyboardEvent().Key == sdl.K_ESCAPE {
			return true
		}
	}
	return false
}

// pollResponse drains SDL events and returns the first key matching respKeys,
// along with its SDL hardware timestamp (ns). Returns 0,0 if nothing matched.
func pollResponse(respKeys []sdl.Keycode) (sdl.Keycode, uint64) {
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
		if ev.Type == sdl.EVENT_QUIT {
			return sdl.K_ESCAPE, 0
		}
		if ev.Type == sdl.EVENT_KEY_DOWN {
			k := ev.KeyboardEvent().Key
			if k == sdl.K_ESCAPE {
				return sdl.K_ESCAPE, 0
			}
			for _, rk := range respKeys {
				if k == rk {
					return rk, ev.KeyboardEvent().Timestamp
				}
			}
		}
	}
	return 0, 0
}

// ── Experiment 1: Subjective Report ──────────────────────────────────────────

func runExp1(exp *control.Experiment, trials []trial, fill rune) error {
	screen := exp.Screen
	kb := exp.Keyboard

	mask := stimuli.NewTextLine(maskString, 0, 0, control.White)
	_ = stimuli.PreloadVisualOnScreen(screen, mask)

	exp.AddDataVariableNames([]string{
		"trial", "condition", "soa_ms", "length",
		"merged", "odd", "even",
		"response", "rt_ms",
	})

	instrText := "In each trial, a brief sequence of letter strings will appear.\n\n" +
		"After each sequence, report how many words you read:\n\n" +
		"  Press  0  — no words\n" +
		"  Press  1  — one word\n" +
		"  Press  2  — two words\n\n" +
		"(Use the main keyboard number row or the numeric keypad.)\n\n" +
		"Press SPACE to begin."
	exp.ShowInstructions(instrText)

	respKeys := []sdl.Keycode{
		sdl.K_0, sdl.K_1, sdl.K_2,
		sdl.K_KP_0, sdl.K_KP_1, sdl.K_KP_2,
	}

	for trialIdx, t := range trials {
		evenSt := t.evenStim(control.White, fill)
		oddSt := t.oddStim(control.White, fill)
		_ = stimuli.PreloadVisualOnScreen(screen, evenSt)
		_ = stimuli.PreloadVisualOnScreen(screen, oddSt)

		// Frame rect: stable border around the stimulus area, drawn on every
		// flip to provide a spatial anchor and reduce apparent motion.
		tlX, tlY := screen.CenterToSDL(-oddSt.Width/2, oddSt.Height/2)
		frame := &sdl.FRect{
			X: tlX - framePad,
			Y: tlY - framePad,
			W: oddSt.Width + 2*framePad,
			H: oddSt.Height + 2*framePad,
		}

		// Fixation: show only the frame so the subject can anchor attention.
		flipStim(screen, nil, frame)
		exp.Wait(fixationDurationMS)

		// Critical RSVP sequence — disable GC for precise frame timing.
		func() {
			old := debug.SetGCPercent(-1)
			defer debug.SetGCPercent(old)

			for cycle := 0; cycle < nCyclesExp1; cycle++ {
				flipStim(screen, evenSt, frame)
				for f := 1; f < stimFrames; f++ {
					flipBlank(screen, frame) // extra frames if stimFrames > 1
				}
				for f := 0; f < t.isiFrames; f++ {
					flipBlank(screen, frame)
				}
				flipStim(screen, oddSt, frame)
				for f := 1; f < stimFrames; f++ {
					flipBlank(screen, frame)
				}
				for f := 0; f < t.isiFrames; f++ {
					flipBlank(screen, frame)
				}
			}
			// Mask.
			flipStim(screen, mask, frame)
			// One blank frame to separate mask from response screen.
			flipBlank(screen, nil)
		}()

		if drainEvents() {
			return control.EndLoop
		}

		// Response: wait for 0 / 1 / 2 (no timeout — participant must respond).
		kb.Clear()
		key, rtMs, err := kb.WaitKeysRT(respKeys, -1)
		if err != nil {
			return err
		}

		resp := ""
		switch key {
		case sdl.K_0, sdl.K_KP_0:
			resp = "0"
		case sdl.K_1, sdl.K_KP_1:
			resp = "1"
		case sdl.K_2, sdl.K_KP_2:
			resp = "2"
		}

		exp.Data.Add(
			trialIdx+1, t.condition, t.soaMS, t.length,
			t.merged, t.oddLetters, t.evenLetters,
			resp, rtMs,
		)

		_ = evenSt.Unload()
		_ = oddSt.Unload()
	}

	exp.ShowInstructions("Experiment finished. Thank you!")
	time.Sleep(2 * time.Second)
	return control.EndLoop
}

// ── Experiment 2: Objective Lexical Decision ──────────────────────────────────

func runExp2(exp *control.Experiment, trials []trial, fill rune) error {
	screen := exp.Screen

	exp.AddDataVariableNames([]string{
		"trial", "condition", "length", "soa_ms",
		"merged", "odd", "even",
		"response", "correct", "rt_ms",
	})

	instrText := fmt.Sprintf(
		"A rapid sequence of letters will appear and keep cycling.\n\n"+
			"Decide as quickly as possible whether the letters form a real word:\n\n"+
			"  %-3s — WORD  (press with right index finger)\n"+
			"  %-3s — NOT A WORD  (press with left index finger)\n\n"+
			"The sequence will stop as soon as you respond.\n\n"+
			"Press SPACE to begin.",
		strings.ToUpper(string(rune(exp2WordKey))),
		strings.ToUpper(string(rune(exp2PwdKey))),
	)
	exp.ShowInstructions(instrText)

	respKeys := []sdl.Keycode{exp2WordKey, exp2PwdKey}

	for trialIdx, t := range trials {
		evenSt := t.evenStim(control.White, fill)
		oddSt := t.oddStim(control.White, fill)
		_ = stimuli.PreloadVisualOnScreen(screen, evenSt)
		_ = stimuli.PreloadVisualOnScreen(screen, oddSt)

		// Frame rect: stable border around the stimulus area.
		tlX, tlY := screen.CenterToSDL(-oddSt.Width/2, oddSt.Height/2)
		frame := &sdl.FRect{
			X: tlX - framePad,
			Y: tlY - framePad,
			W: oddSt.Width + 2*framePad,
			H: oddSt.Height + 2*framePad,
		}

		// Fixation: show only the frame so the subject can anchor attention.
		flipStim(screen, nil, frame)
		exp.Wait(fixationDurationMS)

		// Ongoing alternation — disable GC.
		var (
			rtOnsetNS uint64 // onset of the first odd component
			respKey   sdl.Keycode
			respTS    uint64
		)

		func() {
			old := debug.SetGCPercent(-1)
			defer debug.SetGCPercent(old)

			cycle := 0
			for {
				// Even component (1 frame).
				flipStim(screen, evenSt, frame)
				if k, ts := pollResponse(respKeys); k != 0 {
					respKey, respTS = k, ts
					return
				}

				// ISI after even.
				for f := 0; f < t.isiFrames; f++ {
					flipBlank(screen, frame)
					if k, ts := pollResponse(respKeys); k != 0 {
						respKey, respTS = k, ts
						return
					}
				}

				// Odd component (1 frame).
				oddOnset := flipStim(screen, oddSt, frame)
				if cycle == 0 {
					rtOnsetNS = oddOnset // RT measured from here
				}
				if k, ts := pollResponse(respKeys); k != 0 {
					respKey, respTS = k, ts
					return
				}

				// ISI after odd.
				for f := 0; f < t.isiFrames; f++ {
					flipBlank(screen, frame)
					if k, ts := pollResponse(respKeys); k != 0 {
						respKey, respTS = k, ts
						return
					}
				}

				cycle++
			}
		}()

		// ESC or quit from pollResponse returns K_ESCAPE with ts==0.
		if respKey == sdl.K_ESCAPE {
			return control.EndLoop
		}

		// RT in ms from onset of first odd component.
		rtMs := int64(0)
		if rtOnsetNS > 0 && respTS >= rtOnsetNS {
			rtMs = int64(respTS-rtOnsetNS) / 1_000_000
		}

		resp := "word"
		if respKey == exp2PwdKey {
			resp = "pseudoword"
		}
		correct := 0
		if t.condition == resp {
			correct = 1
		}

		exp.Data.Add(
			trialIdx+1, t.condition, t.length, t.soaMS,
			t.merged, t.oddLetters, t.evenLetters,
			resp, correct, rtMs,
		)

		_ = evenSt.Unload()
		_ = oddSt.Unload()
	}

	exp.ShowInstructions("Experiment finished. Thank you!")
	time.Sleep(2 * time.Second)
	return control.EndLoop
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	fillFlag := flag.String("fill", "X", "placeholder character for alternating letter positions")
	timescaleFlag := flag.Float64("timescale", 1.0, "time scaling factor (1–10): multiplies stimulus and ISI frame counts")
	flag.Parse()

	fillRunes := []rune(*fillFlag)
	if len(fillRunes) != 1 {
		log.Fatalf("-fill must be exactly one character, got %q", *fillFlag)
	}
	fill := fillRunes[0]

	scale := *timescaleFlag
	if scale < 1.0 || scale > 10.0 {
		log.Fatalf("-timescale must be between 1 and 10, got %g", scale)
	}
	stimFrames = max(1, int(math.Round(float64(stimFrames)*scale)))
	for i := range soaTable {
		soaTable[i].isiFrames = max(1, int(math.Round(float64(soaTable[i].isiFrames)*scale)))
		soaTable[i].soaMS = int(math.Round(float64(soaTable[i].soaMS) * scale))
	}

	fields := []control.InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		{
			Name:    "experiment",
			Label:   "Experiment",
			Default: "1",
			Type:    control.FieldSelect,
			Options: []string{"1", "2"},
		},
		control.FullscreenField,
	}
	info, err := control.GetParticipantInfo("Temporal Integration — Word Recognition", fields)
	if err != nil {
		log.Fatalf("dialog: %v", err)
	}

	subjectID, _ := strconv.Atoi(info["subject_id"])
	expNum, _ := strconv.Atoi(info["experiment"])
	fullscreen := info["fullscreen"] == "true"
	width, height := 0, 0
	if !fullscreen {
		width, height = 1024, 768
	}

	exp := control.NewExperiment(
		"Temporal Integration — Word Recognition",
		width, height, fullscreen,
		control.Black, control.White, fontSizePt,
	)
	exp.SubjectID = subjectID
	exp.Info = info
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()
	_ = exp.HideCursor()

	allWords, lexicon, err := loadWords(bytes.NewReader(wordsData))
	if err != nil {
		log.Fatalf("loading words.tsv: %v", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var trials []trial

	switch expNum {
	case 1:
		words4 := byLength(allWords, 4)
		words6 := byLength(allWords, 6)
		words8 := byLength(allWords, 8)

		ww := buildWholeWordTrials(words6, words8, nStimuliPerCondExp1, rng)
		cw := buildComponentWordTrials(words4, nStimuliPerCondExp1, lexicon, rng)
		nw := buildNonwordTrials(words4, nStimuliPerCondExp1, lexicon, rng)

		trials = append(trials, ww...)
		trials = append(trials, cw...)
		trials = append(trials, nw...)

		fmt.Printf("Exp 1: %d whole-word + %d component-word + %d nonword trials = %d total\n",
			len(ww), len(cw), len(nw), len(ww)+len(cw)+len(nw))

	case 2:
		wt := buildWordTrialsExp2(allWords, nStimuliPerCellExp2, rng)
		pw := buildPseudowordTrialsExp2(allWords, nStimuliPerCellExp2, lexicon, rng)

		trials = append(trials, wt...)
		trials = append(trials, pw...)

		fmt.Printf("Exp 2: %d word + %d pseudoword trials = %d total\n",
			len(wt), len(pw), len(wt)+len(pw))

	default:
		log.Fatalf("unknown experiment %d", expNum)
	}

	design.ShuffleList(trials)

	var runErr error
	if err := exp.Run(func() error {
		switch expNum {
		case 1:
			runErr = runExp1(exp, trials, fill)
		case 2:
			runErr = runExp2(exp, trials, fill)
		}
		return runErr
	}); err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
