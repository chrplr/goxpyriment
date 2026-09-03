// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_tobii checks a Tobii Pro tracker reached through tobii_bridge.py, and
// measures what it actually delivers.
//
// It answers four questions, in order of how much they matter:
//
//  1. Which way up are the coordinates? Tobii reports gaze in normalized
//     display-area coordinates and the SDK headers shipped with it never say
//     where the origin is. -corners shows a target in each corner in turn and
//     prints the gaze measured against the position expected, so the answer
//     comes from the tracker rather than from documentation. Get this wrong and
//     every gaze position is mirrored vertically, which reads as a bad
//     calibration rather than a units bug. Run it FIRST.
//
//  2. Does calibration work, and did it store anything? Unlike an EyeLink,
//     nothing in the Tobii SDK draws a target: goxpyriment does, through
//     control.Experiment.CalibrateTracker, on its own flip clock. The stored
//     result including the per-target sample counts goes to the -info.txt.
//
//  3. What sample rate arrives? Counted over a measured interval and compared
//     with the rate the tracker says it is running at. A tracker set to 600 Hz
//     that delivers 300 is a finding; so is a nonzero Dropped().
//
//  4. How do the two clocks relate? The tracker clock and this machine's clock
//     are sampled before and after the run; the change between them is the
//     drift. Tobii's system_time_stamp is CLOCK_MONOTONIC microseconds on the
//     bridge's machine, so when the bridge runs here the offset should be a
//     constant rather than a drifting rate — which is what the drift figure
//     tests.
//
// Usage:
//
//	# on the display machine, in one terminal:
//	PYTHONPATH=~/tobii_eyetracker_pythonlib \
//	    python3 eyetracker/bridge/tobii_bridge.py --edf-dir /tmp
//
//	# in another — settle the coordinate origin before anything else:
//	go run ./tests/test_tobii -w -s 999 -corners
//
//	# then a calibrated run with a live gaze dot:
//	go run ./tests/test_tobii -w -s 999 -calibrate -gaze -fetch /tmp/gaze.tsv
//
//	# no hardware at all — bridge started with --simulate:
//	python3 eyetracker/bridge/tobii_bridge.py --simulate
//	go run ./tests/test_tobii -w -s 999
//
// The gaze data itself is written by the bridge, not by this program: Tobii has
// no tracker-side data file, so the bridge writes a full-field TSV at the
// tracker's full rate and -fetch copies it here. The CSV this program writes
// holds one row per trial measured from this side, which is a different and
// much coarser thing.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/eyetracker"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	// Flags must be declared before NewExperimentFromFlags, which parses.
	fBridge := flag.String("bridge", eyetracker.DefaultBridgeAddr, "address of tobii_bridge.py")
	fAddress := flag.String("tracker-address", "", "tracker URI, e.g. tet-tcp://169.254.1.2 (empty: the bridge's own choice)")
	fFile := flag.String("file", "goxtest_tobii.tsv", "name of the gaze file the bridge writes")
	fFetch := flag.String("fetch", "", "after the run, copy the gaze file here")
	fTrials := flag.Int("trials", 20, "number of fixation trials")
	fHold := flag.Duration("hold", 800*time.Millisecond, "how long each fixation target stays on")
	fISI := flag.Duration("isi", 400*time.Millisecond, "blank interval between trials")
	fCalibrate := flag.Bool("calibrate", false, "calibrate before recording (goxpyriment draws the targets)")
	fPoints := flag.Int("points", 9, "calibration targets: 3, 5, 9 or 13")
	fDwell := flag.Int("dwell", 700, "ms each calibration target is shown before it is sampled")
	fGaze := flag.Bool("gaze", false, "before the trials, show a live gaze dot until a key is pressed")
	fCorners := flag.Bool("corners", false, "measure the coordinate origin against four corners, then exit")
	fRateSecs := flag.Float64("rate-secs", 3.0, "seconds over which to measure the sample rate (0: skip)")
	fSync := flag.Int("sync", 20, "clock round trips per synchronisation")

	exp := control.NewExperimentFromFlags("Tobii Bridge Test", control.Black, control.White, 32)
	defer exp.End()

	cfg := config{
		bridgeAddr: *fBridge,
		address:    *fAddress,
		file:       *fFile,
		fetch:      *fFetch,
		trials:     *fTrials,
		hold:       *fHold,
		isi:        *fISI,
		calibrate:  *fCalibrate,
		points:     *fPoints,
		dwell:      *fDwell,
		gaze:       *fGaze,
		corners:    *fCorners,
		rateSecs:   *fRateSecs,
		syncN:      *fSync,
	}

	if err := run(exp, cfg); err != nil {
		log.Printf("test_tobii: %v", err)
		os.Exit(1)
	}
}

