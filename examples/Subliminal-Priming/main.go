// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Dehaene-Subliminal-Priming replicates the stimulus stream from:
//
//	Dehaene et al. (2001) "Experimental and Theoretical Approaches to
//	Conscious Processing". Neuron.
//
// Participants see a continuous rapid stream of masks and blank screens.
// Embedded within the stream are four-letter words that are either visible
// (surrounded by blank frames) or masked (surrounded by mask frames).
// Two control conditions replace the word with a blank, keeping context
// identical.
//
// Trial types:
//
//	visible_word  – word surrounded by blank context frames (71 ms each)
//	visible_blank – blank surrounded by blank context frames
//	masked_word   – word surrounded by mask context frames (71 ms each)
//	masked_blank  – blank surrounded by mask context frames
//
// Word/blank duration is varied across trials (wordDurationLevelsMs) to
// estimate the effect of presentation duration on detectability.
//
// Each 2400 ms trial embeds N target sequences at 500 ms intervals (N is set
// by the -targets flag, default 1); the rest of the trial is a random filler
// stream (72% masks, 28% blanks, each 43, 57, or 71 ms long).
//
// Data saved per trial: subject_id, trial_num, condition, word,
// word_duration_ms, estimated_word_duration_ms, response, rt_ms, reported_word.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Timing (all durations in ms) ─────────────────────────────────────────────

const (
	contextDurationMs = 71   // each surrounding context event
	targetIntervalMs  = 500  // word-onset to word-onset
	trialDurationMs   = 2400 // total trial duration
	firstWordOnsetMs  = 284  // ~4 × 71 ms of filler before first word

	maskProbability   = 0.72 // P(mask) in the random filler stream
	nMasksInPool      = 12   // number of distinct pre-rendered mask canvases
	maskWidth         = 400  // mask canvas width (px) — covers a 4-letter word
	maskHeight        = 120  // mask canvas height (px)
	maskShapeSize     = 28.0 // outer dimension of each hollow shape (px)
	maskStrokeWidth   = 4.0  // outline stroke width — matched to letter stroke at 64 pt
	maskShapesPerMask = 48   // default shapes per canvas (mix of squares and diamonds)

	wordFontSize = 64 // pt — large enough to read at arm's length

	nTrialsPerType = 10 // trials per (condition × duration) cell → 160 experimental trials total
	nLeadingBlanks = 2  // leading all-filler warm-up trials
)

var fillerDurationsMs = [3]int{43, 57, 71}

// wordDurationLevelsMs lists the target/blank slot durations varied across
// trials to estimate the effect of presentation duration on detectability.
// Values correspond to approximately 1–4 frames at 60 Hz.
var wordDurationLevelsMs = []int{14, 29, 43, 57}

// ── Trial types ───────────────────────────────────────────────────────────────

type trialType int

const (
	visibleWord  trialType = iota // word visible, blank context
	visibleBlank                  // no word, blank context
	maskedWord                    // word masked, mask context
	maskedBlank                   // no word, mask context
)

func (t trialType) String() string {
	return [...]string{"visible_word", "visible_blank", "masked_word", "masked_blank"}[t]
}

type trial struct {
	typ            trialType
	word           string // empty for blank conditions
	wordDurationMs int    // presentation duration of the target/blank slot
}

// ── Stream element ────────────────────────────────────────────────────────────

type streamItem struct {
	stim       stimuli.VisualStimulus
	durationMs int
	isTarget   bool // true for the word/blank target slot (used for timing measurement)
}

// ── Word pool ─────────────────────────────────────────────────────────────────

// fourLetterNouns mirrors the 4-letter noun lists used in the original study
// (translated to English for this implementation).
var fourLetterNouns = []string{
	"LION", "BEAR", "WOLF", "DEER", "FROG", "HAWK", "CRAB", "MOTH",
	"FISH", "WORM", "HAND", "FOOT", "KNEE", "NOSE", "HAIR", "NAIL",
	"MILK", "SALT", "RICE", "CORN", "BEAN", "SOUP", "MEAT", "CAKE",
	"ROAD", "HILL", "LAKE", "FIRE", "SNOW", "RAIN", "WIND", "LEAF",
	"BOOK", "DOOR", "WALL", "BELL", "RING", "LOCK", "KNOT", "ROPE",
}

