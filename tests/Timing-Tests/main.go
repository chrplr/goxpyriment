// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// Timing-Tests — hardware timing verification suite
//
// Six sub-tests, selected with -test <name>. `av` is the default and the one
// that matters; the rest are diagnostics for when it reports something odd.
//
// Needs no hardware:
//
//	check    Go/no-go: does this machine display and make noise at all
//	display  Frame-interval statistics and the true refresh rate
//	latency  Audio pipeline delay — how long does SDL hold PCM before it sounds
//
// Needs a photodiode and/or a trigger box:
//
//	av       THE stimulus timing test. Five squares + a synchronised tone + a
//	         TTL pulse, every cycle. -no-sound / -no-ttl drop a modality.
//	vrr      Variable-refresh sweep: 1 ms to N ms in 1 ms steps
//	rt       SDL3 event-timestamp reaction-time precision
//
// Two tests that used to live here now have their own directories:
// tests/test_gv_sync (.gv playback synchronisation) and tests/test_dlpio8
// (DLP-IO8-G square-wave characterisation).
//
// Why the console numbers are not the answer
//
// Every statistic printed below comes from software timestamps — when
// goxpyriment *believes* a flip happened. Those stayed textbook-perfect
// throughout a presentation bug that left the panel showing stale frames for
// seconds at a time. Judge timing from the photodiode and TTL recording; treat
// the console output as a cross-check on the software side only.
//
// Usage:
//
//	go run ./tests/Timing-Tests                          # av, the default
//	go run ./tests/Timing-Tests -frames-on 12 -frames-off 18 -cycles 100
//	go run ./tests/Timing-Tests -no-sound -no-ttl        # visual only, no hardware
//	go run ./tests/Timing-Tests -test display
//
// Common flags:
//
//	-test string      Sub-test to run (default "av")
//	-trigger-device   TTL output device: dlpio8 | parallel | gpio | ft232h |
//	                  labjackt4 (default "dlpio8")
//	-port string      Serial port for DLP-IO8-G (default: auto-detect)
//	-labjack-host     LabJack T4 address, e.g. 192.168.1.100 (required for
//	                  -trigger-device labjackt4; there is nothing to auto-detect)
//	-trigger-pin int  Output pin, 1-8, on whichever device was selected (default 1)
//	-trigger-ms int   Trigger pulse duration in ms (default 5)
//	-cycles int       Number of cycles (default 120)
//	-warmup int       Leading cycles discarded from statistics (default 10)
//	-w                Windowed mode: 1024×768 window instead of fullscreen
//	-d int            Display index: monitor to use (-1 = primary)
//	-sysinfo          Print system information and exit
//	-outdir string    Where to write the .csv/-info.txt results
//	                  (default $HOME/goxpy_data)
//	-gc               Leave the garbage collector running during timing loops.
//	                  Off by default. Run a test twice, with and without, to
//	                  measure the collector's effect.
//
// Per-test flags — av:
//
//	-frames-on int    Bright frames per cycle (default 12 = 200 ms at 60 Hz)
//	-frames-off int   Dark frames per cycle (default 18 = 300 ms at 60 Hz)
//	-square-px int    Side of each of the five squares, renderer px (default 200)
//	-level-a int      Dark luminance 0–255 (default 0)
//	-level-b int      Bright luminance 0–255 (default 255)
//	-soa-ms float     Visual-to-audio SOA in ms; negative = audio first (default 0)
//	-freq-hz float    Tone frequency in Hz (default 1000)
//	-hz float         Refresh rate used to derive the tone duration (default 60)
//	-no-sound         Do not play the tone
//	-no-ttl           Do not fire the TTL trigger
//	-audio-frames int Audio hardware buffer, sample frames (0 = SDL default).
//	                  Sets the floor on audio-onset quantisation: 256 frames at
//	                  44100 Hz quantises tone onsets to 5.8 ms steps.
//
// Per-test flags — display:  -duration-s float (default 10)
// Per-test flags — latency:  -freq-hz float, -drain-reps int (default 10)
// Per-test flags — vrr:      -vrr-max-ms int (default 20), -vrr-reps int (default 5)
// Per-test flags — rt:       -iti-ms float, mean ITI jittered ±50 % (default 1000)
//
// Placing the photodiodes
//
// The five squares sit at the four corners and the centre. An LCD scans out
// top-to-bottom, so a bottom square lags a top one by close to a full frame
// period — put a photodiode on a top square and another on a bottom one and the
// difference is your panel's scan-out gradient, in µs per pixel row. Two squares
// on the same row are only microseconds apart and serve as a sanity check.
//
// See docs/TimingTests.md for equipment setup and interpretation.

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/sysinfo"
	"github.com/chrplr/goxpyriment/tests/internal/timingstats"
	"github.com/chrplr/goxpyriment/triggers"
)

// ── Flags ─────────────────────────────────────────────────────────────────────

var (
	fTest        = flag.String("test", "av", "Sub-test: av | vrr | rt | check | display | latency")
	fTrigDevice  = flag.String("trigger-device", "dlpio8", "TTL output device: dlpio8 (USB serial) | parallel (LPT port via ppdev) |\n\tgpio (Linux GPIO character device) | ft232h (Adafruit FT232H over USB) |\n\tlabjackt4 (LabJack T4 over Modbus TCP).\n\tThey are not interchangeable: parallel and gpio write through a local ioctl,\n\tdlpio8 and ft232h cross a USB link, and labjackt4 crosses the network — each\n\tstep adds latency and jitter between the flip and the TTL edge. Prefer\n\tparallel on a desktop with an LPT port, gpio on a single-board computer\n\t(Raspberry Pi, Rock Pi, …). Recorded as 'trigger' in the results header.")
	fPort        = flag.String("port", "", "Serial port for DLP-IO8-G (empty = auto-detect) [trigger-device=dlpio8]")
	fLJHost      = flag.String("labjack-host", "", "LabJack T4 address, e.g. 192.168.1.100 or 192.168.1.100:502 (required)\n\t[trigger-device=labjackt4]. -trigger-pin selects the line: pin 1 is DIO4 =\n\tscrew terminal FIO4, pin 8 is DIO11 = EIO3 on the DB15.")
	fParPort     = flag.String("parallel-port", "", "Parallel port device, e.g. /dev/parport0 (empty = first accessible one)\n\t[trigger-device=parallel]. -trigger-pin selects the data line: pin 1 is D0,\n\twhich is DB25 pin 2.")
	fGPIOChip    = flag.String("gpio-chip", "/dev/gpiochip0", "GPIO chip device path [trigger-device=gpio]")
	fGPIOPins    = flag.String("gpio-pins", "17,27,22,5,6,13,19,26", "The 8 GPIO output pins, comma-separated, chip-relative (BCM numbering on a\n\tRaspberry Pi) [trigger-device=gpio]. -trigger-pin selects among them: pin 1 is\n\tthe first in this list.")
	fTriggerPin  = flag.Int("trigger-pin", 1, "Output pin (1–8). On dlpio8 this is the number on the terminal block;\n\ton gpio it is the position in -gpio-pins, NOT the BCM number; on ft232h it is\n\tthe D-bus line, pin 1 = AD0; on labjackt4 it is the position in DIO4–DIO11,\n\tpin 1 = DIO4 = FIO4.")
	fTriggerMs   = flag.Int("trigger-ms", 5, "Trigger pulse duration (ms)")
	fCycles      = flag.Int("cycles", 120, "Number of cycles [av / rt]")
	fLevelA      = flag.Int("level-a", 0, "Dark luminance 0–255 (surround) [av / vrr]")
	fLevelB      = flag.Int("level-b", 255, "Bright luminance 0–255 (squares) [av / vrr]")
	fFramesOn    = flag.Int("frames-on", 12, "Bright frames per cycle (12 = 200 ms at 60 Hz) [av]")
	fFramesOff   = flag.Int("frames-off", 18, "Dark frames per cycle (18 = 300 ms at 60 Hz) [av]")
	fSoaMs       = flag.Float64("soa-ms", 0, "Visual-to-audio SOA ms; negative = audio first [av]")
	fItiMs       = flag.Float64("iti-ms", 1000, "Mean inter-trial interval ms, jittered ±50 % [rt]")
	fFreqHz      = flag.Float64("freq-hz", 1000, "Tone frequency Hz [av / latency]")
	fDurationS   = flag.Float64("duration-s", 10, "Measurement duration in seconds [display]")
	fAudioFrames = flag.Int("audio-frames", 0, "Audio hardware buffer size in sample frames, e.g. 256, 512, 1024,... (0=SDL default) ")
	fHz          = flag.Float64("hz", 60.0, "Expected display refresh rate in Hz; sets the tone duration (frames-on × 1/hz) [av]")
	fWarmup      = flag.Int("warmup", 10, "Leading cycles discarded from statistics [av / display]")
	fDrainReps   = flag.Int("drain-reps", 10, "Repetitions per tone duration [latency]")
	fVRRMaxMs    = flag.Int("vrr-max-ms", 20, "Maximum stimulus duration to sweep, in 1 ms steps [vrr]")
	fVRRReps     = flag.Int("vrr-reps", 5, "Repetitions per duration step [vrr].\n\tThe defaults (20 steps x 5 reps) run in about 20 s; the sweep is a\n\tpass/fail check that arbitrary durations are presentable, so it does not\n\tneed the cycle counts the av test uses. Raise both for tighter SDs.")
	fWindowed    = flag.Bool("w", false, "Windowed mode (1024×768 window instead of fullscreen)")
	fDisplay     = flag.Int("d", -1, "Display index: monitor where the window/fullscreen will open (-1 = primary)")
	fExclusiveFS = flag.String("exclusive-fullscreen", "auto", "Fullscreen presentation: auto | on (exclusive, bypasses the compositor where possible) | off (fullscreen-desktop).\n\tRecorded as 'sys fullscreen_mode' in the results header; the two are not comparable.")
	fSysInfo     = flag.Bool("sysinfo", false, "Print system information and exit")
	fOutDir      = flag.String("outdir", "", "Directory for the .csv/-info.txt results (default: $HOME/goxpy_data).\n\tUse it to keep a session's data files beside its other outputs.")
	fGC          = flag.Bool("gc", false, "Leave the garbage collector RUNNING during timing-critical loops.\n\tBy default the collector is suspended; pass -gc to measure its effect on timing\n\t(run the same test twice, with and without, to obtain the comparison).")
	fSquarePx    = flag.Int("square-px", 0, "Side of each of the five stimulus squares, in renderer pixels;\n\t0 = one quarter of the render height, which keeps each square's centre\n\tclear of the bezel and fixes the top↔bottom separation at 0.750 [av / vrr]")
	fNoSound     = flag.Bool("no-sound", false, "Do not play the tone [av]")
	fNoTTL       = flag.Bool("no-ttl", false, "Do not fire the TTL trigger [av]")
)

