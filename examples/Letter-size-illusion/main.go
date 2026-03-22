// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
// Letter Height Superiority Illusion
// Replication of New et al. (2015), Psychonomic Bulletin & Review.
//
// Two experiments investigating whether letters (Exp 1) and words (Exp 2) are
// perceived as taller than matched pseudoletters or mirror-image stimuli.
//
// Usage:
//
//	go run main.go -exp 1 -s <subject_id> [-d]
//	go run main.go -exp 2 -s <subject_id> [-d]
//
// Flags:
//
//	-exp  int    Experiment number: 1 (letters) or 2 (words). Default: 1.
//	-s    int    Subject ID. Default: 0.
//	-d           Development mode: windowed 1024×768, not fullscreen.
//
// NOTE ON STIMULI: Mirror stimuli are rendered by horizontally flipping the
// source texture (SDL FLIP_HORIZONTAL), matching the paper's "vertical mirror
// symmetry transformation". Pseudoletters (Exp 1) and nonwords made of
// pseudoletters (Exp 2) are approximated with vertical flips (FLIP_VERTICAL)
// because the original custom-drawn pseudoletter glyphs are not available in
// standard fonts. Replace these approximations with the actual stimuli from
// the authors for a faithful replication.
//
// NOTE ON FONT: Liberation Serif Regular is used as a metric-compatible
// substitute for Times New Roman. Point sizes are computed to match the visual
// angles reported in the paper (64 cm viewing distance, 17-in 1024×768 screen).

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

//go:embed LiberationSerif-Regular.ttf
var serifFontData []byte

// ─── Viewing geometry ──────────────────────────────────────────────────────────
// Match the original study: 17-in 1024×768 monitor at 64 cm viewing distance.
const (
	viewDistCM  = 64.0
	scrW        = 1024
	scrH        = 768
	scrDiagInch = 17.0

	// Approximate glyph-height ratios for Liberation Serif.
	xhRatioLC  = float64(0.46) // x-height / em  (lowercase)
	capRatioUC = float64(0.68) // cap-height / em (uppercase)
)

// vaToPx converts a vertical visual angle (degrees) to pixels on the
// reference screen described by the constants above.
func vaToPx(deg float64) float32 {
	dpi := math.Sqrt(float64(scrW*scrW+scrH*scrH)) / scrDiagInch
	pitch := 2.54 / dpi
	return float32(2 * viewDistCM * math.Tan(deg*math.Pi/360.0) / pitch)
}

// eccToPx converts an eccentricity angle (degrees from center) to pixels.
func eccToPx(deg float64) float32 {
	dpi := math.Sqrt(float64(scrW*scrW+scrH*scrH)) / scrDiagInch
	pitch := 2.54 / dpi
	return float32(viewDistCM * math.Tan(deg*math.Pi/180.0) / pitch)
}

// fontPt computes the Liberation Serif point size that yields approximately
// the desired ink height (pixels) for lowercase (lc=true) or uppercase text.
func fontPt(targetPx float32, lc bool) float32 {
	r := capRatioUC
	if lc {
		r = xhRatioLC
	}
	return targetPx / float32(r)
}

// ─── Comparison (control) types ────────────────────────────────────────────────
const (
	compLetter  = 0 // same text, no flip
	compMirror  = 1 // same text, FLIP_HORIZONTAL (vertical mirror)
	compPseudo  = 2 // same text, FLIP_VERTICAL   (pseudoletter approximation)
	compRevSyl  = 3 // different text, no flip    (reversed syllables; Exp 2)
	compNonword = 4 // same text, FLIP_VERTICAL   (nonword approximation; Exp 2)
)

var compNames = [5]string{
	"letter", "mirror", "pseudoletter", "reversed_syllable", "nonword",
}

func flipFor(ct int) sdl.FlipMode {
	switch ct {
	case compMirror:
		return sdl.FLIP_HORIZONTAL
	case compPseudo, compNonword:
		return sdl.FLIP_VERTICAL
	}
	return sdl.FLIP_NONE
}

// ─── Texture cache ─────────────────────────────────────────────────────────────

// texEntry holds a GPU texture and its source dimensions.
type texEntry struct {
	tex  *sdl.Texture
	w, h float32
}

// texPair holds textures for one stimulus string at both sizes.
type texPair struct {
	small, tall *texEntry
}