type config struct {
	bridgeAddr string
	address    string
	file       string
	fetch      string
	trials     int
	hold       time.Duration
	isi        time.Duration
	calibrate  bool
	points     int
	dwell      int
	gaze       bool
	corners    bool
	rateSecs   float64
	syncN      int
}

func run(exp *control.Experiment, cfg config) error {
	w, h, err := exp.Screen.Size()
	if err != nil {
		return fmt.Errorf("reading the screen size: %w", err)
	}
	geom := eyetracker.Geometry{WidthPx: int(w), HeightPx: int(h)}

	// The clock matters: samples are stamped with whatever this returns, and
	// comparing them against Screen.FlipTS only means anything if both are on
	// SDL's clock. The default is the Go monotonic clock, which is NOT.
	tracker := eyetracker.NewBridge(cfg.bridgeAddr,
		eyetracker.WithGeometry(geom),
		eyetracker.WithHost(cfg.address),
		eyetracker.WithEDF(cfg.file),
		eyetracker.WithClock(func() int64 { return int64(control.TicksNS()) }),
	)

	fmt.Printf("connecting to the bridge at %s ...\n", cfg.bridgeAddr)
	if err := tracker.Open(); err != nil {
		return err
	}
	defer tracker.Close()

	fmt.Printf("bridge %q, protocol ok, screen %dx%d\n", tracker.BridgeID(), w, h)
	exp.AddExperimentInfo(fmt.Sprintf("eyetracker bridge: %s at %s", tracker.BridgeID(), cfg.bridgeAddr))
	exp.AddExperimentInfo(fmt.Sprintf("eyetracker simulated: %t", tracker.Simulated()))
	exp.AddExperimentInfo(fmt.Sprintf("eyetracker gaze file: %s", cfg.file))
	exp.AddExperimentInfo("eyetracker pupil units: diameter in mm (Tobii)")
	if tracker.Simulated() {
		fmt.Println("\n*** THE BRIDGE IS SIMULATING THE TRACKER — this run contains no real gaze data ***")
	}

	before, err := tracker.Sync(cfg.syncN)
	if err != nil {
		return fmt.Errorf("synchronising clocks: %w", err)
	}
	reportOffset("before", before)
	exp.AddExperimentInfo(fmt.Sprintf("clock offset before: %.3f ms (best RTT %v, n=%d)",
		before.DeltaMs, before.BestRTT, before.Samples))

	if cfg.calibrate {
		fmt.Printf("calibrating with %d targets, %d ms each — seat the participant first.\n",
			len(eyetracker.StandardPoints(cfg.points)), cfg.dwell)
		if err := exp.CalibrateTracker(tracker, eyetracker.CalibrationOptions{
			Points:  cfg.points,
			DwellMs: cfg.dwell,
		}); err != nil {
			// Carrying on is deliberate for the timing and rate figures, which
			// do not depend on where the eye is pointing. It is NOT acceptable
			// for the corner check or for any real experiment.
			fmt.Printf("\ncalibration failed: %v\ncontinuing — the rate and clock "+
				"figures are still valid, the gaze POSITIONS are not\n", err)
		} else if msg, ok := tracker.CalibrationMessage(); ok {
			fmt.Printf("calibration stored: %s\n", msg)
		}
	}

	if err := tracker.StartRecording(); err != nil {
		return fmt.Errorf("starting the recording: %w", err)
	}

	if cfg.corners {
		if err := checkCorners(exp, tracker, geom); err != nil {
			return err
		}
		if err := tracker.StopRecording(); err != nil {
			return fmt.Errorf("stopping the recording: %w", err)
		}
		return fetch(tracker, cfg)
	}

	if cfg.gaze {
		if err := showLiveGaze(exp, tracker, geom); err != nil {
			return err
		}
	}

	if cfg.rateSecs > 0 {
		measureRate(exp, tracker, cfg.rateSecs)
	}

	rows, err := runTrials(exp, tracker, geom, cfg)
	if err != nil {
		return err
	}

	if err := tracker.StopRecording(); err != nil {
		return fmt.Errorf("stopping the recording: %w", err)
	}

	after, err := tracker.Sync(cfg.syncN)
	if err != nil {
		return fmt.Errorf("re-synchronising clocks: %w", err)
	}
	reportOffset("after", after)
	exp.AddExperimentInfo(fmt.Sprintf("clock offset after: %.3f ms (best RTT %v, n=%d)",
		after.DeltaMs, after.BestRTT, after.Samples))

	elapsedS := float64(after.LocalNs-before.LocalNs) / 1e9
	driftMs := after.DeltaMs - before.DeltaMs
	if elapsedS > 0 {
		fmt.Printf("\nclock drift: %+.3f ms over %.1f s = %+.2f ppm\n",
			driftMs, elapsedS, driftMs/1000/elapsedS*1e6)
		fmt.Println("  (Tobii's system_time_stamp is CLOCK_MONOTONIC on the bridge's")
		fmt.Println("   machine. With the bridge on THIS machine a drift far from zero")
		fmt.Println("   means the two are not the same counter after all.)")
		exp.AddExperimentInfo(fmt.Sprintf("clock drift: %+.3f ms over %.1f s (%+.2f ppm)",
			driftMs, elapsedS, driftMs/1000/elapsedS*1e6))
	}

	summarise(rows, tracker)
	return fetch(tracker, cfg)
}