// ── Run outcome ───────────────────────────────────────────────────────────────

// aborted records that the experimenter stopped a run with Esc rather than
// letting it finish. Every test previously unwound through control.EndLoop
// whether it completed or was cut short, which left the two indistinguishable
// from outside and made the process exit 0 either way.
//
// The distinction matters because the exit status is now read by a machine.
// bbtk-capture -- cmd starts the stimulus inside a hardware capture window and
// uses its status to decide whether the recording is worth keeping: a non-zero
// status aborts the capture and saves nothing, which is the right outcome, since
// a window already spent cannot be re-recorded and an incomplete stimulus makes
// the trace uninterpretable.
//
// Partial data is still written — see the exit path in main — so an aborted run
// remains available for inspection. It is simply not reported as a completed
// step.
var aborted bool

// ── Garbage-collector control ─────────────────────────────────────────────────

// suspendGC turns the garbage collector off for the duration of a
// timing-critical loop and returns the function that restores it.
func suspendGC() func() {
	if *fGC {
		return func() {}
	}
	oldGC := debug.SetGCPercent(-1)
	return func() { debug.SetGCPercent(oldGC) }
}

// gcLabel describes the collector state, for report headers and data comments.
func gcLabel() string {
	if *fGC {
		return "on"
	}
	return "suspended"
}

// ── Screen fill helper ─────────────────────────────────────────────────────────

// gray builds an opaque gray from a single 0–255 level.
func gray(level byte) control.Color { return control.RGB(level, level, level) }

// paintFunc renders one stimulus frame into the backbuffer: fg is the stimulus
// colour, bg the surround. It does not present — the caller flips.
type paintFunc func(fg, bg control.Color)

// newPainter builds the per-frame paint function shared by every visual test.
//
// It paints five squares of -square-px side — one at each screen corner and one
// centred — over a bg-coloured surround. Placing a photodiode on each square
// measures the delay between screen regions: an LCD scans out top-to-bottom, so
// the bottom squares lag the top ones by close to a full frame period. (The two
// squares on a given row are only microseconds apart, being on the same
// scanline; they serve as a sanity check rather than a measurement.)
//
// The rectangles are laid out once, here, rather than per frame: the geometry
// never changes, and the logical-size query is a CGo call that has no business
// inside a VSYNC-locked loop.
//
// Coordinates are the renderer's own — the logical presentation size when one is
// set — so -square-px is in renderer pixels and the corners stay at the corners
// under HiDPI scaling. A square's *physical* size therefore varies with the
// logical/physical scale factor, which matters when comparing photodiode signal
// amplitude across video drivers or displays.
func newPainter(exp *control.Experiment) (paintFunc, error) {
	r := exp.Screen.Renderer

	// Drawing happens in the logical presentation space when one is set; fall
	// back to the raw output size when it is not.
	w, h, _, err := r.LogicalPresentation()
	if err != nil || w <= 0 || h <= 0 {
		w, h, err = exp.Screen.Size()
		if err != nil {
			return nil, fmt.Errorf("newPainter: querying render size: %w", err)
		}
	}

	fw, fh := float32(w), float32(h)

	// Default the square to a quarter of the render height rather than a fixed
	// pixel count. That size scales with the panel, is large enough on any
	// display for a photodiode to sit at the square's centre well clear of the
	// bezel — and, because the centres then land at H/8 and 7H/8, it fixes the
	// top↔bottom separation at exactly 0.750 of screen height on every display,
	// so the figure the gradient is divided by no longer varies per machine.
	side := float32(*fSquarePx)
	sideAuto := ""
	if *fSquarePx <= 0 {
		side = fh / 4
		sideAuto = " (auto: ¼ of render height)"
	}
	if side*2 > fw || side*2 > fh {
		return nil, fmt.Errorf("newPainter: a %.0f px square does not fit a %dx%d render area "+
			"(corner squares would overlap)", side, w, h)
	}

	rects := []control.FRect{
		{X: 0, Y: 0, W: side, H: side},                             // top-left
		{X: fw - side, Y: 0, W: side, H: side},                     // top-right
		{X: 0, Y: fh - side, W: side, H: side},                     // bottom-left
		{X: fw - side, Y: fh - side, W: side, H: side},             // bottom-right
		{X: (fw - side) / 2, Y: (fh - side) / 2, W: side, H: side}, // centre
	}
	// Report the geometry rather than leaving it to be reconstructed later. The
	// scan-out gradient is (bottom onset − top onset) divided by the separation
	// below, so that fraction is part of the measurement, not a detail — and
	// recomputing it after the fact is exactly how a run gets misread.
	//
	// The separation holds only if each photodiode sits at its square's CENTRE.
	// A small square puts that centre close to the bezel, where a diode cannot
	// lie flat; it then ends up further out, the true separation exceeds the
	// figure below, and the derived sweep time comes out too large — possibly
	// larger than a frame period, which is the tell that this has happened.
	// The default of a quarter of the render height keeps the centres clear of
	// the edge on any panel; -square-px overrides it when you need a specific
	// size.
	sepPx := fh - side
	sepFrac := float64(sepPx) / float64(fh)
	cxL, cxR, cxC := side/2, fw-side/2, fw/2
	cyT, cyB, cyC := side/2, fh-side/2, fh/2

	fmt.Printf("stimulus: five %.0f px squares%s (corners + centre) on a %.0fx%.0f render area\n",
		side, sideAuto, fw, fh)
	fmt.Printf("  centres:  x = %.0f / %.0f / %.0f    y = %.0f / %.0f / %.0f\n",
		cxL, cxC, cxR, cyT, cyC, cyB)
	fmt.Printf("  top↔bottom separation: %.0f px = %.4f of screen height\n", sepPx, sepFrac)
	fmt.Printf("  place each photodiode at its square's CENTRE — the separation assumes it\n")

	if exp.Data != nil {
		exp.Data.WriteComment(fmt.Sprintf("stim square-px: %.0f", side))
		exp.Data.WriteComment(fmt.Sprintf("stim render-area: %.0fx%.0f", fw, fh))
		exp.Data.WriteComment(fmt.Sprintf("stim square-centres-x: %.0f,%.0f,%.0f", cxL, cxC, cxR))
		exp.Data.WriteComment(fmt.Sprintf("stim square-centres-y: %.0f,%.0f,%.0f", cyT, cyC, cyB))
		exp.Data.WriteComment(fmt.Sprintf("stim top-bottom-separation-px: %.0f", sepPx))
		exp.Data.WriteComment(fmt.Sprintf("stim top-bottom-separation-frac: %.4f", sepFrac))
	}

	return func(fg, bg control.Color) {
		r.SetDrawColor(bg.R, bg.G, bg.B, 255)
		r.Clear()
		r.SetDrawColor(fg.R, fg.G, fg.B, 255)
		for i := range rects {
			_ = r.RenderFillRect(&rects[i])
		}
	}, nil
}

