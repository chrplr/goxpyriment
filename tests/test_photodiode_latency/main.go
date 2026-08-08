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
// -diode defaults to topleft for that reason: scanout starts at the top-left
// corner, so a diode there adds the least of this term. The named positions
// cover all four corners, the four edge midpoints and the centre; anything else
// is "x,y" in pixels from the screen centre.
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
//	go run ./tests/test_photodiode_latency                    # fullscreen, diode at top-left
//	go run ./tests/test_photodiode_latency -diode center -trials 200
//	go run ./tests/test_photodiode_latency -isi-frames 15     # steady-state flipping
//	go run ./tests/test_photodiode_latency -calibrate         # LED zero point
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
	"github.com/chrplr/goxpyriment/apparatus"
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
	fDiode := flag.String("diode", "all",
		`where to draw patches: "all" (4 corners + centre), "corners", one or more `+
			`of topleft/top/topright/left/center/right/bottomleft/bottom/bottomright `+
			`joined with "+", or a single "x,y" in pixels from the centre (+y is up)`)
	fPatch := flag.Int("patch", 240, "side of the white patch, in pixels")
	fFrames := flag.Int("frames", 2, "frames the patch stays on")
	fISI := flag.Duration("isi", 500*time.Millisecond, "blank interval between trials")
	fISIFrames := flag.Int("isi-frames", 0,
		"if >0, blank by flipping this many frames instead of sleeping -- keeps the\n"+
			"display in steady-state flipping, so the stimulus flip stays vsync-locked")
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

	// Positions must be computed in the space the renderer draws in, which is
	// the logical presentation size when one is set and the output size
	// otherwise -- exactly what Screen.CenterToSDL falls back through. Sizing
	// from RenderOutputSize instead puts a corner patch off-screen on any
	// display that uses logical presentation.
	var dw, dh float32
	if ls := exp.Screen.LogicalSize; ls != nil {
		dw, dh = ls.X, ls.Y
	} else {
		ow, oh, err := exp.Screen.Renderer.CurrentOutputSize()
		if err != nil {
			log.Fatalf("screen size: %v", err)
		}
		dw, dh = float32(ow), float32(oh)
	}
	spots, err := diodePositions(*fDiode, dw, dh, float32(*fPatch))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("drawing space %.0fx%.0f, patch %d px at:\n", dw, dh, *fPatch)
	for _, p := range spots {
		fmt.Printf("    %-12s (%+.0f, %+.0f) from centre\n", p.name, p.x, p.y)
	}
	fmt.Printf("trigger: line %d on %s\n", *fLine, port)
	fmt.Println("\nPut the photodiode ON the patch. The instrument measures the interval")
	fmt.Println("from the TTL rising edge to the light rising edge; add gap_us back to it.")

	patches := make([]*stimuli.Rectangle, len(spots))
	for i, p := range spots {
		patches[i] = stimuli.NewRectangle(p.x, p.y, float32(*fPatch), float32(*fPatch), control.White)
	}
	// A stimulus that draws every patch, so one ShowTS presents them all in the
	// same frame and the diode can sit at any of them without reconfiguring.
	patch := &multiPatch{patches}

	exp.AddDataVariableNames([]string{
		"trial", "flip_ts_ns", "trigger_ts_ns", "gap_us",
		"patch_positions", "patch_xy", "patch_px", "frames",
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
		//
		// Two ways to wait, and they are not equivalent. Sleeping leaves the
		// display idle, and the first flip after an idle period need not be
		// vsync-locked: with no frames queued there is nothing to block on, so
		// present can return at an arbitrary phase and the light then waits for
		// the next scanout. That puts up to a frame of spread on the onset.
		// Flipping through the ISI keeps the pipeline in steady state, which is
		// the condition test_vsync_blocking measures. -isi-frames selects it.
		if *fISIFrames > 0 {
			for f := 0; f < *fISIFrames; f++ {
				if err := exp.Screen.ClearAndUpdate(); err != nil {
					return err
				}
			}
		} else {
			if err := exp.Screen.ClearAndUpdate(); err != nil {
				return err
			}
			time.Sleep(*fISI)
		}

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
			spotNames(spots), spotCoords(spots), *fPatch, *fFrames,
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

// spot is one patch position, named for the data file.
type spot struct {
	name string
	x, y float32
}

// multiPatch draws several rectangles as one stimulus, so a single ShowTS puts
// them all on the same frame.
type multiPatch struct{ rects []*stimuli.Rectangle }

func (m *multiPatch) Draw(sc *apparatus.Screen) error {
	for _, r := range m.rects {
		if err := r.Draw(sc); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiPatch) Present(sc *apparatus.Screen, clear, update bool) error {
	return stimuli.PresentDrawable(m, sc, clear, update)
}

func (m *multiPatch) GetPosition() sdl.FPoint  { return sdl.FPoint{} }
func (m *multiPatch) SetPosition(p sdl.FPoint) {}
func (m *multiPatch) Preload() error           { return nil }
func (m *multiPatch) Unload() error            { return nil }

func spotNames(spots []spot) string {
	out := make([]string, len(spots))
	for i, p := range spots {
		out[i] = p.name
	}
	return strings.Join(out, "+")
}

func spotCoords(spots []spot) string {
	out := make([]string, len(spots))
	for i, p := range spots {
		out[i] = fmt.Sprintf("%.0f,%.0f", p.x, p.y)
	}
	return strings.Join(out, "+")
}

// diodePositions parses -diode into one or more patch centres, in
// goxpyriment's centre-based coordinates.
//
// Note the sign convention: Screen.CenterToSDL computes centreY - y, so +y is
// UP. A "top" position therefore has a POSITIVE y. Getting this backwards puts
// the square at the bottom of the screen, which is exactly what it looks like:
// the program reports "top" and the panel shows it at the bottom.
//
// Accepted: a single name, several names joined with "+", the keywords "all"
// (four corners and the centre) and "corners", or one "x,y" pair in pixels.
//
// Prefer "topleft" when the question is the display's latency. Scanout begins
// at the top-left corner and sweeps down, so a diode there sees light with the
// least scanout delay added -- at 60 Hz a diode at the bottom sees the same
// frame nearly 16.7 ms later, and that term is larger than everything else this
// test measures put together. "all" lights every corner at once, so the diode
// can be moved between runs without changing anything.
func diodePositions(spec string, w, h, patch float32) ([]spot, error) {
	inset := patch/2 + 8
	left, right := -w/2+inset, w/2-inset
	top, bottom := h/2-inset, -h/2+inset // +y is up

	named := map[string]spot{
		"center": {"center", 0, 0}, "centre": {"center", 0, 0},
		"top": {"top", 0, top}, "bottom": {"bottom", 0, bottom},
		"left": {"left", left, 0}, "right": {"right", right, 0},
		"topleft": {"topleft", left, top}, "top-left": {"topleft", left, top},
		"tl":       {"topleft", left, top},
		"topright": {"topright", right, top}, "top-right": {"topright", right, top},
		"tr":         {"topright", right, top},
		"bottomleft": {"bottomleft", left, bottom}, "bottom-left": {"bottomleft", left, bottom},
		"bl":          {"bottomleft", left, bottom},
		"bottomright": {"bottomright", right, bottom}, "bottom-right": {"bottomright", right, bottom},
		"br": {"bottomright", right, bottom},
	}

	spec = strings.ToLower(strings.TrimSpace(spec))
	switch spec {
	case "all":
		return []spot{named["topleft"], named["topright"], named["center"],
			named["bottomleft"], named["bottomright"]}, nil
	case "corners":
		return []spot{named["topleft"], named["topright"],
			named["bottomleft"], named["bottomright"]}, nil
	}

	var out []spot
	for _, part := range strings.Split(spec, "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if p, ok := named[part]; ok {
			out = append(out, p)
			continue
		}
		var x, y float32
		if _, err := fmt.Sscanf(part, "%f,%f", &x, &y); err != nil {
			return nil, fmt.Errorf(`-diode %q: want "all", "corners", one or more of `+
				`topleft top topright left center right bottomleft bottom bottomright `+
				`joined with "+", or a single "x,y" in pixels from the screen centre `+
				`(+y is up)`, spec)
		}
		out = append(out, spot{fmt.Sprintf("%.0f,%.0f", x, y), x, y})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-diode %q: no position given", spec)
	}
	return out, nil
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
