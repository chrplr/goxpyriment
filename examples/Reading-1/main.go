// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Reading-1 — lexical decision on 5-letter strings whose per-letter visibility
// is sampled independently in four successive time windows.
//
// Each trial presents a 5-letter string for 200 ms, divided into four 50 ms
// windows. In every window all five letters are on screen simultaneously, but
// each letter is drawn independently at high or low contrast (p = 0.5). A trial
// therefore carries 20 independently randomised visibility values — 5 letter
// positions x 4 time windows — all of which are written to the data file, since
// they are the design matrix a reverse-correlation (classification-image)
// analysis regresses the response on.
//
// The four windows are followed by a "#####" mask and a word / non-word lexical
// decision (F = word, J = non-word).
//
// Half the items are frequent 5-letter English words whose five letters are all
// different; the other half are pseudowords obtained by transposing two
// consecutive letters of such a word. Which base word appears in which form is
// counterbalanced across subjects (see buildTrials).
//
// Usage:
//
//	go run ./examples/Reading-1 -w -s 1
//	go run ./examples/Reading-1 -w -s 1 -n 20 -practice 4 -frame 500   # slow, watchable
package main

import (
	_ "embed"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const (
	nPositions = 5 // letters per string

	wordKey    = control.K_F
	nonWordKey = control.K_J

	maskString = "#####"

	// Point size of the letters and the mask. Larger than the experiment's
	// default font so a single letter is comfortably legible at low contrast.
	stimFontSize = 64

	// Fixation cross geometry.
	fixSize      = 20
	fixLineWidth = 3

	// Feedback dot radius used in the practice block.
	feedbackRadius = 8
)

// Practice items, kept apart from stimuli.csv so the practice block does not
// consume any of the 100 experimental base words.
var (
	practiceWords       = []string{"TABLE", "SMILE", "BEACH", "MIXED", "WATER", "BAKER"}
	practicePseudowords = []string{"CORWN", "TARIN", "SOTNE", "CBAIN"}
)

// ── Flags ─────────────────────────────────────────────────────────────────────
//
// Declared at package level so they are registered with the flag package before
// control.NewExperimentFromFlags calls flag.Parse().

var (
	hiLevel     = flag.Int("hi", 255, "High-contrast letter luminance (0-255)")
	loLevel     = flag.Int("lo", 64, "Low-contrast letter luminance (0-255)")
	frameMS     = flag.Int("frame", 50, "Duration of one visibility window, ms")
	nFrames     = flag.Int("nframes", 4, "Number of visibility windows per trial")
	maskMS      = flag.Int("mask", 200, "Mask duration, ms")
	fixMS       = flag.Int("fix", 500, "Fixation cross duration, ms")
	itiMS       = flag.Int("iti", 500, "Blank inter-trial interval, ms")
	timeoutMS   = flag.Int("timeout", 3000, "Response deadline, ms (measured from stimulus onset)")
	nTrials     = flag.Int("n", 100, "Number of experimental trials (capped at the stimulus list size)")
	nPractice   = flag.Int("practice", 8, "Number of practice trials (0 to skip the practice block)")
	stimFile    = flag.String("stim", "", "Stimulus CSV to use instead of the embedded list")
	fontSizeArg = flag.Float64("fontsize", stimFontSize, "Point size of the letters and the mask")
)

// ── Stimulus list ─────────────────────────────────────────────────────────────

// defaultStimuli is the built-in list of 100 base words: frequent 5-letter
// English words whose letters are all different, each with a precomputed
// transposition position that yields a non-word.
//
//go:embed stimuli.csv
var defaultStimuli string

// baseWord is one row of the stimulus list.
type baseWord struct {
	word    string // 5 uppercase letters, all different
	swapPos int    // 1..4: index of the first of the two letters to transpose
}

// loadItems reads the stimulus list. When path is empty the embedded default
// list is used. Rows are "word,swap_pos"; the first row is a header.
func loadItems(path string) ([]baseWord, error) {
	var reader *csv.Reader
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening stimulus file: %w", err)
		}
		defer f.Close()
		reader = csv.NewReader(f)
	} else {
		reader = csv.NewReader(strings.NewReader(defaultStimuli))
	}

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading stimulus file: %w", err)
	}

	var items []baseWord
	for i, rec := range records {
		if i == 0 {
			continue // header
		}
		if len(rec) < 2 {
			log.Printf("Reading-1: skipping malformed row %d: %#v", i+1, rec)
			continue
		}
		w := strings.ToUpper(strings.TrimSpace(rec[0]))
		p, err := strconv.Atoi(strings.TrimSpace(rec[1]))
		if err != nil {
			log.Printf("Reading-1: skipping row %d, bad swap_pos %q", i+1, rec[1])
			continue
		}
		if len([]rune(w)) != nPositions {
			log.Printf("Reading-1: skipping row %d, %q is not %d letters", i+1, w, nPositions)
			continue
		}
		if p < 1 || p > nPositions-1 {
			log.Printf("Reading-1: skipping row %d, swap_pos %d out of range 1..%d", i+1, p, nPositions-1)
			continue
		}
		items = append(items, baseWord{word: w, swapPos: p})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("stimulus list is empty")
	}
	return items, nil
}

