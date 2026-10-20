// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Command MEG-localizer3 combines the multifocal retinotopic mapping stimulus
// of Kurki, Hyvarinen, Henriksson & Vanni, "Dynamics of retinotopic spatial
// attention revealed by multifocal MEG", NeuroImage 263 (2022) 119643, with a
// two-button task in two modalities.
//
// Twenty-four regions of the visual field -- three annuli crossed with eight
// 45-degree sectors -- are stimulated simultaneously, each following its own
// quasi-orthogonal binary sequence. Because the sequences are near-orthogonal,
// 24 region-specific responses can be deconvolved from a single continuous
// recording of about two minutes, instead of the much longer runs a
// travelling-wave design needs (compare examples/Retinotopy).
//
// Stimulation is pattern-ONSET, not pattern-reversal: on a trial where a region
// is "on", its checkerboard appears abruptly at full contrast and then fades
// linearly back to the mid-grey background over the trial.
//
// The task runs throughout. Every 3-8 s the fixation cross is replaced for
// 800 ms by a '<' (press the LEFT button) or a '>' (press the RIGHT button).
// Every 10 +- 2 s one of two sounds is played: sounds/left.wav asks for the
// LEFT button, sounds/right.wav for the RIGHT one. Any two task events are at
// least 2 s apart, so every press belongs to at most one of them.
//
// Parameters the paper does not state are flags, and are listed in README.md.
package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/units"
)

// The two task sounds are embedded so the binary is self-contained.
//
//go:embed sounds/left.wav sounds/right.wav
var soundFS embed.FS

const (
	expName = "MEG-localizer3"

	// Trial onset asynchrony range, in seconds (paper: 217-317 ms). Converted
	// to a whole number of frames at the display's measured refresh rate.
	toaMinSec = 0.217
	toaMaxSec = 0.317

	// Duration of an arrow target replacing the fixation cross, in seconds
	// (the paper's colour-change target lasted 800 ms).
	targetDurSec = 0.800
	// Bounds on the gap between the end of one arrow and the start
	// of the next. The paper does not give a distribution; these keep the
	// task attentionally demanding without crowding the run.
	targetGapMinSec = 3.0
	targetGapMaxSec = 8.0

	// Default bounds on the gap between successive sound onsets: 10 +- 2 s.
	soundGapMinSec = 8.0
	soundGapMaxSec = 12.0

	// No task event starts less than minSepSec after the previous one, and a
	// press is attributed to an event only within respWindowSec of its onset.
	// The window must not exceed the separation, or a press could belong to
	// two events.
	minSepSec     = 2.0
	respWindowSec = 2.0

	// The silent keep-alive tone (see runner.keepAlive) and how often it is
	// replayed. Its period must stay well under PipeWire's ~5 s
	// suspend-on-idle.
	keepAliveToneMs = 500
	keepAliveSec    = 2.0

	fixCrossSizePx = 16
	fixCrossLinePx = 3

	// Onset-marker defaults for this paradigm (see control.MarkerFlags): the
	// photodiode square is white for one frame at every trial onset, in the
	// flip whose timestamp is recorded as onset_ms, and the TTL pulse is 5 ms.
	// Both must fit inside the shortest trial (13 frames at 60 Hz).
	markerFrames = 1
	markerTTLMs  = 5
)

// fixColor is the colour of the fixation cross and of the arrows that replace
// it: darker than the mid-grey background, so they stay visible. The arrows
// differ from the cross in shape only, not in colour or luminance.
var fixColor = control.DarkGray

// arrowHalfWidthPx is half the width of a '<' or '>'; its height is the
// cross's, fixCrossSizePx.
const arrowHalfWidthPx = 5

// newArrow returns a '<' (side left) or '>' (side right) centred on the
// fixation point, drawn with the cross's line width. Points are +Y up.
func newArrow(side int) *stimuli.PolyLine {
	w, h := float32(arrowHalfWidthPx), float32(fixCrossSizePx)/2
	if side == right {
		w = -w
	}
	return stimuli.NewPolyLine([]control.FPoint{{X: w, Y: h}, {X: -w, Y: 0}, {X: w, Y: -h}},
		false, fixCrossLinePx, fixColor)
}