// ── Mask canvas pool ──────────────────────────────────────────────────────────

// makeMaskPool creates n Canvas stimuli, each filled with a random arrangement
// of hollow squares and hollow diamonds — matching the Dehaene et al. masks.
//
// Every shape has the same outer size (maskShapeSize) and stroke width
// (maskStrokeWidth).  Hollow squares use two concentric filled rectangles
// (outer white, inner black).  Hollow diamonds use two concentric filled
// diamond polygons via stimuli.Shape.
func makeMaskPool(exp *control.Experiment, rng *rand.Rand, n, shapesPerMask int) []*stimuli.Canvas {
	hw := float32(maskWidth) / 2.0
	hh := float32(maskHeight) / 2.0

	// Diamond geometry: outer half-diagonal and inner half-diagonal.
	// The perpendicular distance from center to a diamond edge is r/√2.
	// Removing strokeWidth from that distance gives an inner r of r - sw*√2.
	outerR := float32(maskShapeSize / 2.0)
	innerR := outerR - float32(maskStrokeWidth)*float32(math.Sqrt2)

	outerDiamondPts := []sdl.FPoint{
		{X: 0, Y: outerR},  // top
		{X: outerR, Y: 0},  // right
		{X: 0, Y: -outerR}, // bottom
		{X: -outerR, Y: 0}, // left
	}
	innerDiamondPts := []sdl.FPoint{
		{X: 0, Y: innerR},
		{X: innerR, Y: 0},
		{X: 0, Y: -innerR},
		{X: -innerR, Y: 0},
	}

	outerSq := float32(maskShapeSize)
	innerSq := float32(maskShapeSize - 2*maskStrokeWidth)

	pool := make([]*stimuli.Canvas, n)
	for i := range pool {
		c := stimuli.NewCanvas(maskWidth, maskHeight, control.Black)
		for s := 0; s < shapesPerMask; s++ {
			x := rng.Float32()*float32(maskWidth) - hw
			y := rng.Float32()*float32(maskHeight) - hh
			pos := sdl.FPoint{X: x, Y: y}

			if rng.Intn(2) == 0 {
				// Hollow square: outer white rect punched through by inner black rect.
				outer := stimuli.NewRectangle(x, y, outerSq, outerSq, control.White)
				inner := stimuli.NewRectangle(x, y, innerSq, innerSq, control.Black)
				if err := c.Blit(outer, exp.Screen); err != nil {
					log.Printf("warning: mask blit: %v", err)
				}
				if err := c.Blit(inner, exp.Screen); err != nil {
					log.Printf("warning: mask blit: %v", err)
				}
			} else {
				// Hollow diamond: outer white diamond punched through by inner black diamond.
				outer := stimuli.NewShape(outerDiamondPts, control.White)
				outer.SetPosition(pos)
				inner := stimuli.NewShape(innerDiamondPts, control.Black)
				inner.SetPosition(pos)
				if err := c.Blit(outer, exp.Screen); err != nil {
					log.Printf("warning: mask blit: %v", err)
				}
				if err := c.Blit(inner, exp.Screen); err != nil {
					log.Printf("warning: mask blit: %v", err)
				}
			}
		}
		pool[i] = c
	}
	return pool
}

// ── Stream builders ───────────────────────────────────────────────────────────

