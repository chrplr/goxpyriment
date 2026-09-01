// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_eyelink checks an EyeLink reached through eyelink_bridge.py, and
// measures what the bridge costs.
//
// It answers three questions, in order of how much they matter:
//
//  1. Do TTL pulses from this machine land in the EDF? Every trial pulses a
//     TTL line whose rising edge is issued on the flip thread, immediately
//     after the flip. The Host PC records it as an INPUT event, timestamped by
//     the Host itself. This is the path an experiment should use to mark a
//     stimulus onset.
//
//  2. What does the bridge add? Every trial ALSO sends a message through the
//     bridge, right after the TTL. In the EDF, the MSG lands later than the
//     INPUT by however long the socket, the Python process and the link took —
//     measured on the tracker's own clock, which is the only clock that can
//     compare them. That difference is the number that says whether a message
//     can ever be used for timing. (Expect it to say no.)
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
//	# in another:
//	go run ./tests/test_eyelink -w -s 999 -trigger parport -device /dev/parport0
//
//	# no hardware at all — bridge started with --simulate:
//	go run ./tests/test_eyelink -w -s 999
//
// Wiring for (1): a TTL output line from this machine to the EyeLink Host PC's
// parallel port input. The Host records the edge as an INPUT event only if the
// EDF is configured to keep them, which the bridge asks for at open
// (file_event_filter includes INPUT).
//
// Analysis: convert the EDF with edf2asc and compare, per trial, the INPUT line
// against the following MSG line. The difference is the bridge's added latency
// on the Host clock. The CSV this program writes holds the same trials measured
// from this side.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/eyetracker"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/tests/internal/safeexit"
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
	fTrigger := flag.String("trigger", "none", `TTL device: "none", "parport", "dlpio8", "dlpio20" or "megttl"`)
	fDevice := flag.String("device", "", "device path for -trigger (empty: auto-detect where supported)")
	fLine := flag.Int("line", 0, "TTL output line to pulse (0-7)")
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
		trigger:     *fTrigger,
		device:      *fDevice,
		line:        *fLine,
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
	trigger     string
	device      string
	line        int
	pulse       time.Duration
	syncN       int
}

func run(exp *control.Experiment, cfg config) error {
	w, h, err := exp.Screen.Size()
	if err != nil {
		return fmt.Errorf("reading the screen size: %w", err)
	}
	geom := eyetracker.Geometry{WidthPx: int(w), HeightPx: int(h)}

	trig, trigName, err := openTrigger(cfg.trigger, cfg.device)
	if err != nil {
		return fmt.Errorf("opening the TTL device: %w", err)
	}
	defer trig.Close()

	// Ctrl-C must not leave a line HIGH into the Host PC's parallel port, and
	// must not stop working while we try to prevent that.
	safeexit.OnSignal(2*time.Second, func() {
		trig.AllLow()
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
	exp.AddExperimentInfo(fmt.Sprintf("TTL device: %s line %d pulse %v", trigName, cfg.line, cfg.pulse))
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
		fmt.Println("calibrating — the tracker owns the screen until the operator is done")
		if err := tracker.Calibrate(eyetracker.CalibrationOptions{Points: cfg.points}); err != nil {
			// Calibration graphics are the known gap in this bridge. Saying so
			// and carrying on is more useful than dying: everything the test
			// actually measures works uncalibrated.
			fmt.Printf("calibration unavailable: %v\ncontinuing without it — "+
				"gaze positions will be wrong, timing will not\n", err)
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
		fmt.Printf("EDF written to %s — convert it with edf2asc and compare each "+
			"INPUT line against the MSG that follows it\n", cfg.fetch)
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

func runTrials(exp *control.Experiment, tracker *eyetracker.Bridge, trig triggers.OutputTTLDevice, cfg config) ([]trialRow, error) {
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
		triggers.FireTriggerSync(trig, cfg.line, cfg.pulse)
		afterTTL := control.TicksNS()

		// And now the same event down the slow path, for comparison. In the
		// EDF this MSG lands after the INPUT above by the bridge's latency.
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
	fmt.Println("\nThe TTL figure is measured on this machine and says when the edge was")
	fmt.Println("issued, not when the Host recorded it. The number that matters is in the")
	fmt.Println("EDF: the gap between each INPUT event and the MSG that follows it.")
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

// openTrigger opens the requested TTL output device, returning a no-op device
// when none was asked for so the trial loop needs no special case.
func openTrigger(kind, device string) (triggers.OutputTTLDevice, string, error) {
	switch kind {
	case "", "none":
		return triggers.NullOutputTTLDevice{}, "none (no TTL will reach the Host PC)", nil

	case "parport":
		if device == "" {
			ports := triggers.AvailableParallelPorts()
			if len(ports) == 0 {
				return nil, "", fmt.Errorf("no parallel port found; pass -device")
			}
			device = ports[0]
		}
		pp := triggers.NewParallelPort(device)
		if err := pp.Open(); err != nil {
			return nil, "", fmt.Errorf("opening %s: %w", device, err)
		}
		return pp, "parallel port " + device, nil

	case "dlpio8":
		if device == "" {
			d, port, err := triggers.AutoDetectDLPIO8()
			if err != nil {
				return nil, "", err
			}
			return d, "DLP-IO8 on " + port, nil
		}
		d, err := triggers.NewDLPIO8(device)
		if err != nil {
			return nil, "", err
		}
		return d, "DLP-IO8 on " + device, nil

	case "megttl":
		if device == "" {
			device = "/dev/ttyACM0"
		}
		d, err := triggers.NewMEGTTLBox(device)
		if err != nil {
			return nil, "", err
		}
		return d, "MEG TTL box on " + device, nil

	case "dlpio20":
		if device == "" {
			d, port, err := triggers.AutoDetectDLPIO20()
			if err != nil {
				return nil, "", err
			}
			return d, "DLP-IO20 on " + port, nil
		}
		d, err := triggers.NewDLPIO20(device)
		if err != nil {
			return nil, "", err
		}
		return d, "DLP-IO20 on " + device, nil
	}
	return nil, "", fmt.Errorf("unknown -trigger %q (none, parport, dlpio8, dlpio20, megttl)", kind)
}