func main() {
	// -verify runs the orthogonality check with no display at all, so the
	// design can be inspected on a machine that has no screen. It must be
	// handled before NewExperimentFromFlags, which opens a window.
	if code, done := runVerify(); done {
		os.Exit(code)
	}

	// Registered before NewExperimentFromFlags, which calls flag.Parse().
	var (
		flagPrime   = flag.Int("p", DefaultPrime, "Sequence period: a prime congruent to 3 (mod 4)")
		flagShift   = flag.Int("shift", 0, "Cyclic shift between regions (0 = p/regions)")
		flagTrials  = flag.Int("trials", 0, "Trials in the run (0 = one full period, p)")
		flagHabit   = flag.Int("habituation", 20, "Full-field flashes before the run (paper: 20)")
		flagMaxEcc  = flag.Float64("max-ecc", MaxEccentricityDeg, "Outer radius in degrees of visual angle")
		flagFull    = flag.Bool("fullfield", false, "Show the static full-contrast dartboard and wait for a key")
		flagAuto    = flag.Bool("autostart", false, "Skip the instruction screen (for test runs)")
		flagSeed    = flag.Uint64("seed", 1, "Random seed for trial durations and task events")
		flagWidthCm = flag.Float64("screen-width-cm", 30, "Screen width in cm (used when the info dialog is skipped)")
		flagDistCm  = flag.Float64("viewing-distance-cm", 50, "Viewing distance in cm (used when the info dialog is skipped)")

		flagLeftKey  = flag.String("left-key", "left", "Key sent by the LEFT response button (left, right, up, down, space, a-z, 0-9)")
		flagRightKey = flag.String("right-key", "right", "Key sent by the RIGHT response button")
		flagSoundMin = flag.Float64("sound-gap-min", soundGapMinSec, "Shortest gap between sound onsets, in s")
		flagSoundMax = flag.Float64("sound-gap-max", soundGapMaxSec, "Longest gap between sound onsets, in s")

		flagWaitTrig  = flag.Bool("wait-trigger", false, "Wait for the scanner pulse (key 't') before starting the run")
		flagTTLLine   = flag.Int("ttl-line", 0, "TTL line (0-7) pulsed at every trial onset with -ttl")
		flagTTLArrowL = flag.Int("ttl-arrow-left", 1, "TTL line (0-7) raised with the trial onset at which '<' appears")
		flagTTLArrowR = flag.Int("ttl-arrow-right", 2, "TTL line (0-7) raised with the trial onset at which '>' appears")
		flagTTLSoundL = flag.Int("ttl-sound-left", 3, "TTL line (0-7) raised with the trial onset at which left.wav starts")
		flagTTLSoundR = flag.Int("ttl-sound-right", 4, "TTL line (0-7) raised with the trial onset at which right.wav starts")
	)
	// -photodiode (on by default here), -photodiode-size, -marker-frames, -ttl
	// and -ttl-ms mark every trial onset for the recording.
	control.MarkerFlags(control.MarkerConfig{
		Photodiode:  true,
		Frames:      markerFrames,
		TTLDuration: markerTTLMs * time.Millisecond,
	})
	flag.Bool("verify", false, "Print the sequence orthogonality report and exit without opening a window")

	exp := control.NewExperimentFromFlags(expName, control.Gray, control.White, 24)
	defer exp.End()

	// The linear contrast fade is implemented as a linear alpha ramp, which is
	// only equivalent to a contrast ramp when the background is the mean
	// luminance of the checkerboard. Fail loudly rather than present a
	// stimulus whose contrast does not do what the data file claims.
	if exp.BackgroundColor != control.Gray {
		log.Fatalf("%s: background must be mid-grey for the contrast fade to be correct, got %v",
			expName, exp.BackgroundColor)
	}

	// Every event has its own line so the STI channel can tell them apart even
	// when they share a flip with the trial onset.
	lines := []struct {
		name string
		line int
	}{
		{"-ttl-line", *flagTTLLine}, {"-ttl-arrow-left", *flagTTLArrowL}, {"-ttl-arrow-right", *flagTTLArrowR},
		{"-ttl-sound-left", *flagTTLSoundL}, {"-ttl-sound-right", *flagTTLSoundR},
	}
	for i, l := range lines {
		if l.line < 0 || l.line > 7 {
			log.Fatalf("%s=%d out of range (0-7)", l.name, l.line)
		}
		for _, m := range lines[:i] {
			if m.line == l.line {
				log.Fatalf("%s and %s are both line %d; every event needs its own line",
					m.name, l.name, l.line)
			}
		}
	}
	// One onset's pulse must be over before the next onset.
	if d := exp.Marker.TTLDuration(); exp.Marker.TTLEnabled() && d >= time.Duration(toaMinSec*float64(time.Second)) {
		log.Fatalf("-ttl-ms: the %v TTL pulse must be shorter than the shortest trial (%.0f ms)",
			d, toaMinSec*1000)
	}

	leftKey, err := parseKey(*flagLeftKey)
	if err != nil {
		log.Fatalf("-left-key: %v", err)
	}
	rightKey, err := parseKey(*flagRightKey)
	if err != nil {
		log.Fatalf("-right-key: %v", err)
	}
	if leftKey == rightKey {
		log.Fatalf("-left-key and -right-key are both %q", *flagLeftKey)
	}
	if *flagSoundMin < minSepSec || *flagSoundMax < *flagSoundMin {
		log.Fatalf("-sound-gap-min/-max: need %.1f <= min <= max, got %.2f, %.2f",
			minSepSec, *flagSoundMin, *flagSoundMax)
	}

	seqs, err := BuildSequences(*flagPrime, NRegions, *flagShift, *flagTrials)
	if err != nil {
		log.Fatalf("%s: %v", expName, err)
	}
	report := seqs.Report()
	for _, line := range report.Lines() {
		log.Print(line)
	}
	if !report.OK(1e-6) {
		log.Fatalf("%s: the generated design is not orthogonal enough to deconvolve; "+
			"check -p and -shift", expName)
	}
	if !report.Balanced() {
		log.Printf("Warning: regions are stimulated on %.1f%%-%.1f%% of trials, not ~50%%. "+
			"A run of fewer than p=%d trials is not balanced; do not analyse it as a full run.",
			100*report.MinOnFrac, 100*report.MaxOnFrac, report.P)
	}

	if herr := exp.HideCursor(); herr != nil {
		log.Printf("Warning: could not hide cursor: %v", herr)
	}

	runErr := exp.Run(func() error {
		// ── Display geometry ────────────────────────────────────────────────
		widthCm := infoFloat(exp.Info, "screen_width_cm", *flagWidthCm)
		distCm := infoFloat(exp.Info, "viewing_distance_cm", *flagDistCm)
		widthPx, heightPx := exp.Screen.Width, exp.Screen.Height
		heightCm := widthCm * float64(heightPx) / float64(widthPx)
		mon := units.NewMonitor(widthCm, heightCm, widthPx, heightPx, distCm)
		if verr := mon.Validate(); verr != nil {
			return fmt.Errorf("monitor: %w", verr)
		}

		// Eccentricity is a tangent projection, not pixels-per-degree times
		// degrees: at 8.4 deg the linear approximation misplaces the outer
		// ring by several percent.
		ppcm := mon.PPcmX()
		eccToPx := func(deg float64) float64 {
			return distCm * math.Tan(deg*math.Pi/180) * ppcm
		}
		pxToEcc := func(px float64) float64 {
			return math.Atan(px/ppcm/distCm) * 180 / math.Pi
		}

		var ringDeg, ringPx [NRings + 1]float64
		designScale := *flagMaxEcc / MaxEccentricityDeg
		for i, d := range RingRadiiDeg {
			ringDeg[i] = d * designScale
			ringPx[i] = eccToPx(ringDeg[i])
		}
		// Clamp to the screen. Scaling in pixels (not degrees) keeps the
		// dartboard's proportions; the achieved degrees are recomputed and
		// recorded so the data never claims an eccentricity it did not show.
		maxRadiusPx := float64(min(widthPx, heightPx)) / 2
		clamped := false
		if ringPx[NRings] > maxRadiusPx {
			k := maxRadiusPx / ringPx[NRings]
			for i := range ringPx {
				ringPx[i] *= k
			}
			clamped = true
		}
		for i := range ringPx {
			ringDeg[i] = pxToEcc(ringPx[i])
		}
		sizePx := 2 * int(math.Ceil(ringPx[NRings]))

		// ── Stimulus textures ───────────────────────────────────────────────
		imgs := RasterizeRegions(sizePx, ringPx)
		square := exp.Screen.CenteredRect(control.Point(0, 0), float32(sizePx), float32(sizePx))
		textures := make([]*apparatus.Texture, NRegions)
		dst := make([]*control.FRect, NRegions)
		defer func() {
			for _, t := range textures {
				if t != nil {
					t.Destroy()
				}
			}
		}()
		for i := range imgs {
			im := &imgs[i]
			if im.W == 0 || im.H == 0 {
				return fmt.Errorf("region %d rasterised to an empty image", im.Index)
			}
			tex, terr := exp.Screen.TextureFromRGBA(im.W, im.H, im.Pix, im.W*4)
			if terr != nil {
				return fmt.Errorf("region %d: %w", im.Index, terr)
			}
			textures[im.Index] = tex
			dst[im.Index] = &control.FRect{
				X: square.X + float32(im.OffsetX),
				Y: square.Y + float32(im.OffsetY),
				W: float32(im.W), H: float32(im.H),
			}
		}

		// ── Sounds ──────────────────────────────────────────────────────────
		var sounds [2]*stimuli.Sound // indexed by side
		soundMs := [2]float64{}
		for side, name := range [2]string{"sounds/left.wav", "sounds/right.wav"} {
			data, rerr := soundFS.ReadFile(name)
			if rerr != nil {
				return fmt.Errorf("%s: %w", name, rerr)
			}
			snd := stimuli.NewSoundFromMemory(data)
			if perr := snd.PreloadDevice(exp.AudioDevice); perr != nil {
				return fmt.Errorf("%s: %w", name, perr)
			}
			sounds[side] = snd
			soundMs[side] = pcmDurationMs(snd)
		}
		keepAlive := stimuli.NewTone(440, keepAliveToneMs, 0)
		if perr := keepAlive.PreloadDevice(exp.AudioDevice); perr != nil {
			return fmt.Errorf("keep-alive tone: %w", perr)
		}
		audioLatency, lerr := sounds[0].OutputLatency()
		if lerr != nil {
			return fmt.Errorf("audio output latency: %w", lerr)
		}

		// ── Frame-locked timing ─────────────────────────────────────────────
		refresh := float64(exp.Screen.RefreshRate())
		if refresh <= 0 {
			refresh = 60
			log.Print("Warning: refresh rate unknown, assuming 60 Hz")
		}
		minFrames := maxInt(1, int(math.Round(toaMinSec*refresh)))
		maxFrames := maxInt(minFrames, int(math.Round(toaMaxSec*refresh)))
		frameMs := 1000.0 / refresh
		secToFrames := func(s float64) int { return maxInt(1, int(math.Round(s*refresh))) }

		// The photodiode square sits at the top-left corner of the drawing
		// area; its inner corner must lie outside the dartboard, or the square
		// would hide checks and the photodiode would see the stimulus fade as
		// well as the flash.
		if side := exp.Marker.Size(); exp.Marker.PhotodiodeEnabled() {
			w, h := exp.DrawArea()
			if math.Hypot(float64(w/2-side), float64(h/2-side)) < ringPx[NRings] {
				log.Printf("Warning: the %.0f px photodiode square overlaps the stimulus (outer radius %.0f px)",
					side, ringPx[NRings])
			}
		}

		r := &runner{
			exp:            exp,
			textures:       textures,
			dst:            dst,
			fix:            stimuli.NewFixCross(fixCrossSizePx, fixCrossLinePx, fixColor),
			arrows:         [2]*stimuli.PolyLine{newArrow(left), newArrow(right)},
			sounds:         sounds,
			keepAlive:      keepAlive,
			keys:           [2]control.Keycode{leftKey, rightKey},
			ttlCode:        byte(1) << uint(*flagTTLLine),
			arrowCodes:     [2]byte{byte(1) << uint(*flagTTLArrowL), byte(1) << uint(*flagTTLArrowR)},
			soundCodes:     [2]byte{byte(1) << uint(*flagTTLSoundL), byte(1) << uint(*flagTTLSoundR)},
			frameMs:        frameMs,
			targetFrames:   secToFrames(targetDurSec),
			keepAliveEvery: secToFrames(keepAliveSec),
			respWindowNS:   uint64(respWindowSec * 1e9),
		}

		// ── Trial durations and task-event schedule ─────────────────────────
		// Every trial duration is drawn before the run, so the task events can
		// be planned over the whole run and their sides exactly balanced.
		rng := rand.New(rand.NewPCG(*flagSeed, uint64(exp.SubjectID)))
		trialFrames := make([]int, *flagHabit+seqs.NTrials)
		for i := range trialFrames {
			trialFrames[i] = minFrames + rng.IntN(maxFrames-minFrames+1)
		}
		plan, perr := planEvents(trialFrames, scheduleParams{
			TargetFrames:   r.targetFrames,
			GapMinFrames:   secToFrames(targetGapMinSec),
			GapMaxFrames:   maxInt(2, secToFrames(targetGapMaxSec)),
			SoundMinFrames: secToFrames(*flagSoundMin),
			SoundMaxFrames: secToFrames(*flagSoundMax),
			MinSepFrames:   secToFrames(minSepSec),
		}, rng)
		if perr != nil {
			return perr
		}
		var planned [2]int
		for _, ev := range plan {
			if ev != nil {
				planned[ev.Modality]++
			}
		}

		// ── Data file header ────────────────────────────────────────────────
		exp.Data.WriteComment("--MEG LOCALIZER 3")
		exp.Data.WriteComment("m paradigm: multifocal retinotopy (Kurki et al. 2022, NeuroImage 263:119643, mapping localizer only)")
		exp.Data.WriteComment("m stimulation: pattern onset, contrast fades linearly to 0 over each trial")
		exp.Data.WriteComment("m task: '<' replaces the cross -> LEFT, '>' -> RIGHT; sounds/left.wav -> LEFT, sounds/right.wav -> RIGHT")
		for _, line := range report.Lines() {
			exp.Data.WriteComment("m " + line)
		}
		exp.Data.WriteComment(fmt.Sprintf("m monitor: %s", mon.String()))
		exp.Data.WriteComment(fmt.Sprintf("m refresh_hz: %.4f  frame_ms: %.4f", refresh, frameMs))
		exp.Data.WriteComment(fmt.Sprintf("m toa_frames: %d-%d (%.1f-%.1f ms)",
			minFrames, maxFrames, float64(minFrames)*frameMs, float64(maxFrames)*frameMs))
		exp.Data.WriteComment(fmt.Sprintf("m stimulus_size_px: %d  clamped_to_screen: %t", sizePx, clamped))
		exp.Data.WriteComment(fmt.Sprintf("m max_eccentricity_deg: %.3f (requested %.3f)",
			ringDeg[NRings], *flagMaxEcc))
		exp.Data.WriteComment(fmt.Sprintf("m ring_radii_deg: %.3f %.3f %.3f %.3f",
			ringDeg[0], ringDeg[1], ringDeg[2], ringDeg[3]))
		exp.Data.WriteComment(fmt.Sprintf("m seed: %d", *flagSeed))
		exp.Data.WriteComment(fmt.Sprintf("m keys: left=%s right=%s", *flagLeftKey, *flagRightKey))
		exp.Data.WriteComment(fmt.Sprintf("m fix_target: %.0f ms, gap %.1f-%.1f s after its end",
			targetDurSec*1000, targetGapMinSec, targetGapMaxSec))
		exp.Data.WriteComment(fmt.Sprintf("m sound_gap_s: %.1f-%.1f between onsets", *flagSoundMin, *flagSoundMax))
		exp.Data.WriteComment(fmt.Sprintf("m sound_ms: left=%.1f right=%.1f", soundMs[0], soundMs[1]))
		exp.Data.WriteComment(fmt.Sprintf("m planned_events: visual %d  audio %d (half left, half right in each, in random order)",
			planned[visual], planned[audio]))
		exp.Data.WriteComment(fmt.Sprintf("m min_event_separation_s: %.1f  response_window_s: %.1f", minSepSec, respWindowSec))
		exp.Data.WriteComment("m task events start on a trial-onset flip; a sound starts when that flip returns")
		exp.Data.WriteComment(fmt.Sprintf("m audio_output_latency_ms: %.2f (software estimate: the sound is heard about this long after onset_ms)",
			float64(audioLatency)/float64(time.Millisecond)))
		if exp.Marker.TTLEnabled() {
			exp.Data.WriteComment(fmt.Sprintf("m ttl_lines: trial=%d arrow_left=%d arrow_right=%d sound_left=%d sound_right=%d "+
				"(event bits are OR'd with the trial bit on the same onset)",
				*flagTTLLine, *flagTTLArrowL, *flagTTLArrowR, *flagTTLSoundL, *flagTTLSoundR))
		}
		for _, line := range RegionTableLines(ringDeg) {
			exp.Data.WriteComment("m " + line)
		}
		exp.AddDataVariableNames([]string{
			"event_type", "trial", "onset_ms", "toa_frames", "toa_ms",
			"n_on", "regions", "modality", "side", "button", "outcome", "rt_ms",
		})
		log.Printf("planned events: visual %d, audio %d, each exactly half left/half right", planned[visual], planned[audio])
		log.Printf("sounds: left %.0f ms, right %.0f ms; audio output latency %.1f ms (estimate)",
			soundMs[0], soundMs[1], float64(audioLatency)/float64(time.Millisecond))

		// ── Static geometry check ───────────────────────────────────────────
		if *flagFull {
			log.Printf("full-field check: max eccentricity %.2f deg, %d px square",
				ringDeg[NRings], sizePx)
			return r.showFullField()
		}

		// ── Start ───────────────────────────────────────────────────────────
		if !*flagAuto {
			msg := "Keep your eyes on the central cross at all times.\n\n" +
				"When the cross becomes  <  press the LEFT button.\n" +
				"When the cross becomes  >  press the RIGHT button.\n\n" +
				"Also, independently of the arrows, you will hear from time to time\n" +
				"the words \"Right!\" or \"Left!\".\n" +
				"When you hear the word \"Right!\", press the RIGHT button.\n" +
				"When you hear the word \"Left!\", press the LEFT button.\n\n" +
				"Press SPACE to continue."
			if ierr := exp.ShowInstructions(msg); ierr != nil {
				return ierr
			}
		}
		if *flagWaitTrig {
			if serr := exp.Screen.Clear(); serr != nil {
				return serr
			}
			if serr := r.fix.Draw(exp.Screen); serr != nil {
				return serr
			}
			if serr := exp.Screen.Flip(); serr != nil {
				return serr
			}
			if kerr := exp.Keyboard.WaitKey(control.K_T); kerr != nil {
				return kerr
			}
		}
		// Wake the audio sink now, after the last keypress: an idle sink is
		// suspended and the resume swallows the start of the first sound.
		if aerr := keepAlive.Play(); aerr != nil {
			return aerr
		}
		// t0 anchors every onset. It is on the SDL clock, the same one that
		// stamps flips (FlipTS) and key events, so onsets and responses are
		// directly comparable without crossing clocks.
		r.t0NS = control.TicksNS()

		// ── Habituation: full-field flashes ─────────────────────────────────
		allOn := make([]bool, NRegions)
		for i := range allOn {
			allOn[i] = true
		}
		for i := 0; i < *flagHabit; i++ {
			frames := trialFrames[i]
			onset, perr := r.presentTrial(allOn, frames, plan[i])
			if perr != nil {
				return perr
			}
			r.addRow(row{event: "habituation", trial: i, tsNS: onset, frames: frames, nOn: NRegions, on: allOn})
		}

		// ── Mapping sequence ────────────────────────────────────────────────
		for t := 0; t < seqs.NTrials; t++ {
			on := seqs.On[t]
			frames := trialFrames[*flagHabit+t]
			onset, perr := r.presentTrial(on, frames, plan[*flagHabit+t])
			if perr != nil {
				return perr
			}
			nOn := 0
			for _, v := range on {
				if v {
					nOn++
				}
			}
			r.addRow(row{event: "trial", trial: t, tsNS: onset, frames: frames, nOn: nOn, on: on})
		}
		// An event still inside its window when the run ends is scored now.
		r.expire(math.MaxUint64)

		// Return to a uniform grey field before the end message.
		if berr := exp.Screen.ClearAndUpdate(); berr != nil {
			return berr
		}
		for m, name := range modalityNames {
			for side, sname := range sideNames {
				s := r.score[m][side]
				exp.Data.WriteComment(fmt.Sprintf("m %s %s: events %d  correct %d  wrong %d  misses %d",
					name, sname, s.events, s.correct, s.wrong, s.misses))
				log.Printf("%s %s: events %d, correct %d, wrong %d, misses %d",
					name, sname, s.events, s.correct, s.wrong, s.misses)
			}
		}
		exp.Data.WriteComment(fmt.Sprintf("m false_alarms: %d  extra_presses: %d", r.falseAlarms, r.extraPresses))
		log.Printf("false alarms %d, extra presses %d", r.falseAlarms, r.extraPresses)

		// The result screen waits for a keypress, which would hang an
		// unattended run, so -autostart skips it as well as the instructions.
		if !*flagAuto {
			if eerr := r.showResults(); eerr != nil {
				return eerr
			}
		}
		return control.EndLoop
	})
	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("%s: %v", expName, runErr)
	}
}