// renderOneText creates a GPU texture from the current font and text.
func renderOneText(exp *control.Experiment, font *ttf.Font, text string) (*texEntry, error) {
	surf, err := font.RenderTextBlended(text, control.Black)
	if err != nil {
		return nil, fmt.Errorf("render %q: %w", text, err)
	}
	w, h := float32(surf.W), float32(surf.H)
	tex, err := exp.Screen.Renderer.CreateTextureFromSurface(surf)
	surf.Destroy()
	if err != nil {
		return nil, fmt.Errorf("create texture %q: %w", text, err)
	}
	return &texEntry{tex: tex, w: w, h: h}, nil
}

// prerenderPairs renders each text string at both smallPt and tallPt font sizes.
// The returned font is left at tallPt; the caller is responsible for closing it.
func prerenderPairs(exp *control.Experiment, texts []string, smallPt, tallPt float32) (map[string]*texPair, *ttf.Font, error) {
	font, err := control.FontFromMemory(serifFontData, smallPt)
	if err != nil {
		return nil, nil, fmt.Errorf("open font at %.1fpt: %w", smallPt, err)
	}

	cache := make(map[string]*texPair, len(texts))
	for _, t := range texts {
		cache[t] = &texPair{}
	}

	// Render at small size.
	for _, text := range texts {
		e, err := renderOneText(exp, font, text)
		if err != nil {
			font.Close()
			return nil, nil, err
		}
		cache[text].small = e
	}

	// Resize and render at tall size.
	if err := font.SetSize(tallPt); err != nil {
		font.Close()
		return nil, nil, fmt.Errorf("set font size %.1fpt: %w", tallPt, err)
	}
	for _, text := range texts {
		e, err := renderOneText(exp, font, text)
		if err != nil {
			font.Close()
			return nil, nil, err
		}
		cache[text].tall = e
	}

	return cache, font, nil
}

// pickTex selects the small or tall texture from a pair.
func pickTex(p *texPair, useSmall bool) *texEntry {
	if useSmall {
		return p.small
	}
	return p.tall
}

// drawTex draws a texEntry centered at screen-center coordinates (cx, cy).
func drawTex(exp *control.Experiment, e *texEntry, cx, cy float32, flip sdl.FlipMode) error {
	sx, sy := exp.Screen.CenterToSDL(cx, cy)
	dst := sdl.FRect{X: sx - e.w/2, Y: sy - e.h/2, W: e.w, H: e.h}
	return exp.Screen.Renderer.RenderTextureRotated(e.tex, nil, &dst, 0.0, nil, flip)
}

// ─── Trial structure ───────────────────────────────────────────────────────────

// Trial encodes one experimental trial.
type Trial struct {
	LeftText, RightText   string
	LeftType, RightType   int  // comparison types (determines flip mode)
	LeftSmall, RightSmall bool // true = render at small font size

	// Metadata logged to the data file.
	AnchorText string
	AnchorPos  string // "left" or "right"
	CompType   int
	HeightCond string // same_small | same_tall | anchor_tall | anchor_small

	// Physical correct response (used only during training for feedback).
	CorrectKey sdl.Keycode
}

// makeTrial builds one Trial from anchor and comparison specifications.
func makeTrial(anchor string, anchorSmall bool,
	comp string, ct int, compSmall bool,
	anchorLeft bool, cond string) Trial {

	t := Trial{AnchorText: anchor, CompType: ct, HeightCond: cond}
	if anchorLeft {
		t.LeftText, t.LeftType, t.LeftSmall = anchor, compLetter, anchorSmall
		t.RightText, t.RightType, t.RightSmall = comp, ct, compSmall
		t.AnchorPos = "left"
	} else {
		t.LeftText, t.LeftType, t.LeftSmall = comp, ct, compSmall
		t.RightText, t.RightType, t.RightSmall = anchor, compLetter, anchorSmall
		t.AnchorPos = "right"
	}

	// Correct key based on physical heights (small < tall).
	switch {
	case anchorSmall == compSmall:
		t.CorrectKey = control.K_DOWN
	case !anchorSmall && compSmall && anchorLeft: // anchor taller, anchor on left
		t.CorrectKey = control.K_LEFT
	case !anchorSmall && compSmall && !anchorLeft: // anchor taller, anchor on right
		t.CorrectKey = control.K_RIGHT
	case anchorSmall && !compSmall && anchorLeft: // comp taller, comp on right
		t.CorrectKey = control.K_RIGHT
	default: // comp taller, comp on left
		t.CorrectKey = control.K_LEFT
	}
	return t
}

