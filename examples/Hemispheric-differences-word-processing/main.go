// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// Hemispheric asymmetries in the time course of recognition memory
// (Federmeier & Benjamin, 2005, Psychonomic Bulletin & Review, 12(6), 993-998).
//
// A continuous recognition memory task with lateralized study items and central
// test probes. The whole experiment is ONE stream of presentations in which
// study and test items are intermingled:
//
//   - a studied word appears for 200 ms in the left or right visual field
//     (LVF/RVF), biasing initial processing to the right or left hemisphere,
//     and the participant only memorizes it -- no response;
//   - that same word reappears at the centre of the screen exactly `lag`
//     presentations later, and the participant judges it old or new;
//   - unstudied words appear centrally, once each, scattered through the same
//     stream, so old and new probes are indistinguishable in advance.
//
// Lag is therefore counted in words since study (1, 2, 3, 5, 7, 10, 20, 30, 50),
// not in seconds -- it is the paper's independent variable, and the prediction
// is that the RVF/LH advantage seen at short lags attenuates and reverses at
// long ones.
//
// A fixation cross sits 0.5 degrees below the vertical centre and stays on the
// screen for the entire run; the participant is asked never to look away from it.
//
// Departures from the original are listed in README.md; the main one is that eye
// movements are not monitored, so study trials contaminated by a saccade to the
// lateralized word cannot be excluded as they were in the paper.

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strings"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/units"
)

// Word pools, embedded so the browser build -- which has no filesystem -- works
// exactly like the desktop one. -stimuli / -practice-stimuli override them.
//
//go:embed words.txt
var embeddedWords string

//go:embed practice_words.txt
var embeddedPracticeWords string

// Timing, in milliseconds (paper, p. 995).
const (
	StudyDurationMs = 200  // lateralized word duration
	StudyISIMs      = 2300 // fixation-only interval after a study word
	TestISIMs       = 2500 // fixation-only interval after a response
)

// Geometry, in degrees of visual angle (paper, p. 995).
const (
	// WordEccentricityDeg is the distance from the horizontal centre to the
	// NEAREST edge of a lateralized word, so eccentricity does not drift with
	// word length.
	WordEccentricityDeg = 2.0
	// FixationBelowDeg is how far below the vertical centre the cross sits.
	FixationBelowDeg = 0.5
	FixationSizeDeg  = 0.4
	// TargetWordWidthDeg is the width aimed for on a five-letter word, the
	// middle of the paper's "2 to 3 degrees depending on word length".
	TargetWordWidthDeg = 2.5
	// fontFitReference is the five-letter word the font size is fitted to.
	fontFitReference = "HOUSE"
)

// nLists is the number of distinct item-to-condition assignments, as in the
// paper's 16 experimental lists.
const nLists = 16

