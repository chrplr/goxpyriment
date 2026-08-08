// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// test_photodiode_latency — measure the delay between the timestamp ShowTS
// returns and the moment light actually leaves the screen.
//
// Each trial blanks the screen, presents a white patch with ShowTS, and
// immediately raises a TTL line on a DLP-IO8. An external two-channel
// instrument — an Analog Discovery 3, or a Black Box ToolKit — records the TTL
// edge and a photodiode on the patch. The interval between them is measured on
// the instrument's own timebase, so the instrument's clock offset cancels.
//
// # What the instrument measures is not what you want, quite
//
// The quantity of interest is
//
//	delta = T_light - T_ShowTS
//
// and the instrument gives M = T_light - T_TTL. Those differ:
//
//	delta = M + (T_write - T_ShowTS) + L_dlp
//
// The middle term is this program's own software gap between ShowTS returning
// and the TTL write being issued. It is on the host clock, so it is measured
// and logged per trial as gap_us — add it back rather than assuming it is
// small. It is small only if the write is issued synchronously on the flip
// thread, which is why this program does NOT fire the trigger from a goroutine
// the way triggers.FireTrigger does; at normal priority under CPU load that
// path has been measured at +0.73 ms with about 1 ms of spread.
//
// L_dlp is the DLP's own host-write-to-wire latency and cannot be measured
// here: nothing on the bench shares a clock with the device, and the module has
// no clock of its own. It is bounded at 0.793 ms by arithmetic alone, and at a
// few tens of microseconds by extrapolating the FTDI latency-timer sweep — see
// "Why no absolute latency is quoted here" in
// https://github.com/chrplr/dlp-io8-g. Both L_dlp and gap are non-negative, so
// the instrument's figure is a LOWER bound on delta.
//
// # The photodiode's position is a systematic worth more than everything else
//
// ShowTS returns a host-side timestamp taken after the flip; the photons for a
// given screen row appear when the scanout reaches that row. A diode at the top
// of the panel sees light almost immediately, one at the bottom nearly a frame
// later — 16.7 ms at 60 Hz, which is sixteen times the millisecond this whole
// exercise is trying to resolve. So the patch is drawn AT the diode, its
// position is a flag, and the position is written into every row of the data.
// Comparing two runs that placed the diode differently is meaningless.
//
// Pixel response is the other one: an LCD takes milliseconds to go from black
// to white, and where the instrument's threshold falls on that curve sets the
// answer. An Analog Discovery records the whole waveform, so the threshold
// becomes a choice made in the analysis; a comparator-based instrument makes it
// for you, once, invisibly.
//
// # Zero point
//
// -calibrate emits TTL pulses and draws nothing. Point the photodiode at an LED
// driven from the same TTL line: the interval the instrument then reports is its
// own TTL-input-versus-optical-input asymmetry plus the LED's rise, and an LED
// rises in microseconds. Subtract it from the real runs. This matters most for
// an instrument whose two inputs are different circuits; on an Analog Discovery
// both channels are the same ADC and the asymmetry should be negligible, which
// this mode lets you confirm rather than assume.
//
// Usage:
//
//	go run ./tests/test_photodiode_latency                      # fullscreen, diode at centre
//	go run ./tests/test_photodiode_latency -diode top -trials 200
//	go run ./tests/test_photodiode_latency -calibrate           # LED zero point
//
// Controls:
//
//	ESC / Q — quit early
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
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/sysinfo"
	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	// Flags must be declared before NewExperimentFromFlags, which parses.
	fTrials := flag.Int("trials", 100, "number of trials")
	fLine := flag.Int("line", 0, "TTL output line to pulse (0-7)")
	fPort := flag.String("port", "", "serial port of the DLP-IO8 (default: auto-detect)")
	fDiode := flag.String("diode", "center",
		`photodiode position: "top", "center", "bottom", or "x,y" in pixels from the centre`)
	fPatch := flag.Int("patch", 240, "side of the white patch, in pixels")
	fFrames := flag.Int("frames", 2, "frames the patch stays on")
	fISI := flag.Duration("isi", 500*time.Millisecond, "blank interval between trials")
	fCal := flag.Bool("calibrate", false, "emit TTL pulses only, for the LED zero point")

	exp := control.NewExperimentFromFlags("Photodiode Latency Test", control.Black, control.White, 32)
	defer exp.End()

	sched := sysinfo.Scheduling()
	fmt.Printf("scheduling: %s priority %d", sched.Policy, sched.Priority)
	if sched.RealTime {
		fmt.Println("  REAL-TIME")
	} else {
		fmt.Println("  (not real-time: the gap_us column will be larger and more variable;\n" +
			"             see docs/SettingPriorityUnderLinux.md)")
	}

	trig, port := setupTrigger(*fPort)
	defer trig.Close()
	if *fLine < 0 || *fLine > 7 {
		log.Fatalf("-line %d out of range (0-7)", *fLine)
	}
	if err := trig.AllLow(); err != nil {
		log.Fatalf("driving lines low: %v", err)
	}

	if *fCal {
		calibrate(exp, trig, *fLine, *fTrials, *fISI)
		return
	}

	w, h, err := exp.Screen.Size()
	if err != nil {
		log.Fatalf("screen size: %v", err)
	}
	px, py, err := diodePosition(*fDiode, w, h, int32(*fPatch))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("screen %dx%d, patch %d px centred at (%+.0f, %+.0f) from screen centre\n",
		w, h, *fPatch, px, py)
	fmt.Printf("trigger: line %d on %s\n", *fLine, port)
	fmt.Println("\nPut the photodiode ON the patch. The instrument measures the interval")
	fmt.Println("from the TTL rising edge to the light rising edge; add gap_us back to it.")

	patch := stimuli.NewRectangle(px, py, float32(*fPatch), float32(*fPatch), control.White)

	exp.AddDataVariableNames([]string{
		"trial", "flip_ts_ns", "trigger_ts_ns", "gap_us",
		"patch_x", "patch_y", "patch_px", "frames",
		"policy", "priority", "isi_ms", "onset_interval_ms",
	})

	fmt.Printf("\n%-6s  %-12s  %-10s  %-16s\n", "trial", "flip_ts_ns", "gap_us", "onset_interval_ms")

	var gaps []float64
	var prevFlip uint64
	trial := 0
	runErr := exp.Run(func() error {
		if trial >= *fTrials {
			return control.EndLoop
		}

		// Blank for the ISI, so the diode sees a clean black-to-white step.
		if err := exp.Screen.ClearAndUpdate(); err != nil {
			return err
		}
		time.Sleep(*fISI)

		// The measurement. ShowTS presents and timestamps the flip; the TTL goes
		// up on the very next statement, on this thread, with nothing between.
		flipTS, err := exp.ShowTS(patch)
		if err != nil {
			return err
		}
		if err := trig.SetHigh(*fLine); err != nil {
			return err
		}
		trigTS := sdl.TicksNS()

		// Hold the patch, then drop both together.
		for f := 1; f < *fFrames; f++ {
			if err := patch.Draw(exp.Screen); err != nil {
				return err
			}
			if err := exp.Screen.Flip(); err != nil {
				return err
			}
		}
		if err := trig.SetLow(*fLine); err != nil {
			return err
		}

		// trigTS and flipTS are both sdl.TicksNS, so this subtraction is on one
		// clock. It is the term to add back to the instrument's interval.
		gapUS := float64(trigTS-flipTS) / 1000
		gaps = append(gaps, gapUS)

		// Onset to onset, not a frame interval: it spans the ISI and the whole
		// trial. Logged because a trial that took much longer than the rest is
		// one whose timing was disturbed, and that is worth seeing in the data.
		onsetMS := 0.0
		if prevFlip != 0 {
			onsetMS = float64(flipTS-prevFlip) / 1e6
		}
		prevFlip = flipTS

		fmt.Printf("%-6d  %-12d  %-10.1f  %-16.1f\n", trial, flipTS, gapUS, onsetMS)
		exp.Data.Add(trial, flipTS, trigTS, fmt.Sprintf("%.1f", gapUS),
			fmt.Sprintf("%.0f", px), fmt.Sprintf("%.0f", py), *fPatch, *fFrames,
			sched.Policy, sched.Priority,
			fmt.Sprintf("%.0f", float64(fISI.Milliseconds())),
			fmt.Sprintf("%.3f", onsetMS))

		if exp.PollEvents(nil).QuitRequested {
			return control.EndLoop
		}
		trial++
		return nil
	})
	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("experiment error: %v", runErr)
	}

	_ = exp.Screen.ClearAndUpdate()
	_ = trig.AllLow()
	summarise(gaps, trial)
}