// transpose swaps the letters at 1-based positions pos and pos+1.
func transpose(word string, pos int) string {
	r := []rune(word)
	r[pos-1], r[pos] = r[pos], r[pos-1]
	return string(r)
}

// ── Trials ────────────────────────────────────────────────────────────────────

type trial struct {
	item     string // the string actually presented
	base     string // the base word it was derived from
	isWord   bool
	swapPos  int      // 0 for word trials, 1..4 for pseudoword trials
	vis      [][]bool // [window][position]; true = high contrast
	practice bool
}

// buildTrials assigns each base word to the word or the pseudoword condition
// and returns the shuffled trial list.
//
// The form is decided by the parity of (index + subjectID) over the list in its
// file order, so consecutive subject IDs see each base word in the opposite
// form: over a pair of subjects every item contributes to both conditions,
// while within a session no letter string is ever repeated. Only the sequence
// is shuffled, never the list order the parity is computed from.
func buildTrials(items []baseWord, subjectID, n, windows int) []trial {
	if n > len(items) {
		n = len(items)
	}
	trials := make([]trial, 0, n)
	for i, it := range items[:n] {
		t := trial{base: it.word}
		if (i+subjectID)%2 == 0 {
			t.item, t.isWord, t.swapPos = it.word, true, 0
		} else {
			t.item, t.isWord, t.swapPos = transpose(it.word, it.swapPos), false, it.swapPos
		}
		t.vis = randomVisibility(windows)
		trials = append(trials, t)
	}
	design.ShuffleList(trials)
	return trials
}

// buildPracticeTrials makes a shuffled practice list from the hardcoded items,
// alternating words and pseudowords so both response keys are exercised.
func buildPracticeTrials(n, windows int) []trial {
	var trials []trial
	for i := 0; i < n; i++ {
		var t trial
		if i%2 == 0 {
			w := practiceWords[(i/2)%len(practiceWords)]
			t = trial{item: w, base: w, isWord: true, practice: true}
		} else {
			w := practicePseudowords[(i/2)%len(practicePseudowords)]
			t = trial{item: w, base: w, isWord: false, practice: true}
		}
		t.vis = randomVisibility(windows)
		trials = append(trials, t)
	}
	design.ShuffleList(trials)
	return trials
}

// randomVisibility draws the [windows][nPositions] visibility matrix: every
// cell independently high (true) or low (false) with p = 0.5.
func randomVisibility(windows int) [][]bool {
	vis := make([][]bool, windows)
	for f := range vis {
		vis[f] = make([]bool, nPositions)
		for p := range vis[f] {
			vis[f][p] = design.CoinFlip(0.5)
		}
	}
	return vis
}

// correctKeyFor returns the key that counts as correct for a trial.
func correctKeyFor(t trial) control.Keycode {
	if t.isWord {
		return wordKey
	}
	return nonWordKey
}

// ── Per-letter rendering ──────────────────────────────────────────────────────

// letterRow draws the five letters of one visibility window. It is a composite
// VisualStimulus so the window can be presented with Experiment.ShowFrames,
// which redraws before every flip — a frame carrying no draw calls is not
// reliably scanned out under a compositor.
type letterRow struct {
	stimuli.BaseVisual
	letters [nPositions]*stimuli.TextLine
}