// ── the task ───────────────────────────────────────────────────────────────

var (
	modalityNames = [2]string{"visual", "audio"}
	sideNames     = [2]string{"left", "right"}
	// eventNames[modality][side] is the event_type of a task event's row.
	eventNames = [2][2]string{{"arrow_left", "arrow_right"}, {"sound_left", "sound_right"}}
)

// taskEvent is one arrow or sound, from its onset flip until its
// response window closes.
type taskEvent struct {
	modality, side int
	onsetNS        uint64
	responded      bool
}

// tally counts the outcomes of the events of one modality and side. An event
// is correct if its first press in the window was the button it asked for;
// wrong and misses are both errors.
type tally struct {
	events, correct, wrong, misses int
}

// ── the frame loop ─────────────────────────────────────────────────────────

// runner holds the state the frame loop mutates: it exists so the habituation
// block and the mapping sequence share one presentation path, and so the task
// keeps running across both.
type runner struct {
	exp        *control.Experiment
	textures   []*apparatus.Texture
	dst        []*control.FRect
	fix        *stimuli.FixCross
	arrows     [2]*stimuli.PolyLine // '<', '>'
	sounds     [2]*stimuli.Sound    // by side
	keepAlive  *stimuli.Tone
	keys       [2]control.Keycode // by side
	ttlCode    byte               // the trial onset's code: one bit, the -ttl-line
	arrowCodes [2]byte            // '<', '>'
	soundCodes [2]byte            // left.wav, right.wav

	frameMs        float64
	targetFrames   int
	keepAliveEvery int
	respWindowNS   uint64

	t0NS  uint64
	frame int // frames since the run started
	quit  bool

	targetEndFrame int // the arrow replaces the cross until this frame
	targetSide     int // which arrow

	cur *taskEvent // the last event, while its response window is open

	score                     [2][2]tally // by modality, then side
	falseAlarms, extraPresses int
}