// avNowMs returns the SDL high-resolution clock in milliseconds.
//
// The av loop must take every timestamp from ONE clock. Screen.FlipTS() — and
// so Tone.PlaySyncedWithFlip, which wraps it — returns sdl.TicksNS(), whose
// origin is SDL_Init. clock.GetTimeNS() counts from Go package init, tens of
// milliseconds earlier (see the warning in clock/clock.go). Mixing the two made
// bright_duration_ms read ~30 ms long on every sound-enabled run, and left
// t_visual_after earlier than t_visual_before in 100 % of cycles.
//
// Standardising on the SDL clock rather than the Go one keeps the precise flip
// timestamp captured inside PlaySyncedWithFlip, and matches the rt and vrr
// tests, which already timestamp with FlipTS.
func avNowMs() float64 { return float64(control.TicksNS()) / 1e6 }

// fillGray paints one stimulus frame at a uniform gray level (0–255) and
// presents it. Returns the time just before and just after RenderPresent (the
// VSYNC wait), in milliseconds with sub-millisecond precision.
func fillGray(exp *control.Experiment, paint paintFunc, level byte) (tBefore, tAfter float64) {
	paint(gray(level), gray(byte(*fLevelA)))
	tBefore = float64(clock.GetTimeNS()) / 1e6
	exp.Screen.Update() // blocks until VSYNC
	tAfter = float64(clock.GetTimeNS()) / 1e6
	return
}

// ── Trigger setup ──────────────────────────────────────────────────────────────

// setupTrigger opens the TTL output device named by -trigger-device.
//
// It returns the device and a one-line description of what was actually opened.
// The description is written to the results header rather than merely printed:
// the devices do not produce the same trigger→luminance figure — a GPIO write is
// an ioctl on a local chip, a DLP-IO8 or FT232H write crosses a USB link, a
// LabJack T4 write crosses the network — so which one fired is part of the
// measurement, not a detail of the session. An empty description means no device
// was opened.
func setupTrigger() (triggers.OutputTTLDevice, string) {
	switch *fTrigDevice {
	case "dlpio8":
		return setupDLPIO8()
	case "parallel":
		return setupParallel()
	case "gpio":
		return setupGPIO()
	case "ft232h":
		return setupFT232H()
	case "labjackt4":
		return setupLabJackT4()
	default:
		log.Fatalf("-trigger-device %q: %s", *fTrigDevice, trigDeviceChoices)
		return nil, "" // unreachable; log.Fatalf exits
	}
}

// trigDeviceChoices is the one message listing the accepted -trigger-device
// names. It is shared by setupTrigger and checkTriggerFlags: they used to carry
// separate copies, so adding a device to one and not the other would accept a
// name at startup and then reject it after the window had opened.
const trigDeviceChoices = "choose dlpio8, parallel, gpio, ft232h or labjackt4"

func setupDLPIO8() (triggers.OutputTTLDevice, string) {
	var trig triggers.OutputTTLDevice
	var portName string
	var err error

	if *fPort != "" {
		d, openErr := triggers.NewDLPIO8(*fPort)
		if openErr != nil {
			log.Printf("warning: DLP-IO8 on %s: %v — triggers disabled", *fPort, openErr)
			trig = triggers.NullOutputTTLDevice{}
		} else {
			trig, portName = d, *fPort
		}
	} else {
		trig, portName, err = triggers.AutoDetectDLPIO8()
		if err != nil {
			log.Printf("warning: DLP-IO8 auto-detect: %v — triggers disabled", err)
		}
	}
	if portName == "" {
		return trig, ""
	}
	fmt.Printf("DLP-IO8-G found on %s (trigger pin %d, pulse %d ms)\n",
		portName, *fTriggerPin, *fTriggerMs)
	return trig, fmt.Sprintf("dlpio8 port=%s pin=%d", portName, *fTriggerPin)
}

func setupParallel() (triggers.OutputTTLDevice, string) {
	device := *fParPort
	if device == "" {
		ports := triggers.AvailableParallelPorts()
		if len(ports) == 0 {
			log.Printf("warning: no accessible parallel port found — triggers disabled")
			log.Printf("         (needs Linux with the ppdev module loaded, and rw access:")
			log.Printf("          sudo modprobe ppdev; sudo usermod -aG lp $USER, then log in again)")
			return triggers.NullOutputTTLDevice{}, ""
		}
		// Report the choice rather than making it silently: a machine with two
		// LPT ports would otherwise fire whichever enumerated first, and the
		// trace gives no clue which one that was.
		device = ports[0]
		if len(ports) > 1 {
			fmt.Printf("parallel: %d ports accessible %v — using the first; pass -parallel-port to choose\n",
				len(ports), ports)
		}
	}

	p := triggers.NewParallelPort(device)
	if err := p.Open(); err != nil {
		log.Printf("warning: parallel port %s: %v — triggers disabled", device, err)
		return triggers.NullOutputTTLDevice{}, ""
	}

	// D0-D7 are DB25 pins 2-9, so the connector pin is the line index + 2.
	// Print it: the data-line number is what the API takes, but the DB25 pin is
	// what the probe clips onto, and confusing the two is a silent miswiring.
	line := triggerLine()
	fmt.Printf("Parallel port %s opened (trigger pin %d = D%d = DB25 pin %d, pulse %d ms)\n",
		device, *fTriggerPin, line, line+2, *fTriggerMs)
	fmt.Printf("  ground is any of DB25 pins 18-25.\n")
	return p, fmt.Sprintf("parallel device=%s pin=%d line=D%d db25=%d",
		device, *fTriggerPin, line, line+2)
}

func setupGPIO() (triggers.OutputTTLDevice, string) {
	pins, err := parsePins(*fGPIOPins)
	if err != nil {
		log.Fatalf("-gpio-pins %q: %v", *fGPIOPins, err)
	}

	dev, err := triggers.NewLinuxGPIOTrigger(
		triggers.WithGPIOChip(*fGPIOChip),
		triggers.WithGPIOOutputPins(pins),
	)
	if err != nil {
		// Matches the DLP-IO8 path: warn and continue without triggers rather
		// than exiting, so the visual measurement of a run is still obtained.
		// Watch for it — a run that loses its TTL still exits zero, and with
		// BBTK_CAPTURE=1 that spends a capture window on an untriggered trace.
		log.Printf("warning: GPIO on %s: %v — triggers disabled", *fGPIOChip, err)
		log.Printf("         (needs Linux, kernel >= 5.10, and rw access to the chip:")
		log.Printf("          sudo usermod -aG gpio $USER, then log in again)")
		return triggers.NullOutputTTLDevice{}, ""
	}

	// Print the pin actually driven, not just its index. -trigger-pin is a
	// position in -gpio-pins, so on the defaults -trigger-pin 1 is BCM 17 — a
	// number nowhere in the command line, and the one to put the probe on.
	bcm := pins[triggerLine()]
	fmt.Printf("GPIO %s opened, output pins %v\n", *fGPIOChip, pins)
	fmt.Printf("  trigger pin %d = chip line %d (BCM %d on a Raspberry Pi), pulse %d ms\n",
		*fTriggerPin, bcm, bcm, *fTriggerMs)
	fmt.Printf("  NOTE: these lines swing 0–3.3 V, not 5 V. Check that your recorder's\n")
	fmt.Printf("        TTL input triggers at 3.3 V before committing to a long capture.\n")
	return dev, fmt.Sprintf("gpio chip=%s pin=%d line=%d pins=%v",
		*fGPIOChip, *fTriggerPin, bcm, pins)
}

func setupFT232H() (triggers.OutputTTLDevice, string) {
	// NewFT232H rather than AutoDetectFT232H: the latter turns a failure into a
	// NullOutputTTLDevice and keeps the reason to itself, and the reason here is
	// usually actionable (ftdi_sio holding the interface, or no rw access).
	dev, err := triggers.NewFT232H()
	if err != nil {
		// Same policy as the other devices: warn and continue, so the visual
		// measurement is still obtained. The `av` test refuses to start without
		// a working device unless -no-ttl was given.
		log.Printf("warning: FT232H: %v — triggers disabled", err)
		log.Printf("         (Linux only; the ftdi_sio module must not hold the device,")
		log.Printf("          and /dev/bus/usb/... must be readable and writable:")
		log.Printf("          sudo rmmod ftdi_sio, then a udev rule or the plugdev group)")
		return triggers.NullOutputTTLDevice{}, ""
	}

	// Print the AD line, not just the pin index: AD0 is what the probe clips
	// onto, and the board silkscreen numbers the D-bus from 0 while
	// -trigger-pin counts from 1.
	line := triggerLine()
	fmt.Printf("FT232H opened, outputs AD0–AD7\n")
	fmt.Printf("  trigger pin %d = AD%d, pulse %d ms\n", *fTriggerPin, line, *fTriggerMs)
	fmt.Printf("  NOTE: these lines swing 0–3.3 V, not 5 V. Check that your recorder's\n")
	fmt.Printf("        TTL input triggers at 3.3 V before committing to a long capture.\n")
	return dev, fmt.Sprintf("ft232h pin=%d line=AD%d", *fTriggerPin, line)
}