// generateTrials creates the 8 height×position configurations for every
// anchor × comparison-type pair. This yields 216 unique trials for Exp 1
// (9 anchors × 3 types × 8 configs) and 288 for Exp 2 (9 × 4 × 8).
func generateTrials(anchors []string, revSylMap map[string]string, cts []int) []Trial {
	var out []Trial
	for _, anchor := range anchors {
		for _, ct := range cts {
			comp := anchor
			if ct == compRevSyl {
				comp = revSylMap[anchor]
			}
			// Same-height: {both-small, both-tall} × {anchor-left, anchor-right}.
			for _, small := range []bool{true, false} {
				cond := "same_tall"
				if small {
					cond = "same_small"
				}
				for _, al := range []bool{true, false} {
					out = append(out, makeTrial(anchor, small, comp, ct, small, al, cond))
				}
			}
			// Different-height: {anchor-tall, anchor-small} × {anchor-left, anchor-right}.
			for _, anchorSmall := range []bool{true, false} {
				cond := "anchor_tall"
				if anchorSmall {
					cond = "anchor_small"
				}
				for _, al := range []bool{true, false} {
					out = append(out, makeTrial(anchor, anchorSmall, comp, ct, !anchorSmall, al, cond))
				}
			}
		}
	}
	return out
}

// ─── Trial runner ──────────────────────────────────────────────────────────────

type trialResult struct {
	key     sdl.Keycode
	rtMs    int64
	correct bool
}

// runTrial executes one trial: fixation → stimuli → response → ITI.
// If feedback is true, correct/wrong is shown after each response (training).
func runTrial(exp *control.Experiment, t Trial,
	cache map[string]*texPair,
	eccPx float32, stimMs int,
	fix *stimuli.FixCross, feedback bool) (trialResult, error) {

	// 1. Fixation cross 200 ms.
	if err := exp.Show(fix); err != nil {
		return trialResult{}, err
	}
	clock.Wait(200)
	exp.Keyboard.Clear() // discard any stale keys before the stimulus appears

	// 2. Two stimuli presented simultaneously for stimMs.
	lt := pickTex(cache[t.LeftText], t.LeftSmall)
	rt := pickTex(cache[t.RightText], t.RightSmall)
	exp.Screen.Clear()
	if err := drawTex(exp, lt, -eccPx, 0, flipFor(t.LeftType)); err != nil {
		return trialResult{}, err
	}
	if err := drawTex(exp, rt, eccPx, 0, flipFor(t.RightType)); err != nil {
		return trialResult{}, err
	}
	exp.Screen.Update()
	clock.Wait(stimMs)

	// 3. Clear screen then collect response — no Keyboard.Clear() here so that
	// presses made just as the screen blanks are never lost.
	exp.Screen.Clear()
	exp.Screen.Update()

	responseKeys := []sdl.Keycode{control.K_LEFT, control.K_RIGHT, control.K_DOWN}
	key, rtMs, err := exp.Keyboard.WaitKeysRT(responseKeys, -1)
	if err != nil {
		return trialResult{}, err
	}
	correct := key == t.CorrectKey

	// Feedback (training phase only).
	if feedback {
		color := control.Red
		msg := "WRONG"
		if correct {
			color = control.Green
			msg = "CORRECT"
		}
		fb := stimuli.NewTextLine(msg, 0, 0, color)
		if err := exp.Show(fb); err != nil {
			return trialResult{}, err
		}
		clock.Wait(600)
		if err := fb.Unload(); err != nil {
			return trialResult{}, err
		}
	}

	// 4. Inter-trial interval 750 ms.
	if err := exp.Blank(750); err != nil {
		return trialResult{}, err
	}

	return trialResult{key: key, rtMs: rtMs, correct: correct}, nil
}

// ─── Training phase ────────────────────────────────────────────────────────────

