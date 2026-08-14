// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Vblank Drift Test — does Screen.FlipTS track the display, without a photodiode?
//
// # The question
//
// FlipTS is the timestamp every experiment measures reaction times from, and
// every trigger is fired off its return. On a driver whose SDL_RenderPresent
// blocks, it carries the present's own return instant and follows the hardware.
// On one that does not, Screen.Update holds the frame itself and stamps the flip
// with its own SCHEDULE — a computed time that never touched the display. That
// schedule advances at the nominal refresh rate, so if the panel's true rate
// differs at all, the two walk apart at a fixed rate, for as long as the block
// lasts.
//
// This is the failure mode that cost a whole measurement campaign. On a
// Raspberry Pi 4 the framework's flip timestamps ended an 8-minute run 14 ms
// away from the actual photons, growing linearly, while the program's own
// console output was immaculate: period SD 0.006 ms, zero frames more than
// 0.5 ms late. Nothing computed from the flip timestamps alone can see it,
// because the error is in the flip timestamps.
//
// # Why this can see it anyway
//
// The kernel stamps every vertical blank, and DRM_IOCTL_WAIT_VBLANK hands that
// stamp over. It is an independent clock on the display itself — the same role a
// photodiode plays, minus the photons. Regressing the flip period against the
// vblank period gives the drift in ppm directly.
//
// Validated against the one machine where both were available: consecutive
// vblank timestamps on a Precision 5490 gave 60.0384 Hz where a BBTK photodiode
// gave 60.0385 Hz over 1000 cycles — agreement to 1.3 ppm.
//
// # What it does NOT measure
//
// The vblank is when the panel starts scanning out, not when a photon leaves the
// pixel the participant is looking at. Scanout position, panel rise time and the
// display's own pipeline are all outside this test — they are constant offsets a
// photodiode measures and this cannot. What this measures is whether the offset
// is CONSTANT, which is the part that matters for EEG/MEG and the part that no
// amount of host-side statistics can otherwise reach.
//
// Usage:
//
//	go run ./tests/test_vblank_drift              # fullscreen, 30 s
//	go run ./tests/test_vblank_drift -w           # windowed
//	go run ./tests/test_vblank_drift -frames 3600 # 60 s at 60 Hz
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/media/present"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/tests/internal/report"
)

var (
	fFrames = flag.Int("frames", 1800, "Frames to measure (1800 = 30 s at 60 Hz)")
	fWarmup = flag.Int("warmup", 10, "Leading frames discarded from the statistics")
)

// sample is one flip and the vblank the kernel reported for it.
type sample struct {
	flipNS   uint64
	vblankNS uint64
}

func main() {
	exp := control.NewExperimentFromFlags("Vblank Drift Test", control.Black, control.White, 24)
	defer exp.End()

	// Run re-invokes the logic every frame until it returns EndLoop, so measure
	// ends with one rather than nil — returning nil restarts the whole
	// measurement forever.
	if err := exp.Run(func() error { return measure(exp) }); err != nil && !control.IsEndLoop(err) {
		log.Fatalf("%v", err)
	}
}