// buildTrialStream constructs the sequence of (stimulus, duration) pairs for
// one 2400 ms experimental trial. The filler stream is continuous; nTargets
// target sequences are embedded at 500 ms intervals starting at firstWordOnsetMs.
// wordFont, if non-nil, is applied to the word stimulus so it is rendered at
// the desired size independently of the experiment's default font.
func buildTrialStream(t trial, maskPool []*stimuli.Canvas, blank stimuli.VisualStimulus, wordFont *ttf.Font, rng *rand.Rand, nTargets int) []streamItem {
	// Target stimulus: the word for word conditions, blank otherwise.
	var wordStim stimuli.VisualStimulus = blank
	if t.typ == visibleWord || t.typ == maskedWord {
		tl := stimuli.NewTextLine(t.word, 0, 0, control.White)
		tl.Font = wordFont // apply large font; nil falls back to screen default
		wordStim = tl
	}

	// Context type: blank frames for visible conditions, masks for masked.
	pickContext := func() streamItem {
		if t.typ == visibleWord || t.typ == visibleBlank {
			return streamItem{blank, contextDurationMs, false}
		}
		return streamItem{maskPool[rng.Intn(len(maskPool))], contextDurationMs, false}
	}

	// Random filler: 72% masks, 28% blanks; duration drawn from {43, 57, 71} ms.
	pickFiller := func() streamItem {
		ms := fillerDurationsMs[rng.Intn(len(fillerDurationsMs))]
		if rng.Float64() < maskProbability {
			return streamItem{maskPool[rng.Intn(len(maskPool))], ms, false}
		}
		return streamItem{blank, ms, false}
	}

	var items []streamItem
	elapsed := 0

	// Word onset times: first at firstWordOnsetMs, then every targetIntervalMs.
	wordOnsets := make([]int, nTargets)
	wordOnsets[0] = firstWordOnsetMs
	for i := 1; i < nTargets; i++ {
		wordOnsets[i] = wordOnsets[0] + i*targetIntervalMs
	}

	for _, onset := range wordOnsets {
		// The two context events start 2 × contextDurationMs before the word.
		preContextStart := onset - 2*contextDurationMs

		// Fill with filler events up to the pre-context boundary.
		for elapsed < preContextStart {
			item := pickFiller()
			if elapsed+item.durationMs > preContextStart {
				item.durationMs = preContextStart - elapsed
			}
			if item.durationMs <= 0 {
				break
			}
			items = append(items, item)
			elapsed += item.durationMs
		}
		elapsed = preContextStart // hard-align to avoid drift

		// [context][context][word/blank][context][context]
		for j := 0; j < 2; j++ {
			ctx := pickContext()
			items = append(items, ctx)
			elapsed += ctx.durationMs
		}
		items = append(items, streamItem{wordStim, t.wordDurationMs, true})
		elapsed += t.wordDurationMs
		for j := 0; j < 2; j++ {
			ctx := pickContext()
			items = append(items, ctx)
			elapsed += ctx.durationMs
		}
	}

	// Fill remaining trial time with filler events.
	for elapsed < trialDurationMs {
		item := pickFiller()
		if elapsed+item.durationMs > trialDurationMs {
			item.durationMs = trialDurationMs - elapsed
		}
		if item.durationMs <= 0 {
			break
		}
		items = append(items, item)
		elapsed += item.durationMs
	}

	return items
}

// buildLeadingBlankStream returns a 2400 ms all-filler stream with no target
// events, used as a warm-up at the start of each block.
func buildLeadingBlankStream(maskPool []*stimuli.Canvas, blank stimuli.VisualStimulus, rng *rand.Rand) []streamItem {
	var items []streamItem
	for elapsed := 0; elapsed < trialDurationMs; {
		ms := fillerDurationsMs[rng.Intn(len(fillerDurationsMs))]
		if elapsed+ms > trialDurationMs {
			ms = trialDurationMs - elapsed
		}
		if ms <= 0 {
			break
		}
		var stim stimuli.VisualStimulus
		if rng.Float64() < maskProbability {
			stim = maskPool[rng.Intn(len(maskPool))]
		} else {
			stim = blank
		}
		items = append(items, streamItem{stim, ms, false})
		elapsed += ms
	}
	return items
}

// ── VSYNC-locked stream presenter ────────────────────────────────────────────

// timingEntry records the intended vs actual duration for one stream item.
type timingEntry struct {
	intendedMs int
	actualMs   float64 // actual on-screen duration in ms, computed from SDL VSYNC timestamps
	onsetNS    uint64  // SDL3 nanosecond timestamp of the VSYNC flip that turned this item on
	isTarget   bool
}

