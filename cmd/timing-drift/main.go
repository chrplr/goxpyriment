// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Command timing-drift joins an external instrument's event log to
// goxpyriment's own run CSV and reports whether the framework's idea of when a
// frame appeared walks away from the actual photons.
//
// # Why this exists
//
// A trigger-to-photodiode interval that slides steadily across a run is the
// worst failure this stack can have — it cannot be averaged out, and it is
// invisible in every statistic computed within a single channel. Both event
// trains stay perfectly regular; only their relative phase moves. The host's
// own console output looks immaculate while it happens, because the flip
// timestamps and the TTL sit on the same drifting grid and therefore agree with
// each other.
//
// It is also invisible in the one summary everybody quotes. Measured on a
// Raspberry Pi 4 (V3D/kmsdrm, 1010 cycles over 505 s, BBTK v3), the
// trigger-to-photodiode SD was 4.07 ms — and essentially all of it was a
// monotonic 14.3 ms ramp, not jitter. De-trended, the same data has an SD of
// about 0.13 ms. An SD alone cannot tell those two apart, so this tool reports
// the slope first and the residual scatter second.
//
// # What it reports
//
//   - trigger -> light, in the instrument's own timebase: slope (µs/cycle and
//     ppm), scatter after removing that slope, and how much of the variance the
//     ramp accounts for. The instrument's clock offset cancels in this
//     subtraction, so it needs nothing but the events file.
//   - with the run CSV as well: the instrument-versus-host clock rate, fitted
//     rather than assumed, and then the quantity actually wanted — flip
//     timestamp to photons, on the host timebase, with that clock rate divided
//     out.
//   - the panel's true frame period implied by the photon train, against the
//     period the host believed in. A mismatch here is a drift the pacing
//     schedule will reproduce for as long as the run lasts.
//
// # Usage
//
//	timing-drift [options] <events.csv> [run.csv]
//
// The events file is the instrument's onset log, with a Type,Onset,Duration
// header — the format a BBTK v3 capture writes. The run CSV is what
// tests/Timing-Tests -test av saves; when it is given, its sibling -info.txt is
// read for the nominal refresh rate unless -hz overrides it.
//
// Options:
//
//	-ttl NAME      trigger channel in the events file (default TTLin1)
//	-light NAME    photodiode channel (default Opto1)
//	-hz F          refresh rate for frame arithmetic (default: from -info.txt)
//	-warmup N      leading cycles to discard (default 10, as the harness does)
//	-out PATH      write the joined per-cycle series as CSV
package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	var (
		fTTL    = flag.String("ttl", "TTLin1", "trigger channel name in the events file")
		fLight  = flag.String("light", "Opto1", "photodiode channel name in the events file")
		fHz     = flag.Float64("hz", 0, "refresh rate in Hz for frame arithmetic (0 = read the run's -info.txt)")
		fWarmup = flag.Int("warmup", 10, "leading cycles to discard")
		fOut    = flag.String("out", "", "write the joined per-cycle series to this CSV")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: timing-drift [options] <events.csv> [run.csv]\n\n")
		fmt.Fprintf(os.Stderr, "Reports whether the flip timestamp drifts against the photons.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 || flag.NArg() > 2 {
		flag.Usage()
		os.Exit(2)
	}
	eventsPath := flag.Arg(0)
	runPath := ""
	if flag.NArg() == 2 {
		runPath = flag.Arg(1)
	}

	if err := run(eventsPath, runPath, *fTTL, *fLight, *fHz, *fWarmup, *fOut); err != nil {
		fmt.Fprintf(os.Stderr, "timing-drift: %v\n", err)
		os.Exit(1)
	}
}