// runTraining loops over shuffled training trials (letter-letter or word-word
// pairs only) with immediate feedback until the participant reaches 80 % accuracy.
func runTraining(exp *control.Experiment,
	trainAnchors []string, cache map[string]*texPair,
	eccPx float32, stimMs int, fix *stimuli.FixCross) error {

	// Training uses only compLetter pairs (no mirror/pseudo/nonword).
	trials := generateTrials(trainAnchors, nil, []int{compLetter})

	for {
		rand.Shuffle(len(trials), func(i, j int) { trials[i], trials[j] = trials[j], trials[i] })

		nCorr := 0
		for _, t := range trials {
			r, err := runTrial(exp, t, cache, eccPx, stimMs, fix, true)
			if err != nil {
				return err
			}
			if r.correct {
				nCorr++
			}
		}

		acc := float64(nCorr) / float64(len(trials))
		if acc >= 0.80 {
			break
		}

		msg := stimuli.NewTextBox(
			fmt.Sprintf(
				"Training accuracy: %.0f%%\n\n"+
					"You need at least 80%% correct to continue.\n"+
					"Press SPACE to try again.",
				acc*100),
			700, control.Point(0, 0), control.Black)
		if err := exp.Show(msg); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}
		if err := msg.Unload(); err != nil {
			return err
		}
	}
	return nil
}

// waitSpace shows a text box and waits for the SPACE key.
func waitSpace(exp *control.Experiment, text string) error {
	msg := stimuli.NewTextBox(text, 900, control.Point(0, 0), control.Black)
	if err := exp.Show(msg); err != nil {
		return err
	}
	if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
		return err
	}
	return msg.Unload()
}

// ─── Main experiment ───────────────────────────────────────────────────────────

// runExperiment runs the main trial loop, saving data and offering breaks.
func runExperiment(exp *control.Experiment,
	trials []Trial, cache map[string]*texPair,
	eccPx float32, stimMs, breakEvery int,
	fix *stimuli.FixCross) error {

	total := len(trials)
	for i, t := range trials {
		// Offer a break at the prescribed interval (but not at the very start).
		if i > 0 && i%breakEvery == 0 {
			msg := fmt.Sprintf(
				"Break time!\nTrial %d of %d completed.\n\nPress SPACE to continue.",
				i, total)
			if err := waitSpace(exp, msg); err != nil {
				return err
			}
		}

		r, err := runTrial(exp, t, cache, eccPx, stimMs, fix, false)
		if err != nil {
			return err
		}

		respStr := "SAME"
		if r.key == control.K_LEFT {
			respStr = "LEFT"
		} else if r.key == control.K_RIGHT {
			respStr = "RIGHT"
		}

		exp.Data.Add(
			i+1,
			t.AnchorText,
			t.AnchorPos,
			compNames[t.CompType],
			t.HeightCond,
			respStr,
			r.rtMs,
		)
	}
	return nil
}

// ─── main ──────────────────────────────────────────────────────────────────────