// runStream presents a sequence of stream items with hardware VSYNC-locked
// timing. The GC is suspended for the duration of the stream to minimise
// jitter. Each item is positioned at screen centre (0, 0).
// frameDuration is the display's frame period (query once with displayFrameDuration).
// If header is non-nil it is drawn at the top-centre of every frame (e.g. a
// trial counter). After the stream it prints a timing summary to stdout.
// It returns the mean actual duration (ms) of items flagged isTarget, and any error.
func runStream(exp *control.Experiment, items []streamItem, header stimuli.VisualStimulus, frameDuration time.Duration) (float64, error) {
	screen := exp.Screen

	// Pre-load unique stimuli only (avoids redundant texture allocations).
	seen := make(map[stimuli.VisualStimulus]bool)
	for _, item := range items {
		if !seen[item.stim] {
			if err := stimuli.PreloadVisualOnScreen(screen, item.stim); err != nil {
				log.Printf("warning: preload failed: %v", err)
			}
			seen[item.stim] = true
		}
	}

	// Position the header at the top-centre of the screen (20 px below the edge).
	if header != nil {
		if err := stimuli.PreloadVisualOnScreen(screen, header); err != nil {
			log.Printf("warning: preload header failed: %v", err)
		}
		_, h, _ := screen.Size()
		header.SetPosition(sdl.FPoint{X: 0, Y: float32(h)/2 - 20})
	}

	// Suspend GC for precise timing.
	old := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(old)

	// Drain any stale events before starting.
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
	}

	timings := make([]timingEntry, 0, len(items))

	for _, item := range items {
		dur := time.Duration(item.durationMs) * time.Millisecond
		frames := int((dur + frameDuration/2) / frameDuration)
		if frames < 1 {
			frames = 1
		}
		item.stim.SetPosition(sdl.FPoint{X: 0, Y: 0})

		var onsetNS uint64
		for f := 0; f < frames; f++ {
			if err := screen.Clear(); err != nil {
				return 0, err
			}
			if err := item.stim.Draw(screen); err != nil {
				return 0, err
			}
			if header != nil {
				if err := header.Draw(screen); err != nil {
					return 0, err
				}
			}
			if f == 0 {
				// Capture the SDL nanosecond timestamp of the actual VSYNC flip.
				ts, err := screen.FlipNS()
				if err != nil {
					return 0, err
				}
				onsetNS = ts
			} else {
				if err := screen.Update(); err != nil { // VSYNC blocks here
					return 0, err
				}
			}
			// Poll events every frame so ESC is responsive.
			for sdl.PollEvent(&ev) {
				if ev.Type == sdl.EVENT_QUIT {
					return 0, sdl.EndLoop
				}
				if ev.Type == sdl.EVENT_KEY_DOWN && ev.KeyboardEvent().Key == sdl.K_ESCAPE {
					return 0, sdl.EndLoop
				}
			}
		}
		timings = append(timings, timingEntry{intendedMs: item.durationMs, onsetNS: onsetNS, isTarget: item.isTarget})
	}

	// Compute actual durations from consecutive VSYNC flip timestamps.
	// This gives sub-millisecond precision instead of the Go-clock estimate.
	for i := range timings {
		if i+1 < len(timings) {
			timings[i].actualMs = float64(timings[i+1].onsetNS-timings[i].onsetNS) / 1e6
		} else {
			timings[i].actualMs = float64(sdl.TicksNS()-timings[i].onsetNS) / 1e6
		}
	}

	printTimingSummary(timings, frameDuration)

	// Compute mean actual duration of the target slots.
	var targetSum float64
	var targetN int
	for _, t := range timings {
		if t.isTarget {
			targetSum += t.actualMs
			targetN++
		}
	}
	if targetN == 0 {
		return 0, nil
	}
	return targetSum / float64(targetN), nil
}