func run(eventsPath, runPath, ttlName, lightName string, hz float64, warmup int, outPath string) error {
	trains, err := readEvents(eventsPath)
	if err != nil {
		return err
	}
	ttl, ok := trains[ttlName]
	if !ok {
		return fmt.Errorf("no %q events in %s (found: %s)", ttlName, eventsPath, strings.Join(trainNames(trains), ", "))
	}
	light, ok := trains[lightName]
	if !ok {
		return fmt.Errorf("no %q events in %s (found: %s)", lightName, eventsPath, strings.Join(trainNames(trains), ", "))
	}

	fmt.Printf("events   : %s\n", eventsPath)
	if runPath != "" {
		fmt.Printf("run      : %s\n", runPath)
	}

	// Pair the two trains. The instrument's own clock offset cancels in the
	// subtraction, which is the whole reason for recording both on one device.
	pairs, unpairedTTL, unpairedLight := pairTrains(ttl, light)
	fmt.Printf("\n── Pairing ───────────────────────────────────────────────\n")
	fmt.Printf("  %-20s : %d\n", ttlName, len(ttl))
	fmt.Printf("  %-20s : %d\n", lightName, len(light))
	fmt.Printf("  %-20s : %d\n", "paired", len(pairs))
	if unpairedTTL > 0 || unpairedLight > 0 {
		fmt.Printf("  %-20s : %d %s / %d %s\n", "UNPAIRED", unpairedTTL, ttlName, unpairedLight, lightName)
		fmt.Printf("  %-20s   an event with no partner within half a cycle usually means a\n", "")
		fmt.Printf("  %-20s   detection threshold that is too low, or a capture window that\n", "")
		fmt.Printf("  %-20s   clipped the run. Check before reading anything below.\n", "")
	}
	if len(pairs) <= warmup+2 {
		return fmt.Errorf("only %d paired cycles after a warmup of %d: nothing to fit", len(pairs), warmup)
	}
	pairs = pairs[warmup:]
	fmt.Printf("  %-20s : %d (first %d discarded)\n", "analysed", len(pairs), warmup)

	// Cycle period, needed to convert a per-cycle slope into ppm.
	ttlOn := make([]float64, len(pairs))
	lightOn := make([]float64, len(pairs))
	delay := make([]float64, len(pairs))
	for i, p := range pairs {
		ttlOn[i] = p.a
		lightOn[i] = p.b
		delay[i] = p.b - p.a
	}
	cycleMs := medianDiff(ttlOn)

	// Frame period: -hz wins, then the run's -info.txt, then nothing.
	frameMs, frameSrc := 0.0, ""
	if runPath != "" {
		info = readRefreshInfo(runPath)
	}
	switch {
	case hz > 0:
		frameMs, frameSrc = 1000.0/hz, "-hz"
	case info.nominal > 0:
		frameMs, frameSrc = 1000.0/info.nominal, "nominal in "+info.path
	case info.measured > 0:
		frameMs, frameSrc = 1000.0/info.measured, "measured in "+info.path
	}

	fmt.Printf("\n── %s → %s (instrument timebase) ──────────────────\n", ttlName, lightName)
	reportSeries(delay, ttlOn, cycleMs, frameMs)
	if frameMs > 0 {
		fmt.Printf("  %-20s : %.5f ms (%.4f Hz, from %s)\n", "frame period", frameMs, 1000.0/frameMs, frameSrc)
	}

	var joined [][]string
	if outPath != "" {
		joined = append(joined, []string{"cycle", "ttl_ms", "light_ms", "ttl_to_light_ms"})
		for i := range pairs {
			joined = append(joined, []string{
				strconv.Itoa(i), f(ttlOn[i]), f(lightOn[i]), f(delay[i]),
			})
		}
	}

	if runPath != "" {
		flips, err := readRunFlips(runPath)
		if err != nil {
			return err
		}
		if err := reportAgainstHost(flips, ttlOn, lightOn, cycleMs, frameMs, warmup, &joined); err != nil {
			return err
		}
	}

	if outPath != "" {
		if err := writeCSV(outPath, joined); err != nil {
			return err
		}
		fmt.Printf("\nwrote %s (%d rows)\n", outPath, len(joined)-1)
	}
	return nil
}