func fetch(tracker *eyetracker.Bridge, cfg config) error {
	if cfg.fetch == "" {
		fmt.Println("\nthe gaze file stays where the bridge wrote it; use -fetch to copy it here")
		return nil
	}
	if err := tracker.ReceiveDataFile(cfg.fetch); err != nil {
		return fmt.Errorf("fetching the gaze file: %w", err)
	}
	fmt.Printf("gaze file written to %s\n", cfg.fetch)
	return nil
}

// checkCorners settles which way up the tracker's normalized coordinates are.
//
// It shows a target at each of four corners and the centre, averages the gaze
// during each, and prints the normalized position measured beside the one
// expected. If the Y column comes back mirrored (a target at 0.1 measuring
// near 0.9), the conversion in tobii_bridge.py is upside down and every gaze
// position in every recording is wrong.
//
// This asks the hardware a question the documentation does not answer, so its
// output belongs in the commit message that settles it.
func checkCorners(exp *control.Experiment, tracker *eyetracker.Bridge,
	geom eyetracker.Geometry) error {

	type probe struct {
		name   string
		nx, ny float64
	}
	probes := []probe{
		{"centre", 0.5, 0.5},
		{"top-left", 0.1, 0.1},
		{"top-right", 0.9, 0.1},
		{"bottom-left", 0.1, 0.9},
		{"bottom-right", 0.9, 0.9},
	}

	fmt.Println("\ncoordinate origin check — fixate each target as it appears")
	target := stimuli.NewCircle(16, control.White)

	type result struct {
		probe
		obsX, obsY float64
		n          int
	}
	var out []result

	for _, p := range probes {
		x, y := geom.ToCentre(p.nx*float64(geom.WidthPx), p.ny*float64(geom.HeightPx))
		target.SetPosition(sdl.FPoint{X: float32(x), Y: float32(y)})
		if _, err := exp.ShowTS(target); err != nil {
			return err
		}
		// Let the eye get there before anything is counted.
		if err := exp.Wait(500); err != nil {
			return err
		}
		tracker.DrainSamples()
		if err := exp.Wait(1000); err != nil {
			return err
		}
		var sx, sy float64
		var n int
		for _, s := range tracker.DrainSamples() {
			if !s.Valid {
				continue
			}
			sx += s.X
			sy += s.Y
			n++
		}
		r := result{probe: p, n: n}
		if n > 0 {
			r.obsX = sx / float64(n) / float64(geom.WidthPx)
			r.obsY = sy / float64(n) / float64(geom.HeightPx)
		}
		out = append(out, r)
		if err := exp.Blank(200); err != nil {
			return err
		}
	}

	fmt.Println("\ntarget          expected nx,ny    measured nx,ny    samples")
	var yErrSame, yErrFlipped float64
	for _, r := range out {
		if r.n == 0 {
			fmt.Printf("%-14s  %.2f, %.2f        (no valid samples)\n", r.name, r.nx, r.ny)
			continue
		}
		fmt.Printf("%-14s  %.2f, %.2f        %.3f, %.3f     %d\n",
			r.name, r.nx, r.ny, r.obsX, r.obsY, r.n)
		yErrSame += math.Abs(r.obsY - r.ny)
		yErrFlipped += math.Abs(r.obsY - (1 - r.ny))
	}
	// The verdict, stated rather than left to the reader: comparing the
	// residual against the mirrored expectation is the whole point of the run.
	fmt.Printf("\nmean |Y error| as-is: %.3f    if Y were mirrored: %.3f\n",
		yErrSame/float64(len(out)), yErrFlipped/float64(len(out)))
	verdict := "the normalized origin is TOP-LEFT, as the bridge assumes"
	if yErrFlipped < yErrSame {
		verdict = "*** Y IS MIRRORED: the bridge's normalized→pixel conversion " +
			"is upside down and must be fixed before any recording is trusted ***"
	}
	fmt.Println(verdict)
	exp.AddExperimentInfo("coordinate origin check: " + verdict)
	return nil
}