func measure(exp *control.Experiment) error {
	screen := exp.Screen

	timer := present.AutoDetect(screen)
	defer timer.Close()

	fmt.Printf("vblank backend: %s\n", timer.Description())
	if timer.Precision() != present.HardwareVerified {
		// Refusing is the whole point. The fallback timer returns the flip
		// timestamp as the onset, so every number below would come out exactly
		// zero — a clean bill of health from a measurement that never happened.
		fmt.Fprintf(os.Stderr, "\nNo hardware vblank source on this system, so there is nothing\n"+
			"to compare the flip timestamps against. This test cannot run here.\n\n"+
			"On Linux it needs read/write on /dev/dri/cardN — usually membership\n"+
			"of the 'video' group, though a local login often grants it already.\n"+
			"On Windows there is no backend yet.\n")
		return fmt.Errorf("no hardware-verified vblank source")
	}

	nominal := screen.FrameDuration()
	nominalMs := float64(nominal.Nanoseconds()) / 1e6
	fmt.Printf("nominal frame : %.5f ms (%.4f Hz)\n", nominalMs, 1000/nominalMs)
	fmt.Printf("measuring %d frames (~%.0f s)…\n", *fFrames, float64(*fFrames)*nominalMs/1000)

	// A real draw call every frame: a frame carrying only a clear is not
	// reliably scanned out under a compositor (see apparatus.fillWholeTarget),
	// and a frame that never reaches the panel is not a frame this test can
	// reason about. The patch alternates so the run is visibly alive.
	patch := stimuli.NewRectangle(0, 0, 200, 200, control.White)
	defer func() { _ = patch.Unload() }()

	// Save the raw per-frame pairs, not just the summary. The summary is printed
	// to stdout and nowhere else, so on a machine someone else is running it on,
	// the numbers exist only until the terminal scrolls. The per-frame rows let
	// the fit be redone — and disagreed with — afterwards.
	exp.AddDataVariableNames([]string{"frame", "flip_ms", "vblank_ms", "phase_ms"})

	samples := make([]sample, 0, *fFrames)
	missing := 0
	screen.ResetPacingStats()

	for i := 0; i < *fFrames; i++ {
		if err := screen.Clear(); err != nil {
			return err
		}
		if i%2 == 0 {
			if err := patch.Draw(screen); err != nil {
				return err
			}
		}
		ts, err := screen.FlipTS()
		if err != nil {
			return err
		}
		timer.RecordFlip(ts)
		vts, _, ok := timer.OnsetForFlip(ts)
		if !ok {
			missing++
			continue
		}
		samples = append(samples, sample{flipNS: ts, vblankNS: vts})
		exp.Data.Add(i,
			fmt.Sprintf("%.6f", float64(ts)/1e6),
			fmt.Sprintf("%.6f", float64(vts)/1e6),
			fmt.Sprintf("%.6f", float64(ts-vts)/1e6))
	}
	ps := screen.PacingStats()

	if len(samples) > *fWarmup {
		samples = samples[*fWarmup:]
	}
	if len(samples) < 60 {
		return fmt.Errorf("only %d usable samples (%d flips had no vblank): too few to fit", len(samples), missing)
	}

	analyse(exp, samples, missing, nominalMs, ps)
	return control.EndLoop
}