// longestCleanRun returns the widest half-open range [lo,hi) of d containing no
// step bigger than 0.4 of a frame — that is, no dropped or added frame.
//
// Every slope in this program must be fitted inside such a range, because a
// single frame-sized step biases a least-squares slope far more than intuition
// suggests. For a step of height h at index k in n points, the fitted slope is
// 6*h*m*k/n^3 with m = n-k, which peaks at the midpoint at 1.5*h/n. At 60 Hz
// over a 1000-cycle run that is 25 us/cycle, or 50 ppm of a 500 ms cycle —
// TEN TIMES the ~5 ppm effect these captures exist to resolve, from ONE dropped
// frame in eight minutes. The step is not noise that averages down with more
// cycles: n grows, but so does the lever arm.
//
// Verified against a 20-cycle Pi capture with one drop at cycle 13: predicted
// 6*16.66*7*13/20^3 = 1.137 ms/cycle, fitted -1.132 ms/cycle. The tool reported
// that as -2265 ppm of drift on a panel whose real error is single-digit ppm.
//
// Ties go to the earliest range, which is arbitrary but stable across runs.
func longestCleanRun(d []float64, frameMs float64) (lo, hi int) {
	if frameMs <= 0 || len(d) < 2 {
		return 0, len(d)
	}
	bestLo, bestHi, curLo := 0, 0, 0
	for i := 1; i <= len(d); i++ {
		if i == len(d) || math.Abs(d[i]-d[i-1]) > 0.4*frameMs {
			if i-curLo > bestHi-bestLo {
				bestLo, bestHi = curLo, i
			}
			curLo = i
		}
	}
	return bestLo, bestHi
}

// longestCleanIntervalRun returns the widest half-open range [lo,hi) of an
// ONSET train whose consecutive intervals are all within 0.4 of a frame of the
// median interval, plus how many cycles fell outside that tolerance.
//
// longestCleanRun above is the same idea applied to a DIFFERENCE series, and
// the two are not interchangeable. A dropped frame shifts every channel
// together, so it is invisible in a difference and unmissable in an onset
// train: use this one whenever the quantity being fitted is a period, and that
// one whenever it is a delay.
//
// The tolerance is on the median interval rather than a nominal one so this
// needs no assumption about the cycle length, and the median survives the drops
// it is being used to find.
func longestCleanIntervalRun(t []float64, frameMs float64) (lo, hi, dropped int) {
	if frameMs <= 0 || len(t) < 3 {
		return 0, len(t), 0
	}
	iv := make([]float64, len(t)-1)
	for i := 1; i < len(t); i++ {
		iv[i-1] = t[i] - t[i-1]
	}
	sorted := append([]float64(nil), iv...)
	sort.Float64s(sorted)
	med := sorted[len(sorted)/2]

	bestLo, bestHi, curLo := 0, 0, 0
	for i := 0; i <= len(iv); i++ {
		// An interval outside tolerance ends the current stretch: index i is the
		// last usable point of it, and the next stretch starts at i+1 so the
		// long cycle itself is in neither.
		if i == len(iv) || math.Abs(iv[i]-med) > 0.4*frameMs {
			if i < len(iv) {
				dropped++
			}
			if i+1-curLo > bestHi-bestLo {
				bestLo, bestHi = curLo, i+1
			}
			curLo = i + 1
		}
	}
	return bestLo, bestHi, dropped
}