func main() {
	expNum := flag.Int("exp", 1, "Experiment number: 1 (letters) or 2 (words)")
	exp := control.NewExperimentFromFlags("Letter Height Superiority Illusion", control.White, control.Black, 20)
	defer exp.End()

	if err := exp.SetLogicalSize(scrW, scrH); err != nil {
		log.Printf("warning: set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"trial", "anchor", "anchor_pos", "comp_type",
		"height_cond", "response", "rt_ms",
	})

	// ── Experiment-specific parameters ──────────────────────────────────────
	var (
		mainAnchors  []string
		trainAnchors []string
		compTypes    []int
		revSylMap    map[string]string
		smallDeg     float64
		tallDeg      float64
		eccDeg       float64
		stimMs       int
		breakEvery   int
		isLC         bool // lowercase (Exp 1) vs uppercase (Exp 2)
	)

	switch *expNum {
	case 1:
		mainAnchors = []string{"a", "c", "e", "m", "r", "s", "v", "w", "z"}
		trainAnchors = []string{"u", "n", "x"}
		compTypes = []int{compLetter, compMirror, compPseudo}
		smallDeg, tallDeg = 0.28, 0.30
		eccDeg = 2.75
		stimMs = 700
		breakEvery = 108
		isLC = true

	case 2:
		mainAnchors = []string{
			"BATEAU", "BUREAU", "CAMION", "CANAL", "GENOU",
			"JARDIN", "LAPIN", "PARFUM", "TUYAU",
		}
		trainAnchors = []string{"RADIO", "PAPIER", "MAISON"}
		compTypes = []int{compLetter, compMirror, compRevSyl, compNonword}
		// Reversed-syllable pseudowords from Appendix of New et al. (2015).
		revSylMap = map[string]string{
			"BATEAU": "TEAUBA",
			"BUREAU": "REAUBU",
			"CAMION": "MIONCA",
			"CANAL":  "NALCA",
			"GENOU":  "NOUGE",
			"JARDIN": "DINJAR",
			"LAPIN":  "PINLA",
			"PARFUM": "FUMPAR",
			"TUYAU":  "YUTAU",
		}
		smallDeg, tallDeg = 0.40, 0.44
		eccDeg = 0.9
		stimMs = 500
		breakEvery = 144
		isLC = false

	default:
		log.Fatalf("unknown experiment %d: use -exp 1 or -exp 2", *expNum)
	}

	// ── Derived geometric parameters ─────────────────────────────────────────
	smallPt := fontPt(vaToPx(smallDeg), isLC)
	tallPt := fontPt(vaToPx(tallDeg), isLC)
	eccPx := eccToPx(eccDeg)

	log.Printf("Exp %d: small=%.2fpt, tall=%.2fpt, ecc=%.1fpx",
		*expNum, smallPt, tallPt, eccPx)

	// ── Run ───────────────────────────────────────────────────────────────────
	err := exp.Run(func() error {
		// ── Instructions ────────────────────────────────────────────────────
		instrText := fmt.Sprintf(
			"Experiment %d – Height Comparison\n\n"+
				"You will see two stimuli side by side.\n"+
				"Compare their HEIGHTS and press:\n\n"+
				"  ←  Left arrow   : LEFT stimulus is taller\n"+
				"  →  Right arrow  : RIGHT stimulus is taller\n"+
				"  ↓  Down arrow   : They are the SAME height\n\n"+
				"First you will practice until you reach 80%% accuracy.\n\n"+
				"Press SPACE to start practice.", *expNum)
		if err := waitSpace(exp, instrText); err != nil {
			return err
		}

		// ── Collect all unique texts to pre-render ───────────────────────────
		seen := make(map[string]bool)
		var allTexts []string
		addText := func(s string) {
			if !seen[s] {
				seen[s] = true
				allTexts = append(allTexts, s)
			}
		}
		for _, a := range mainAnchors {
			addText(a)
		}
		for _, a := range trainAnchors {
			addText(a)
		}
		for _, v := range revSylMap {
			addText(v)
		}

		// ── Pre-render all stimuli at both sizes ─────────────────────────────
		cache, font, err := prerenderPairs(exp, allTexts, smallPt, tallPt)
		if err != nil {
			return fmt.Errorf("prerender: %w", err)
		}
		defer font.Close()

		// ── Fixation cross ───────────────────────────────────────────────────
		fix := stimuli.NewFixCross(15, 2, control.Black)

		// ── Training phase ───────────────────────────────────────────────────
		trainCache := make(map[string]*texPair, len(trainAnchors))
		for _, a := range trainAnchors {
			trainCache[a] = cache[a]
		}
		if err := runTraining(exp, trainAnchors, trainCache, eccPx, stimMs, fix); err != nil {
			return err
		}

		// ── Transition to main experiment ────────────────────────────────────
		if err := waitSpace(exp,
			"Practice complete!\n\n"+
				"The main experiment will now begin.\n"+
				"You will no longer receive feedback.\n\n"+
				"Press SPACE to continue."); err != nil {
			return err
		}

		// ── Generate main trials (unique configs × 3 repetitions) ────────────
		base := generateTrials(mainAnchors, revSylMap, compTypes)
		var allTrials []Trial
		for rep := 0; rep < 3; rep++ {
			block := make([]Trial, len(base))
			copy(block, base)
			rand.Shuffle(len(block), func(i, j int) { block[i], block[j] = block[j], block[i] })
			allTrials = append(allTrials, block...)
		}

		log.Printf("Starting main experiment: %d trials", len(allTrials))

		// ── Main experiment ──────────────────────────────────────────────────
		if err := runExperiment(exp, allTrials, cache, eccPx, stimMs, breakEvery, fix); err != nil {
			return err
		}

		// ── End screen ───────────────────────────────────────────────────────
		return waitSpace(exp,
			"The experiment is complete.\nThank you for your participation!\n\nPress SPACE to exit.")
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