// setupTrigger opens the DLP-IO8, refusing to run without one: a run whose
// trigger silently did nothing looks exactly like a run whose instrument was
// mis-wired, and the whole measurement is the trigger.
func setupTrigger(port string) (triggers.OutputTTLDevice, string) {
	if port != "" {
		d, err := triggers.NewDLPIO8(port)
		if err != nil {
			log.Fatalf("opening the DLP-IO8 on %s: %v", port, err)
		}
		return d, port
	}
	d, name, err := triggers.AutoDetectDLPIO8()
	if err != nil {
		log.Fatalf("looking for a DLP-IO8: %v", err)
	}
	if _, isNull := d.(triggers.NullOutputTTLDevice); isNull {
		log.Fatal("no DLP-IO8 found. This test measures the interval between a TTL " +
			"edge and the screen, so it cannot run without one; pass -port to name it.")
	}
	return d, name
}

// calibrate emits TTL pulses and draws nothing, for the LED zero point.
func calibrate(exp *control.Experiment, trig triggers.OutputTTLDevice, line, n int, isi time.Duration) {
	fmt.Println("\nCALIBRATION: no visual stimulus.")
	fmt.Println("Wire an LED (with its resistor) to the TTL line and point the photodiode")
	fmt.Println("at the LED. The interval the instrument reports is its own TTL-versus-optical")
	fmt.Println("input asymmetry plus the LED's rise time, which is microseconds. Subtract")
	fmt.Println("that offset from the real runs.")
	fmt.Printf("\nemitting %d pulses on line %d, one every %v\n\n", n, line, isi)

	if err := exp.Screen.ClearAndUpdate(); err != nil {
		log.Fatalf("clearing the screen: %v", err)
	}
	for i := 0; i < n; i++ {
		if err := trig.SetHigh(line); err != nil {
			log.Fatalf("pulse %d: %v", i, err)
		}
		time.Sleep(20 * time.Millisecond)
		if err := trig.SetLow(line); err != nil {
			log.Fatalf("pulse %d: %v", i, err)
		}
		if (i+1)%10 == 0 {
			fmt.Printf("\r  %d/%d", i+1, n)
		}
		time.Sleep(isi)
	}
	fmt.Printf("\r  %d pulses sent\n", n)
}