func setupLabJackT4() (triggers.OutputTTLDevice, string) {
	// The output group is left at the driver's default, DIO4–DIO11: the T4's
	// DIO0–DIO3 are the analog inputs AIN0–AIN3 and cannot be driven, and moving
	// the base up would collide with the input group at DIO12. So there is
	// nothing to configure here beyond the address.
	dev, err := triggers.NewLabJackT4(*fLJHost)
	if err != nil {
		log.Printf("warning: LabJack T4 at %s: %v — triggers disabled", *fLJHost, err)
		log.Printf("         (the T4 must be reachable on Modbus TCP port 502; check with")
		log.Printf("          go run ./tests/test_labjackt4 -host %s -hold)", *fLJHost)
		return triggers.NullOutputTTLDevice{}, ""
	}

	dio := t4OutputBase + triggerLine()
	fmt.Printf("LabJack T4 at %s opened, outputs DIO4–DIO11 (FIO4–FIO7 + EIO0–EIO3)\n", *fLJHost)
	fmt.Printf("  trigger pin %d = DIO%d = %s, pulse %d ms\n",
		*fTriggerPin, dio, t4TerminalName(dio), *fTriggerMs)
	fmt.Printf("  NOTE: these lines swing 0–3.3 V, not 5 V, and every write crosses the\n")
	fmt.Printf("        network — expect more flip→TTL latency and jitter than from a\n")
	fmt.Printf("        parallel port or a GPIO chip. Read the TTL channel of the\n")
	fmt.Printf("        recording, not the console, for the figure that matters.\n")
	return dev, fmt.Sprintf("labjackt4 host=%s pin=%d dio=%d terminal=%s",
		*fLJHost, *fTriggerPin, dio, t4TerminalName(dio))
}

// t4OutputBase is the DIO number of the T4 output line 0 — the driver's default
// (triggers.WithT4OutputBase is left unset), repeated here only so the printed
// pin can be resolved to a screw terminal.
const t4OutputBase = 4

// t4TerminalName maps a T4 DIO number to the label printed on the hardware.
// The DIO numbering is contiguous but the terminals are not: DIO4 is FIO4 on the
// screw block while DIO8 is EIO0 on the DB15, so the number in the API is not
// the number to look for on the device.
func t4TerminalName(dio int) string {
	switch {
	case dio >= 0 && dio <= 3:
		return fmt.Sprintf("AIN%d (analog only)", dio)
	case dio <= 7:
		return fmt.Sprintf("FIO%d", dio)
	case dio <= 15:
		return fmt.Sprintf("EIO%d", dio-8)
	case dio <= 19:
		return fmt.Sprintf("CIO%d", dio-16)
	}
	return fmt.Sprintf("DIO%d", dio)
}

// parsePins parses a comma-separated list of exactly 8 GPIO pin numbers.
func parsePins(s string) ([8]int, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 8 {
		return [8]int{}, fmt.Errorf("expected 8 comma-separated pin numbers, got %d", len(parts))
	}
	var pins [8]int
	seen := make(map[int]int, 8)
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return [8]int{}, fmt.Errorf("pin %d: %w", i+1, err)
		}
		if n < 0 {
			return [8]int{}, fmt.Errorf("pin %d: %d is negative", i+1, n)
		}
		// A duplicate would make two -trigger-pin values drive the same line,
		// which is invisible in the trace and would be read as a wiring fault.
		if first, dup := seen[n]; dup {
			return [8]int{}, fmt.Errorf("pin %d repeats line %d, already given as pin %d", i+1, n, first)
		}
		seen[n] = i + 1
		pins[i] = n
	}
	return pins, nil
}

// printStatsVsMean reports a series against its own measured mean rather than a
// target derived from -hz. ComputeStats must run twice: the first pass exists
// only to obtain the mean that the second pass uses as the deviation reference.
func printStatsVsMean(label string, xs []float64) {
	if len(xs) == 0 {
		fmt.Printf("\n%s: no samples\n", label)
		return
	}
	s := timingstats.ComputeStats(xs, 0)
	s = timingstats.ComputeStats(xs, s.Mean)
	timingstats.PrintStats(label, s, s.Mean)
}

// ── Test: av ──────────────────────────────────────────────────────────────────

