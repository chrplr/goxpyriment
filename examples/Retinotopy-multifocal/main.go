// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Command Retinotopy-multifocal presents the multifocal retinotopic mapping
// stimulus of Kurki, Hyvarinen, Henriksson & Vanni, "Dynamics of retinotopic
// spatial attention revealed by multifocal MEG", NeuroImage 263 (2022) 119643.
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
// This program implements the mapping localizer only. The attention
// manipulation that is the subject of the paper (attend-fixation /
// attend-left / attend-right) is not reproduced; the task here is the central
// fixation colour-change task, which keeps fixation and gives a behavioural
// check that the subject was engaged.
//
// Parameters the paper does not state are flags, and are listed in README.md.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/units"
)

const (
	expName = "Retinotopy-multifocal"

	// Trial onset asynchrony range, in seconds (paper: 217-317 ms). Converted
	// to a whole number of frames at the display's measured refresh rate.
	toaMinSec = 0.217
	toaMaxSec = 0.317

	// Fixation colour-change target, in seconds (paper: 800 ms).
	targetDurSec = 0.800
	// Bounds on the gap between successive targets. The paper does not give a
	// distribution; these keep the task attentionally demanding without
	// crowding the 2-minute run.
	targetGapMinSec = 3.0
	targetGapMaxSec = 8.0

	fixCrossSizePx = 16
	fixCrossLinePx = 3
)

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
		flagSeed    = flag.Uint64("seed", 1, "Random seed for trial durations and target times")
		flagWidthCm = flag.Float64("screen-width-cm", 30, "Screen width in cm (used when the info dialog is skipped)")
		flagDistCm  = flag.Float64("viewing-distance-cm", 50, "Viewing distance in cm (used when the info dialog is skipped)")

		flagWaitTrig = flag.Bool("wait-trigger", false, "Wait for the scanner pulse (key 't') before starting the run")
		flagTrigger  = flag.String("trigger", "none", "TTL device for per-trial markers: "+TriggerDeviceNames)
		flagTrigDev  = flag.String("trigger-device", "", "Serial port or host for -trigger devices that need one")
		flagTrigLine = flag.Int("trigger-line", 0, "TTL line to pulse (0-7)")
		flagTrigMs   = flag.Int("trigger-ms", 5, "TTL pulse width in ms; must be shorter than the minimum trial")
	)
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

	if *flagTrigMs >= int(toaMinSec*1000) {
		log.Fatalf("-trigger-ms=%d must be shorter than the shortest trial (%.0f ms)",
			*flagTrigMs, toaMinSec*1000)
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

	fire, closeTrigger, err := openTrigger(*flagTrigger, *flagTrigDev, *flagTrigLine, *flagTrigMs)
	if err != nil {
		log.Fatalf("%s: %v", expName, err)
	}
	defer closeTrigger()

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

		// ── Frame-locked timing ─────────────────────────────────────────────
		refresh := float64(exp.Screen.RefreshRate())
		if refresh <= 0 {
			refresh = 60
			log.Print("Warning: refresh rate unknown, assuming 60 Hz")
		}
		minFrames := maxInt(1, int(math.Round(toaMinSec*refresh)))
		maxFrames := maxInt(minFrames, int(math.Round(toaMaxSec*refresh)))
		frameMs := 1000.0 / refresh

		r := &runner{
			exp:          exp,
			textures:     textures,
			dst:          dst,
			fix:          stimuli.NewFixCross(fixCrossSizePx, fixCrossLinePx, control.Red),
			fire:         fire,
			frameMs:      frameMs,
			targetFrames: maxInt(1, int(math.Round(targetDurSec*refresh))),
			gapMinFrames: maxInt(1, int(math.Round(targetGapMinSec*refresh))),
			gapMaxFrames: maxInt(2, int(math.Round(targetGapMaxSec*refresh))),
			rng:          rand.New(rand.NewPCG(*flagSeed, uint64(exp.SubjectID))),
		}

		// ── Data file header ────────────────────────────────────────────────
		exp.Data.WriteComment("--MULTIFOCAL RETINOTOPY")
		exp.Data.WriteComment("m paradigm: Kurki et al. 2022, NeuroImage 263:119643 (mapping localizer only)")
		exp.Data.WriteComment("m stimulation: pattern onset, contrast fades linearly to 0 over each trial")
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
		for _, line := range RegionTableLines(ringDeg) {
			exp.Data.WriteComment("m " + line)
		}
		exp.AddDataVariableNames([]string{
			"event_type", "trial", "onset_ms", "toa_frames", "toa_ms",
			"n_on", "regions", "rt_ms",
		})

		// ── Static geometry check ───────────────────────────────────────────
		if *flagFull {
			log.Printf("full-field check: max eccentricity %.2f deg, %d px square",
				ringDeg[NRings], sizePx)
			return r.showFullField()
		}

		// ── Start ───────────────────────────────────────────────────────────
		if !*flagAuto {
			msg := "Multifocal retinotopy\n\n" +
				"Keep your eyes on the central cross at all times.\n\n" +
				"Press SPACE whenever the cross turns green.\n\n" +
				"Press SPACE to continue."
			if ierr := exp.ShowInstructions(msg); ierr != nil {
				return ierr
			}
		}
		if *flagWaitTrig {
			green := stimuli.NewFixCross(fixCrossSizePx, fixCrossLinePx, control.Green)
			if serr := exp.Show(green); serr != nil {
				return serr
			}
			if kerr := exp.Keyboard.WaitKey(control.K_T); kerr != nil {
				return kerr
			}
		}
		// t0 anchors every onset. It is on the SDL clock, the same one that
		// stamps flips (FlipTS) and key events, so onsets and responses are
		// directly comparable without crossing clocks.
		r.t0NS = control.TicksNS()
		r.nextTargetFrame = r.gapMinFrames

		// ── Habituation: full-field flashes ─────────────────────────────────
		allOn := make([]bool, NRegions)
		for i := range allOn {
			allOn[i] = true
		}
		for i := 0; i < *flagHabit; i++ {
			frames := minFrames + r.rng.IntN(maxFrames-minFrames+1)
			onset, perr := r.presentTrial(allOn, frames)
			if perr != nil {
				return perr
			}
			r.addRow("habituation", i, onset, frames, NRegions, allOn, "")
		}

		// ── Mapping sequence ────────────────────────────────────────────────
		for t := 0; t < seqs.NTrials; t++ {
			on := seqs.On[t]
			frames := minFrames + r.rng.IntN(maxFrames-minFrames+1)
			onset, perr := r.presentTrial(on, frames)
			if perr != nil {
				return perr
			}
			nOn := 0
			for _, v := range on {
				if v {
					nOn++
				}
			}
			r.addRow("trial", t, onset, frames, nOn, on, "")
		}

		// Return to a uniform grey field before the end message.
		if berr := exp.Screen.ClearAndUpdate(); berr != nil {
			return berr
		}
		exp.Data.WriteComment(fmt.Sprintf("m targets: %d  hits: %d  responses: %d",
			r.nTargets, r.nHits, r.nResponses))
		log.Printf("targets %d, hits %d, responses %d", r.nTargets, r.nHits, r.nResponses)

		// ShowEndMessage waits for a keypress, which would hang an unattended
		// run, so -autostart skips it as well as the instructions.
		if !*flagAuto {
			summary := fmt.Sprintf("Run finished.\n\nYou caught %d of %d colour changes.\n\nThank you!",
				r.nHits, r.nTargets)
			if eerr := exp.ShowEndMessage(summary); eerr != nil {
				return eerr
			}
		}
		return control.EndLoop
	})
	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("%s: %v", expName, runErr)
	}
}