// reportSeries prints the statistics that separate a ramp from jitter.
//
// It deliberately returns nothing. It used to hand back the index range its
// slope was fitted over, "so the caller can fit the period estimates on the
// same cycles" — and that was wrong: this range comes from a DIFFERENCE series,
// where a dropped frame is invisible because it shifts both channels together.
// Reusing it for a period fit walked straight into the drops it appeared to
// exclude. Period fits use longestCleanIntervalRun on an onset train instead.
//
// The raw SD is printed because it is the number everybody quotes, immediately
// beside the two that decompose it. A series that is 99% ramp and a series that
// is 99% scatter can share a raw SD to three significant figures and mean
// completely different things about the apparatus.
func reportSeries(d, t []float64, cycleMs, frameMs float64) {
	n := len(d)
	lo, hi := longestCleanRun(d, frameMs)
	seg := d[lo:hi]
	slope, intercept := leastSquares(seg) // per cycle index within the segment
	resid := make([]float64, len(seg))
	for i := range seg {
		resid[i] = seg[i] - (intercept + slope*float64(i))
	}
	rawSD := sd(d)
	detSD := sd(resid)

	fmt.Printf("  %-20s : %d\n", "n", n)
	fmt.Printf("  %-20s : %.3f ms\n", "mean", mean(d))
	fmt.Printf("  %-20s : %.3f / %.3f ms\n", "min / max", minOf(d), maxOf(d))
	fmt.Printf("  %-20s : %.4f ms   <- the number usually quoted\n", "raw SD", rawSD)
	if hi-lo < n {
		// Never let the fitted span be implicit. A slope quoted over 40% of a
		// run and a slope quoted over all of it are different measurements, and
		// the difference is invisible once the number is copied into a table.
		fmt.Printf("  %-20s : cycles %d..%d (%d of %d) — longest stretch with no dropped frame\n",
			"fitted over", lo, hi-1, hi-lo, n)
	}
	fmt.Printf("  %-20s : %+.3f us/cycle", "slope", slope*1000)
	if cycleMs > 0 {
		fmt.Printf(" (%+.2f ppm of the %.3f ms cycle)", slope/cycleMs*1e6, cycleMs)
	}
	fmt.Println()
	if len(t) == n && hi-lo > 1 {
		elapsedMin := (t[hi-1] - t[lo]) / 60000.0
		if elapsedMin > 0 {
			fmt.Printf("  %-20s : %+.3f ms/min over %.1f min\n", "", slope*float64(hi-lo-1)/elapsedMin, elapsedMin)
		}
	}
	fmt.Printf("  %-20s : %.4f ms   <- the real trial-to-trial scatter\n", "de-trended SD", detSD)
	// Decompose the variance INSIDE the fitted span, not against the whole
	// series: with a frame-sized step excluded from the fit, the full-series SD
	// is mostly that step, and the ratio would credit the ramp with variance it
	// never explained (or exceed 100 % outright).
	if segRawSD := sd(seg); segRawSD > 0 {
		frac := 1 - (detSD*detSD)/(segRawSD*segRawSD)
		if frac < 0 {
			frac = 0
		}
		fmt.Printf("  %-20s : %.1f %% of the variance within that span\n", "ramp accounts for", frac*100)
	}
	total := slope * float64(hi-lo-1)
	fmt.Printf("  %-20s : %+.3f ms over the fitted span", "total drift", total)
	if frameMs > 0 {
		fmt.Printf(" (%.2f frame)", math.Abs(total)/frameMs)
	}
	fmt.Println()

	if frameMs > 0 {
		jumps := 0
		for i := 1; i < n; i++ {
			if math.Abs(d[i]-d[i-1]) > 0.4*frameMs {
				jumps++
			}
		}
		fmt.Printf("  %-20s : %d\n", "one-frame jumps", jumps)
		if jumps > 0 {
			// Say what a drop costs rather than only that it happened: it is
			// excluded from the fit here, but it is still a frame the
			// participant did or did not see, and a run full of them is not a
			// usable capture however clean the surviving stretch looks.
			fmt.Printf("  %-20s   excluded from the fit above; each is a frame the display\n", "")
			fmt.Printf("  %-20s   missed. Investigate the load if there are more than a few.\n", "")
		}
	}

	fmt.Printf("  %-20s : %s\n", "VERDICT", verdict(slope, detSD, sd(seg), cycleMs))
}

// verdict names the failure mode rather than passing or failing, because the
// acceptable slope depends on how long the experiment's blocks are.
func verdict(slopePerCycle, detSD, rawSD, cycleMs float64) string {
	slopePPM := 0.0
	if cycleMs > 0 {
		slopePPM = math.Abs(slopePerCycle / cycleMs * 1e6)
	}
	ramp := 0.0
	if rawSD > 0 {
		ramp = 1 - (detSD*detSD)/(rawSD*rawSD)
	}
	switch {
	case slopePPM > 5 && ramp > 0.5:
		return fmt.Sprintf("DRIFT DOMINATES (%.1f ppm). The two trains are on different "+
			"clocks;\n%24sthe scatter is only %.3f ms once the ramp is removed.", slopePPM, "", detSD)
	case slopePPM > 5:
		return fmt.Sprintf("DRIFT PRESENT (%.1f ppm) on top of real scatter.", slopePPM)
	case detSD > 1.0:
		return fmt.Sprintf("NO DRIFT, but %.3f ms of scatter.", detSD)
	default:
		return fmt.Sprintf("STABLE: no meaningful drift, %.3f ms scatter.", detSD)
	}
}