// runAV is the suite's primary stimulus-timing test. By default every cycle
// presents all three modalities at once:
//
//   - visual: five squares (four corners + centre) go bright for -frames-on
//     frames, then the screen is dark for -frames-off frames
//   - audio:  a tone of frames-on × frame-period, synchronised to the visual
//     onset (or offset from it by -soa-ms)
//   - TTL:    a -trigger-ms pulse at the visual onset
//
// -no-sound and -no-ttl drop a modality, which is how this test covers what used
// to be three separate ones: -no-sound is a pure visual-onset test, and the audio
// channel of a recording gives tone-onset jitter over a long session.
//
// Put a photodiode on one square and the trigger on a second recorder channel;
// the quantity of interest is the offset between them and its stability. Software
// timestamps alone cannot answer that — they only show when goxpyriment
// *believes* the flip happened, which stayed textbook-perfect throughout a
// presentation bug that left the panel showing stale frames for seconds. The
// recording is the ground truth; the console statistics below are a cross-check.
//
// Two squares at different heights measure the panel's scan-out gradient: an LCD
// draws top-to-bottom, so a bottom square lags a top one by close to a full frame
// period, and a stimulus's onset therefore depends on where it sits on screen.
func runAV(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	framesOn, framesOff := *fFramesOn, *fFramesOff
	frameMs := 1000.0 / *fHz
	toneDurMs := int(math.Round(float64(framesOn) * frameMs))

	withSound := !*fNoSound
	withTTL := !*fNoTTL
	// Refuse to run rather than quietly dropping the TTL. This used to print
	// "no DLP-IO8-G found" — naming one device regardless of -trigger-device,
	// so a GPIO chip that failed to open reported a missing serial box — and
	// then carried on to a successful exit with an empty TTL channel. Under
	// BBTK_CAPTURE that silently spends a capture window on an untriggered
	// trace, and the loss is only discovered when the events file is read.
	//
	// -no-ttl is how a visual-only run is requested, so there is no need to
	// infer one from a hardware failure.
	if _, isNull := trig.(triggers.NullOutputTTLDevice); isNull && withTTL {
		log.Fatalf("av: -trigger-device %s could not be opened, and the TTL is part of "+
			"this measurement.\n"+
			"    Fix the device (see the warning above), or pass -no-ttl for a "+
			"visual-only run.", *fTrigDevice)
	}

	modalities := "visual"
	if withSound {
		modalities += "+audio"
	}
	if withTTL {
		modalities += "+ttl"
	}
	syncMethod := "goroutine"
	if *fSoaMs == 0 {
		syncMethod = "PlaySyncedWithFlip"
	}

	fmt.Printf("av: %s  frames-on=%d (%.1f ms) frames-off=%d (%.1f ms)  cycles=%d  warmup=%d\n",
		modalities, framesOn, float64(framesOn)*frameMs, framesOff, float64(framesOff)*frameMs,
		*fCycles, *fWarmup)
	if withSound {
		fmt.Printf("av: tone %.0f Hz for %d ms  soa=%.1f ms  sync=%s\n",
			*fFreqHz, toneDurMs, *fSoaMs, syncMethod)
	}

	if *fWarmup >= *fCycles {
		return fmt.Errorf("av: -warmup %d discards all %d cycles; lower it or raise -cycles",
			*fWarmup, *fCycles)
	}

	exp.Data.WriteComment(fmt.Sprintf(
		"test=av level-a=%d level-b=%d square-px=%d frames-on=%d frames-off=%d cycles=%d warmup=%d hz=%.3f sound=%v ttl=%v soa-ms=%.1f freq-hz=%.0f",
		*fLevelA, *fLevelB, *fSquarePx, framesOn, framesOff, *fCycles, *fWarmup, *fHz,
		withSound, withTTL, *fSoaMs, *fFreqHz))
	exp.AddDataVariableNames([]string{
		"cycle",
		"t_visual_before_ms", "t_visual_after_ms",
		"bright_duration_ms", "period_ms",
		"t_audio_queued_ms", "soa_intended_ms", "soa_actual_ms",
	})

	var tone *stimuli.Tone
	if withSound {
		tone = stimuli.NewTone(*fFreqHz, toneDurMs, 0.8)
		if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
			return fmt.Errorf("av: preload tone: %w", err)
		}
		defer tone.Unload()
	}

	soaDur := time.Duration(math.Abs(*fSoaMs) * float64(time.Millisecond))
	audioFirst := *fSoaMs < 0

	var brightDurations, periods, soaActuals []float64
	var prevBrightStart float64

	return exp.Run(func() error {
		restoreGC := suspendGC()
		defer restoreGC()

		// Update holds to the frame boundary itself; there is no longer a
		// separate paced variant to select between (the -paced-flip flag is
		// gone with it).
		flip := exp.Screen.Update

		paint, err := newPainter(exp)
		if err != nil {
			return err
		}
		bright, dark := gray(byte(*fLevelB)), gray(byte(*fLevelA))

		// fire pulses the trigger on its own goroutine: FireTrigger holds the
		// line high for -trigger-ms, which would otherwise stall the frame loop.
		fire := func() {
			if withTTL {
				go triggers.FireTrigger(trig, triggerLine(), time.Duration(*fTriggerMs)*time.Millisecond)
			}
		}

		// showFrame paints one frame and presents it. PumpEvents runs every
		// frame, not once per cycle: under a compositor the backend must be
		// serviced on the same cadence as the flips.
		showFrame := func(fg control.Color) {
			paint(fg, dark)
			flip()
			control.PumpEvents()
		}

		// holdBright keeps the bright squares up for the remaining frames of the
		// phase, redrawing each one so double-buffering never flips dark into view.
		holdBright := func(remaining int) {
			for f := 0; f < remaining; f++ {
				showFrame(bright)
			}
		}

		for cycle := 0; cycle < *fCycles; cycle++ {
			var tVisB, tVisA, tAudioQ float64

			switch {
			case withSound && audioFirst:
				// Audio leads: queue the tone, wait out the SOA, then flip.
				tAudioQ = avNowMs()
				_ = tone.Play()
				time.Sleep(soaDur)
				paint(bright, dark)
				tVisB = avNowMs()
				flip()
				tVisA = avNowMs()
				control.PumpEvents()
				fire()
				holdBright(framesOn - 1)

			case withSound && soaDur == 0:
				// SOA=0: PlaySyncedWithFlip pre-fills the audio buffer before the
				// flip and resumes the device immediately after VSYNC. This removes
				// goroutine scheduling jitter from the audio path; the onset then
				// lags by at most one callback period, which is why -audio-frames
				// sets the floor on audio-onset quantisation.
				paint(bright, dark)
				tVisB = avNowMs()
				flipNS, _ := tone.PlaySyncedWithFlip(exp.Screen)
				tVisA = float64(flipNS) / 1e6
				// Timestamp AFTER the call returns, which is after it resumes the
				// audio device. Setting tAudioQ = tVisA instead made
				// soa_actual_ms identically zero by construction: the column, and
				// the "mean 0.000, SD 0.000" summary it produced, measured
				// nothing. This measures the real gap between the flip and the
				// device being released — the software-side part of the audio
				// path. True audio-visual sync still comes only from the Mic and
				// Opto channels of a recording.
				tAudioQ = avNowMs()
				control.PumpEvents()
				fire()
				holdBright(framesOn - 1)

			case withSound:
				// Visual leads by soaDur: schedule the tone off the visual onset
				// while holdBright keeps the screen bright concurrently.
				paint(bright, dark)
				tVisB = avNowMs()
				flip()
				tVisA = avNowMs()
				control.PumpEvents()
				fire()
				tAudioQCh := make(chan float64, 1)
				go func() {
					time.Sleep(soaDur)
					tAudioQCh <- avNowMs()
					_ = tone.Play()
				}()
				holdBright(framesOn - 1)
				tAudioQ = <-tAudioQCh

			default:
				// Visual only.
				paint(bright, dark)
				tVisB = avNowMs()
				flip()
				tVisA = avNowMs()
				control.PumpEvents()
				fire()
				holdBright(framesOn - 1)
			}

			// Dark phase: frames-off frames as the ITI between stimuli. Its first
			// flip ends the bright phase, so its timestamp gives the duration.
			var tDarkStart float64
			for f := 0; f < framesOff; f++ {
				showFrame(dark)
				if f == 0 {
					tDarkStart = avNowMs()
				}
			}

			brightMs := tDarkStart - tVisA
			var periodMs float64
			if prevBrightStart > 0 {
				periodMs = tVisA - prevBrightStart
			}
			prevBrightStart = tVisA

			if cycle >= *fWarmup {
				brightDurations = append(brightDurations, brightMs)
				if periodMs > 0 {
					periods = append(periods, periodMs)
				}
				if withSound {
					soaActuals = append(soaActuals, tAudioQ-tVisA)
				}
			}

			exp.Data.Add(cycle,
				fmt.Sprintf("%.3f", tVisB), fmt.Sprintf("%.3f", tVisA),
				fmt.Sprintf("%.3f", brightMs), fmt.Sprintf("%.3f", periodMs),
				fmt.Sprintf("%.3f", tAudioQ),
				fmt.Sprintf("%.1f", *fSoaMs), fmt.Sprintf("%.3f", tAudioQ-tVisA))

			if exp.PollEvents(nil).QuitRequested {
				aborted = true
				return control.EndLoop
			}
		}

		// Frame counts are the authoritative unit, so the measured mean is the
		// deviation reference rather than a target derived from -hz.
		printStatsVsMean(fmt.Sprintf("Bright-phase duration (frames-on=%d)", framesOn), brightDurations)
		printStatsVsMean(fmt.Sprintf("Period (frames-on=%d + frames-off=%d)", framesOn, framesOff), periods)
		if withSound {
			// Software-side only: when the tone was handed to the device, not
			// when it reached the speaker. Audio-visual sync must be read from
			// the Mic vs Opto channels of a photodiode/microphone recording.
			printStatsVsMean(fmt.Sprintf("Audio released − visual onset, software side (intended SOA %.1f ms)", *fSoaMs), soaActuals)
		}

		fmt.Printf("\nav: %d cycles complete (%d discarded as warm-up).\n", *fCycles, *fWarmup)
		fmt.Println("Software timestamps only — the photodiode/TTL recording is the ground truth.")
		return control.EndLoop
	})
}

// ── Test: jitter ───────────────────────────────────────────────────────────────

// runJitter measures raw frame-interval variance by repeatedly flipping a gray screen.
func runJitter(exp *control.Experiment) error {
	fmt.Printf("display: %.1f s of frames  warmup=%d  (ESC to stop early)\n", *fDurationS, *fWarmup)

	exp.Data.WriteComment(fmt.Sprintf("test=display duration-s=%.1f warmup=%d", *fDurationS, *fWarmup))
	exp.AddDataVariableNames([]string{"frame", "t_before_ms", "t_after_ms", "interval_ms"})

	var intervals []float64
	var prevT float64

	return exp.Run(func() error {
		restoreGC := suspendGC()
		defer restoreGC()

		paint, err := newPainter(exp)
		if err != nil {
			return err
		}

		level := byte(128)
		deadline := time.Now().Add(time.Duration(*fDurationS * float64(time.Second)))
		frame := 0

		for time.Now().Before(deadline) {
			// Zero the pacing tallies at the warm-up boundary so they cover
			// exactly the frames the interval statistics below are computed
			// from, and not the first flips of the session.
			if frame == *fWarmup {
				exp.Screen.ResetPacingStats()
			}

			tB, tA := fillGray(exp, paint, level)

			var intervalMs float64
			if prevT > 0 {
				intervalMs = tA - prevT
				if frame >= *fWarmup {
					intervals = append(intervals, intervalMs)
				}
			}
			prevT = tA
			exp.Data.Add(frame, fmt.Sprintf("%.3f", tB), fmt.Sprintf("%.3f", tA), fmt.Sprintf("%.3f", intervalMs))
			frame++

			state := exp.PollEvents(nil)
			if state.QuitRequested {
				aborted = true
				break
			}
		}

		// Compute stats using the measured mean as target so that >0.5 ms / >1.0 ms
		// counts reflect deviation from actual frame rate, not a hardcoded 60 Hz assumption.
		s := timingstats.ComputeStats(intervals, 16.67) // first pass to obtain mean
		estimatedHz := 0.0
		if s.Mean > 0 {
			estimatedHz = 1000.0 / s.Mean
			s = timingstats.ComputeStats(intervals, s.Mean) // recompute late counts against actual mean
		}
		fmt.Printf("\nEstimated refresh rate: %.3f Hz  (pass -hz %.2f to av so the tone matches a frame)\n",
			estimatedHz, estimatedHz)
		timingstats.PrintStats("Frame intervals", s, s.Mean)
		printPacingStats(exp.Screen.PacingStats(), exp.Screen.FrameDuration())
		return control.EndLoop
	})
}

