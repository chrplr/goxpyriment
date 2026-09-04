// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_eyelink checks an EyeLink reached through eyelink_bridge.py, and
// measures what the bridge costs.
//
// It answers three questions, in order of how much they matter:
//
//  1. What does the bridge add? Every trial sends a message through the bridge
//     right after the onset flip, and the full round trip — socket, Python
//     process, link, reply — is timed on this machine. It is an upper bound on
//     the one-way latency, and it is the number that says whether a message can
//     ever be used for timing. (Expect it to say no: use a TTL for onsets and
//     keep MSG for labels.)
//
//  2. Does the TTL leave this machine on time? Every trial also pulses a TTL
//     line whose rising edge is issued on the flip thread, immediately after
//     the flip; the gap between the flip timestamp and the return of the raise
//     is recorded. That is the path an experiment should use to mark a stimulus
//     onset. Where the pulse LANDS depends on the wiring, and on the MEG rig it
//     is not the EDF — see below.
//
//  3. Does the sample stream work, and how do the two clocks relate? The
//     tracker's clock and this machine's clock are sampled before and after
//     the run; the change between them is the drift.
//
// Usage:
//
//	# on the display machine, in one terminal:
//	python3 eyetracker/bridge/eyelink_bridge.py --tracker-host 100.1.1.1
//
//	# in another (the TTL device is named by one -device spec; -h prints the
//	# syntax, and pin= is 1-8 as printed on the hardware):
//	go run ./tests/test_eyelink -w -s 999 -device parallel:port=/dev/parport0,pin=1
//	go run ./tests/test_eyelink -w -s 999 -device megttlbox:port=/dev/ttyACM0,pin=1
//
//	# no hardware at all — bridge started with --simulate:
//	go run ./tests/test_eyelink -w -s 999
//
// Wiring. On the MEG rig the TTL goes to the MEG acquisition's STI channel, and
// the Host PC sends gaze to the same acquisition's MISC channels as analog X, Y
// and pupil; the two therefore meet on the MEG's clock, not in the EDF. Nothing
// is connected to the Host's parallel-port INPUT, so the EDF carries the MSG
// marks but no TTL — an INPUT line in it, if any appears, is the unconnected
// port idling at 127.
//
// That leaves every latency figure here measured on this side, which is why
// (1) times a round trip rather than the one-way gap. Wiring a TTL line to the
// Host's DB25 would give the better measurement without changing this program:
// each INPUT and the MSG that follows it, on the tracker's own clock, which is
// the only clock that can compare them. The EDF is asked to keep INPUT events
// at open already (file_event_filter includes INPUT).
//
// Analysis: the CSV this program writes holds the per-trial timings. Convert
// the EDF with edf2asc to check that every trial's MSG is there, and look at
// the MEG STI channel for the pulses.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/eyetracker"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/tests/internal/safeexit"
	"github.com/chrplr/goxpyriment/tests/internal/trigdev"
	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	// Flags must be declared before NewExperimentFromFlags, which parses.
	fBridge := flag.String("bridge", eyetracker.DefaultBridgeAddr, "address of eyelink_bridge.py")
	fTrackerHost := flag.String("tracker-host", "", "EyeLink Host PC address (empty: the bridge's own default)")
	fEDF := flag.String("edf", "goxtest.edf", "name of the data file opened on the Host PC (8 characters max)")
	fFetch := flag.String("fetch", "", "after the run, copy the EDF here (empty: leave it on the Host)")
	fTrials := flag.Int("trials", 20, "number of trials")
	fISI := flag.Duration("isi", 700*time.Millisecond, "blank interval between trials")
	fFrames := flag.Int("frames", 2, "frames the patch stays on")
	fCalibrate := flag.Bool("calibrate", false, "run the tracker's calibration before recording")
	fPoints := flag.Int("points", 9, "calibration points (with -calibrate)")
	fGaze := flag.Bool("gaze", false, "before the trials, show a live gaze dot until a key is pressed")
	fDevice := flag.String("device", "null", "TTL output device pulsed at each flip.\n"+trimBlank(trigdev.Usage))
	fPulse := flag.Duration("pulse", 5*time.Millisecond, "TTL pulse width")
	fSync := flag.Int("sync", 20, "clock round trips per synchronisation")

	exp := control.NewExperimentFromFlags("EyeLink Bridge Test", control.Black, control.White, 32)
	defer exp.End()

	cfg := config{
		bridgeAddr:  *fBridge,
		trackerHost: *fTrackerHost,
		edf:         *fEDF,
		fetch:       *fFetch,
		trials:      *fTrials,
		isi:         *fISI,
		frames:      *fFrames,
		calibrate:   *fCalibrate,
		points:      *fPoints,
		gaze:        *fGaze,
		device:      *fDevice,
		pulse:       *fPulse,
		syncN:       *fSync,
	}

	if err := run(exp, cfg); err != nil {
		log.Printf("test_eyelink: %v", err)
		os.Exit(1)
	}
}

