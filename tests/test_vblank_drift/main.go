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
	"strings"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/media/present"
	"github.com/chrplr/goxpyriment/stimuli"
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

	report(exp, samples, missing, nominalMs, ps)
	return control.EndLoop
}

// tee prints to stdout and keeps a copy, so the report a person reads is the
// same text that lands in the file.
//
// Rewriting the report into compact key=value comments was the first attempt,
// and it saved every number while losing the thing that makes them usable: the
// labels, the units, the source-soundness check and the verdict. Someone
// running this on a machine you cannot see should be able to send you the file
// rather than a screenshot of a terminal.
type tee struct{ lines []string }

func (t *tee) printf(format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	fmt.Print(line)
	t.lines = append(t.lines, line)
}

// flush writes the captured report into the data file's companion info file,
// one comment line per output line.
func (t *tee) flush(exp *control.Experiment) {
	exp.Data.WriteComment("--- vblank drift report ---")
	for _, chunk := range t.lines {
		for _, line := range strings.Split(strings.TrimRight(chunk, "\n"), "\n") {
			exp.Data.WriteComment(line)
		}
	}
}

func report(exp *control.Experiment, s []sample, missing int, nominalMs float64, ps control.PacingStats) {
	out := &tee{}
	defer out.flush(exp)

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

	out.printf("\n── Vblank source ─────────────────────────────────────────\n")
	out.printf("  %-22s : %d used, %d flips with no vblank\n", "samples", n, missing)
	out.printf("  %-22s : %s\n", "sequence steps", fmtSteps(steps))
	out.printf("  %-22s : %.4f ms\n", "residual about grid", gridSD)
	if gridSD > 0.5*nominalMs {
		out.printf("  %-22s   THE SOURCE IS NOT SOUND. The kernel's stamps do not fall on a\n", "")
		out.printf("  %-22s   regular grid, so nothing below can be trusted. Panel self-refresh\n", "")
		out.printf("  %-22s   stopping the CRTC will do this; try kmsdrm, or a photodiode.\n", "")
		return
	}
	out.printf("  %-22s   a clean grid: the stamps are usable as a display reference.\n", "")
	exp.Data.WriteComment(fmt.Sprintf("vblank samples=%d missing=%d steps=%s grid_resid_ms=%.4f",
		n, missing, fmtSteps(steps), gridSD))

	out.printf("\n── Panel grid (kernel vblank timestamps) ─────────────────\n")
	out.printf("  %-22s : %.5f ms = %.4f Hz\n", "TRUE frame", panelMs, 1000/panelMs)
	out.printf("  %-22s : %.5f ms = %.4f Hz -> %+.1f ppm\n", "nominal (display mode)",
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

	out.printf("\n── Flip timestamps vs the panel ──────────────────────────\n")
	sortedPhase := append([]float64(nil), phase...)
	sort.Float64s(sortedPhase)
	out.printf("  %-22s : median %.3f ms, range %.3f–%.3f (frame = %.3f)\n", "phase (flip - vblank)",
		sortedPhase[n/2], sortedPhase[0], sortedPhase[n-1], panelMs)
	out.printf("  %-22s : %d of %d frames\n", "longest clean stretch", segLen, n)

	if segLen < 60 {
		out.printf("  %-22s   TOO FRAGMENTED TO FIT. Frames are being dropped often enough\n", "")
		out.printf("  %-22s   that no clean stretch survives. Fix the drops first — try\n", "")
		out.printf("  %-22s   fullscreen, or a session without a compositor.\n", "")
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
	out.printf("  %-22s : %+.4f us/frame = %+.2f ppm\n", "DRIFT", driftPerFrame*1000, driftPPM)
	out.printf("  %-22s : %+.3f ms/min, %+.2f ms over an 8-min block\n", "",
		driftPerFrame*60000/panelMs, driftPerFrame*480000/panelMs)

	out.printf("\n── Pacing branches ───────────────────────────────────────\n")
	total := ps.Blocked + ps.Paced
	pacedPct := 0.0
	if total > 0 {
		pacedPct = 100 * float64(ps.Paced) / float64(total)
	}
	out.printf("  %-22s : %d blocked / %d paced (%.1f %% paced)\n", "branches", ps.Blocked, ps.Paced, pacedPct)
	out.printf("  %-22s : mean %.3f ms, max %.3f ms\n", "early-return wait",
		float64(ps.WaitMean().Nanoseconds())/1e6, float64(ps.WaitMax.Nanoseconds())/1e6)

	out.printf("\n  %-22s : %s\n", "VERDICT", verdict(driftPPM, driftPerFrame, pacedPct))
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