// printPacingStats reports which anchor the presents in this run used — whether
// SDL_RenderPresent blocked to the retrace on its own, or Update had to hold
// each frame to the boundary itself (apparatus.paceToFrame).
//
// This is here because the frame-interval statistics above cannot tell the two
// apart: pacing makes a non-blocking driver produce intervals just as regular as
// a blocking one, and on the software side that is the whole point. The
// difference only shows physically. A paced machine stamps its onsets with the
// schedule rather than with a hardware instant, so those timestamps slide
// against the panel at whatever the nominal refresh rate is wrong by — measured
// at 14 ms over 8 minutes on a Raspberry Pi 4, and only visible because a
// photodiode was watching. Printing the branch counts turns that into a number
// this test reports on its own.
//
// "Estimated refresh rate" above answers a related but different question: it
// comes from the paced intervals, so it reports the cadence the loop achieved,
// not whether the driver imposed it. CalibrateRefresh is the one that bypasses
// pacing.
func printPacingStats(p control.PacingStats, frameDur time.Duration) {
	total := p.Blocked + p.Paced
	fmt.Printf("\n── Frame pacing ───────────────────────────────\n")
	if total == 0 {
		fmt.Printf("  no paced presents (VSync off, or too few frames)\n")
		return
	}
	pacedPct := 100 * float64(p.Paced) / float64(total)
	fmt.Printf("  presents: %d\n", total)
	fmt.Printf("  blocked : %d (%.1f %%)  — SDL_RenderPresent returned at or after the frame boundary\n",
		p.Blocked, 100-pacedPct)
	fmt.Printf("  paced   : %d (%.1f %%)  — returned early; Update held the frame\n",
		p.Paced, pacedPct)
	if p.Paced > 0 {
		fmt.Printf("  wait    : mean %.3f ms  max %.3f ms   (frame = %.3f ms)\n",
			float64(p.WaitMean())/float64(time.Millisecond),
			float64(p.WaitMax)/float64(time.Millisecond),
			float64(frameDur)/float64(time.Millisecond))
	}
	switch {
	case p.Paced == 0:
		fmt.Printf("  verdict : the driver blocks. Flip timestamps carry the display's own\n")
		fmt.Printf("            instant, and cannot drift against the panel.\n")
	case pacedPct < 5:
		fmt.Printf("  verdict : the driver blocks, with occasional jitter around the boundary.\n")
		fmt.Printf("            Compare the mean wait against the frame: a few hundred µs is\n")
		fmt.Printf("            jitter, most of a frame is buffering that happened to be rare.\n")
	default:
		fmt.Printf("  verdict : the driver does NOT block — Update is pacing the loop. Onsets\n")
		fmt.Printf("            are stamped with the schedule, so they drift against the panel\n")
		fmt.Printf("            by however wrong the nominal refresh rate is. Check the\n")
		fmt.Printf("            estimated rate above against the nominal one, and read any\n")
		fmt.Printf("            photodiode onset on this machine as relative, not absolute.\n")
	}
}

// sleepUntil sleeps until the given absolute time, with sub-millisecond
// busy-spin for the last 500 µs to reduce overshoot on Linux.
func sleepUntil(t time.Time) {
	remaining := time.Until(t)
	if remaining > 500*time.Microsecond {
		time.Sleep(remaining - 500*time.Microsecond)
	}
	for time.Now().Before(t) {
		// busy-spin
	}
}

// ── Test: rt ──────────────────────────────────────────────────────────────────

// runRT measures keyboard reaction time using SDL3 event timestamps.
//
// Each trial: a white flash appears for one frame; the participant presses any
// key as fast as possible. RT is computed as event.Timestamp − onset_ns, where
// onset_ns is the SDL nanosecond tick returned by Screen.FlipTS() — the moment
// SDL_RenderPresent returned when the driver blocks to the retrace, and the
// scheduled frame boundary when it does not (see apparatus.paceToFrame).
//
// Because both timestamps come from the same SDL nanosecond clock (SDL_GetTicksNS),
// RT reflects the interval between actual display output and the hardware
// keyboard interrupt — without any polling latency on the response side.
//
// Use with a hardware response box connected as a USB keyboard for ground-truth
// RT validation. Compare against the photodiode onset (frames test) to obtain
// the full stimulus-onset → button-press chain in nanoseconds.
func runRT(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	nTrials := *fCycles
	meanItiMs := *fItiMs
	fmt.Printf("rt: %d trials  mean ITI %.0f ms  press any key each flash\n", nTrials, meanItiMs)

	exp.Data.WriteComment(fmt.Sprintf("test=rt cycles=%d iti-ms=%.0f", nTrials, meanItiMs))
	exp.AddDataVariableNames([]string{
		"trial",
		"onset_ns", "event_ts_ns", "rt_ns", "rt_ms",
	})

	var rtValues []float64 // milliseconds for statistics

	return exp.Run(func() error {
		instructions := stimuli.NewTextLine(
			"Press any key as fast as possible when the screen flashes white.",
			0, 50, control.White,
		)
		hint := stimuli.NewTextLine("(press SPACE to start)", 0, -50, control.Gray)
		exp.Screen.Clear()
		instructions.Draw(exp.Screen)
		hint.Draw(exp.Screen)
		exp.Screen.Update()
		exp.Keyboard.WaitKey(control.K_SPACE)

		paint, err := newPainter(exp)
		if err != nil {
			return err
		}

		restoreGC := suspendGC()
		defer restoreGC()

		for i := 0; i < nTrials; i++ {
			// Jittered ITI: meanItiMs ± 50 %
			jitter := (rand.Float64() - 0.5) * meanItiMs
			itiDur := time.Duration((meanItiMs + jitter) * float64(time.Millisecond))
			exp.Screen.Clear()
			exp.Screen.Update()
			time.Sleep(itiDur)

			// Flash: paint the stimulus and flip, capturing SDL nanosecond onset.
			paint(control.White, control.Black)
			onsetNS, _ := exp.Screen.FlipTS()

			// Trigger pulse after VSYNC: pixels are now on screen.
			_, isNull := trig.(triggers.NullOutputTTLDevice)
			if !isNull {
				go triggers.FireTrigger(trig, triggerLine(), time.Duration(*fTriggerMs)*time.Millisecond)
			}

			// Wait for keypress — returns SDL event timestamp (nanoseconds).
			_, eventTS, err := exp.Keyboard.GetKeyEventTS(nil, 5000)
			if control.IsEndLoop(err) {
				aborted = true
				return control.EndLoop
			}

			rtNS := int64(eventTS - onsetNS)
			rtMs := float64(rtNS) / 1e6
			rtValues = append(rtValues, rtMs)

			exp.Data.Add(i, onsetNS, eventTS, rtNS, fmt.Sprintf("%.3f", rtMs))
			fmt.Printf("trial %3d  RT = %.1f ms\n", i, rtMs)
		}

		timingstats.PrintStats("RT (ms, event-timestamp method)", timingstats.ComputeStats(rtValues, 0), 0)
		return control.EndLoop
	})
}

// ── Test: check ───────────────────────────────────────────────────────────────

// runCheck is a combined go/no-go sanity check for both display and audio.
// It shows a bright white screen for one second (verify you see a flash on the
// monitor), then plays a buzzer followed by a ping (verify you hear both
// sounds through your speakers or headphones).
// No data is recorded; this is a "does it basically work?" step before running
// any of the quantitative tests.
func runCheck(exp *control.Experiment) error {
	fmt.Println("check: verifying display and audio output — watch for a bright flash, then listen for two sounds")
	exp.Data.WriteComment("test=check")
	return exp.Run(func() error {
		// ── Step 1: bright flash on display ───────────────────────────────────
		label := stimuli.NewTextLine("DISPLAY CHECK — you should see this bright screen for ~1 second.", 0, 0, control.Black)
		r := exp.Screen.Renderer
		r.SetDrawColor(255, 255, 255, 255)
		r.Clear()
		label.Draw(exp.Screen)
		exp.Screen.Update()
		time.Sleep(1 * time.Second)

		// Brief return to dark so the transition is clearly visible.
		r.SetDrawColor(0, 0, 0, 255)
		r.Clear()
		exp.Screen.Update()
		time.Sleep(300 * time.Millisecond)

		// ── Step 2: buzzer ────────────────────────────────────────────────────
		msg1 := stimuli.NewTextLine("AUDIO CHECK — listen for a buzzer…", 0, 0, control.White)
		if err := exp.Show(msg1); err != nil {
			return err
		}
		fmt.Println("check: playing buzzer…")
		if err := stimuli.PlayBuzzer(exp.AudioDevice); err != nil {
			log.Printf("check: error playing buzzer: %v", err)
		}
		clock.Wait(1000)

		// ── Step 3: ping ──────────────────────────────────────────────────────
		msg2 := stimuli.NewTextLine("AUDIO CHECK — …then a ping.", 0, 0, control.White)
		if err := exp.Show(msg2); err != nil {
			return err
		}
		fmt.Println("check: playing ping…")
		if err := stimuli.PlayPing(exp.AudioDevice); err != nil {
			log.Printf("check: error playing ping: %v", err)
		}
		clock.Wait(1000)

		fmt.Println("check: done. Did you see the bright flash and hear both sounds? If yes, proceed to the measurement tests.")
		return control.EndLoop
	})
}