// reportAgainstHost brings the host's own flip timestamps into the comparison.
//
// The instrument and the host each run off their own crystal, so their clocks
// differ by tens of ppm before anything interesting has happened — 47 ppm in the
// Pi capture. That offset has to be fitted out, not assumed away, or it is
// indistinguishable from the drift being hunted.
//
// Fitting the trigger train against the flip timestamps is what does it: the
// trigger is raised immediately after the flip returns, so that relation carries
// the clock-rate ratio and nothing else. The photon train is on the panel's own
// grid, so whatever is left over after applying that ratio is real.
func reportAgainstHost(flips, ttlOn, lightOn []float64, cycleMs, frameMs float64, warmup int, joined *[][]string) error {
	off, ok := alignOffset(flips, ttlOn, warmup)
	if !ok {
		return fmt.Errorf("cannot align %d run rows to %d instrument cycles: no offset in [-10,10] gives a stable fit", len(flips), len(ttlOn))
	}
	n := len(ttlOn)
	if off+n > len(flips) {
		n = len(flips) - off
	}
	if n < 3 {
		return fmt.Errorf("only %d cycles overlap between the run CSV and the events file", n)
	}
	fl := flips[off : off+n]
	tt := ttlOn[:n]
	li := lightOn[:n]

	// Fit instrument = a + b*host over the trigger/flip pair.
	b, a := leastSquaresXY(fl, tt)
	fitResid := make([]float64, n)
	for i := range tt {
		fitResid[i] = tt[i] - (a + b*fl[i])
	}

	fmt.Printf("\n── Host clock vs instrument clock ────────────────────────\n")
	if off != 0 {
		fmt.Printf("  %-20s : %d run rows skipped to align\n", "offset", off)
	}
	fmt.Printf("  %-20s : %d\n", "n", n)
	fmt.Printf("  %-20s : %+.2f ppm\n", "instrument rate", (b-1)*1e6)
	fmt.Printf("  %-20s : %.4f ms\n", "fit residual SD", sd(fitResid))
	fmt.Printf("  %-20s   fitted from trigger-vs-flip, which carries the clock ratio\n", "")
	fmt.Printf("  %-20s   and nothing else. Divided out below.\n", "")

	// Map the photons back onto the host clock and subtract the flip timestamp.
	fp := make([]float64, n)
	for i := range li {
		photonHost := (li[i] - a) / b
		fp[i] = photonHost - fl[i]
	}
	fmt.Printf("\n── flip timestamp → photons (host timebase) ──────────────\n")
	reportSeries(fp, fl, cycleMs, frameMs)
	fmt.Printf("  %-20s   this is the quantity that lands in a reaction time.\n", "")

	// The panel's true period against the one the host ran on.
	//
	// Both periods come from a least-squares fit against the cycle index, NOT
	// from a median interval. The instrument quantises onsets to 0.25 ms, so a
	// median interval is only good to about 250 ppm per frame — five times the
	// effect being looked for, and enough on its own to invent a drift that is
	// not there. Fitting a line through a thousand points averages that
	// quantisation down to well under a ppm.
	//
	// A dropped frame biases a fitted period exactly as it biases a fitted
	// slope, so this fit needs a dropped-frame-free span too — but NOT the one
	// the difference series was fitted over.
	//
	// That was the bug. A cycle that runs one frame long delays the TTL, the
	// photons and the flip stamp alike, so it leaves NO step in their
	// difference: longestCleanRun over the delay series correctly reports zero
	// one-frame jumps and hands back the whole run. Fitting the period over that
	// span then walks straight into the drop. Measured on a 1000-cycle Pi 4
	// capture on 2026-08-16 with exactly one drop: the tool reported the panel
	// 29.9 ppm away from the display mode, while the median cycle put it at
	// 0.1 ppm — the whole discrepancy was that one cycle. Earlier, on a 20-cycle
	// capture, the same mechanism produced 2265 ppm.
	//
	// So the span for the period comes from the ONSET intervals, where a drop is
	// visible as a cycle a frame longer than its neighbours.
	plo, phi, dropped := longestCleanIntervalRun(fl, frameMs)
	panelSlope, _ := leastSquares(li[plo:phi])
	hostSlope, _ := leastSquares(fl[plo:phi])
	panelMs := panelSlope / b
	hostMs := hostSlope
	fmt.Printf("\n── Cycle period: panel vs host ───────────────────────────\n")
	fmt.Printf("  %-20s : %.5f ms  (fitted over %d cycles)\n", "from photons", panelMs, phi-plo)
	fmt.Printf("  %-20s : %.5f ms\n", "from flip stamps", hostMs)
	if dropped > 0 {
		// Say what was excluded. A period quoted over 43 % of the run because
		// the drops fell badly is a different measurement from one over 99 %,
		// and the reader cannot tell from the number itself.
		fmt.Printf("  %-20s : %d cycle(s) ran a frame or more long; the fit uses the\n",
			"frames dropped", dropped)
		fmt.Printf("  %-20s   longest clean stretch, %d of %d cycles (%.0f %%).\n", "",
			phi-plo, len(fl), 100*float64(phi-plo)/float64(len(fl)))
	}
	if panelMs > 0 {
		mism := (hostMs - panelMs) / panelMs * 1e6
		// Accumulated over the cycles the periods were actually fitted on, not
		// over the whole capture: quoting a total for cycles the fit excluded
		// would overstate it in exactly the runs that had drops.
		fmt.Printf("  %-20s : %+.2f ppm  (%+.3f ms over %d cycles)\n", "mismatch", mism,
			(hostMs-panelMs)*float64(phi-plo-1), phi-plo)
		if frameMs > 0 {
			fpc := math.Round(panelMs / frameMs)
			if fpc >= 1 {
				// Dividing the cycle period by a whole number of frames only
				// gives the panel's rate if the cycle really contained that
				// many frames. When the compositor drops or adds frames the
				// cycle is systematically longer, and the quotient is a blend
				// of the refresh rate and the drop rate — which reads as a
				// wildly wrong panel, not as the dropped frames it is.
				//
				// 500 ppm is the line. A panel genuinely disagreeing with its
				// own display mode lands well under it (0.1 and 25 ppm on the
				// two clean runs measured here); one dropped frame every few
				// dozen cycles lands well over (990 ppm for one in 34).
				excess := panelMs - fpc*frameMs
				if math.Abs(excess/panelMs) > 500e-6 {
					fmt.Printf("\n  %-27s : %+.3f ms/cycle beyond %.0f x nominal frame\n", "DROPPED/ADDED FRAMES", excess, fpc)
					fmt.Printf("  %-27s   about 1 extra frame every %.0f cycles. The panel rate\n", "", math.Abs(frameMs/excess))
					fmt.Printf("  %-27s   cannot be recovered from this run — fix the drops first.\n", "")
				} else {
					fmt.Printf("\n  %-27s : %.5f ms = %.4f Hz  (over %.0f frames/cycle)\n", "TRUE frame", panelMs/fpc, 1000.0/(panelMs/fpc), fpc)
					fmt.Printf("  %-27s : %.5f ms = %.4f Hz\n", "host frame", hostMs/fpc, 1000.0/(hostMs/fpc))
					reportRefreshEstimates(panelMs / fpc)
				}
			}
		}
		fmt.Printf("\n  %-20s   pacing that advances on a period other than the TRUE one\n", "")
		fmt.Printf("  %-20s   reproduces this mismatch for as long as the block lasts.\n", "")
	}

	if *joined != nil {
		hdr := (*joined)[0]
		(*joined)[0] = append(hdr, "flip_ms", "flip_to_photon_ms")
		for i := 0; i < n && i+1 < len(*joined); i++ {
			(*joined)[i+1] = append((*joined)[i+1], f(fl[i]), f(fp[i]))
		}
	}
	return nil
}