// printTimingSummary prints min/mean/max actual durations broken down by
// intended duration bucket, so it is easy to spot systematic timing errors.
func printTimingSummary(timings []timingEntry, frameDuration time.Duration) {
	type bucket struct {
		sum, min, max float64
		n             int
	}
	buckets := make(map[int]*bucket)
	for _, t := range timings {
		b := buckets[t.intendedMs]
		if b == nil {
			b = &bucket{min: 1e9}
			buckets[t.intendedMs] = b
		}
		b.sum += t.actualMs
		b.n++
		if t.actualMs < b.min {
			b.min = t.actualMs
		}
		if t.actualMs > b.max {
			b.max = t.actualMs
		}
	}

	frameMs := float64(frameDuration) / float64(time.Millisecond)
	fmt.Printf("[timing]  intended  min    mean   max    n    (frame=%.2fms)\n", frameMs)
	allDurations := append([]int{contextDurationMs,
		fillerDurationsMs[0], fillerDurationsMs[1], fillerDurationsMs[2]},
		wordDurationLevelsMs...)
	for _, intended := range allDurations {
		b := buckets[intended]
		if b == nil {
			continue
		}
		fmt.Printf("[timing]  %4d ms   %5.1f  %5.1f  %5.1f  %4d\n",
			intended, b.min, b.sum/float64(b.n), b.max, b.n)
	}
	fmt.Println()
}

// ── Main ──────────────────────────────────────────────────────────────────────

const defaultFontPath = "assets/font/octin_college_rg.ttf"