// measureRate counts the samples that arrive over a measured interval.
//
// It reports the count and the duration alongside the rate, because a rate on
// its own cannot be checked and cannot be compared with another run.
func measureRate(exp *control.Experiment, tracker *eyetracker.Bridge, secs float64) {
	fmt.Printf("\nmeasuring the sample rate over %.1f s ...\n", secs)
	cross := stimuli.NewFixCross(40, 3, control.White)
	_, _ = exp.ShowTS(cross)

	tracker.DrainSamples()
	droppedBefore := tracker.Dropped()
	t0 := control.TicksNS()
	var n, valid int
	for {
		elapsed := float64(control.TicksNS()-t0) / 1e9
		if elapsed >= secs {
			break
		}
		for _, s := range tracker.DrainSamples() {
			n++
			if s.Valid {
				valid++
			}
		}
		if err := exp.Wait(10); err != nil {
			return
		}
	}
	elapsed := float64(control.TicksNS()-t0) / 1e9
	for _, s := range tracker.DrainSamples() {
		n++
		if s.Valid {
			valid++
		}
	}
	dropped := tracker.Dropped() - droppedBefore

	// The bridge emits one sample event PER EYE, so the gaze rate is half the
	// event rate on a binocular tracker. Reporting only one of the two numbers
	// is how a 600 Hz tracker gets written up as 1200 Hz.
	fmt.Printf("  %d sample events (%d valid) in %.3f s = %.1f events/s\n",
		n, valid, elapsed, float64(n)/elapsed)
	fmt.Printf("  = %.1f gaze samples/s if binocular (one event per eye)\n",
		float64(n)/elapsed/2)
	if dropped > 0 {
		fmt.Printf("  *** %d samples DROPPED: the client is not draining fast enough "+
			"and the record has holes ***\n", dropped)
	}
	exp.AddExperimentInfo(fmt.Sprintf(
		"sample rate: %d events (%d valid) in %.3f s = %.1f events/s (%.1f gaze/s binocular), %d dropped",
		n, valid, elapsed, float64(n)/elapsed, float64(n)/elapsed/2, dropped))
}

// trialRow is one fixation trial as measured from this machine.
type trialRow struct {
	trial     int
	flipNS    uint64
	targetX   float64 // centre-origin screen coordinates, +Y up
	targetY   float64
	gazeX     float64 // mean gaze, same coordinates
	gazeY     float64
	errPx     float64
	pupilMM   float64
	nSamples  int
	nValid    int
	trackerMs float64
}

func runTrials(exp *control.Experiment, tracker *eyetracker.Bridge,
	geom eyetracker.Geometry, cfg config) ([]trialRow, error) {

	exp.AddDataVariableNames([]string{
		"trial", "flip_ns", "target_x", "target_y", "gaze_x", "gaze_y",
		"error_px", "pupil_mm", "n_samples", "n_valid", "tracker_ms", "dropped",
	})

	target := stimuli.NewCircle(12, control.White)
	// A ring of positions rather than the centre every time, so a systematic
	// offset shows up as a pattern instead of a constant.
	positions := eyetracker.StandardPoints(9)
	rows := make([]trialRow, 0, cfg.trials)

	for i := 0; i < cfg.trials; i++ {
		if aborted(exp) {
			fmt.Println("aborted by the operator")
			break
		}
		p := positions[i%len(positions)]
		tx, ty := geom.ToCentre(p[0]*float64(geom.WidthPx), p[1]*float64(geom.HeightPx))
		target.SetPosition(sdl.FPoint{X: float32(tx), Y: float32(ty)})

		flipNS, err := exp.ShowTS(target)
		if err != nil {
			return rows, fmt.Errorf("trial %d: %w", i+1, err)
		}
		// Discard whatever was in flight from the last trial: the eye was
		// somewhere else then, and averaging it in would bias every row.
		tracker.DrainSamples()
		if err := exp.Wait(int(cfg.hold.Milliseconds())); err != nil {
			return rows, err
		}

		row := trialRow{trial: i + 1, flipNS: flipNS, targetX: tx, targetY: ty}
		var sx, sy, pupil float64
		// Draining every trial is the cadence the buffer is sized for; see
		// Bridge.Dropped.
		for _, s := range tracker.DrainSamples() {
			row.nSamples++
			if !s.Valid {
				continue
			}
			row.nValid++
			gx, gy := geom.ToCentre(s.X, s.Y)
			sx += gx
			sy += gy
			pupil += s.PupilArea
			row.trackerMs = s.TrackerMs
		}
		if row.nValid > 0 {
			row.gazeX = sx / float64(row.nValid)
			row.gazeY = sy / float64(row.nValid)
			row.pupilMM = pupil / float64(row.nValid)
			row.errPx = math.Hypot(row.gazeX-tx, row.gazeY-ty)
		}
		rows = append(rows, row)

		exp.Data.Add(row.trial, row.flipNS,
			fmt.Sprintf("%.1f", row.targetX), fmt.Sprintf("%.1f", row.targetY),
			fmt.Sprintf("%.1f", row.gazeX), fmt.Sprintf("%.1f", row.gazeY),
			fmt.Sprintf("%.1f", row.errPx), fmt.Sprintf("%.2f", row.pupilMM),
			row.nSamples, row.nValid, fmt.Sprintf("%.0f", row.trackerMs),
			tracker.Dropped())

		if err := exp.Blank(int(cfg.isi.Milliseconds())); err != nil {
			return rows, err
		}
	}
	return rows, nil
}