// ---------- input ----------

type pair struct{ a, b float64 }

// readEvents reads an instrument events file (Type,Onset,Duration[,...]) into
// one onset train per channel, sorted by time.
func readEvents(path string) (map[string][]float64, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("%s: no data rows", path)
	}
	typeCol, onsetCol := -1, -1
	for i, h := range rows[0] {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "type":
			typeCol = i
		case "onset":
			onsetCol = i
		}
	}
	if typeCol < 0 || onsetCol < 0 {
		return nil, fmt.Errorf("%s: need Type and Onset columns, got %v", path, rows[0])
	}
	out := map[string][]float64{}
	for _, r := range rows[1:] {
		if len(r) <= typeCol || len(r) <= onsetCol {
			continue
		}
		v, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(r[onsetCol]), `"`), 64)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(r[typeCol])
		out[name] = append(out[name], v)
	}
	for _, v := range out {
		sort.Float64s(v)
	}
	return out, nil
}

// readRunFlips pulls the visual-onset flip timestamps out of a Timing-Tests
// run CSV. t_visual_after_ms is the flip's return — the instant the trigger is
// raised from — so it is the one to compare the instrument against.
func readRunFlips(path string) ([]float64, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("%s: no data rows", path)
	}
	col := -1
	for i, h := range rows[0] {
		if strings.TrimSpace(h) == "t_visual_after_ms" {
			col = i
		}
	}
	if col < 0 {
		return nil, fmt.Errorf("%s: no t_visual_after_ms column (is this a -test av run?)", path)
	}
	var out []float64
	for _, r := range rows[1:] {
		if len(r) <= col {
			continue
		}
		v, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(r[col]), `"`), 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// refreshInfo holds the two refresh rates the framework recorded for a run:
// what the display mode claimed (nominal) and what Screen.CalibrateRefresh
// measured over its 60 startup frames (measured).
//
// Both are estimates of the same physical quantity, and this tool can compute a
// third that is far better than either, so it is worth printing all three side
// by side — whichever one the pacing schedule advances on is the one whose error
// becomes a drift.
type refreshInfo struct {
	nominal  float64
	measured float64
	path     string
	ok       bool
}

// info is populated from the run's sibling -info.txt when one is given.
var info refreshInfo

// readRefreshInfo reads both refresh rates out of the run's sibling -info.txt.
func readRefreshInfo(runPath string) refreshInfo {
	infoPath := strings.TrimSuffix(runPath, ".csv") + "-info.txt"
	fh, err := os.Open(infoPath)
	if err != nil {
		return refreshInfo{}
	}
	defer fh.Close()
	out := refreshInfo{path: infoPath}
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		line := sc.Text()
		for _, k := range []struct {
			key string
			dst *float64
		}{
			{"# sys refresh_nominal_hz:", &out.nominal},
			{"# sys refresh_measured_hz:", &out.measured},
		} {
			if rest, found := strings.CutPrefix(line, k.key); found {
				if v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64); err == nil {
					*k.dst = v
				}
			}
		}
	}
	out.ok = out.nominal > 0 || out.measured > 0
	return out
}

// reportRefreshEstimates scores the framework's two refresh-rate estimates
// against the rate this tool just derived from the photons.
//
// This is the number that decides what a pacing schedule should advance on. It
// is not obvious in advance which estimate wins: CalibrateRefresh takes the
// median of 59 intervals, which on a quantised or non-blocking driver can be a
// worse estimate of the panel than the display mode's own nominal figure.
func reportRefreshEstimates(trueFrameMs float64) {
	if !info.ok || trueFrameMs <= 0 {
		return
	}
	trueHz := 1000.0 / trueFrameMs
	for _, e := range []struct {
		name string
		hz   float64
	}{
		{"nominal (display mode)", info.nominal},
		{"measured (CalibrateRefresh)", info.measured},
	} {
		if e.hz <= 0 {
			continue
		}
		errPPM := (1000.0/e.hz - trueFrameMs) / trueFrameMs * 1e6
		fmt.Printf("  %-27s : %.4f Hz -> %+.1f ppm vs TRUE\n", e.name, e.hz, errPPM)
	}
	_ = trueHz
}

func readCSV(path string) ([][]string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	r := csv.NewReader(fh)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	return r.ReadAll()
}

