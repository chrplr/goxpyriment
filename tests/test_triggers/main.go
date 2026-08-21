// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_triggers fires the same pulse train on two or more TTL output devices at
// once, so an oscilloscope can measure how far apart their edges actually land.
//
// # What this measures, and what it does not
//
// Every trigger device in the triggers package has been characterised alone.
// Nothing put two of them on one timebase, so "a parallel-port write is an
// ioctl, a DLP-IO8 write crosses USB, a LabJack T4 write crosses the network —
// each step adds latency" rested on runs that were never compared. This program
// removes the comparison problem by removing the two runs: the devices are
// pulsed on one absolute schedule, and the instrument reads the difference.
//
// The number this produces is device-to-device *relative* latency and jitter.
// The absolute host→wire latency of a single device needs an instrument that
// also sees the host clock — that is test_photodiode_latency and Timing-Tests.
//
// The program also records, per pulse, when the host issued each write. That is
// how a skew seen on the scope is attributed: a large scope skew with a small
// host skew is the device; a large host skew is this program (or the OS) having
// issued the writes late. Host-issue skew is not wire skew, and the summary
// says so.
//
// # Usage
//
//	# parallel port against a DLP-IO8, 40 pulses every 500 ms, 5 ms wide
//	go run ./tests/test_triggers \
//	    -device parallel:pin=1 -device dlpio8:port=/dev/ttyUSB0,pin=1
//
//	# what naive experiment code does: both writes from one thread, in order
//	go run ./tests/test_triggers -sequential \
//	    -device parallel:pin=1 -device dlpio8:port=/dev/ttyUSB0,pin=1
//
//	# no scope: let a MEG TTL box timestamp the other devices' edges itself
//	go run ./tests/test_triggers -loopback 3 \
//	    -device parallel:pin=1 -device gpio:pin=1 -device megttlbox:port=/dev/ttyACM0
//
// See README.md for wiring, and trigdev.Usage (printed by -h) for the -device
// syntax.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chrplr/goxpyriment/results"
	"github.com/chrplr/goxpyriment/sysinfo"
	"github.com/chrplr/goxpyriment/tests/internal/report"
	"github.com/chrplr/goxpyriment/tests/internal/safeexit"
	"github.com/chrplr/goxpyriment/tests/internal/trigdev"
	"github.com/chrplr/goxpyriment/triggers"
)

// spinSlack is how much of each wait is spent busy-spinning rather than
// sleeping. time.Sleep overshoots by milliseconds on a loaded system, and an
// overshoot here is indistinguishable from device latency in the trace.
const spinSlack = 1500 * time.Microsecond

// leadIn is the delay between the firing threads being ready and the first
// pulse. It gives every thread time to be scheduled at least once before it
// matters, and gives the operator time to see the trace start.
const leadIn = 1 * time.Second

var (
	fDevices  trigdev.Flags
	fN        = flag.Int("n", 40, "Recorded pulses")
	fISIMs    = flag.Float64("isi-ms", 500, "Interval between pulse onsets, in ms")
	fWidthMs  = flag.Float64("width-ms", 5, "Pulse width, in ms")
	fWarmup   = flag.Int("warmup", 5, "Pulses fired before the recorded block.\n\tThey are written to the data file with phase=warmup and left out of the\n\tstatistics: the first write to a USB device wakes a driver and is\n\tmeasurably different from the rest.")
	fSeq      = flag.Bool("sequential", false, "Fire every device from ONE thread, in -device order, instead of one\n\tthread per device waiting on a common deadline. This is what experiment\n\tcode that pulses two boxes in a row does, and it measures what the first\n\tdevice's blocking write costs the one queued behind it.")
	fLoopback = flag.Int("loopback", 0, "Use the Nth -device (1-based) as a timestamping witness instead of a\n\tsource: it fires nothing, and its input lines record the other devices'\n\tedges on its own microsecond clock. Only a MEG TTL box with\n\tCAP_TIMESTAMPS can do this; every other input path polls at ~5 ms.\n\t0 = no witness.")
	fLoopMap  = flag.String("loopback-map", "", "Which witness input line each source is wired to, comma-separated, 1-8,\n\tin -device order with the witness skipped (default 1,2,3,...).")
	fPrio     = flag.Int("realtime-priority", 50, "SCHED_FIFO priority requested for each firing thread (1-99), or 0 to\n\tdecline. Needs privileges; a failure is reported and the run continues.")
	fGC       = flag.Bool("gc", false, "Leave the garbage collector RUNNING during the pulse train.\n\tBy default it is suspended; pass -gc to measure its effect.")
	fSubj     = flag.Int("s", 999, "Subject id used in the data-file name")
	fOutDir   = flag.String("outdir", "", "Directory for the .csv/-info.txt results (default: $HOME/goxpy_data)")
	fNoPrompt = flag.Bool("no-prompt", false, "Do not wait for Enter after printing the wiring table")
)