func main() {
	nShapes := flag.Int("shapes", maskShapesPerMask, "number of shapes per mask canvas")
	nTargets := flag.Int("targets", 1, "number of target (word/blank) slots per trial")
	fontPath := flag.String("font", "", "path to a TTF font file to use for word stimuli (overrides built-in default)")
	exp := control.NewExperimentFromFlags("Dehaene-Subliminal-Priming", control.Black, control.White, 32)
	defer exp.End()

	if err := exp.SetVSync(1); err != nil {
		log.Printf("warning: could not enable vsync: %v", err)
	}

	exp.AddDataVariableNames([]string{"trial_num", "condition", "word", "word_duration_ms", "estimated_word_duration_ms", "response", "rt_ms", "reported_word"})
	if err := exp.Data.Save(); err != nil {
		log.Fatalf("failed to write data header: %v", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Large font for word stimuli (64 pt, independent of the 32 pt UI font).
	// Priority: -font flag > assets/font/octin_college_rg.ttf > embedded Inconsolata.
	var wordFont *ttf.Font
	var err error
	resolvedPath := *fontPath
	if resolvedPath == "" {
		if _, statErr := os.Stat(defaultFontPath); statErr == nil {
			resolvedPath = defaultFontPath
		}
	}
	if resolvedPath != "" {
		wordFont, err = control.FontFromFile(resolvedPath, wordFontSize)
		if err != nil {
			log.Fatalf("failed to load font %q: %v", resolvedPath, err)
		}
		fmt.Printf("[font] using %q\n", resolvedPath)
	} else {
		wordFont, err = control.FontFromMemory(assets_embed.InconsolataFont, wordFontSize)
		if err != nil {
			log.Fatalf("failed to load word font: %v", err)
		}
		fmt.Println("[font] using embedded Inconsolata")
	}
	defer wordFont.Close()

	// Record which font was used for word stimuli.
	if resolvedPath != "" {
		exp.Data.WriteComment(fmt.Sprintf("word_font: %s %dpt", resolvedPath, wordFontSize))
	} else {
		exp.Data.WriteComment(fmt.Sprintf("word_font: Inconsolata (embedded) %dpt", wordFontSize))
	}
	if err := exp.Data.Save(); err != nil {
		log.Printf("warning: could not write font comment: %v", err)
	}

	// Pre-render mask canvases (must happen after screen initialisation).
	maskPool := makeMaskPool(exp, rng, nMasksInPool, *nShapes)

	// Blank stimulus: zero-size rectangle — draws nothing after screen.Clear().
	blank := stimuli.NewRectangle(0, 0, 0, 0, control.Black)

	// Response prompt shown after every experimental trial.
	seenPrompt := stimuli.NewTextBox(
		"Did you see a word?\n\n[S] Seen     [U] Unseen",
		700, control.FPoint{X: 0, Y: 0}, control.White,
	)

	// Word-report input shown only on "seen" responses.
	wordReport := stimuli.NewTextInput(
		"Which word did you see? (type it and press ENTER)",
		control.FPoint{X: 0, Y: 0},
		500,
		control.RGB(30, 30, 30),
		control.White,
		control.White,
	)

	// Instructions.
	instrText := "SUBLIMINAL PRIMING\n\n" +
		"You will see a rapid stream of shapes and blank screens.\n\n" +
		"Occasionally, a 4-letter word flashes briefly.\n" +
		"Try to silently read each word you notice.\n\n" +
		"After each stream, report whether you saw a word:\n" +
		"   [S] = Seen   [U] = Unseen\n\n" +
		"Press SPACE to begin."
	instr := stimuli.NewTextBox(instrText, 1600, control.FPoint{X: 0, Y: 0}, control.White)
	if err := exp.Show(instr); err != nil {
		if control.IsEndLoop(err) {
			return
		}
		log.Fatalf("instruction error: %v", err)
	}
	if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
		if control.IsEndLoop(err) {
			return
		}
		log.Fatalf("wait key error: %v", err)
	}

	// Shuffle word pool and build a cyclic word dispenser.
	wordPool := make([]string, len(fourLetterNouns))
	copy(wordPool, fourLetterNouns)
	design.ShuffleList(wordPool)
	wordIdx := 0
	nextWord := func() string {
		if wordIdx >= len(wordPool) {
			wordIdx = 0
			design.ShuffleList(wordPool)
		}
		w := wordPool[wordIdx]
		wordIdx++
		return w
	}

	// Build the trial list: leading warm-up blanks + randomised experimental trials.
	type trialEntry struct {
		t         trial
		isLeading bool
	}
	var trialList []trialEntry

	for i := 0; i < nLeadingBlanks; i++ {
		trialList = append(trialList, trialEntry{isLeading: true})
	}

	var expTrials []trialEntry
	for _, tt := range []trialType{visibleWord, visibleBlank, maskedWord, maskedBlank} {
		for _, dur := range wordDurationLevelsMs {
			for i := 0; i < nTrialsPerType; i++ {
				word := ""
				if tt == visibleWord || tt == maskedWord {
					word = nextWord()
				}
				expTrials = append(expTrials, trialEntry{t: trial{typ: tt, word: word, wordDurationMs: dur}})
			}
		}
	}
	design.ShuffleList(expTrials)
	trialList = append(trialList, expTrials...)

	// Query the display refresh rate once, before the trial loop.
	frameDuration := exp.Screen.FrameDuration()
	fmt.Printf("[display] refresh rate: frame ≈ %.2f ms\n",
		float64(frameDuration)/float64(time.Millisecond))

	// Run the experiment stream.
	trialNum := 0
	err = exp.Run(func() error {
		for _, rec := range trialList {
			if rec.isLeading {
				if _, err := runStream(exp, buildLeadingBlankStream(maskPool, blank, rng), nil, frameDuration); err != nil {
					return err
				}
			} else {
				counterLabel := stimuli.NewTextLine(
					fmt.Sprintf("%d / %d", trialNum+1, len(expTrials)),
					0, 0, control.White,
				)
				estimatedWordDurationMs, err := runStream(exp,
					buildTrialStream(rec.t, maskPool, blank, wordFont, rng, *nTargets),
					counterLabel, frameDuration)
				if err != nil {
					return err
				}

				// Post-trial seen/unseen response (RT from prompt VSYNC flip).
				onsetNS, err := exp.ShowNS(seenPrompt)
				if err != nil {
					return err
				}
				key, eventTS, err := exp.Keyboard.WaitKeysEventRT([]control.Keycode{control.K_S, sdl.K_U}, -1)
				rt := int64(eventTS-onsetNS) / 1_000_000
				if err != nil {
					return err
				}
				response := "unseen"
				reportedWord := ""
				if key == control.K_S {
					response = "seen"
					wordReport.UserText = ""
					reportedWord, err = wordReport.Get(exp.Screen, exp.Keyboard)
					if err != nil {
						return err
					}
				}

				exp.Data.Add(trialNum+1, rec.t.typ.String(), rec.t.word, rec.t.wordDurationMs, fmt.Sprintf("%.2f", estimatedWordDurationMs), response, rt, reportedWord)
				trialNum++
				if saveErr := exp.Data.Save(); saveErr != nil {
					log.Printf("warning: data save failed: %v", saveErr)
				}
			}
		}
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
	fmt.Printf("Results saved in %s\n", exp.Data.FullPath)
}