func analyse(exp *control.Experiment, s []sample, missing int, nominalMs float64, ps control.PacingStats) {
	out := &report.Tee{}
	defer out.Flush(exp.Data, "vblank drift report")

	n := len(s)

	// Recover which vblank each sample landed on. The ioctl returns the most
	// recent vblank, so with one flip per frame the stamp advances by one frame
	// each time; a dropped frame shows up as a step of two. Counting steps
	// rather than assuming one lets the grid fit survive the drops instead of
	// being skewed by them.
	seq := make([]float64, n)
	steps := map[int]int{}
	for i := 1; i < n; i++ {
		d := float64(s[i].vblankNS-s[i-1].vblankNS) / 1e6
		k := int(math.Round(d / nominalMs))
		if k < 0 {
			k = 0
		}
		steps[k]++
		seq[i] = seq[i-1] + float64(k)
	}

	// The panel's own grid, fitted from the kernel's stamps.
	vbMs := make([]float64, n)
	flMs := make([]float64, n)
	idx := make([]float64, n)
	for i := range s {
		vbMs[i] = float64(s[i].vblankNS) / 1e6
		flMs[i] = float64(s[i].flipNS) / 1e6
		idx[i] = float64(i)
	}
	panelMs, panelIntercept := leastSquares(seq, vbMs)
	gridResid := make([]float64, n)
	for i := range vbMs {
		gridResid[i] = vbMs[i] - (panelIntercept + panelMs*seq[i])
	}
	gridSD := sd(gridResid)

	out.Printf("\n── Vblank source ─────────────────────────────────────────\n")
	out.Printf("  %-22s : %d used, %d flips with no vblank\n", "samples", n, missing)
	out.Printf("  %-22s : %s\n", "sequence steps", fmtSteps(steps))
	// Grade the grid against what a good driver actually delivers, not against
	// half a frame.
	//
	// Half a frame was the first threshold and it was useless: it passed a
	// Raspberry Pi 4 (V3D) at 73.7 µs with the words "a clean grid", when the
	// same test on amdgpu gives 0.8 µs and on i915 about 3 µs. Ninety times the
	// scatter of known-good hardware is worth saying out loud — it does not
	// invalidate the slope, whose standard error stays well under a ppm at this
	// sample count, but it is the first thing to suspect if the numbers below
	// disagree with a photodiode.
	const (
		gridGoodMs  = 0.010 // amdgpu 0.0008, i915 0.003 — comfortably inside
		gridNoisyMs = 0.200 // V3D 0.0737 lands here: usable, but not comparable
	)
	out.Printf("  %-22s : %.4f ms\n", "residual about grid", gridSD)
	switch {
	case gridSD > gridNoisyMs:
		out.Printf("  %-22s   THE SOURCE IS NOT SOUND. The kernel's stamps do not fall on a\n", "")
		out.Printf("  %-22s   regular grid, so nothing below can be trusted. Panel self-refresh\n", "")
		out.Printf("  %-22s   stopping the CRTC will do this; try kmsdrm, or a photodiode.\n", "")
		exp.Data.WriteComment(fmt.Sprintf("vblank samples=%d missing=%d steps=%s grid_resid_ms=%.4f grade=UNSOUND",
			n, missing, fmtSteps(steps), gridSD))
		return
	case gridSD > gridGoodMs:
		out.Printf("  %-22s   NOISY (%.0fx a well-behaved driver, which gives under %.3f ms).\n", "", gridSD/gridGoodMs, gridGoodMs)
		out.Printf("  %-22s   Read the confidence interval on the drift below before quoting it:\n", "")
		out.Printf("  %-22s   this scatter is autocorrelated, so it widens that interval far more\n", "")
		out.Printf("  %-22s   than the sample count alone suggests.\n", "")
	default:
		out.Printf("  %-22s   a clean grid: the stamps are usable as a display reference.\n", "")
	}
	exp.Data.WriteComment(fmt.Sprintf("vblank samples=%d missing=%d steps=%s grid_resid_ms=%.4f",
		n, missing, fmtSteps(steps), gridSD))

	out.Printf("\n── Panel grid (kernel vblank timestamps) ─────────────────\n")
	out.Printf("  %-22s : %.5f ms = %.4f Hz\n", "TRUE frame", panelMs, 1000/panelMs)
	out.Printf("  %-22s : %.5f ms = %.4f Hz -> %+.1f ppm\n", "nominal (display mode)",
		nominalMs, 1000/nominalMs, (nominalMs-panelMs)/panelMs*1e6)

	// The measurement is the PHASE: how far into the frame the flip timestamp
	// falls, and whether that walks.
	//
	// The tempting shortcut — flip period from index versus panel period from
	// sequence — is wrong, and wrong in the flattering direction. It divides
	// elapsed time by loop iterations on one side and by vblanks on the other,
	// so a single dropped frame counts as a whole frame of "drift". Measured on
	// this laptop, three drops in 890 frames turned a real drift under 100 ppm
	// into a reported 5105 ppm.
	//
	// Phase is immune to that: a dropped frame leaves the flip's position within
	// the frame exactly where it was. What phase cannot survive is a boundary
	// crossing, where the flip slips past a vblank and the phase resets by a
	// whole frame — indistinguishable from a drop in a single sample. So the fit
	// runs over the longest stretch with no step of any kind, and its length is
	// reported so the lever arm is visible rather than assumed.
	phase := make([]float64, n)
	for i := range s {
		phase[i] = flMs[i] - vbMs[i]
	}
	lo, hi := longestCleanRun(phase, seq, panelMs)
	segLen := hi - lo + 1

	out.Printf("\n── Flip timestamps vs the panel ──────────────────────────\n")
	sortedPhase := append([]float64(nil), phase...)
	sort.Float64s(sortedPhase)
	out.Printf("  %-22s : median %.3f ms, range %.3f–%.3f (frame = %.3f)\n", "phase (flip - vblank)",
		sortedPhase[n/2], sortedPhase[0], sortedPhase[n-1], panelMs)
	out.Printf("  %-22s : %d of %d frames\n", "longest clean stretch", segLen, n)

	if segLen < 60 {
		out.Printf("  %-22s   TOO FRAGMENTED TO FIT. Frames are being dropped often enough\n", "")
		out.Printf("  %-22s   that no clean stretch survives. Fix the drops first — try\n", "")
		out.Printf("  %-22s   fullscreen, or a session without a compositor.\n", "")
		return
	}

	driftPerFrame, _ := leastSquares(idx[lo:hi+1], phase[lo:hi+1])
	driftPPM := driftPerFrame / panelMs * 1e6
	exp.Data.WriteComment(fmt.Sprintf("panel true_frame_ms=%.5f true_hz=%.4f nominal_hz=%.4f nominal_err_ppm=%+.1f",
		panelMs, 1000/panelMs, 1000/nominalMs, (nominalMs-panelMs)/panelMs*1e6))
	exp.Data.WriteComment(fmt.Sprintf("drift us_per_frame=%+.4f ppm=%+.2f fitted_over=%d_of_%d_frames",
		driftPerFrame*1000, driftPPM, segLen, n))
	exp.Data.WriteComment(fmt.Sprintf("pacing blocked=%d paced=%d wait_mean_ms=%.3f wait_max_ms=%.3f",
		ps.Blocked, ps.Paced, float64(ps.WaitMean().Nanoseconds())/1e6, float64(ps.WaitMax.Nanoseconds())/1e6))

	// Uncertainty on the slope, corrected for autocorrelated residuals.
	//
	// The textbook standard error assumes independent residuals. These are not:
	// on a V3D/kmsdrm run the lag-1 autocorrelation is 0.98, which makes the
	// naive figure about ten times too optimistic — it claimed "well under a
	// ppm" on a machine whose answer moved 26 ppm between two runs thirteen
	// minutes apart. Quoting a precision the data does not support is the same
	// error this whole test exists to catch, so the correction is applied here
	// rather than left to the reader.
	//
	// The AR(1) variance inflation factor (1+rho)/(1-rho) is the standard
	// first-order correction. It is a lower bound when the residual is closer to
	// a random walk than to AR(1), so treat the interval as optimistic even
	// after this.
	seSlope := slopeStdErr(idx[lo:hi+1], phase[lo:hi+1], driftPerFrame)
	drift95PPM := 1.96 * seSlope / panelMs * 1e6

	out.Printf("  %-22s : %+.4f us/frame = %+.2f ppm  (95%% CI +-%.2f)\n", "DRIFT",
		driftPerFrame*1000, driftPPM, drift95PPM)
	out.Printf("  %-22s : %+.3f ms/min, %+.2f ms over an 8-min block\n", "",
		driftPerFrame*60000/panelMs, driftPerFrame*480000/panelMs)
	if math.Abs(driftPPM) < drift95PPM {
		out.Printf("  %-22s   NOT RESOLVED — the drift is inside its own confidence interval.\n", "")
		out.Printf("  %-22s   Run longer, or on a driver with less noise in its vblank stamps.\n", "")
	}
	exp.Data.WriteComment(fmt.Sprintf("drift ci95_ppm=%.2f", drift95PPM))

	// Is the phase actually a straight line? A slope alone cannot say.
	//
	// The paced branch stamps the flip with the schedule, but the BLOCKED branch
	// re-anchors to the present's real return, which tracks the hardware. Each
	// blocked frame therefore claws back part of the accumulated schedule error,
	// and a run with a scattering of them has a phase that ramps and partially
	// resets — a sawtooth whose fitted slope understates the schedule's true
	// rate error. The resets are far too small to trip the boundary-crossing
	// check (tens of µs against a 0.4-frame threshold), so nothing else here
	// would show them.
	//
	// This is not hypothetical: an AMD run with 1 blocked frame in 1800 matched
	// its predicted drift to 0.03 ppm, while a Pi 4 run with 24 blocked frames
	// came in 8 ppm below prediction. Lag-1 autocorrelation of the residuals
	// separates the two cases — white residuals mean a clean line, strongly
	// positive ones mean structure the slope is averaging over.
	fitResid := make([]float64, 0, segLen)
	for i := lo; i <= hi; i++ {
		fitResid = append(fitResid, phase[i]-(phase[lo]+driftPerFrame*float64(i-lo)))
	}
	residSD := sd(fitResid)
	residAC1 := autocorr1(fitResid)
	out.Printf("  %-22s : SD %.4f ms, lag-1 autocorr %+.3f\n", "fit residual", residSD, residAC1)
	switch {
	case residAC1 > 0.8 && ps.Blocked > 2:
		out.Printf("  %-22s   STRUCTURED, not noise, and %d presents took the blocked branch.\n", "", ps.Blocked)
		out.Printf("  %-22s   Those re-anchor the schedule to hardware, so the slope above is a\n", "")
		out.Printf("  %-22s   LOWER BOUND on the schedule's rate error. Compare it with the\n", "")
		out.Printf("  %-22s   nominal-vs-TRUE figure above: a gap is this, not measurement noise.\n", "")
	case residAC1 > 0.8:
		out.Printf("  %-22s   STRUCTURED, not noise. Something is modulating the phase on a\n", "")
		out.Printf("  %-22s   timescale longer than a frame; the slope is an average over it.\n", "")
	default:
		out.Printf("  %-22s   residuals look like noise: the phase is a straight line and the\n", "")
		out.Printf("  %-22s   slope is the whole story.\n", "")
	}
	exp.Data.WriteComment(fmt.Sprintf("drift fit_resid_sd_ms=%.4f fit_resid_ac1=%+.3f", residSD, residAC1))

	out.Printf("\n── Pacing branches ───────────────────────────────────────\n")
	total := ps.Blocked + ps.Paced
	pacedPct := 0.0
	if total > 0 {
		pacedPct = 100 * float64(ps.Paced) / float64(total)
	}
	out.Printf("  %-22s : %d blocked / %d paced (%.1f %% paced)\n", "branches", ps.Blocked, ps.Paced, pacedPct)
	out.Printf("  %-22s : mean %.3f ms, max %.3f ms\n", "early-return wait",
		float64(ps.WaitMean().Nanoseconds())/1e6, float64(ps.WaitMax.Nanoseconds())/1e6)

	out.Printf("\n  %-22s : %s\n", "VERDICT", verdict(driftPPM, driftPerFrame, pacedPct))
}