func init() {
	flag.Var(&fDevices, "device", "TTL output device to pulse. Repeat it: at least two devices are needed\n\tfor a comparison.\n"+trimBlank(trigdev.Usage))
}

// pulse is one device's record of one repetition. Every field is a host-clock
// instant relative to t0, so the data file needs no absolute times to be read.
type pulse struct {
	rep      int
	warmup   bool
	target   time.Duration
	preHigh  time.Duration
	postHigh time.Duration
	preLow   time.Duration
	postLow  time.Duration
	errHigh  string
	errLow   string

	// Filled in by the -loopback witness, when there is one.
	witRise, witFall time.Duration
	hasRise, hasFall bool
}

// device is an opened trigger device plus the rows it produced.
type device struct {
	trigdev.Opened
	index   int // position in -device order, 1-based, for messages
	witLine int // 0-indexed witness input line this device is wired to
	rows    []pulse
}

func main() {
	log.SetFlags(0)
	flag.Parse()
	checkFlags()

	isi := time.Duration(*fISIMs * float64(time.Millisecond))
	width := time.Duration(*fWidthMs * float64(time.Millisecond))
	total := *fWarmup + *fN

	devices := openAll()
	defer closeAll(devices)
	closeOnSignal(devices)

	witness, sources := splitWitness(devices)
	assignWitnessLines(sources)

	printWiring(devices, witness, isi, total)
	confirm()

	df, err := results.NewDataFile(*fOutDir, *fSubj, "test_triggers")
	if err != nil {
		log.Fatalf("creating the data file: %v", err)
	}
	writeHeader(df, devices, witness, isi, width)

	out := &report.Tee{}
	// The deferred call is the safety net for a panic; the explicit one below
	// is what actually files the report. Flush appends to the info file's
	// buffer, so it has to happen BEFORE Finalize writes that buffer out —
	// deferring it alone put the whole report into a file already on disk, and
	// the run of 2026-08-21 kept its summary only in the terminal.
	defer out.Flush(df, "test_triggers report")

	edges, dropped := run(sources, witness, isi, width, total)
	if witness != nil {
		attribute(sources, edges, isi, total)
	}

	writeRows(df, sources, witness != nil)
	summarise(out, sources, witness, dropped)

	out.Flush(df, "test_triggers report")
	if err := df.Finalize(); err != nil {
		log.Fatalf("closing the data file: %v", err)
	}
	out.Printf("\nData file: %s\n", df.OutputFile.FullPath)
}

// ── flags ─────────────────────────────────────────────────────────────────────