// startEvent turns a planned event into the current one, at the trial onset
// it was planned for.
func (r *runner) startEvent(p *plannedEvent) *taskEvent {
	ev := &taskEvent{modality: p.Modality, side: p.Side}
	if ev.modality == visual {
		r.targetEndFrame = r.frame + r.targetFrames
		r.targetSide = ev.side
	}
	return ev
}

// eventCode is the TTL bit of an event.
func (r *runner) eventCode(ev *taskEvent) byte {
	if ev.modality == audio {
		return r.soundCodes[ev.side]
	}
	return r.arrowCodes[ev.side]
}

// presentTrial shows one pattern-onset trial and returns the SDL timestamp of
// its first flip.
//
// The regions that are on appear at full contrast on the first frame and fade
// linearly to zero, reaching the background exactly as the next trial begins.
// Contrast is set with SetAlphaMod: over a mid-grey background, alpha is
// Michelson contrast, so no pixels are touched on the CPU in this loop.
//
// planned is the task event that starts with this trial, or nil.
func (r *runner) presentTrial(on []bool, frames int, planned *plannedEvent) (uint64, error) {
	var onsetNS uint64
	for f := 0; f < frames; f++ {
		// Task events start only on a trial onset, so their TTL bit shares
		// the trial's flip and the photodiode flashes only at trial onsets.
		var ev *taskEvent
		if f == 0 && planned != nil {
			ev = r.startEvent(planned)
		}
		// Keep the audio sink from suspending between sounds.
		if r.frame%r.keepAliveEvery == 0 {
			if err := r.keepAlive.Play(); err != nil {
				return onsetNS, err
			}
		}

		alpha := byte(math.Round(255 * (1 - float64(f)/float64(frames))))
		if err := r.exp.Screen.Clear(); err != nil {
			return onsetNS, err
		}
		for k, onk := range on {
			if !onk {
				continue
			}
			if err := r.textures[k].SetAlphaMod(alpha); err != nil {
				return onsetNS, err
			}
			if err := r.exp.Screen.Renderer.RenderTexture(r.textures[k], nil, r.dst[k]); err != nil {
				return onsetNS, err
			}
		}
		var center interface{ Draw(*apparatus.Screen) error } = r.fix
		if r.frame < r.targetEndFrame {
			center = r.arrows[r.targetSide]
		}
		if err := center.Draw(r.exp.Screen); err != nil {
			return onsetNS, err
		}
		// The first frame is the trial onset: the marker's square turns white
		// in this flip, and its TTL code is written right after it, tens of
		// microseconds after the flip returns. The photons come one to three
		// frames later, depending on the display stack; measure that lag with
		// the photodiode and subtract it in analysis.
		if f == 0 {
			code := r.ttlCode
			if ev != nil {
				code |= r.eventCode(ev)
			}
			r.exp.Marker.Arm(code)
		}

		// A sound is queued with the device paused and released when this
		// flip returns, so it starts within one audio buffer of onset_ms.
		var ts uint64
		var err error
		if ev != nil && ev.modality == audio {
			ts, err = r.sounds[ev.side].PlaySyncedWithFlip(r.exp.Screen)
		} else {
			ts, err = r.exp.Screen.FlipTS()
		}
		if err != nil {
			return onsetNS, err
		}
		if f == 0 {
			onsetNS = ts
		}
		if ev != nil {
			// The previous event's window closed at least minSepSec ago.
			r.expire(math.MaxUint64)
			ev.onsetNS = ts
			r.cur = ev
			r.score[ev.modality][ev.side].events++
			r.addRow(row{event: eventNames[ev.modality][ev.side], tsNS: ts, ev: ev})
		}

		r.frame++
		if err := r.pollResponses(); err != nil {
			return onsetNS, err
		}
		r.expire(ts)
		if r.quit {
			return onsetNS, control.EndLoop
		}
	}
	return onsetNS, nil
}