// ── Test: vrr ─────────────────────────────────────────────────────────────────

// runVRR characterises Variable Refresh Rate (VRR / FreeSync / G-Sync /
// Adaptive-Sync) stimulus timing by sweeping target durations from 1 ms to
// *fVRRMaxMs in 1 ms steps, with *fCycles repetitions per step.
//
// VSync is disabled for the duration of the test (restored on exit) so that
// every SDL_RenderPresent call returns immediately without blocking for a
// VSYNC edge. On a VRR-capable display the panel dynamically adjusts its
// refresh interval to match the time between consecutive Presents, allowing
// stimuli to be shown for durations that are NOT multiples of the nominal
// frame period (e.g. 1 ms, 7 ms, 17 ms, 23 ms).
//
// At each repetition:
//  1. A bright screen is presented; onsetNS = sdl.TicksNS() is captured
//     immediately after Present() returns.
//  2. A busy-wait loop (sub-millisecond precision) holds for the target duration.
//  3. A blank screen is presented; offsetNS = sdl.TicksNS() is captured.
//  4. actual_ms = (offsetNS − onsetNS) / 1e6.
//
// If a DLP-IO8-G is available, a trigger pulse is sent at each onset, allowing
// the software timestamps to be cross-validated against a photodiode on the
// oscilloscope.
//
// Interpreting the results:
//   - On a VRR display: duration errors should be small (< 0.5 ms) across the
//     entire sweep, confirming arbitrary-duration stimulus presentation works.
//   - On a non-VRR display: duration errors cluster at multiples of the frame
//     period (±half a frame); the test self-diagnoses the absence of VRR.
//   - VRR panels have a supported refresh range (e.g. 48–144 Hz = 6.9–20.8 ms).
//     Outside this range the panel reverts to fixed-rate behaviour: errors grow
//     sharply at the boundary durations, revealing the VRR window directly from
//     the CSV data.
//
// Note: onsetNS / offsetNS are captured right after Present() returns (GPU
// submission time), not at photon emission. The full software-to-photon latency
// is a constant that can be measured independently with the frames test + a
// photodiode. Because this latency is constant, duration accuracy is not
// affected by it.
func runVRR(exp *control.Experiment, trig triggers.OutputTTLDevice) error {
	maxMs := *fVRRMaxMs
	reps := *fVRRReps
	_, isNull := trig.(triggers.NullOutputTTLDevice)
	trigDur := time.Duration(*fTriggerMs) * time.Millisecond

	fmt.Printf("vrr: sweep 1–%d ms in 1 ms steps  reps=%d  level-a=%d  level-b=%d",
		maxMs, reps, *fLevelA, *fLevelB)
	if !isNull {
		fmt.Printf("  trigger pin %d (%d ms pulse)", *fTriggerPin, *fTriggerMs)
	}
	fmt.Println()
	fmt.Println("vrr: disabling VSync — use a VRR-capable monitor for meaningful sub-frame durations")

	if err := exp.Screen.SetVSync(0); err != nil {
		return fmt.Errorf("vrr: could not disable VSync: %w", err)
	}
	defer func() {
		_ = exp.Screen.SetVSync(1)
		fmt.Println("vrr: VSync re-enabled")
	}()

	// Let the driver settle after the vsync change.
	time.Sleep(100 * time.Millisecond)

	exp.Data.WriteComment(fmt.Sprintf(
		"test=vrr vrr-max-ms=%d cycles=%d level-a=%d level-b=%d",
		maxMs, reps, *fLevelA, *fLevelB))
	exp.AddDataVariableNames([]string{
		"target_ms", "rep",
		"actual_ms", "duration_error_ms",
		"onset_ns", "offset_ns",
		"trigger",
	})

	return exp.Run(func() error {
		status := stimuli.NewTextLine(
			fmt.Sprintf("VRR sweep: 1–%d ms, %d reps — press ESC to stop", maxMs, reps),
			0, 0, control.White)
		if err := exp.Show(status); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)

		restoreGC := suspendGC()
		defer restoreGC()

		paint, err := newPainter(exp)
		if err != nil {
			return err
		}
		bright, dark := gray(byte(*fLevelB)), gray(byte(*fLevelA))

		var allErrors []float64

		for targetMs := 1; targetMs <= maxMs; targetMs++ {
			targetDur := time.Duration(targetMs) * time.Millisecond
			var durationErrors []float64

			for rep := 0; rep < reps; rep++ {
				// ── ISI: blank screen ────────────────────────────────────────
				paint(dark, dark)
				exp.Screen.Flip() // non-blocking with vsync=0
				time.Sleep(200 * time.Millisecond)

				// ── Onset: bright screen ─────────────────────────────────────
				if !isNull {
					_ = trig.SetHigh(triggerLine())
				}
				paint(bright, dark)
				onsetNS, _ := exp.Screen.FlipTS() // returns immediately (vsync=0)

				// ── Hold for exactly targetDur using busy-wait ────────────────
				sleepUntil(time.Now().Add(targetDur))

				// ── Offset: blank screen ─────────────────────────────────────
				paint(dark, dark)
				offsetNS, _ := exp.Screen.FlipTS()

				if !isNull {
					go func() {
						time.Sleep(trigDur)
						// triggerLine(), not *fTriggerPin: the SetHigh above
						// uses the 0-indexed line, so passing the 1-indexed pin
						// here dropped the NEIGHBOURING line and left the
						// triggered one HIGH for the rest of the sweep — one
						// edge at the first onset and nothing after it.
						_ = trig.SetLow(triggerLine())
					}()
				}

				// ── Log ───────────────────────────────────────────────────────
				actualMs := float64(offsetNS-onsetNS) / 1e6
				durationError := actualMs - float64(targetMs)
				durationErrors = append(durationErrors, durationError)

				exp.Data.Add(
					targetMs, rep,
					fmt.Sprintf("%.3f", actualMs),
					fmt.Sprintf("%.3f", durationError),
					onsetNS, offsetNS,
					!isNull,
				)
				fmt.Printf("  %3d ms  rep %2d:  actual=%6.3f ms  error=%+6.3f ms\n",
					targetMs, rep, actualMs, durationError)

				state := exp.PollEvents(nil)
				if state.QuitRequested {
					aborted = true
					return control.EndLoop
				}
			}

			s := timingstats.ComputeStats(durationErrors, 0)
			fmt.Printf("── %3d ms: mean=%+.3f ms  SD=%.3f ms\n", targetMs, s.Mean, s.SD)
			allErrors = append(allErrors, durationErrors...)
		}

		timingstats.PrintStats("Duration error across the whole sweep (ms)",
			timingstats.ComputeStats(allErrors, 0), 0)
		return control.EndLoop
	})
}

// ── Test: drain ───────────────────────────────────────────────────────────────