func checkFlags() {
	if len(fDevices) == 0 {
		fmt.Fprintf(os.Stderr, "test_triggers: no -device given.\n\n%s\n\n", trigdev.Usage)
		fmt.Fprintf(os.Stderr, "Example:\n  go run ./tests/test_triggers "+
			"-device parallel:pin=1 -device dlpio8:port=/dev/ttyUSB0,pin=1\n")
		os.Exit(2)
	}
	if *fN < 1 {
		log.Fatalf("-n %d: at least one pulse is needed", *fN)
	}
	if *fWarmup < 0 {
		log.Fatalf("-warmup %d is negative", *fWarmup)
	}
	if *fWidthMs <= 0 {
		log.Fatalf("-width-ms %g: the pulse must have a width", *fWidthMs)
	}
	// A pulse that has not finished before the next one starts would leave the
	// line HIGH across the interval, and the trace would show one long pulse
	// rather than the train being measured.
	if *fISIMs <= *fWidthMs {
		log.Fatalf("-isi-ms %g must be greater than -width-ms %g, or the pulses run together",
			*fISIMs, *fWidthMs)
	}
	if *fPrio < 0 || *fPrio > 99 {
		log.Fatalf("-realtime-priority %d is outside the SCHED_FIFO range 1-99 (0 declines)", *fPrio)
	}
	if *fLoopback < 0 || *fLoopback > len(fDevices) {
		log.Fatalf("-loopback %d: there are %d devices", *fLoopback, len(fDevices))
	}
	if *fLoopback > 0 && len(fDevices) < 2 {
		log.Fatalf("-loopback %d leaves no device to fire: give at least one source besides the witness", *fLoopback)
	}
	if len(fDevices) == 1 && *fLoopback == 0 {
		log.Printf("warning: only one -device given, so there is nothing to compare it with.")
		log.Printf("         (a single-device run is still useful to label a scope channel)")
	}
}

// ── opening ───────────────────────────────────────────────────────────────────

func openAll() []*device {
	devices := make([]*device, 0, len(fDevices))
	for i, spec := range fDevices {
		opened, err := trigdev.Open(spec)
		if err != nil {
			// Fatal, not a fallback to NullOutputTTLDevice: a device that is
			// not there puts a flat trace on a scope channel, and the loss is
			// only discovered when the capture is read.
			closeAll(devices)
			log.Fatalf("device %d (%s): %v", i+1, spec.Raw, err)
		}
		devices = append(devices, &device{Opened: opened, index: i + 1})
	}
	// Start from a known state: a line left HIGH by an earlier run would make
	// the first pulse invisible.
	for _, d := range devices {
		if err := d.Device.AllLow(); err != nil {
			log.Printf("warning: %s: driving all lines low: %v", d.Label, err)
		}
	}
	return devices
}

func closeAll(devices []*device) {
	for _, d := range devices {
		if err := d.Close(); err != nil {
			log.Printf("warning: closing %s: %v", d.Label, err)
		}
	}
}

// closeOnSignal drives the lines LOW on Ctrl-C. Deferred functions do not run
// on a signal, and a device left HIGH keeps driving a recording input.
func closeOnSignal(devices []*device) {
	// Bounded, because closeAll writes to every device: a USB box that has
	// stopped answering would otherwise block the handler, and with the signal
	// caught there would be nothing left that could stop the run.
	safeexit.OnSignal(0, func() { closeAll(devices) })
}

// splitWitness separates the -loopback device, if any, from the sources.
func splitWitness(devices []*device) (witness *device, sources []*device) {
	for _, d := range devices {
		if *fLoopback > 0 && d.index == *fLoopback {
			witness = d
			continue
		}
		sources = append(sources, d)
	}
	if witness == nil {
		return nil, sources
	}
	box, ok := witness.Device.(*triggers.MEGTTLBox)
	if !ok {
		log.Fatalf("-loopback %d names a %s. Only a MEG TTL box can witness edges: "+
			"every other input path in this package polls at ~5 ms plus USB jitter, "+
			"which cannot resolve the skew being measured.", *fLoopback, witness.Spec.Kind)
	}
	if !box.Info().Has(triggers.MEGCapTimestamps) {
		log.Fatalf("-loopback %d: the MEG TTL box on this port reports %q, without CAP_TIMESTAMPS. "+
			"Its host-polled input cannot resolve the skew being measured; reflash the firmware "+
			"or drop -loopback and read the edges on a scope.", *fLoopback, box.Info())
	}
	return witness, sources
}