// ── the frame loop ─────────────────────────────────────────────────────────

// runner holds the state the frame loop mutates: it exists so the habituation
// block and the mapping sequence share one presentation path, and so the
// fixation task keeps running across both.
type runner struct {
	exp      *control.Experiment
	textures []*apparatus.Texture
	dst      []*control.FRect
	fix      *stimuli.FixCross
	fire     func()
	rng      *rand.Rand

	frameMs      float64
	targetFrames int
	gapMinFrames int
	gapMaxFrames int

	t0NS  uint64
	frame int // frames since the run started
	quit  bool

	targetEndFrame  int
	nextTargetFrame int
	lastTargetNS    uint64
	targetPending   bool

	nTargets, nHits, nResponses int
	hitCounted                  bool
}

// presentTrial shows one pattern-onset trial and returns the SDL timestamp of
// its first flip.
//
// The regions that are on appear at full contrast on the first frame and fade
// linearly to zero, reaching the background exactly as the next trial begins.
// Contrast is set with SetAlphaMod: over a mid-grey background, alpha is
// Michelson contrast, so no pixels are touched on the CPU in this loop.
func (r *runner) presentTrial(on []bool, frames int) (uint64, error) {
	var onsetNS uint64
	for f := 0; f < frames; f++ {
		// The fixation target is scheduled in whole frames so its onset is
		// locked to a flip like everything else.
		startedTarget := false
		if r.frame >= r.nextTargetFrame {
			r.targetEndFrame = r.frame + r.targetFrames
			r.nextTargetFrame = r.targetEndFrame + r.gapMinFrames +
				r.rng.IntN(r.gapMaxFrames-r.gapMinFrames+1)
			r.hitCounted = false
			r.nTargets++
			startedTarget = true
		}
		if r.frame < r.targetEndFrame {
			r.fix.Color = control.Green
		} else {
			r.fix.Color = control.Red
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
		if err := r.fix.Draw(r.exp.Screen); err != nil {
			return onsetNS, err
		}

		ts, err := r.exp.Screen.FlipTS()
		if err != nil {
			return onsetNS, err
		}
		if f == 0 {
			onsetNS = ts
			// Immediately after the flip, on this thread: the rising edge
			// lands tens of microseconds after the photons.
			r.fire()
		}
		if startedTarget {
			r.lastTargetNS = ts
			r.targetPending = true
			r.addRow("fix_target", -1, ts, 0, 0, nil, "")
		}

		r.frame++
		if err := r.pollResponses(); err != nil {
			return onsetNS, err
		}
		if r.quit {
			return onsetNS, control.EndLoop
		}
	}
	return onsetNS, nil
}

// pollResponses drains SDL events for this frame, logging key presses and
// noticing ESC or a window close.
func (r *runner) pollResponses() error {
	st := r.exp.PollEvents(nil)
	if st.QuitRequested {
		r.quit = true
	}
	if st.LastKey == control.K_SPACE && st.LastKeyTimestamp != 0 {
		r.nResponses++
		rt := ""
		// A response counts as a hit if it follows a target within the window
		// the target was visible plus a grace period of the same length.
		if r.targetPending && st.LastKeyTimestamp > r.lastTargetNS {
			d := float64(st.LastKeyTimestamp-r.lastTargetNS) / 1e6
			rt = strconv.FormatFloat(d, 'f', 2, 64)
			if !r.hitCounted && d <= 2*targetDurSec*1000 {
				r.nHits++
				r.hitCounted = true
			}
		}
		r.addRow("response", -1, st.LastKeyTimestamp, 0, 0, nil, rt)
	}
	return nil
}

// showFullField draws every region at full contrast and holds until a key is
// pressed. This is the check against Figure 1B of the paper: the sector seams
// must fall on the meridians and the checks must line up across annuli.
func (r *runner) showFullField() error {
	all := make([]bool, NRegions)
	for i := range all {
		all[i] = true
	}
	r.fix.Color = control.Red
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

// addRow writes one line of the data file. Fields that do not apply to the
// event are left empty rather than filled with a sentinel that analysis code
// might mistake for a measurement.
func (r *runner) addRow(event string, trial int, tsNS uint64, frames, nOn int, on []bool, rtMs string) {
	onsetMs := ""
	if tsNS != 0 && tsNS >= r.t0NS {
		onsetMs = strconv.FormatFloat(float64(tsNS-r.t0NS)/1e6, 'f', 3, 64)
	}
	trialField := ""
	if trial >= 0 {
		trialField = strconv.Itoa(trial)
	}
	framesField, toaField, nOnField := "", "", ""
	if frames > 0 {
		framesField = strconv.Itoa(frames)
		toaField = strconv.FormatFloat(float64(frames)*r.frameMs, 'f', 2, 64)
		nOnField = strconv.Itoa(nOn)
	}
	r.exp.Data.Add(event, trialField, onsetMs, framesField, toaField, nOnField, bits(on), rtMs)
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