// runDrain measures audio pipeline latency without any external equipment.
//
// For each tone duration in a fixed set (25, 50, 100, 200, 500 ms) it repeats
// *fDrainReps trials.  Each trial:
//  1. Calls tone.Play() (which queues PCM data into the SDL audio stream).
//  2. Polls stream.Queued() in a tight loop until the device has consumed all
//     queued bytes (Queued returns 0).
//  3. Records drain_ms = elapsed wall-clock time from Play() to drain complete.
//
// The audio pipeline latency is drain_ms − nominal_ms.  It reflects the
// hardware-buffer delay between PutData() and the last sample exiting the DAC.
// The SD of drain_ms across reps captures trial-to-trial jitter in the audio
// scheduler — without needing a microphone or oscilloscope.
func runDrain(exp *control.Experiment) error {
	durations := []int{25, 50, 100, 200, 500} // nominal tone durations in ms
	reps := *fDrainReps
	freqHz := *fFreqHz

	fmt.Printf("drain: freq=%.0f Hz  reps=%d  durations=%v ms\n", freqHz, reps, durations)
	exp.Data.WriteComment(fmt.Sprintf("test=drain freq-hz=%.0f reps=%d durations_ms=%v",
		freqHz, reps, durations))
	exp.AddDataVariableNames([]string{
		"duration_ms", "rep", "drain_ms", "overhead_ms",
	})

	return exp.Run(func() error {
		status := stimuli.NewTextLine(
			fmt.Sprintf("Audio drain test: %.0f Hz tone, %d reps — please wait…", freqHz, reps),
			0, 0, control.White)
		if err := exp.Show(status); err != nil {
			return err
		}

		restoreGC := suspendGC()
		defer restoreGC()

		for _, durMs := range durations {
			tone := stimuli.NewTone(freqHz, durMs, 0.8)
			if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
				return fmt.Errorf("drain: preload tone %d ms: %w", durMs, err)
			}

			var drainVals []float64
			for rep := 0; rep < reps; rep++ {
				// Brief silence between reps so stream is fully empty before Play().
				time.Sleep(50 * time.Millisecond)

				tPlay := time.Now()
				_ = tone.Play()

				// Spin-poll until the device has consumed all queued bytes.
				for {
					queued, err := tone.Stream.Queued()
					if err != nil || queued <= 0 {
						break
					}
					time.Sleep(500 * time.Microsecond)
				}
				drainMs := float64(time.Since(tPlay).Nanoseconds()) / 1e6
				overheadMs := drainMs - float64(durMs)
				drainVals = append(drainVals, drainMs)

				exp.Data.Add(
					durMs, rep,
					fmt.Sprintf("%.3f", drainMs),
					fmt.Sprintf("%.3f", overheadMs),
				)
				fmt.Printf("  %3d ms  rep %2d:  drain=%.1f ms  overhead=%+.1f ms\n",
					durMs, rep, drainMs, overheadMs)

				state := exp.PollEvents(nil)
				if state.QuitRequested {
					aborted = true
					tone.Unload()
					return control.EndLoop
				}
			}

			tone.Unload()
			// Report drain_ms statistics with nominal duration as the target.
			// mean − target = audio pipeline latency; SD = drain-time jitter.
			s := timingstats.ComputeStats(drainVals, float64(durMs))
			fmt.Printf("\n")
			timingstats.PrintStats(fmt.Sprintf("Drain time for %d ms tone (latency = mean − target)", durMs),
				s, float64(durMs))
			fmt.Printf("  pipeline latency ≈ %.1f ms\n", s.Mean-float64(durMs))
		}

		return control.EndLoop
	})
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	// Parse flags early so we can act on -audio-frames before SDL opens the
	// audio device inside NewExperimentFromFlags. flag.Parse() is idempotent;
	// NewExperimentFromFlags will call it again harmlessly.
	flag.Parse()
	checkTriggerFlags()
	if *fSysInfo {
		sysinfo.Collect().Print()
		// SDL's own view, which sysinfo cannot supply: the display indices -d
		// accepts, and the audio devices that can actually be opened.
		control.PrintDevices()
		return
	}
	if *fAudioFrames > 0 {
		control.SetAudioSampleFrames(*fAudioFrames)
		fmt.Printf("audio: requesting %d sample frames hardware buffer\n", *fAudioFrames)
	}

	width, height, fullscreen := 0, 0, true
	if *fWindowed {
		width, height, fullscreen = 1024, 768, false
	}
	// Must precede Initialize: the policy is applied when the window is created.
	policy, err := control.ParseFullscreenPolicy(*fExclusiveFS)
	if err != nil {
		log.Fatalf("-exclusive-fullscreen: %v", err)
	}
	control.SetFullscreenPolicy(policy)

	exp := control.NewExperiment("Timing-Tests", width, height, fullscreen, control.Black, control.White, 24)
	if *fDisplay >= 0 {
		exp.ScreenNumber = *fDisplay
	}
	// Must precede Initialize: that is where the data file is created.
	if *fOutDir != "" {
		exp.SetOutputDirectory(*fOutDir)
	}
	if err := exp.Initialize(); err != nil {
		exp.End() // release any SDL subsystems already initialised before exiting
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Handle Ctrl-C (SIGINT) and SIGTERM so the process exits cleanly.
	// Only save data here — do NOT call exp.End() (which calls sdl.Quit via
	// CGo) from this goroutine while the main goroutine may be inside an SDL
	// CGo call.  Concurrent SDL access from two OS threads causes a SIGSEGV.
	// os.Exit skips deferred functions, so SDL is never touched from here.
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		if exp.Data != nil {
			exp.Data.WriteEndTime()
			if err := exp.Data.Save(); err == nil {
				log.Printf("Results saved in %s", exp.Data.FullPath)
			}
		}
		os.Exit(0)
	}()

	// Log actual audio device format so the user can verify the buffer size.
	if spec, frames, err := exp.AudioDevice.Format(); err == nil {
		fmt.Printf("audio: %d Hz  %d ch  %d sample frames (~%.1f ms latency)\n",
			spec.Freq, spec.Channels, frames,
			float64(frames)/float64(spec.Freq)*1000)
	}

	// Record the collector state in both the console report and the data-file
	// header, so GC-on and GC-off runs cannot be confused during analysis.
	fmt.Printf("gc: %s during timing-critical loops\n", gcLabel())
	if exp.Data != nil {
		exp.Data.WriteComment("gc=" + gcLabel())
	}

	trig, trigDesc := setupTrigger()
	defer trig.Close()
	if exp.Data != nil {
		if trigDesc == "" {
			trigDesc = "none"
		}
		exp.Data.WriteComment("trigger=" + trigDesc)
	}

	var runErr error
	switch *fTest {
	// ── No hardware required ─────────────────────────────────────────────────
	case "check":
		runErr = runCheck(exp)
	case "display":
		runErr = runJitter(exp)
	case "latency":
		runErr = runDrain(exp)
	// ── Photodiode / trigger box required ────────────────────────────────────
	case "av":
		runErr = runAV(exp, trig)
	case "vrr":
		runErr = runVRR(exp, trig)
	case "rt":
		runErr = runRT(exp, trig)
	default:
		exp.End() // release any SDL subsystems already initialised before exiting
		log.Fatalf("unknown test %q — choose from: av vrr rt check display latency\n"+
			"  (gvsync moved to tests/test_gv_sync; trigger moved to tests/test_dlpio8)", *fTest)
	}

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("test error: %v", runErr)
	}

	// A run cut short with Esc measured less than was asked for, so it must not
	// report success — see the comment on `aborted`. exp.End() is called
	// explicitly because os.Exit skips deferred functions, and it is what
	// finalises the data file: whatever was collected before the abort is still
	// written and still available to look at.
	if aborted {
		fmt.Fprintln(os.Stderr, "\nAborted with Esc — this run is incomplete and reports failure.")
		exp.End()
		os.Exit(1)
	}
}

// triggerLine converts the -trigger-pin flag to the line index the
// OutputTTLDevice API expects.
//
// The flag is a pin number as printed on the DLP-IO8 terminal block, 1-8; the
// API takes a 0-indexed line, so line 0 drives pin 1. Passing the flag straight
// through -- which this test did -- fired the NEIGHBOURING pin: the default
// -trigger-pin 1 drove pin 2, and -trigger-pin 8 was out of range and did
// nothing at all. Verified on hardware with an Analog Discovery on pin 1: no
// signal at the default, a clean 5.05 V square wave at -trigger-pin 0.
func triggerLine() int { return *fTriggerPin - 1 }

// checkTriggerFlags rejects an unusable trigger configuration before a run
// starts — that is, before Initialize opens a fullscreen window and, under
// BBTK_CAPTURE, before a capture window has been spent. Every check here is one
// that would otherwise surface from setupTrigger, which runs too late to be
// anything but a black screen and a log line.
func checkTriggerFlags() {
	if *fTriggerPin < 1 || *fTriggerPin > 8 {
		log.Fatalf("-trigger-pin %d is out of range: every device exposes 8 lines, "+
			"numbered 1 to 8", *fTriggerPin)
	}
	switch *fTrigDevice {
	case "dlpio8", "parallel", "ft232h":
	case "gpio":
		if _, err := parsePins(*fGPIOPins); err != nil {
			log.Fatalf("-gpio-pins %q: %v", *fGPIOPins, err)
		}
	case "labjackt4":
		// The T4 is reached over the network and has nothing to auto-detect, so
		// an omitted address is a certain failure — catch it here rather than
		// from setupTrigger, which runs after the window has opened.
		if *fLJHost == "" {
			log.Fatalf("-trigger-device labjackt4 needs -labjack-host, e.g. " +
				"-labjack-host 192.168.1.100")
		}
	default:
		log.Fatalf("-trigger-device %q: %s", *fTrigDevice, trigDeviceChoices)
	}
}