func (r *letterRow) Draw(screen *apparatus.Screen) error {
	for _, l := range r.letters {
		if l == nil {
			continue
		}
		if err := l.Draw(screen); err != nil {
			return fmt.Errorf("letterRow.Draw: %w", err)
		}
	}
	return nil
}

func (r *letterRow) Present(screen *apparatus.Screen, clear, update bool) error {
	return stimuli.PresentDrawable(r, screen, clear, update)
}

// letterCentres returns the centre x of each letter of word, in center-based
// coordinates, such that the five separately-drawn letters occupy exactly the
// positions they would in a single TextLine of the whole string.
//
// It measures the rendered width of every prefix rather than assuming a fixed
// advance, so the layout stays correct if the font is replaced by a
// proportional one.
func letterCentres(font *ttf.Font, word string) ([nPositions]float32, error) {
	var centres [nPositions]float32
	r := []rune(word)

	var prefix [nPositions + 1]float32
	for i := 0; i <= len(r); i++ {
		w, _, err := font.StringSize(string(r[:i]))
		if err != nil {
			return centres, fmt.Errorf("measuring prefix %q: %w", string(r[:i]), err)
		}
		prefix[i] = float32(w)
	}

	total := prefix[len(r)]
	for i := 0; i < len(r); i++ {
		centres[i] = -total/2 + (prefix[i]+prefix[i+1])/2
	}
	return centres, nil
}

// letterStimuli builds and preloads the two TextLines (low, high contrast) for
// each letter position. Indexing is [position][0=low, 1=high].
func letterStimuli(screen *apparatus.Screen, font *ttf.Font, word string, centres [nPositions]float32, lo, hi control.Color) ([nPositions][2]*stimuli.TextLine, error) {
	var out [nPositions][2]*stimuli.TextLine
	for i, ch := range []rune(word) {
		for level, col := range [2]control.Color{lo, hi} {
			t := stimuli.NewTextLine(string(ch), centres[i], 0, col)
			t.Font = font
			if err := stimuli.PreloadVisualOnScreen(screen, t); err != nil {
				return out, fmt.Errorf("preloading letter %q: %w", string(ch), err)
			}
			out[i][level] = t
		}
	}
	return out, nil
}

// unloadLetters releases the GPU textures of a trial's letter pool.
func unloadLetters(pool [nPositions][2]*stimuli.TextLine) {
	for i := range pool {
		for level := range pool[i] {
			if pool[i][level] != nil {
				_ = pool[i][level].Unload()
			}
		}
	}
}

// ── Presentation ──────────────────────────────────────────────────────────────

// presentTrial shows the visibility windows followed by the mask, and returns
// the onset of the first window and the onset of the mask, both on the SDL
// nanosecond clock (the same clock as keyboard event timestamps).
//
// GC is disabled for the whole sequence: nothing inside allocates, since every
// texture is already on the GPU and row.letters is a fixed-size array.
func presentTrial(exp *control.Experiment, row *letterRow, pool [nPositions][2]*stimuli.TextLine, vis [][]bool, mask stimuli.VisualStimulus, framesPerWindow, maskFrames int) (stimOnset, maskOnset uint64, err error) {
	old := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(old)

	for f := range vis {
		for p := 0; p < nPositions; p++ {
			level := 0
			if vis[f][p] {
				level = 1
			}
			row.letters[p] = pool[p][level]
		}
		ts, e := exp.ShowFrames(row, framesPerWindow)
		if e != nil {
			return 0, 0, fmt.Errorf("window %d: %w", f+1, e)
		}
		if f == 0 {
			stimOnset = ts
		}
	}

	maskOnset, err = exp.ShowFrames(mask, maskFrames)
	if err != nil {
		return 0, 0, fmt.Errorf("mask: %w", err)
	}
	return stimOnset, maskOnset, nil
}

// refreshRate returns the display refresh rate in Hz and where it came from.
//
// Screen.RefreshRate reports 0 whenever VSync cannot be queried, which is the
// normal case in the browser: SDL's Emscripten backend paces on
// requestAnimationFrame rather than a VSync the renderer can report. Recording
// that 0 in the info file would be worse than useless — it reads as "no
// display" — so fall back to inverting the frame duration, which comes from the
// display mode and is correct on both platforms, and say which was used.
func refreshRate(exp *control.Experiment, frameDur time.Duration) (float64, string) {
	if hz := exp.Screen.RefreshRate(); hz > 0 {
		return float64(hz), "display mode via VSync"
	}
	if frameDur > 0 {
		return float64(time.Second) / float64(frameDur), "derived from frame duration (VSync not queryable)"
	}
	return 0, "unavailable"
}

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