// expire closes the current event's response window if it ended before
// nowNS, logging a miss when no button was pressed in it.
func (r *runner) expire(nowNS uint64) {
	ev := r.cur
	if ev == nil || nowNS <= ev.onsetNS+r.respWindowNS {
		return
	}
	if !ev.responded {
		r.score[ev.modality][ev.side].misses++
		r.addRow(row{event: "miss", tsNS: ev.onsetNS, ev: ev, outcome: "miss"})
	}
	r.cur = nil
}

// pollResponses drains SDL events for this frame, scoring presses of the two
// response buttons and noticing ESC or a window close. Presses are read from
// the event stream rather than from EventState so that auto-repeat from a
// held button is ignored and two presses in one frame are both kept.
func (r *runner) pollResponses() error {
	type press struct {
		side int
		ts   uint64
	}
	var presses []press
	st := r.exp.PollEvents(func(e control.Event) bool {
		if e.Type != control.EVENT_KEY_DOWN {
			return false
		}
		ke := e.KeyboardEvent()
		if ke.Repeat {
			return false
		}
		for side, k := range r.keys {
			if ke.Key == k {
				presses = append(presses, press{side, ke.Timestamp})
			}
		}
		return false
	})
	if st.QuitRequested {
		r.quit = true
	}
	for _, p := range presses {
		r.scorePress(p.side, p.ts)
	}
	return nil
}