// showLiveGaze draws a dot wherever the tracker says the eye is, until a key is
// pressed. It is the fastest way to see that a calibration is sane.
func showLiveGaze(exp *control.Experiment, tracker *eyetracker.Bridge,
	geom eyetracker.Geometry) error {

	fmt.Println("live gaze — press any key to continue")
	dot := stimuli.NewCircle(12, control.Red)
	cross := stimuli.NewFixCross(40, 3, control.Gray)

	for {
		state := exp.PollEvents(nil)
		if state.QuitRequested || state.LastKey != 0 {
			return nil
		}
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := cross.Draw(exp.Screen); err != nil {
			return err
		}
		if s, ok := tracker.Latest(); ok && s.Valid {
			// Tracker pixels are top-left origin with +Y down; the screen is
			// centre origin with +Y UP. Skipping this flip mirrors the display.
			x, y := geom.ToCentre(s.X, s.Y)
			dot.SetPosition(sdl.FPoint{X: float32(x), Y: float32(y)})
			if err := dot.Draw(exp.Screen); err != nil {
				return err
			}
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}
	}
}

// aborted reports whether the operator pressed a key to stop the run.
func aborted(exp *control.Experiment) bool {
	state := exp.PollEvents(nil)
	return state.QuitRequested || state.LastKey == control.K_ESCAPE
}

func reportOffset(when string, o eyetracker.Offset) {
	if o.Samples == 0 {
		fmt.Printf("clock offset %s: no round trip succeeded\n", when)
		return
	}
	// The uncertainty is half the best round trip and belongs next to the
	// number, not in a footnote: an offset quoted alone is not a measurement.
	fmt.Printf("clock offset %s: tracker − local = %.3f ms ± %.3f (best RTT %v, worst %v, n=%d)\n",
		when, o.DeltaMs, float64(o.BestRTT.Microseconds())/2000, o.BestRTT, o.WorstRTT, o.Samples)
}

func summarise(rows []trialRow, tracker *eyetracker.Bridge) {
	if len(rows) == 0 {
		fmt.Println("\nno trials ran")
		return
	}
	var errs []float64
	var samples []int
	var totalValid, totalSamples int
	for _, r := range rows {
		if r.nValid > 0 {
			errs = append(errs, r.errPx)
		}
		samples = append(samples, r.nSamples)
		totalValid += r.nValid
		totalSamples += r.nSamples
	}
	sort.Float64s(errs)
	sort.Ints(samples)

	fmt.Printf("\n%d trials, %d sample events, %d valid (%.1f%%)\n",
		len(rows), totalSamples, totalValid,
		100*float64(totalValid)/math.Max(1, float64(totalSamples)))
	fmt.Printf("samples per trial: median %d\n", medianInt(samples))
	if len(errs) > 0 {
		fmt.Printf("gaze-to-target distance: median %.1f px (n=%d trials with valid gaze)\n",
			median(errs), len(errs))
		fmt.Println("  (this is accuracy only if the participant actually fixated each")
		fmt.Println("   target; it says nothing on a simulated run or an empty chair.)")
	} else {
		fmt.Println("no trial had a single valid gaze sample")
	}
	if d := tracker.Dropped(); d > 0 {
		fmt.Printf("*** %d samples dropped over the run: the record has holes ***\n", d)
	}
}

func medianInt(sorted []int) int {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[len(sorted)/2]
}

func median(sorted []float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[len(sorted)/2]
}