func writeCSV(path string, rows [][]string) error {
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	w := csv.NewWriter(fh)
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

// ---------- pairing ----------

// pairTrains matches each trigger onset to the nearest light onset within half
// a cycle, rather than pairing by index.
//
// Index pairing breaks on a single spurious event: one extra detection at the
// start shifts every subsequent pair by one cycle and turns the whole series
// into a constant near minus one period. That is not hypothetical — a capture
// taken with every BBTK threshold at 63 produces exactly it, reading -476 ms
// where the true delay is about 20 ms.
func pairTrains(a, b []float64) (pairs []pair, unpairedA, unpairedB int) {
	if len(a) < 2 || len(b) == 0 {
		return nil, len(a), len(b)
	}
	window := medianDiff(a) / 2
	if window <= 0 {
		window = math.Inf(1)
	}
	used := make([]bool, len(b))
	j := 0
	for _, t := range a {
		// b is sorted; advance to the first candidate that could be nearest.
		for j < len(b)-1 && b[j+1] <= t {
			j++
		}
		best, bestD := -1, math.Inf(1)
		for k := j; k < len(b) && b[k]-t <= window; k++ {
			if used[k] {
				continue
			}
			if d := math.Abs(b[k] - t); d < bestD {
				best, bestD = k, d
			}
		}
		// Also look just behind, in case the light led the trigger.
		for k := j; k >= 0 && t-b[k] <= window; k-- {
			if used[k] {
				continue
			}
			if d := math.Abs(b[k] - t); d < bestD {
				best, bestD = k, d
			}
		}
		if best < 0 {
			unpairedA++
			continue
		}
		used[best] = true
		pairs = append(pairs, pair{a: t, b: b[best]})
	}
	for _, u := range used {
		if !u {
			unpairedB++
		}
	}
	return pairs, unpairedA, unpairedB
}

// alignOffset finds how many leading run rows to skip so that run row off+i
// describes the same cycle as instrument cycle i.
//
// The instrument capture window brackets the run, so the counts normally match
// once the warmup is accounted for; this searches a small neighbourhood and
// picks the offset whose trigger-versus-flip fit is tightest, so a capture that
// started a cycle late is corrected rather than silently mis-joined.
func alignOffset(flips, ttl []float64, warmup int) (int, bool) {
	if len(flips) == 0 || len(ttl) < 3 {
		return 0, false
	}
	best, bestSD := -1, math.Inf(1)
	for off := warmup - 10; off <= warmup+10; off++ {
		if off < 0 || off+3 > len(flips) {
			continue
		}
		n := len(ttl)
		if off+n > len(flips) {
			n = len(flips) - off
		}
		if n < 3 {
			continue
		}
		b, a := leastSquaresXY(flips[off:off+n], ttl[:n])
		res := make([]float64, n)
		for i := 0; i < n; i++ {
			res[i] = ttl[i] - (a + b*flips[off+i])
		}
		if s := sd(res); s < bestSD {
			best, bestSD = off, s
		}
	}
	return best, best >= 0
}

// ---------- statistics ----------

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

func minOf(v []float64) float64 {
	m := math.Inf(1)
	for _, x := range v {
		if x < m {
			m = x
		}
	}
	return m
}

func maxOf(v []float64) float64 {
	m := math.Inf(-1)
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}

// leastSquares fits v[i] = intercept + slope*i.
func leastSquares(v []float64) (slope, intercept float64) {
	x := make([]float64, len(v))
	for i := range v {
		x[i] = float64(i)
	}
	return leastSquaresXY(x, v)
}

// leastSquaresXY fits y = intercept + slope*x.
func leastSquaresXY(x, y []float64) (slope, intercept float64) {
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

// medianDiff returns the median interval between consecutive values. The median
// is deliberate: a capture that dropped one event leaves a doubled interval,
// which would move a mean but not this.
func medianDiff(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	d := make([]float64, len(v)-1)
	for i := 1; i < len(v); i++ {
		d[i-1] = v[i] - v[i-1]
	}
	sort.Float64s(d)
	return d[len(d)/2]
}

func trainNames(m map[string][]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func f(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