// scorePress classifies one button press against the current event.
func (r *runner) scorePress(side int, tsNS uint64) {
	ev := r.cur
	var rt string
	var outcome string
	switch {
	case ev == nil || tsNS < ev.onsetNS || tsNS > ev.onsetNS+r.respWindowNS:
		outcome = "false_alarm"
		r.falseAlarms++
		ev = nil
	case ev.responded:
		outcome = "extra"
		r.extraPresses++
	case side == ev.side:
		outcome = "correct"
		r.score[ev.modality][ev.side].correct++
	default:
		outcome = "wrong"
		r.score[ev.modality][ev.side].wrong++
	}
	if ev != nil {
		ev.responded = true
		rt = strconv.FormatFloat(float64(tsNS-ev.onsetNS)/1e6, 'f', 2, 64)
	}
	r.addRow(row{event: "response", tsNS: tsNS, ev: ev, button: sideNames[side], outcome: outcome, rtMs: rt})
}

// showResults displays correct/total for each modality and side and holds
// until SPACE. Only SPACE ends it, so a late press on a response button cannot
// dismiss it before the experimenter has read it.
func (r *runner) showResults() error {
	var b strings.Builder
	b.WriteString("Run finished.\n\nCorrect responses / total:\n\n")
	labels := [2][2]string{{"Arrow  <", "Arrow  >"}, {"Word \"Left!\"", "Word \"Right!\""}}
	for m := range r.score {
		for side := range r.score[m] {
			s := r.score[m][side]
			fmt.Fprintf(&b, "%s:   %d / %d\n", labels[m][side], s.correct, s.events)
		}
		b.WriteString("\n")
	}
	b.WriteString("Thank you!")
	tb := r.exp.FittedTextBox(b.String())
	defer tb.Unload()
	if err := r.exp.Show(tb); err != nil {
		return err
	}
	return r.exp.Keyboard.WaitKey(control.K_SPACE)
}