func main() {
	shortRun := flag.Bool("short", false,
		"Reduced run for development: 2 studied words per lag instead of 16 (66 presentations, ~3.5 min)")
	screenWidthCm := flag.Float64("screen-width", 52.0,
		"Physical width of the display in cm (used to convert degrees of visual angle to pixels)")
	viewingDistanceCm := flag.Float64("viewing-distance", 100.0,
		"Viewing distance in cm (the paper seated participants at 100 cm)")
	stimuliPath := flag.String("stimuli", "",
		"Word-list file to use instead of the embedded words.txt")
	practicePath := flag.String("practice-stimuli", "",
		"Practice-name file to use instead of the embedded practice_words.txt")
	noPractice := flag.Bool("no-practice", false, "Skip the practice run")
	fitWindow := flag.Bool("fit-window", false,
		"Shrink every visual angle so the stimuli fit a small window. Makes a session a preview, "+
			"not a replication: the presented angles are no longer the paper's, and the data file says so")
	seed := flag.Int("seed", -1,
		"List index to use instead of the one derived from the subject ID (see the counterbalancing note in README.md)")

	exp := control.NewExperimentFromFlags("Hemispheric-differences-word-processing",
		control.White, control.Black, 32) // black words on a uniform white background
	defer exp.End()

	words, err := loadWords(embeddedWords, *stimuliPath)
	if err != nil {
		exp.Fatal("word list: %v", err)
	}
	practiceWords, err := loadWords(embeddedPracticeWords, *practicePath)
	if err != nil {
		exp.Fatal("practice word list: %v", err)
	}

	spec := PaperSpec
	if *shortRun {
		spec = ShortSpec
	}
	if len(words) < spec.NWords() {
		exp.Fatal("word list has %d words, the design needs %d", len(words), spec.NWords())
	}

	// Counterbalancing, following the paper's 16 lists plus 16 VF-reversed
	// matched lists. Three independent bits of the subject ID are used, so that
	// visual field and response hand are not confounded with each other:
	//
	//	bit 0 -> visual fields swapped, giving the matched list
	//	bit 1 -> which hand says "old"
	//	bits 2+ -> which of the 16 item-to-condition assignments is used
	//
	// The RNG is seeded with the LIST index, not the subject ID, so subjects 0
	// and 1 get the identical stream with every visual field flipped -- the
	// point of the paper's matched lists, and what makes each word appear in
	// each visual field across participants.
	listIdx := (exp.SubjectID / 4) % nLists
	vfReversed := exp.SubjectID%2 == 1
	oldOnRightHand := (exp.SubjectID/2)%2 == 1
	if *seed >= 0 {
		listIdx = *seed
		vfReversed = false
	}

	oldKey, newKey := control.K_F, control.K_J
	oldHand, newHand := "left", "right"
	if oldOnRightHand {
		oldKey, newKey = control.K_J, control.K_F
		oldHand, newHand = "right", "left"
	}
	respKeys := []control.Keycode{oldKey, newKey}

	stream, err := BuildStream(words, spec, rand.New(rand.NewSource(int64(listIdx))))
	if err != nil {
		exp.Fatal("building the stream: %v", err)
	}
	practiceStream, err := BuildStream(practiceWords, PracticeSpec,
		rand.New(rand.NewSource(int64(listIdx)+nLists)))
	if err != nil {
		exp.Fatal("building the practice stream: %v", err)
	}
	if vfReversed {
		swapVisualFields(stream)
		swapVisualFields(practiceStream)
	}

	exp.AddDataVariableNames([]string{
		"block",      // "practice" or "main"
		"stream_pos", // 1-based position in the stream
		"event",      // "study" (lateralized) or "test" (central)
		"word",
		"pair_id",  // links a study event to its test event; NA for unstudied words
		"vf",       // LVF / RVF for a studied word's two events; NA for unstudied
		"lag",      // presentations since study, on the test of a studied word
		"is_old",   // on a test event: was the word studied earlier in the stream?
		"response", // "old" / "new"
		"key",      // the physical key, so the hand counterbalancing is auditable
		"rt_ms",    // from the stimulus flip to the key event, both SDL timestamps
		"correct",
		"onset_ms", // stimulus onset relative to the first event of the block
	})

	// Written with WriteComment rather than AddExperimentInfo: the latter only
	// stores the strings on the design object, which nothing ever writes out.
	// These land in the session's -info.txt, flushed when the data file is
	// finalized, so a data set carries the design it was collected under.
	taskInfo(exp, "--TASK INFO")
	taskInfo(exp, fmt.Sprintf("task design: %d lags x %d studied words + %d unstudied = %d presentations",
		len(spec.Lags), spec.PerLag, spec.NNew, spec.Len()))
	taskInfo(exp, fmt.Sprintf("task lags (presentations since study): %v", spec.Lags))
	taskInfo(exp, fmt.Sprintf("task list: %d of %d, visual fields reversed: %t", listIdx, nLists, vfReversed))
	taskInfo(exp, fmt.Sprintf("task response: old = %s hand (%s), new = %s hand (%s)",
		oldHand, keyName(oldKey), newHand, keyName(newKey)))
	taskInfo(exp, fmt.Sprintf("task word pool: %d words, %d practice names", len(words), len(practiceWords)))
	taskInfo(exp, fmt.Sprintf("task viewing distance: %.1f cm, screen width: %.1f cm",
		*viewingDistanceCm, *screenWidthCm))

	err = exp.Run(func() error {
		// Pixel density comes from the DISPLAY's native resolution, not the
		// window's, so degrees convert correctly in windowed mode too. No
		// logical size is set anywhere in this program: logical-size scaling
		// would decouple these coordinates from real pixels and silently
		// invalidate every conversion below.
		widthPx, heightPx, err := pixelExtent(exp)
		if err != nil {
			return err
		}
		heightCm := *screenWidthCm * float64(heightPx) / float64(widthPx)
		mon := units.NewMonitor(*screenWidthCm, heightCm, widthPx, heightPx, *viewingDistanceCm)
		if err := mon.Validate(); err != nil {
			return fmt.Errorf("monitor geometry: %w", err)
		}
		g := geometry{mon: mon, scale: 1}

		// Size the font so that a five-letter word subtends TargetWordWidthDeg,
		// then record what was actually achieved rather than what was asked for.
		fontSize, wPx, hPx, err := fitFontToWidth(exp, g, fontFitReference, TargetWordWidthDeg)
		if err != nil {
			return fmt.Errorf("fitting the font: %w", err)
		}

		// A lateralized word must fit between the centre and the edge of the
		// drawing area. If it does not, the words are cropped -- silently, since
		// SDL happily draws past the edge -- so the run is refused rather than
		// collecting data on stimuli the participant only half saw.
		widest, err := widestWordPx(exp, stream, practiceStream)
		if err != nil {
			return fmt.Errorf("measuring the widest word: %w", err)
		}
		areaW, _ := exp.DrawArea()
		if need, have := halfExtentPx(g, widest), areaW/2; need > have {
			if !*fitWindow {
				return fmt.Errorf(
					"the paper's geometry does not fit this drawing area: a lateralized word needs %.0f px "+
						"from the centre (%.1f deg eccentricity + %.0f px of word) but only %.0f px are available.\n"+
						"At %.0f cm the display would have to be at least %.1f cm wide, or %.0f px across at this density.\n"+
						"Fix one of: run fullscreen (drop -w); pass the display's true width with -screen-width "+
						"(currently %.1f cm); reduce -viewing-distance; or pass -fit-window to shrink every visual "+
						"angle to fit, which makes the session a preview rather than a replication",
					need, WordEccentricityDeg, widest, have,
					*viewingDistanceCm, 2*mon.PxToCm(float64(need)), 2*need, *screenWidthCm)
			}
			// Every spatial quantity, the font included, scales with g.scale, so
			// the required extent scales with it too: one division sizes it, and
			// 0.95 leaves a margin against rounding at the edge.
			g.scale = 0.95 * float64(have) / float64(need)
			if fontSize, wPx, hPx, err = fitFontToWidth(exp, g, fontFitReference, TargetWordWidthDeg); err != nil {
				return fmt.Errorf("refitting the font for -fit-window: %w", err)
			}
			if widest, err = widestWordPx(exp, stream, practiceStream); err != nil {
				return fmt.Errorf("re-measuring the widest word: %w", err)
			}
		}

		taskInfo(exp, fmt.Sprintf("task monitor: %s", mon))
		if g.scale != 1 {
			warning := fmt.Sprintf("geometry SCALED to %.3f of true size to fit the drawing area "+
				"-- visual angles below are NOT the paper's; this session is a preview, not a replication",
				g.scale)
			taskInfo(exp, "task "+warning)
			log.Printf("WARNING: %s", warning)
		}
		taskInfo(exp, fmt.Sprintf("task font: %.1f pt (Inconsolata, bundled)", fontSize))
		// Degrees here are what was PRESENTED -- converted back from the pixels
		// actually used, through the unscaled monitor. Under -fit-window they
		// are therefore smaller than the design values they were derived from,
		// which is the point: the record must say what the participant saw, not
		// what was asked for.
		eccPx := g.px(WordEccentricityDeg)
		taskInfo(exp, fmt.Sprintf("task word width: %q measured %.0f px = %.2f deg presented (paper: 2-3 deg)",
			fontFitReference, wPx, mon.PxToDeg(float64(wPx))))
		taskInfo(exp, fmt.Sprintf("task word line box: %.0f px = %.2f deg presented -- the font's full line "+
			"height, NOT the letter height the paper's 0.6 deg refers to; letter height is not measured here",
			hPx, mon.PxToDegY(float64(hPx))))
		taskInfo(exp, fmt.Sprintf("task word eccentricity (nearest edge): %.0f px = %.2f deg presented "+
			"(design value %.2f deg)", eccPx, mon.PxToDeg(eccPx), WordEccentricityDeg))
		taskInfo(exp, fmt.Sprintf("task fixation offset: %.0f px below centre = %.2f deg presented "+
			"(design value %.2f deg)", g.pxY(FixationBelowDeg),
			mon.PxToDegY(g.pxY(FixationBelowDeg)), FixationBelowDeg))
		taskInfo(exp, fmt.Sprintf("task widest word: %.0f px, reaching %.0f px from centre of %.0f available",
			widest, halfExtentPx(g, widest), areaW/2))
		taskInfo(exp, fmt.Sprintf("task study duration: %d ms = %d frames at %.4f Hz",
			StudyDurationMs, framesFor(exp, StudyDurationMs), exp.Screen.RefreshRate()))

		// Echo the geometry to the terminal too: a wrong -screen-width is
		// otherwise invisible until someone opens the info file afterwards.
		log.Printf("geometry: %s | %.1f px/deg | word eccentricity %.0f px | fixation %.0f px below centre",
			mon, g.px(1), g.px(WordEccentricityDeg), g.pxY(FixationBelowDeg))

		// +Y is up in this framework, so 0.5 deg BELOW centre is negative.
		// This offset is the paper's: "a black fixation cross, presented at the
		// horizontal center and 0.5 deg of visual angle below the vertical
		// center" (p. 995). It is meant to look off-centre.
		cross := stimuli.NewFixCross(float32(g.px(FixationSizeDeg)), 3, exp.ForegroundColor)
		cross.SetPosition(control.FPoint{X: 0, Y: -float32(g.pxY(FixationBelowDeg))})

		if err := exp.ShowInstructions(instructions(oldKey, oldHand, newKey, newHand)); err != nil {
			return err
		}

		if !*noPractice {
			if err := runBlock(exp, g, cross, practiceStream, "practice", oldKey, respKeys); err != nil {
				return err
			}
			if err := exp.ShowInstructions(
				"End of the practice run.\n\n" +
					"The real experiment starts now and runs without a break.\n" +
					"Keep your eyes on the cross throughout, and remember every word you see.\n\n" +
					"Press SPACE when you are ready."); err != nil {
				return err
			}
		}

		if err := runBlock(exp, g, cross, stream, "main", oldKey, respKeys); err != nil {
			return err
		}

		if err := exp.ShowEndMessage("The experiment is over.\n\nThank you!\n\nPress any key to quit."); err != nil {
			return err
		}
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}

// runBlock presents one stream from start to finish.
func runBlock(exp *control.Experiment, g geometry, cross *stimuli.FixCross,
	stream []Event, block string, oldKey control.Keycode, respKeys []control.Keycode) error {

	// The fixation cross is up before the first word and never leaves.
	blockStart, err := present(exp, cross)
	if err != nil {
		return err
	}
	if err := exp.Wait(1000); err != nil {
		return err
	}

	for i, ev := range stream {
		pos := i + 1
		switch ev.Kind {
		case KindStudy:
			onset, err := runStudy(exp, g, cross, ev)
			if err != nil {
				return err
			}
			exp.Data.Add(block, pos, ev.Kind.String(), ev.Word, ev.PairID, ev.VF,
				"NA", "NA", "NA", "NA", "NA", "NA", msSince(blockStart, onset))

		case KindTest:
			onset, key, rtMs, err := runTest(exp, cross, ev, respKeys)
			if err != nil {
				return err
			}
			response := "new"
			if key == oldKey {
				response = "old"
			}
			correct := (key == oldKey) == ev.IsOld
			lag := interface{}("NA")
			if ev.IsOld {
				lag = ev.Lag
			}
			exp.Data.Add(block, pos, ev.Kind.String(), ev.Word, naInt(ev.PairID), naStr(ev.VF),
				lag, ev.IsOld, response, keyName(key),
				rtMs, correct, msSince(blockStart, onset))
		}
	}
	return nil
}

// runStudy shows one lateralized word for StudyDurationMs, then the fixation
// cross alone for StudyISIMs. It returns the word's onset timestamp.
func runStudy(exp *control.Experiment, g geometry, cross *stimuli.FixCross, ev Event) (uint64, error) {
	word, err := lateralWord(exp, g, ev.Word, ev.VF)
	if err != nil {
		return 0, err
	}
	defer word.Unload()

	// Hold the word for a whole number of frames: at 60 Hz, 200 ms is exactly
	// 12. The scene is redrawn each frame rather than re-presenting the back
	// buffer, whose contents are undefined after a flip on some drivers.
	frames := framesFor(exp, StudyDurationMs)
	onset, err := present(exp, cross, word)
	if err != nil {
		return 0, err
	}
	for f := 1; f < frames; f++ {
		if _, err := present(exp, cross, word); err != nil {
			return 0, err
		}
	}
	if _, err := present(exp, cross); err != nil {
		return 0, err
	}
	return onset, exp.Wait(StudyISIMs)
}

// runTest shows one word at the centre and waits for an old/new judgement. The
// word stays up until the response, as in the paper. Reaction time comes from
// the SDL event clock: the flip timestamp subtracted from the key event's own
// hardware timestamp.
func runTest(exp *control.Experiment, cross *stimuli.FixCross, ev Event,
	respKeys []control.Keycode) (uint64, control.Keycode, float64, error) {

	word := stimuli.NewTextLine(ev.Word, 0, 0, exp.ForegroundColor)
	if err := stimuli.PreloadVisualOnScreen(exp.Screen, word); err != nil {
		return 0, 0, 0, err
	}
	defer word.Unload()

	// Drop anything already queued before flipping. GetKeyEventTS returns a
	// matching key that is ALREADY in the SDL queue without blocking, so a key
	// pressed during the preceding interstimulus interval would be consumed the
	// instant the probe appeared: the word would flash for one frame and vanish,
	// and its "reaction time" would predate its own onset. Experiment.
	// ShowAndGetRT clears for exactly this reason; this composite frame has to
	// do it itself.
	exp.Keyboard.Clear()
	onset, err := present(exp, cross, word)
	if err != nil {
		return 0, 0, 0, err
	}
	key, keyTS, err := exp.Keyboard.GetKeyEventTS(respKeys, -1)
	if err != nil {
		return 0, 0, 0, err
	}
	// Signed, so an event that somehow still precedes the flip is recorded as a
	// visibly negative RT rather than wrapping to ~1.8e13 through the unsigned
	// subtraction.
	rtMs := float64(int64(keyTS)-int64(onset)) / 1e6

	if _, err := present(exp, cross); err != nil {
		return 0, 0, 0, err
	}
	return onset, key, rtMs, exp.Wait(TestISIMs)
}

// present clears the screen, draws every stimulus in order, and flips,
// returning the flip's SDL timestamp. The stimuli package has no composite
// type, and every frame here is a fixation cross plus at most one word.
func present(exp *control.Experiment, stims ...stimuli.VisualStimulus) (uint64, error) {
	if err := exp.Screen.Clear(); err != nil {
		return 0, err
	}
	for _, s := range stims {
		if err := s.Draw(exp.Screen); err != nil {
			return 0, err
		}
	}
	return exp.Screen.FlipTS()
}

// lateralWord builds a study word positioned so that its NEAREST edge lies
// WordEccentricityDeg from the horizontal centre. The word has to be preloaded
// first, because that is what measures its rendered width.
func lateralWord(exp *control.Experiment, g geometry, word, vf string) (*stimuli.TextLine, error) {
	tl := stimuli.NewTextLine(word, 0, 0, exp.ForegroundColor)
	if err := stimuli.PreloadVisualOnScreen(exp.Screen, tl); err != nil {
		return nil, err
	}
	x := float32(g.px(WordEccentricityDeg)) + tl.Width/2
	if vf == "LVF" {
		x = -x
	}
	tl.SetPosition(control.FPoint{X: x, Y: 0})
	return tl, nil
}

// fitFontToWidth reloads the default font at the point size that makes ref
// subtend targetDeg horizontally, and returns that size with the width and
// height actually measured.
//
// Width is used rather than the paper's 0.6 degree letter height because width
// is the quantity this framework can measure directly: TextLine.Width is the
// width of the rendered surface, so a word's horizontal extent in degrees is
// exact. Letter height is NOT measured -- TextLine.Height is the font's line
// box (ascender to descender), which is substantially larger than the cap
// height the paper quotes, and a true cap height would need
// TTF_GetGlyphMetrics, a symbol the WebAssembly build does not export. Both
// numbers go into the session's -info.txt, each labelled for what it is, so
// the presented size is on the record rather than assumed.
func fitFontToWidth(exp *control.Experiment, g geometry, ref string, targetDeg float64) (float32, float32, float32, error) {
	targetPx := float32(g.px(targetDeg))
	size := exp.DefaultFontSize

	// Three passes: rendered width is very nearly linear in point size, so this
	// converges immediately; the extra passes absorb hinting and rounding.
	for pass := 0; pass < 3; pass++ {
		w, _, err := measure(exp, ref)
		if err != nil {
			return 0, 0, 0, err
		}
		if w <= 0 {
			return 0, 0, 0, fmt.Errorf("font measurement returned a zero width for %q", ref)
		}
		next := size * targetPx / w
		if next < 6 {
			next = 6
		}
		if next > 400 {
			next = 400
		}
		if math.Abs(float64(next-size)) < 0.25 {
			break
		}
		size = next
		if err := exp.LoadFontFromMemory(assets_embed.InconsolataFont, size); err != nil {
			return 0, 0, 0, err
		}
	}

	w, h, err := measure(exp, ref)
	if err != nil {
		return 0, 0, 0, err
	}
	return size, w, h, nil
}

// measure returns the rendered size in pixels of s in the current default font.
func measure(exp *control.Experiment, s string) (float32, float32, error) {
	tl := stimuli.NewTextLine(s, 0, 0, exp.ForegroundColor)
	if err := stimuli.PreloadVisualOnScreen(exp.Screen, tl); err != nil {
		return 0, 0, err
	}
	defer tl.Unload()
	return tl.Width, tl.Height, nil
}

// framesFor converts a duration in milliseconds to a whole number of frames.
func framesFor(exp *control.Experiment, ms int) int {
	frame := exp.Screen.FrameDuration()
	if frame <= 0 {
		return 1
	}
	n := int(math.Round(float64(ms) * 1e6 / float64(frame.Nanoseconds())))
	if n < 1 {
		n = 1
	}
	return n
}

// taskInfo writes one metadata line into the session's companion -info.txt.
//
// Experiment.AddExperimentInfo is not used: it appends to design.Experiment's
// ExperimentInfo slice, which no writer ever reads, so anything recorded that
// way is silently lost.
func taskInfo(exp *control.Experiment, line string) {
	if exp.Data != nil {
		exp.Data.WriteComment(line)
	}
}

// geometry converts degrees of visual angle to drawing-space pixels.
//
// scale is 1.0 for a faithful run, in which one degree really does subtend one
// degree at the participant's eye. It drops below 1.0 only under -fit-window,
// which shrinks every spatial quantity by a common factor so a development
// session fits inside a small window; the visual angles are then NOT the
// paper's, and the session's -info.txt says so.
type geometry struct {
	mon   units.Monitor
	scale float64
}

func (g geometry) px(deg float64) float64  { return g.mon.DegToPx(deg) * g.scale }
func (g geometry) pxY(deg float64) float64 { return g.mon.DegToPxY(deg) * g.scale }

// halfExtentPx is the distance from the horizontal centre to the outer edge of
// the widest lateralized word -- what the drawing area must be able to hold on
// each side.
func halfExtentPx(g geometry, widestWordPx float32) float32 {
	return float32(g.px(WordEccentricityDeg)) + widestWordPx
}

// widestWordPx measures the widest word any stream will present. The bundled
// font is monospaced, so the longest word is the widest one and a single
// measurement settles it.
func widestWordPx(exp *control.Experiment, streams ...[]Event) (float32, error) {
	longest := fontFitReference
	for _, stream := range streams {
		for _, ev := range stream {
			if len([]rune(ev.Word)) > len([]rune(longest)) {
				longest = ev.Word
			}
		}
	}
	w, _, err := measure(exp, longest)
	return w, err
}

// pixelExtent returns the pixel dimensions to base the degree conversions on.
//
// The DISPLAY's native resolution is preferred over the window's, so that a
// windowed session (-w) still converts degrees at the true pixel density of the
// monitor. Where the display mode cannot be read -- notably in the browser, and
// on any backend whose mode query fails -- it falls back to the renderer's own
// output size, which is the best available answer there.
func pixelExtent(exp *control.Experiment) (int, int, error) {
	if d := exp.Screen.DisplayInfo(); d.NativeW > 0 && d.NativeH > 0 {
		return int(d.NativeW), int(d.NativeH), nil
	}
	w, h, err := exp.Screen.Size()
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read the display or window size, needed to convert degrees to pixels: %w", err)
	}
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("renderer reported a %dx%d output size", w, h)
	}
	return int(w), int(h), nil
}