type config struct {
	bridgeAddr  string
	trackerHost string
	edf         string
	fetch       string
	trials      int
	isi         time.Duration
	frames      int
	calibrate   bool
	points      int
	gaze        bool
	device      string // one trigdev spec, e.g. "parallel:port=/dev/parport0,pin=1"
	pulse       time.Duration
	syncN       int
}

func run(exp *control.Experiment, cfg config) error {
	w, h, err := exp.Screen.Size()
	if err != nil {
		return fmt.Errorf("reading the screen size: %w", err)
	}
	geom := eyetracker.Geometry{WidthPx: int(w), HeightPx: int(h)}

	spec, err := trigdev.ParseSpec(cfg.device)
	if err != nil {
		return err
	}
	trig, err := trigdev.Open(spec)
	if err != nil {
		return fmt.Errorf("opening the TTL device: %w", err)
	}
	defer trig.Close()

	// The notes say where to clip the probe, at what logic level, and — when
	// the spec left port= out on a machine with more than one parallel port —
	// which port was chosen and what the alternatives were. Printing them is
	// the only way the operator learns that a choice was made at all.
	fmt.Printf("TTL device: %s\n", trig.Desc)
	for _, n := range trig.Notes {
		fmt.Printf("  - %s\n", n)
	}
	if trig.IsNull() {
		fmt.Println("  (no TTL will reach the MEG or the Host PC)")
	}

	// Ctrl-C must not leave a line HIGH into the MEG's STI channel, and must
	// not stop working while we try to prevent that.
	safeexit.OnSignal(2*time.Second, func() {
		trig.Device.AllLow()
		trig.Close()
	})

	// The clock matters: samples are stamped with whatever this returns, and
	// comparing them against Screen.FlipTS only means anything if both are on
	// SDL's clock. The default is the Go monotonic clock, which is NOT.
	tracker := eyetracker.NewBridge(cfg.bridgeAddr,
		eyetracker.WithGeometry(geom),
		eyetracker.WithHost(cfg.trackerHost),
		eyetracker.WithEDF(cfg.edf),
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
	exp.AddExperimentInfo(fmt.Sprintf("TTL device: %s pulse %v spec=%q", trig.Desc, cfg.pulse, spec.Raw))
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
		fmt.Println("calibrating — the tracker owns the screen until the operator is done.")
		fmt.Println("  C calibrate, V validate, Enter accept a target, Esc leave setup.")
		fmt.Println("  Targets advance by themselves once the eye holds still, so an")
		fmt.Println("  empty chair will sit on target 1 for ever: seat the participant first.")
		err := tracker.Calibrate(eyetracker.CalibrationOptions{Points: cfg.points})
		// pylink's window closed with that call, and closing it hands focus to
		// whatever the window manager finds next — the terminal this was
		// launched from. Ours has to be raised again or the trials run behind
		// it, visible only as lines scrolling past in that terminal.
		exp.ReclaimDisplay()
		if err != nil {
			// Carrying on is deliberate: every figure this test reports is
			// independent of where the eye is pointing, so an uncalibrated run
			// still measures what it exists to measure.
			fmt.Printf("calibration failed: %v\ncontinuing without it — "+
				"gaze positions will be wrong, timing will not\n", err)
		} else if msg, ok := tracker.CalibrationMessage(); ok {
			fmt.Printf("calibration stored: %s\n", msg)
		}
	}

	if err := tracker.StartRecording(); err != nil {
		return fmt.Errorf("starting the recording: %w", err)
	}

	if cfg.gaze {
		if err := showLiveGaze(exp, tracker, geom); err != nil {
			return err
		}
	}

	rows, err := runTrials(exp, tracker, trig, cfg)
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
		exp.AddExperimentInfo(fmt.Sprintf("clock drift: %+.3f ms over %.1f s (%+.2f ppm)",
			driftMs, elapsedS, driftMs/1000/elapsedS*1e6))
	}

	summarise(rows, tracker)

	if cfg.fetch != "" {
		fmt.Printf("fetching the EDF to %s (this can take a while) ...\n", cfg.fetch)
		if err := tracker.ReceiveDataFile(cfg.fetch); err != nil {
			return fmt.Errorf("fetching the EDF: %w", err)
		}
		fmt.Printf("EDF written to %s — convert it with edf2asc and check that "+
			"every trial's MSG is there\n", cfg.fetch)
	}
	return nil
}