// showFullField draws every region at full contrast and holds until a key is
// pressed. This is the check against Figure 1B of the paper: the sector seams
// must fall on the meridians and the checks must line up across annuli.
func (r *runner) showFullField() error {
	all := make([]bool, NRegions)
	for i := range all {
		all[i] = true
	}
	for {
		if err := r.exp.Screen.Clear(); err != nil {
			return err
		}
		for k := range all {
			if err := r.textures[k].SetAlphaMod(255); err != nil {
				return err
			}
			if err := r.exp.Screen.Renderer.RenderTexture(r.textures[k], nil, r.dst[k]); err != nil {
				return err
			}
		}
		if err := r.fix.Draw(r.exp.Screen); err != nil {
			return err
		}
		if _, err := r.exp.Screen.FlipTS(); err != nil {
			return err
		}
		st := r.exp.PollEvents(nil)
		if st.QuitRequested || st.LastKey != 0 {
			return control.EndLoop
		}
	}
}

// row is one line of the data file. Zero fields are written empty.
type row struct {
	event   string
	trial   int // written only when frames > 0 (habituation and trial rows)
	tsNS    uint64
	frames  int
	nOn     int
	on      []bool
	ev      *taskEvent // modality and side, for task events and their responses
	button  string
	outcome string
	rtMs    string
}