// msSince converts two SDL nanosecond timestamps to milliseconds elapsed.
func msSince(start, t uint64) float64 {
	return float64(t-start) / 1e6
}

// swapVisualFields turns a list into its matched list, in which every studied
// word is shown in the other visual field.
func swapVisualFields(stream []Event) {
	for i := range stream {
		switch stream[i].VF {
		case "LVF":
			stream[i].VF = "RVF"
		case "RVF":
			stream[i].VF = "LVF"
		}
	}
}

// naInt and naStr write R's missing-value marker for fields that do not apply
// to an event, rather than a zero that would be read as data.
func naInt(v int) interface{} {
	if v < 0 {
		return "NA"
	}
	return v
}

func naStr(v string) interface{} {
	if v == "" {
		return "NA"
	}
	return v
}

func keyName(key control.Keycode) string {
	switch key {
	case control.K_F:
		return "F"
	case control.K_J:
		return "J"
	}
	return fmt.Sprintf("keycode-%d", uint32(key))
}

func instructions(oldKey control.Keycode, oldHand string, newKey control.Keycode, newHand string) string {
	return fmt.Sprintf(
		"Lateralized Recognition Memory\n\n"+
			"A small cross stays at the centre of the screen. Keep your eyes on it at all times.\n"+
			"Never look at the words that flash to the left or to the right -- just try to remember them.\n\n"+
			"Words will also appear at the CENTRE, one at a time.\n"+
			"For each of those, decide whether you have already seen that word at any point\n"+
			"in this experiment.\n\n"+
			"    Press %s (%s hand) if you have seen it before  --  OLD\n"+
			"    Press %s (%s hand) if you have not             --  NEW\n\n"+
			"Answer as quickly and as accurately as you can.\n\n"+
			"Press SPACE to start.",
		keyName(oldKey), oldHand, keyName(newKey), newHand)
}

// loadWords parses a word pool. Blank lines and lines starting with '#' are
// ignored; words are upper-cased. Duplicates are rejected rather than silently
// accepted, because a word appearing twice in the pool could be drawn as both a
// studied and an unstudied item and would then be scored against itself.
func loadWords(embedded, path string) ([]string, error) {
	text := embedded
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		text = string(data)
	}

	var words []string
	seen := map[string]int{}
	for i, line := range strings.Split(text, "\n") {
		w := strings.ToUpper(strings.TrimSpace(line))
		if w == "" || strings.HasPrefix(w, "#") {
			continue
		}
		if prev, dup := seen[w]; dup {
			return nil, fmt.Errorf("%q appears twice (lines %d and %d)", w, prev, i+1)
		}
		seen[w] = i + 1
		words = append(words, w)
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("no words found")
	}
	return words, nil
}