// ── Session statistics ────────────────────────────────────────────────────────

type sessionStats struct {
	wordTrials, wordHits     int
	pseudoTrials, pseudoHits int
	wordRTs, pseudoRTs       []int64
}

func (s *sessionStats) record(isWord, correct bool, rt int64) {
	if isWord {
		s.wordTrials++
		if correct {
			s.wordHits++
			s.wordRTs = append(s.wordRTs, rt)
		}
		return
	}
	s.pseudoTrials++
	if correct {
		s.pseudoHits++
		s.pseudoRTs = append(s.pseudoRTs, rt)
	}
}

func (s *sessionStats) summary() string {
	rate := func(hits, total int) float64 {
		if total == 0 {
			return 0
		}
		return 100 * float64(hits) / float64(total)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Results\n\n")
	fmt.Fprintf(&b, "Words:        %.1f%% correct (%d/%d), median RT %s\n",
		rate(s.wordHits, s.wordTrials), s.wordHits, s.wordTrials, medianStr(s.wordRTs))
	fmt.Fprintf(&b, "Pseudowords:  %.1f%% correct (%d/%d), median RT %s\n",
		rate(s.pseudoHits, s.pseudoTrials), s.pseudoHits, s.pseudoTrials, medianStr(s.pseudoRTs))
	return b.String()
}

// medianStr renders the median of a slice of RTs, or "n/a" when it is empty.
func medianStr(values []int64) string {
	n := len(values)
	if n == 0 {
		return "n/a (no correct responses)"
	}
	sorted := make([]int64, n)
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var m float64
	if n%2 == 1 {
		m = float64(sorted[n/2])
	} else {
		m = float64(sorted[n/2-1]+sorted[n/2]) / 2
	}
	return fmt.Sprintf("%.0f ms", m)
}

// ── Main ──────────────────────────────────────────────────────────────────────

const instructions = "You will see a string of five letters, flashed very briefly.\n\n" +
	"Some of the letters will be faint, and which ones are faint changes during the flash. " +
	"This is normal — just report what you saw.\n\n" +
	"Decide whether the string was a real English word.\n\n" +
	"If it WAS a word, press 'F'.\n\n" +
	"If it was NOT a word, press 'J'.\n\n" +
	"Answer as quickly and as accurately as you can.\n\n" +
	"Press the SPACE bar to start."

func main() {
	exp := control.NewExperimentFromFlags("Reading-1", control.Black, control.White, 32)
	defer exp.End()

	items, err := loadItems(*stimFile)
	if err != nil {
		exp.Fatal("%v", err)
	}
	if *nFrames < 1 {
		exp.Fatal("-nframes must be >= 1, got %d", *nFrames)
	}

	hi := control.RGB(uint8(*hiLevel), uint8(*hiLevel), uint8(*hiLevel))
	lo := control.RGB(uint8(*loLevel), uint8(*loLevel), uint8(*loLevel))

	frameDur := exp.Screen.FrameDuration()
	refreshHz, refreshSrc := refreshRate(exp, frameDur)
	framesPerWindow := framesFor(*frameMS, frameDur)
	maskFrames := framesFor(*maskMS, frameDur)
	windowMS := float64(framesPerWindow) * frameDur.Seconds() * 1000
	totalMS := float64(*nFrames) * windowMS

	fmt.Printf("Reading-1: %.2f Hz refresh, %s (%.3f ms/frame)\n", refreshHz, refreshSrc, frameDur.Seconds()*1000)
	fmt.Printf("  visibility window: %d ms requested -> %d refreshes = %.2f ms\n", *frameMS, framesPerWindow, windowMS)
	fmt.Printf("  %d windows -> %.2f ms total stimulus\n", *nFrames, totalMS)
	fmt.Printf("  mask: %d ms requested -> %d refreshes = %.2f ms\n", *maskMS, maskFrames, float64(maskFrames)*frameDur.Seconds()*1000)

	// Data columns. subject_id is prepended automatically.
	cols := []string{
		"trial", "item", "base_word", "condition", "swap_pos",
		"key", "response", "correct", "rt_ms", "stim_dur_ms",
	}
	for f := 1; f <= *nFrames; f++ {
		for p := 1; p <= nPositions; p++ {
			cols = append(cols, fmt.Sprintf("f%dp%d", f, p))
		}
	}
	exp.AddDataVariableNames(cols)

	trials := buildTrials(items, exp.SubjectID, *nTrials, *nFrames)
	practice := buildPracticeTrials(*nPractice, *nFrames)

	// Session constants go to the -info.txt companion, so a session is
	// reproducible from its own data file. WriteComment is the call that
	// actually reaches that file; Experiment.AddExperimentInfo only appends to
	// a slice in the Design that nothing writes out.
	exp.Data.WriteComment("--SESSION PARAMETERS")
	for _, line := range []string{
		fmt.Sprintf("stimulus_list: %s", stimListName(*stimFile)),
		fmt.Sprintf("high_luminance: %d", *hiLevel),
		fmt.Sprintf("low_luminance: %d", *loLevel),
		"background: black (0,0,0)",
		fmt.Sprintf("refresh_hz: %.3f", refreshHz),
		fmt.Sprintf("refresh_hz_source: %s", refreshSrc),
		fmt.Sprintf("frame_duration_ms: %.4f", frameDur.Seconds()*1000),
		fmt.Sprintf("window_requested_ms: %d", *frameMS),
		fmt.Sprintf("window_refreshes: %d", framesPerWindow),
		fmt.Sprintf("window_actual_ms: %.4f", windowMS),
		fmt.Sprintf("n_windows: %d", *nFrames),
		fmt.Sprintf("stimulus_actual_ms: %.4f", totalMS),
		fmt.Sprintf("mask_string: %s", maskString),
		fmt.Sprintf("mask_refreshes: %d", maskFrames),
		fmt.Sprintf("mask_actual_ms: %.4f", float64(maskFrames)*frameDur.Seconds()*1000),
		fmt.Sprintf("response_deadline_ms: %d", *timeoutMS),
		fmt.Sprintf("stimulus_font_size_pt: %.0f", *fontSizeArg),
		fmt.Sprintf("n_trials: %d", len(trials)),
		fmt.Sprintf("n_practice: %d", len(practice)),
		"form_assignment: base word i is shown intact when (i + subject_id) is even, transposed otherwise",
		fmt.Sprintf("visibility: each of the %d x %d cells drawn high (1) or low (0) independently, p = 0.5", *nFrames, nPositions),
	} {
		exp.Data.WriteComment(line)
	}
	exp.Data.WriteComment("--TRIAL DATA")

	var stats sessionStats
	responseKeys := []control.Keycode{wordKey, nonWordKey}

	err = exp.Run(func() error {
		font, err := control.FontFromMemory(assets_embed.InconsolataFont, float32(*fontSizeArg))
		if err != nil {
			return fmt.Errorf("loading stimulus font: %w", err)
		}
		defer font.Close()

		// The mask and the letters share the layout, so both are measured with
		// the same font. Every item is 5 letters, so one set of centres serves
		// the whole session with a monospace font; letterCentres is still called
		// per item so a proportional font would place letters correctly.
		mask := stimuli.NewTextLine(maskString, 0, 0, hi)
		mask.Font = font
		if err := stimuli.PreloadVisualOnScreen(exp.Screen, mask); err != nil {
			return fmt.Errorf("preloading mask: %w", err)
		}
		defer mask.Unload()

		fixation := stimuli.NewFixCross(fixSize, fixLineWidth, control.White)
		greenDot := stimuli.NewCircle(feedbackRadius, control.Green)
		redDot := stimuli.NewCircle(feedbackRadius, control.Red)

		row := &letterRow{}

		// runTrial presents one trial and returns the key, the RT in ms measured
		// from stimulus onset, the measured stimulus duration in ms, and whether
		// the response was correct. A zero key means the deadline passed.
		runTrial := func(t trial) (control.Keycode, int64, float64, bool, error) {
			centres, err := letterCentres(font, t.item)
			if err != nil {
				return 0, 0, 0, false, err
			}
			pool, err := letterStimuli(exp.Screen, font, t.item, centres, lo, hi)
			if err != nil {
				return 0, 0, 0, false, err
			}
			defer unloadLetters(pool)

			if err := exp.Blank(*itiMS); err != nil {
				return 0, 0, 0, false, err
			}
			if err := exp.ShowTimed(fixation, *fixMS); err != nil {
				return 0, 0, 0, false, err
			}

			// Cleared before the first flip, so a key pressed during the
			// windows or the mask is still retrieved afterwards, carrying its
			// own hardware timestamp.
			exp.Keyboard.Clear()

			stimOnset, maskOnset, err := presentTrial(exp, row, pool, t.vis, mask, framesPerWindow, maskFrames)
			if err != nil {
				return 0, 0, 0, false, err
			}
			if err := exp.Screen.Clear(); err != nil {
				return 0, 0, 0, false, err
			}
			if _, err := exp.Screen.FlipTS(); err != nil {
				return 0, 0, 0, false, err
			}

			stimDurMS := float64(maskOnset-stimOnset) / 1e6

			// The deadline runs from stimulus onset, so subtract the time
			// already spent on the windows and the mask.
			remaining := *timeoutMS
			if remaining >= 0 {
				elapsed := int((control.TicksNS() - stimOnset) / 1e6)
				remaining -= elapsed
				if remaining < 0 {
					remaining = 0
				}
			}
			key, eventTS, err := exp.Keyboard.GetKeyEventTS(responseKeys, remaining)
			if err != nil {
				return 0, 0, stimDurMS, false, err
			}
			var rt int64 = -1
			if key != 0 {
				rt = int64(eventTS-stimOnset) / 1e6
			}
			return key, rt, stimDurMS, key == correctKeyFor(t), nil
		}

		showFeedback := func(correct bool) {
			if correct {
				_ = exp.ShowTimed(greenDot, 600)
			} else {
				_ = exp.ShowTimed(redDot, 600)
			}
		}

		if err := exp.ShowInstructions(instructions); err != nil {
			return err
		}

		if len(practice) > 0 {
			msg := fmt.Sprintf("First, %d practice trials.\n\n"+
				"After each answer a dot appears: green if you were right, "+
				"red if you were wrong or too slow.\n\n"+
				"Press the SPACE bar to begin.", len(practice))
			if err := exp.ShowInstructions(msg); err != nil {
				return err
			}
			for _, t := range practice {
				_, _, _, correct, err := runTrial(t)
				if err != nil {
					return err
				}
				showFeedback(correct)
			}
			if err := exp.ShowInstructions("End of practice.\n\n" +
				"The real experiment begins now. There is no feedback from here on.\n\n" +
				"Press the SPACE bar to start."); err != nil {
				return err
			}
		}

		for i, t := range trials {
			key, rt, stimDurMS, correct, err := runTrial(t)
			if err != nil {
				return err
			}
			stats.record(t.isWord, correct, rt)

			row := []interface{}{
				i + 1, t.item, t.base, condName(t.isWord), t.swapPos,
				int(key), respName(key), correct, rt, fmt.Sprintf("%.2f", stimDurMS),
			}
			for f := range t.vis {
				for p := range t.vis[f] {
					row = append(row, b2i(t.vis[f][p]))
				}
			}
			exp.Data.Add(row...)

			fmt.Printf("trial %3d/%d  %s  %-6s  resp=%-8s correct=%-5t rt=%5d ms  stim=%.2f ms\n",
				i+1, len(trials), t.item, condName(t.isWord), respName(key), correct, rt, stimDurMS)
		}

		summary := stats.summary()
		fmt.Print(summary)
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}

	// The end message is shown outside Run so it survives an early ESC exit
	// having still written whatever data was collected.
	if control.IsEndLoop(err) || err == nil {
		_ = exp.ShowEndMessage(stats.summary() +
			fmt.Sprintf("\nResults saved in %s\n", exp.Data.FullPath) +
			"\nPress any key to exit.")
	}
}

// ── Small helpers ─────────────────────────────────────────────────────────────

func condName(isWord bool) string {
	if isWord {
		return "word"
	}
	return "pseudo"
}

func respName(key control.Keycode) string {
	switch key {
	case wordKey:
		return "word"
	case nonWordKey:
		return "nonword"
	default:
		return "none"
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func stimListName(path string) string {
	if path == "" {
		return "embedded stimuli.csv"
	}
	return path
}