// diodePosition returns the patch centre in goxpyriment's centre-based
// coordinates. The named positions inset the patch by half its side so it sits
// fully on the panel.
func diodePosition(spec string, w, h, patch int32) (float32, float32, error) {
	inset := float32(patch)/2 + 8
	switch strings.ToLower(strings.TrimSpace(spec)) {
	case "center", "centre":
		return 0, 0, nil
	case "top":
		return 0, -float32(h)/2 + inset, nil
	case "bottom":
		return 0, float32(h)/2 - inset, nil
	}
	var x, y float32
	if _, err := fmt.Sscanf(spec, "%f,%f", &x, &y); err != nil {
		return 0, 0, fmt.Errorf(`-diode %q: want "top", "center", "bottom" or "x,y"`, spec)
	}
	return x, y, nil
}

func summarise(gaps []float64, trials int) {
	fmt.Printf("\n%d trials.\n", trials)
	if len(gaps) == 0 {
		return
	}
	s := append([]float64(nil), gaps...)
	sort.Float64s(s)
	q := func(p float64) float64 { return s[min(int(p*float64(len(s))), len(s)-1)] }
	fmt.Printf("ShowTS -> TTL write gap: n=%d  p50 %.1f  p95 %.1f  max %.1f us\n",
		len(s), q(.5), q(.95), s[len(s)-1])
	fmt.Println()
	fmt.Println("To get the delay from the ShowTS timestamp to light on the panel:")
	fmt.Println("  delta = (instrument's TTL-to-light interval) + gap_us + L_dlp")
	fmt.Println("where L_dlp is the DLP's own write-to-edge latency: tens of microseconds")
	fmt.Println("by extrapolation, 0.793 ms as a hard bound. Both terms are positive, so")
	fmt.Println("the instrument's figure alone is a LOWER bound on delta.")
	if s[len(s)-1] > 1000 {
		fmt.Fprintf(os.Stderr,
			"\nWARNING: the gap reached %.0f us, which is a millisecond or more of the\n"+
				"budget this test exists to resolve. Run at real-time priority and with\n"+
				"nothing else on the machine, or treat these trials as unusable.\n",
			s[len(s)-1])
	}
}