// addRow writes one line of the data file. Fields that do not apply to the
// event are left empty rather than filled with a sentinel that analysis code
// might mistake for a measurement.
func (r *runner) addRow(w row) {
	onsetMs := ""
	if w.tsNS != 0 && w.tsNS >= r.t0NS {
		onsetMs = strconv.FormatFloat(float64(w.tsNS-r.t0NS)/1e6, 'f', 3, 64)
	}
	trialField := ""
	framesField, toaField, nOnField := "", "", ""
	if w.frames > 0 {
		trialField = strconv.Itoa(w.trial)
		framesField = strconv.Itoa(w.frames)
		toaField = strconv.FormatFloat(float64(w.frames)*r.frameMs, 'f', 2, 64)
		nOnField = strconv.Itoa(w.nOn)
	}
	modality, side := "", ""
	if w.ev != nil {
		modality, side = modalityNames[w.ev.modality], sideNames[w.ev.side]
	}
	r.exp.Data.Add(w.event, trialField, onsetMs, framesField, toaField, nOnField, bits(w.on),
		modality, side, w.button, w.outcome, w.rtMs)
}

// bits renders the on/off pattern as one character per region, region 0 first.
func bits(on []bool) string {
	if on == nil {
		return ""
	}
	var b strings.Builder
	b.Grow(len(on))
	for _, v := range on {
		if v {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

// ── helpers ────────────────────────────────────────────────────────────────

// parseKey resolves a -left-key/-right-key value to a key code.
func parseKey(name string) (control.Keycode, error) {
	switch n := strings.ToLower(strings.TrimSpace(name)); {
	case n == "left":
		return control.K_LEFT, nil
	case n == "right":
		return control.K_RIGHT, nil
	case n == "up":
		return control.K_UP, nil
	case n == "down":
		return control.K_DOWN, nil
	case n == "space":
		return control.K_SPACE, nil
	// SDL key codes for letters and digits are their ASCII codes, so the
	// K_A..K_Z and K_0..K_9 ranges are contiguous.
	case len(n) == 1 && n[0] >= 'a' && n[0] <= 'z':
		return control.K_A + control.Keycode(n[0]-'a'), nil
	case len(n) == 1 && n[0] >= '0' && n[0] <= '9':
		return control.K_0 + control.Keycode(n[0]-'0'), nil
	}
	return 0, fmt.Errorf("unknown key %q (use left, right, up, down, space, a-z or 0-9)", name)
}

// pcmDurationMs is the length of a loaded sound. The low byte of an SDL audio
// format is its sample size in bits (SDL_AUDIO_BITSIZE).
func pcmDurationMs(s *stimuli.Sound) float64 {
	bytesPerFrame := int(s.Spec.Channels) * (int(s.Spec.Format) & 0xFF / 8)
	if bytesPerFrame == 0 || s.Spec.Freq <= 0 {
		return 0
	}
	return float64(len(s.Data)/bytesPerFrame) * 1000 / float64(s.Spec.Freq)
}

// infoFloat reads a numeric field from the participant dialog, falling back to
// the flag value when the dialog was skipped (-s given, or a browser build).
func infoFloat(info map[string]string, key string, fallback float64) float64 {
	if info == nil {
		return fallback
	}
	v, ok := info[key]
	if !ok {
		return fallback
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f <= 0 {
		return fallback
	}
	return f
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// runVerify implements -verify: build the design, report its measured
// orthogonality, and exit. It runs before SDL is touched so it works over ssh
// and in CI. It returns (exitCode, true) when -verify was requested.
//
// A private FlagSet is used because the experiment's own flags are not
// registered yet; only the parameters that change the design are honoured, and
// anything else on the command line is ignored rather than rejected, so
// habitual invocations like "-verify -w -s 999" still work.
func runVerify() (int, bool) {
	args := os.Args[1:]
	wanted := map[string]bool{"p": true, "shift": true, "trials": true}
	requested := false
	var kept []string
	for i := 0; i < len(args); i++ {
		name := strings.TrimLeft(args[i], "-")
		value := ""
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name, value = name[:eq], name[eq:]
		}
		switch {
		case name == "verify":
			requested = true
		case wanted[name]:
			if value != "" {
				kept = append(kept, "-"+name+value)
			} else if i+1 < len(args) {
				kept = append(kept, "-"+name, args[i+1])
				i++
			}
		}
	}
	if !requested {
		return 0, false
	}

	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	p := fs.Int("p", DefaultPrime, "")
	shift := fs.Int("shift", 0, "")
	trials := fs.Int("trials", 0, "")
	if err := fs.Parse(kept); err != nil {
		return 2, true
	}

	seqs, err := BuildSequences(*p, NRegions, *shift, *trials)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", expName, err)
		return 1, true
	}
	rep := seqs.Report()
	for _, line := range rep.Lines() {
		fmt.Println(line)
	}
	for k, f := range rep.OnFraction {
		fmt.Printf("region %2d: on %3d/%d trials (%.4f)\n", k, rep.OnCount[k], rep.NTrials, f)
	}
	if !rep.OK(1e-6) {
		fmt.Fprintln(os.Stderr, "FAIL: this design is not orthogonal enough to deconvolve")
		return 1, true
	}
	fmt.Println("OK: all correlations at the 1/p floor")
	if !rep.Balanced() {
		fmt.Println("NOTE: this truncated run is not balanced across regions; " +
			"a full period of p trials is.")
	}
	return 0, true
}