// assignWitnessLines maps each source to a witness input line: source i to line
// i by default, or to the line named by -loopback-map.
func assignWitnessLines(sources []*device) {
	for i, d := range sources {
		d.witLine = i
	}
	if *fLoopMap == "" {
		return
	}
	parts := strings.Split(*fLoopMap, ",")
	if len(parts) != len(sources) {
		log.Fatalf("-loopback-map %q lists %d lines for %d sources", *fLoopMap, len(parts), len(sources))
	}
	seen := map[int]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			log.Fatalf("-loopback-map %q: %q is not a number", *fLoopMap, p)
		}
		if n < 1 || n > 8 {
			log.Fatalf("-loopback-map %q: line %d is out of range (1-8)", *fLoopMap, n)
		}
		if first, dup := seen[n]; dup {
			log.Fatalf("-loopback-map %q: line %d given twice (sources %d and %d)", *fLoopMap, n, first, i+1)
		}
		seen[n] = i + 1
		sources[i].witLine = n - 1
	}
}

// ── wiring table ──────────────────────────────────────────────────────────────

func printWiring(devices []*device, witness *device, isi time.Duration, total int) {
	mode := "one locked thread per device, common deadline"
	if *fSeq {
		mode = "SEQUENTIAL: one thread, writes issued in -device order"
	}
	fmt.Printf("\ntest_triggers — %d device(s), %s\n", len(devices), mode)
	fmt.Printf("  %d pulses (%d warm-up + %d recorded), %.3f ms wide, every %.3f ms → %s of signal\n",
		total, *fWarmup, *fN, *fWidthMs, *fISIMs, (time.Duration(total) * isi).Round(time.Second))
	gcState := "suspended during the train"
	if *fGC {
		gcState = "RUNNING during the train (-gc)"
	}
	fmt.Printf("  realtime priority: %s;  GC: %s\n", prioText(), gcState)

	for _, d := range devices {
		role := fmt.Sprintf("source, pin %d", d.Spec.Pin)
		if d == witness {
			role = "WITNESS — fires nothing, timestamps the others"
		}
		fmt.Printf("\n  [%d] %-22s %s\n", d.index, d.Label, role)
		for _, n := range d.Notes {
			fmt.Printf("      · %s\n", n)
		}
		if d != witness && witness != nil {
			fmt.Printf("      · loopback: wire this output to the witness input line %d (Mega D%d)\n",
				d.witLine+1, 22+d.witLine)
		}
	}
	fmt.Printf("\n  Grounds must be common — between the devices, and with the instrument.\n")
	if witness != nil {
		fmt.Printf("  Witness inputs are INPUT_PULLUP and reported inverted: an idle-LOW source\n")
		fmt.Printf("  reads as 'pressed', and a RISING edge appears as a bit CLEARING.\n")
	}
}

func prioText() string {
	if *fPrio == 0 {
		return "not requested (-realtime-priority 0)"
	}
	return fmt.Sprintf("SCHED_FIFO %d requested", *fPrio)
}