// trialRow is one trial as measured from this machine.
type trialRow struct {
	trial     int
	flipNS    uint64
	ttlGapUS  float64 // flip → TTL raise returned
	markRTTUS float64 // the full bridge round trip for one message
	trackerMs float64
	gazeX     float64
	gazeY     float64
	gazeValid bool
	nSamples  int // samples drained during this trial: the link's own pulse
}

func runTrials(exp *control.Experiment, tracker *eyetracker.Bridge, trig trigdev.Opened, cfg config) ([]trialRow, error) {
	exp.AddDataVariableNames([]string{
		"trial", "flip_ns", "ttl_gap_us", "mark_rtt_us",
		"tracker_ms", "gaze_x", "gaze_y", "gaze_valid", "n_samples", "dropped",
	})

	patch := stimuli.NewRectangle(0, 0, 240, 240, control.White)
	if err := stimuli.PreloadVisualOnScreen(exp.Screen, patch); err != nil {
		return nil, fmt.Errorf("preloading the patch: %w", err)
	}

	rows := make([]trialRow, 0, cfg.trials)
	for i := 1; i <= cfg.trials; i++ {
		if err := exp.Blank(int(cfg.isi.Milliseconds())); err != nil {
			return rows, err
		}
		if aborted(exp) {
			fmt.Println("\naborted at the keyboard")
			break
		}

		// The ONSET flip, and nothing else. ShowFrames(n) would hold the patch
		// for n frames and return the first flip's timestamp, which puts n-1
		// frames between the onset and the raise below — 16.7 ms of it at
		// 60 Hz, measured, not guessed. The remaining frames are held after
		// the trigger is out.
		flipNS, err := exp.ShowTS(patch)
		if err != nil {
			return rows, fmt.Errorf("trial %d: showing the patch: %w", i, err)
		}
		// The rising edge is the marker, so it is raised here, on this thread,
		// with nothing between it and the flip. Only the falling edge is
		// deferred. See triggers.FireTriggerSync.
		triggers.FireTriggerSync(trig.Device, trig.Line, cfg.pulse)
		afterTTL := control.TicksNS()

		// And now the same event down the slow path, for comparison: the round
		// trip timed here is what a MSG would add to the TTL above.
		markStart := control.TicksNS()
		if err := tracker.Mark(fmt.Sprintf("TRIAL %d", i)); err != nil {
			return rows, fmt.Errorf("trial %d: marking: %w", i, err)
		}
		markEnd := control.TicksNS()

		// Hold the patch for the frames the onset flip did not cover. Doing it
		// after the marker costs the marker nothing and keeps the patch on
		// screen long enough for a photodiode to see it.
		if cfg.frames > 1 {
			if _, err := exp.ShowFrames(patch, cfg.frames-1); err != nil {
				return rows, fmt.Errorf("trial %d: holding the patch: %w", i, err)
			}
		}

		row := trialRow{
			trial:     i,
			flipNS:    flipNS,
			ttlGapUS:  float64(afterTTL-flipNS) / 1000,
			markRTTUS: float64(markEnd-markStart) / 1000,
		}
		// Drain the sample buffer every trial. Nothing here needs the samples
		// themselves — the EDF on the Host is the authoritative record — but
		// the buffer is a fixed-size ring, so leaving it to fill would report
		// a huge "dropped" count that measures only this loop's failure to
		// empty it. Drained per trial, a non-zero count means something real.
		// Latest() is kept for the gaze: it tracks the newest sample
		// independently of the ring, so it is correct either way.
		row.nSamples = len(tracker.DrainSamples())
		if s, ok := tracker.Latest(); ok {
			row.trackerMs = s.TrackerMs
			row.gazeX, row.gazeY, row.gazeValid = s.X, s.Y, s.Valid
		}
		rows = append(rows, row)

		exp.Data.Add(row.trial, row.flipNS,
			fmt.Sprintf("%.1f", row.ttlGapUS),
			fmt.Sprintf("%.1f", row.markRTTUS),
			fmt.Sprintf("%.1f", row.trackerMs),
			fmt.Sprintf("%.1f", row.gazeX),
			fmt.Sprintf("%.1f", row.gazeY),
			row.nSamples, tracker.Dropped())

		fmt.Printf("trial %3d  ttl %+7.1f µs  mark %8.1f µs  gaze (%7.1f, %7.1f) %v\n",
			row.trial, row.ttlGapUS, row.markRTTUS, row.gazeX, row.gazeY, row.gazeValid)
	}
	return rows, nil
}