// verdict reads the drift against what an 8-minute block can tolerate. 1 ppm is
// 0.5 ms over such a block, which is under the frame quantisation nobody can
// code around; 10 ppm is 5 ms, which is not.
func verdict(driftPPM, driftPerFrame, pacedPct float64) string {
	const indent = "\n                           "
	over8min := math.Abs(driftPerFrame) * 480000 / 16.6667
	switch {
	case math.Abs(driftPPM) > 10:
		return fmt.Sprintf("DRIFTING (%.1f ppm, %.1f ms per 8-min block).", driftPPM, over8min) +
			indent + fmt.Sprintf("Flip timestamps are walking away from the display. With %.0f %% of", pacedPct) +
			indent + "presents paced, they are being stamped with the schedule, and the" +
			indent + "schedule's rate is wrong. Absolute onsets from this machine are" +
			indent + "not usable; a photodiode is the only way to recover them."
	case math.Abs(driftPPM) > 1:
		return fmt.Sprintf("SLIGHT DRIFT (%.1f ppm, %.1f ms per 8-min block).", driftPPM, over8min) +
			indent + "Below one frame over a long block, but one-signed, so it grows."
	default:
		return fmt.Sprintf("STABLE (%.2f ppm, %.2f ms per 8-min block).", driftPPM, over8min) +
			indent + "The flip timestamps track the display."
	}
}