func confirm() {
	if *fNoPrompt {
		return
	}
	fmt.Printf("\nCheck the probes and the grounds, arm the instrument, then press Enter to start\n")
	fmt.Printf("(Ctrl-C aborts and drives every line low). ")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// ── the pulse train ───────────────────────────────────────────────────────────

// edge is one transition seen by the witness, on its own clock translated to
// the host clock.
type edge struct {
	ts     time.Time
	line   int
	rising bool
}

// run fires the train and returns the witness's edges, if there is a witness.
func run(sources []*device, witness *device, isi, width time.Duration, total int) ([]edge, bool) {
	for _, d := range sources {
		d.rows = make([]pulse, total) // preallocated: the loop must not allocate
	}

	if !*fGC {
		// Same convention as the VSYNC-locked loops in stimuli/: a collection
		// landing between the SetHigh calls would be recorded as device skew.
		defer debug.SetGCPercent(debug.SetGCPercent(-1))
	}

	var box *triggers.MEGTTLBox
	var prevMask byte
	if witness != nil {
		box = witness.Device.(*triggers.MEGTTLBox)
		if err := box.SetDebounce(0); err != nil {
			log.Printf("warning: witness SetDebounce(0): %v", err)
		}
		if err := box.DrainEvents(); err != nil {
			log.Printf("warning: witness DrainEvents: %v", err)
		}
		m, err := box.ReadAll()
		if err != nil {
			log.Fatalf("witness: reading the idle input state: %v", err)
		}
		prevMask = m
	}

	var (
		ready, done sync.WaitGroup
		release     = make(chan struct{})
		t0          time.Time
		edges       []edge
		stopWitness = make(chan struct{})
		witnessDone sync.WaitGroup
	)

	if *fSeq {
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			lockAndRaise("firing thread")
			ready.Done()
			<-release
			for k := 0; k < total; k++ {
				target := t0.Add(time.Duration(k) * isi)
				spinUntil(target)
				for _, d := range sources {
					fireHigh(d, k, target, t0)
				}
				spinUntil(target.Add(width))
				for _, d := range sources {
					fireLow(d, k, t0)
				}
			}
		}()
	} else {
		for _, d := range sources {
			ready.Add(1)
			done.Add(1)
			go func(d *device) {
				defer done.Done()
				lockAndRaise(d.Label)
				ready.Done()
				<-release
				// Threads share nothing but the deadline. Waking them through a
				// channel every repetition would re-serialise exactly what is
				// being measured.
				for k := 0; k < total; k++ {
					target := t0.Add(time.Duration(k) * isi)
					spinUntil(target)
					fireHigh(d, k, target, t0)
					spinUntil(target.Add(width))
					fireLow(d, k, t0)
				}
			}(d)
		}
	}

	ready.Wait()
	t0 = time.Now().Add(leadIn)
	absT0 = t0

	if box != nil {
		witnessDone.Add(1)
		go func() {
			defer witnessDone.Done()
			edges = collectEdges(box, prevMask, stopWitness)
		}()
	}

	fmt.Printf("\nFiring %d pulses …\n", total)
	close(release) // t0 is written before this, so every thread sees it set
	done.Wait()

	dropped := false
	if box != nil {
		// The last falling edge may still be in flight.
		time.Sleep(200 * time.Millisecond)
		close(stopWitness)
		witnessDone.Wait()
		dropped = box.EventsDropped()
	}
	fmt.Printf("Done.\n")
	return edges, dropped
}

// fireHigh raises one line and records both sides of the write. Errors are
// recorded and the train continues: aborting mid-capture wastes the recording
// as surely as the failure does, and the summary counts them.
func fireHigh(d *device, k int, target, t0 time.Time) {
	row := &d.rows[k]
	row.rep = k - *fWarmup
	row.warmup = k < *fWarmup
	row.target = target.Sub(t0)
	row.preHigh = time.Since(t0)
	err := d.Device.SetHigh(d.Line)
	row.postHigh = time.Since(t0)
	if err != nil {
		row.errHigh = err.Error()
	}
}

func fireLow(d *device, k int, t0 time.Time) {
	row := &d.rows[k]
	row.preLow = time.Since(t0)
	err := d.Device.SetLow(d.Line)
	row.postLow = time.Since(t0)
	if err != nil {
		row.errLow = err.Error()
	}
}

// lockAndRaise pins the goroutine to its thread and asks for real-time
// scheduling. RaiseToRealTime applies to the calling thread, so each firing
// goroutine has to ask for itself.
func lockAndRaise(what string) {
	runtime.LockOSThread()
	if *fPrio > 0 {
		if err := sysinfo.RaiseToRealTime(*fPrio); err != nil {
			log.Printf("warning: %s: real-time priority %d refused: %v", what, *fPrio, err)
		}
	}
}

// spinUntil sleeps most of the way to t, then busy-spins the last spinSlack.
func spinUntil(t time.Time) {
	if d := time.Until(t); d > spinSlack {
		time.Sleep(d - spinSlack)
	}
	for time.Now().Before(t) {
		// busy-spin
	}
}