// showLiveGaze draws a dot wherever the tracker says the eye is, until a key is
// pressed. It is the fastest way to see whether the stream is alive and whether
// the calibration is sane.
func showLiveGaze(exp *control.Experiment, tracker *eyetracker.Bridge, geom eyetracker.Geometry) error {
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
		fmt.Println("\nno trials completed")
		return
	}
	ttl := make([]float64, len(rows))
	mark := make([]float64, len(rows))
	valid := 0
	for i, r := range rows {
		ttl[i] = r.ttlGapUS
		mark[i] = r.markRTTUS
		if r.gazeValid {
			valid++
		}
	}
	sort.Float64s(ttl)
	sort.Float64s(mark)

	counts := make([]int, 0, len(rows))
	totalSamples := 0
	for _, r := range rows {
		counts = append(counts, r.nSamples)
		totalSamples += r.nSamples
	}
	sort.Ints(counts)

	fmt.Printf("\n%d trials\n", len(rows))
	fmt.Printf("  flip → TTL raise : median %.1f µs, max %.1f µs\n", median(ttl), ttl[len(ttl)-1])
	fmt.Printf("  bridge round trip: median %.1f µs, max %.1f µs\n", median(mark), mark[len(mark)-1])
	fmt.Printf("  samples with valid gaze: %d/%d\n", valid, len(rows))
	fmt.Printf("  samples received: %d (median %d per trial)\n", totalSamples, medianInt(counts))
	if d := tracker.Dropped(); d > 0 {
		fmt.Printf("  WARNING: %d samples overflowed THIS PROGRAM's buffer between\n"+
			"    drains — the EDF on the Host PC is complete and unaffected.\n"+
			"    Expect this only if a trial outran the buffer; check the link.\n", d)
	}
	fmt.Println("\nBoth figures are measured on this machine. The TTL one says when the edge")
	fmt.Println("was issued, not when anything downstream recorded it — for that, look at the")
	fmt.Println("MEG STI channel, or at a scope. The bridge round trip is an upper bound on")
	fmt.Println("the one-way latency of a MSG: mark stimulus onsets with a TTL, not a message.")
}

func medianInt(sorted []int) int {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[len(sorted)/2]
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// trimBlank drops trailing whitespace from a multi-line usage block. The flag
// package re-indents every line itself, so nothing has to be added here — but a
// line that is blank must stay blank rather than become a run of spaces.
func trimBlank(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}