// longestCleanRun returns the widest index range over which the vblank sequence
// advanced by exactly one per flip and the phase did not jump.
//
// Both conditions matter and they catch different things: a sequence step of two
// is a dropped frame, while a phase jump with a step of one is the flip slipping
// across a vblank boundary. Either breaks a straight-line fit through the phase,
// and neither is visible in the fitted slope afterwards — the fit just comes out
// wrong, with a plausible-looking standard error.
func longestCleanRun(phase, seq []float64, panelMs float64) (lo, hi int) {
	n := len(phase)
	if n == 0 {
		return 0, -1
	}
	bestLo, bestHi, curLo := 0, 0, 0
	for i := 1; i < n; i++ {
		stepped := math.Abs(seq[i]-seq[i-1]-1) > 0.5
		jumped := math.Abs(phase[i]-phase[i-1]) > 0.4*panelMs
		if stepped || jumped {
			curLo = i
			continue
		}
		if i-curLo > bestHi-bestLo {
			bestLo, bestHi = curLo, i
		}
	}
	return bestLo, bestHi
}

func fmtSteps(steps map[int]int) string {
	keys := make([]int, 0, len(steps))
	for k := range steps {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := ""
	for _, k := range keys {
		if out != "" {
			out += "  "
		}
		out += fmt.Sprintf("%dx:%d", k, steps[k])
	}
	return out
}

// leastSquares fits y = intercept + slope*x.
func leastSquares(x, y []float64) (slope, intercept float64) {
	n := len(x)
	if n != len(y) || n < 2 {
		return 0, 0
	}
	mx, my := mean(x), mean(y)
	var sxy, sxx float64
	for i := 0; i < n; i++ {
		dx := x[i] - mx
		sxy += dx * (y[i] - my)
		sxx += dx * dx
	}
	if sxx == 0 {
		return 0, my
	}
	slope = sxy / sxx
	return slope, my - slope*mx
}

// slopeStdErr returns the standard error of a fitted slope, inflated for
// autocorrelation in the residuals.
//
// With independent residuals the error is sd/sqrt(Sxx). Real phase residuals are
// nothing like independent — a slow wander in the driver's vblank stamps makes
// consecutive ones almost equal — and ignoring that understates the uncertainty
// by roughly sqrt((1+rho)/(1-rho)), which at rho = 0.98 is a factor of ten.
//
// rho is clamped below 1 because the factor diverges there, and a diverging
// error bar is less useful than a large one.
func slopeStdErr(x, y []float64, slope float64) float64 {
	n := len(x)
	if n < 3 {
		return math.Inf(1)
	}
	resid := make([]float64, n)
	for i := range x {
		resid[i] = y[i] - (y[0] + slope*(x[i]-x[0]))
	}
	mx := mean(x)
	var sxx float64
	for _, v := range x {
		sxx += (v - mx) * (v - mx)
	}
	if sxx == 0 {
		return math.Inf(1)
	}
	naive := sd(resid) / math.Sqrt(sxx)
	rho := autocorr1(resid)
	if rho < 0 {
		rho = 0
	}
	if rho > 0.995 {
		rho = 0.995
	}
	return naive * math.Sqrt((1+rho)/(1-rho))
}

// autocorr1 is the lag-1 autocorrelation of v: ~0 for white noise, near 1 for a
// slow ramp or sawtooth. It is what separates "the phase is a line plus noise"
// from "the phase has structure the slope is averaging over".
func autocorr1(v []float64) float64 {
	if len(v) < 3 {
		return 0
	}
	m := mean(v)
	var num, den float64
	for i := range v {
		d := v[i] - m
		den += d * d
		if i > 0 {
			num += d * (v[i-1] - m)
		}
	}
	if den == 0 {
		return 0
	}
	return num / den
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func sd(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	m := mean(v)
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)))
}