// collectEdges drains the witness's event queue until stop is closed, turning
// each mask change into one edge per changed line.
//
// The polarity is inverted on purpose, not by accident: the box's inputs are
// INPUT_PULLUP and reported with invert=true, so a bit SET means the line is
// LOW. A source's rising edge therefore clears a bit.
func collectEdges(box *triggers.MEGTTLBox, prev byte, stop <-chan struct{}) []edge {
	edges := make([]edge, 0, 256)
	for {
		select {
		case <-stop:
			return edges
		default:
		}
		ev, ok, err := box.PollEvent()
		if err != nil {
			log.Printf("warning: witness PollEvent: %v", err)
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if !ok {
			time.Sleep(500 * time.Microsecond)
			continue
		}
		changed := prev ^ ev.Mask
		prev = ev.Mask
		for line := 0; line < 8; line++ {
			bit := byte(1) << uint(line)
			if changed&bit == 0 {
				continue
			}
			edges = append(edges, edge{ts: ev.TS, line: line, rising: ev.Mask&bit == 0})
		}
	}
}

// ── loopback attribution ──────────────────────────────────────────────────────

// attribute assigns each witnessed edge to the repetition whose scheduled onset
// it is closest to, and to the source wired to that input line.
func attribute(sources []*device, edges []edge, isi time.Duration, total int) {
	byLine := make(map[int]*device, len(sources))
	for _, d := range sources {
		byLine[d.witLine] = d
	}
	var unmatched, duplicate int
	for _, e := range edges {
		d, ok := byLine[e.line]
		if !ok {
			unmatched++ // an input line nothing was wired to, or a wrong -loopback-map
			continue
		}
		// e.ts is on the host clock; d.rows[k].target is relative to t0, and
		// so is every other time here. rows[k].preHigh is the host's own record
		// of the same repetition, which is what anchors the two.
		// e.ts is an absolute host instant; every other time in a row is
		// measured from t0, so put it on the same axis before storing it.
		rel := e.ts.Sub(absT0)
		k := nearestRep(rel, d, isi, total)
		if k < 0 {
			unmatched++
			continue
		}
		row := &d.rows[k]
		switch {
		case e.rising && !row.hasRise:
			row.witRise, row.hasRise = rel, true
		case !e.rising && !row.hasFall:
			row.witFall, row.hasFall = rel, true
		default:
			duplicate++
		}
	}
	if unmatched > 0 {
		log.Printf("warning: %d witnessed edge(s) could not be matched to a repetition "+
			"(check -loopback-map and the wiring)", unmatched)
	}
	if duplicate > 0 {
		log.Printf("warning: %d extra edge(s) on repetitions that already had one "+
			"(contact bounce, or a line shared by two sources)", duplicate)
	}
}

// absT0 is the run's t0. Witness timestamps are absolute host instants and
// every recorded time is relative to t0, so one of the two has to be moved onto
// the other's axis; this is what does it.
var absT0 time.Time

// nearestRep returns the repetition whose scheduled onset is closest to rel (a
// time measured from t0), or -1 if it is more than half an interval away from
// every one of them.
func nearestRep(rel time.Duration, d *device, isi time.Duration, total int) int {
	k := int(math.Round(float64(rel) / float64(isi)))
	if k < 0 || k >= total {
		return -1
	}
	if abs(rel-d.rows[k].target) > isi/2 {
		return -1
	}
	return k
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// ── data file ─────────────────────────────────────────────────────────────────

func writeHeader(df *results.DataFile, devices []*device, witness *device, isi, width time.Duration) {
	df.WriteHostInfo(sysinfo.Host())
	df.WriteComment("--TRIGGER COMPARISON")
	mode := "parallel"
	if *fSeq {
		mode = "sequential"
	}
	df.WriteComment(fmt.Sprintf("t mode: %s", mode))
	df.WriteComment(fmt.Sprintf("t pulses: %d (warmup %d + recorded %d)", *fWarmup+*fN, *fWarmup, *fN))
	df.WriteComment(fmt.Sprintf("t isi_ms: %.4f", float64(isi)/float64(time.Millisecond)))
	df.WriteComment(fmt.Sprintf("t width_ms: %.4f", float64(width)/float64(time.Millisecond)))
	df.WriteComment(fmt.Sprintf("t realtime_priority: %d", *fPrio))
	df.WriteComment(fmt.Sprintf("t gc_during_train: %t", *fGC))
	for _, d := range devices {
		role := "source"
		if d == witness {
			role = "witness"
		}
		df.WriteComment(fmt.Sprintf("t device%d: %s role=%s spec=%q", d.index, d.Desc, role, d.Spec.Raw))
	}
	if witness != nil {
		for _, d := range devices {
			if d == witness {
				continue
			}
			df.WriteComment(fmt.Sprintf("t loopback%d: %s → witness line %d (D%d)",
				d.index, d.Label, d.witLine+1, 22+d.witLine))
		}
	}
	df.WriteComment("#")
}

func writeRows(df *results.DataFile, sources []*device, loopback bool) {
	names := []string{
		"rep", "phase", "dev_index", "dev_kind", "dev_label", "line",
		"target_ns", "pre_high_ns", "post_high_ns", "pre_low_ns", "post_low_ns",
		"write_high_us", "issue_skew_us", "width_us", "err",
	}
	if loopback {
		names = append(names, "wit_rise_ns", "wit_fall_ns", "wit_skew_us")
	}
	df.AddVariableNames(names)

	ref := sources[0]
	for k := range ref.rows {
		for _, d := range sources {
			r := d.rows[k]
			phase := "measure"
			if r.warmup {
				phase = "warmup"
			}
			row := []interface{}{
				r.rep, phase, d.index, d.Spec.Kind, d.Label, d.Line,
				int64(r.target), int64(r.preHigh), int64(r.postHigh), int64(r.preLow), int64(r.postLow),
				us(r.postHigh - r.preHigh),
				us(r.preHigh - ref.rows[k].preHigh),
				us(r.preLow - r.preHigh),
				strings.TrimSpace(r.errHigh + " " + r.errLow),
			}
			if loopback {
				row = append(row,
					optNs(r.witRise, r.hasRise),
					optNs(r.witFall, r.hasFall),
					optSkew(r, ref.rows[k]))
			}
			df.Add(row...)
		}
	}
}

func us(d time.Duration) float64 { return float64(d) / float64(time.Microsecond) }

func optNs(d time.Duration, ok bool) interface{} {
	if !ok {
		return "NA"
	}
	return int64(d)
}

func optSkew(r, ref pulse) interface{} {
	if !r.hasRise || !ref.hasRise {
		return "NA"
	}
	return us(r.witRise - ref.witRise)
}

// ── summary ───────────────────────────────────────────────────────────────────

// stats is the small summary this test needs. timingstats.ComputeStats is not
// used here: it is built around a target frame interval in milliseconds and
// counts "late by more than 0.5/1 ms", which says nothing about microsecond
// skews between two devices.
type stats struct {
	n                          int
	median, mean, sd, min, max float64
	p95                        float64
}

func computeStats(v []float64) stats {
	if len(v) == 0 {
		return stats{}
	}
	s := stats{n: len(v)}
	sorted := append([]float64(nil), v...)
	sort.Float64s(sorted)
	s.min, s.max = sorted[0], sorted[len(sorted)-1]
	s.median = sorted[len(sorted)/2]
	s.p95 = sorted[(len(sorted)*95)/100]
	var sum float64
	for _, x := range v {
		sum += x
	}
	s.mean = sum / float64(len(v))
	if len(v) > 1 {
		var sq float64
		for _, x := range v {
			sq += (x - s.mean) * (x - s.mean)
		}
		s.sd = math.Sqrt(sq / float64(len(v)-1))
	}
	return s
}

func (s stats) line() string {
	if s.n == 0 {
		return "no data"
	}
	return fmt.Sprintf("n=%3d  median %8.1f  mean %8.1f  sd %7.1f  min %8.1f  p95 %8.1f  max %8.1f",
		s.n, s.median, s.mean, s.sd, s.min, s.p95, s.max)
}

func summarise(out *report.Tee, sources []*device, witness *device, dropped bool) {
	ref := sources[0]
	out.Printf("\n═══ test_triggers ═══════════════════════════════════════════════════════\n")
	mode := "one locked thread per device, common deadline"
	if *fSeq {
		mode = "sequential, one thread, -device order"
	}
	out.Printf("mode: %s\n", mode)
	out.Printf("%d recorded pulses of %.3f ms every %.3f ms; %d warm-up pulses excluded\n",
		*fN, *fWidthMs, *fISIMs, *fWarmup)
	out.Printf("reference device (skews are measured against it): [%d] %s\n", ref.index, ref.Label)
	out.Printf("realtime priority: %s;  GC %s during the train\n\n",
		prioText(), map[bool]string{true: "running", false: "suspended"}[*fGC])

	failures := 0
	for _, d := range sources {
		for _, r := range d.rows {
			if r.errHigh != "" || r.errLow != "" {
				failures++
			}
		}
	}
	nulls := 0
	for _, d := range sources {
		if d.IsNull() {
			nulls++
		}
	}
	if nulls > 0 {
		out.Printf("!! %d of %d source(s) are 'null' devices: they drive NO hardware. What follows\n", nulls, len(sources))
		out.Printf("   measures this program, not any signal. Nothing reached an instrument.\n\n")
	}

	if failures > 0 {
		out.Printf("!! %d pulse(s) reported a write error — read the err column before using this run\n\n", failures)
	}

	out.Println("── host: cost of the SetHigh call (µs) ──────────────────────────────────")
	for _, d := range sources {
		out.Printf("  [%d] %-22s %s\n", d.index, d.Label, computeStats(collect(d, func(r pulse) float64 {
			return us(r.postHigh - r.preHigh)
		})).line())
	}

	out.Println("\n── host: when the write was ISSUED, against the reference (µs) ──────────")
	for _, d := range sources {
		if d == ref {
			out.Printf("  [%d] %-22s (reference)\n", d.index, d.Label)
			continue
		}
		out.Printf("  [%d] %-22s %s\n", d.index, d.Label, computeStats(pairs(d, ref, func(r, refRow pulse) float64 {
			return us(r.preHigh - refRow.preHigh)
		})).line())
	}

	out.Println("\n── host: realised pulse width, SetHigh call to SetLow call (µs) ─────────")
	for _, d := range sources {
		out.Printf("  [%d] %-22s %s\n", d.index, d.Label, computeStats(collect(d, func(r pulse) float64 {
			return us(r.preLow - r.preHigh)
		})).line())
	}

	if witness != nil {
		out.Printf("\n── witness %s: rising edges against the reference (µs) ──\n", witness.Label)
		if dropped {
			out.Println("  !! the witness dropped events: presses were LOST, not delayed — treat this run as suspect")
		}
		for _, d := range sources {
			if d == ref {
				out.Printf("  [%d] %-22s (reference)\n", d.index, d.Label)
				continue
			}
			vals := pairsIf(d, ref, func(r, refRow pulse) (float64, bool) {
				if !r.hasRise || !refRow.hasRise {
					return 0, false
				}
				return us(r.witRise - refRow.witRise), true
			})
			out.Printf("  [%d] %-22s %s\n", d.index, d.Label, computeStats(vals).line())
		}
		out.Println("  Both edges are stamped by the same micros() counter, so the device→host")
		out.Println("  clock offset cancels in the difference: this skew is good to a few µs even")
		out.Println("  though the absolute alignment is only good to a few hundred.")
	}

	out.Println("\n── how to read this ─────────────────────────────────────────────────────")
	out.Println("  These are HOST-side numbers: when this program issued each write, not when")
	out.Println("  the edge reached the wire. The instrument's trace is the measurement; the")
	out.Println("  numbers above say whether a skew seen there came from the device or from")
	out.Println("  the host having issued the writes late.")
	out.Println("  Match a scope edge to a row with target_ns: repetition k was scheduled at")
	out.Printf("  k × %.3f ms after the first pulse.\n", *fISIMs)
}

func collect(d *device, f func(pulse) float64) []float64 {
	vals := make([]float64, 0, len(d.rows))
	for _, r := range d.rows {
		if r.warmup || r.errHigh != "" || r.errLow != "" {
			continue
		}
		vals = append(vals, f(r))
	}
	return vals
}

func pairs(d, ref *device, f func(pulse, pulse) float64) []float64 {
	return pairsIf(d, ref, func(r, rr pulse) (float64, bool) { return f(r, rr), true })
}

func pairsIf(d, ref *device, f func(pulse, pulse) (float64, bool)) []float64 {
	vals := make([]float64, 0, len(d.rows))
	for k, r := range d.rows {
		rr := ref.rows[k]
		if r.warmup || r.errHigh != "" || rr.errHigh != "" {
			continue
		}
		if v, ok := f(r, rr); ok {
			vals = append(vals, v)
		}
	}
	return vals
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
