// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_megttlbox exercises the NeuroSpin Arduino Mega TTL box through the
// goxpyriment triggers package.
//
// It tests:
//   - Firmware identification (get_info, opcode 1) and legacy detection
//   - Output: Send, SetHigh/SetLow, Pulse, PulseMask — Arduino pins D30–D37
//   - Input:  ReadAll, ReadLine, WaitForInput — Arduino pins D22–D29
//
// Loopback wiring. The output and input banks can be jumpered together to get
// an automated pass/fail: D30→D22, D31→D23, … D37→D29 (output line N to input
// line N). Note the readings are INVERTED: the firmware configures inputs as
// INPUT_PULLUP and reports them with invert=true, so a line driven LOW reads as
// 1 ("pressed") and a line driven HIGH reads as 0. Hence ReadAll() == ^mask
// after Send(mask). -loopback checks exactly that.
//
// Usage:
//
//	go run .                        # auto-detect the port, run the sequence
//	go run . -device /dev/ttyACM0   # explicit port
//	go run . -list                  # list serial ports and exit
//	go run . -loopback              # automated check, needs the 8 jumpers
//	go run . -watch                 # live input display
//	go run . -set 0xAA              # drive one pattern and hold it
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/triggers"
	"go.bug.st/serial"
)

func main() {
	deviceFlag := flag.String("device", "", "serial port (e.g. /dev/ttyACM0); auto-detected if empty")
	listFlag := flag.Bool("list", false, "list available serial ports and exit")
	loopFlag := flag.Bool("loopback", false, "automated output→input check (needs D30→D22 … D37→D29)")
	eventsFlag := flag.Bool("events", false, "live display of firmware-timestamped input events")
	rtFlag := flag.Int("rtloop", 0, "measure host→timestamp latency N times (needs a D30→D22 jumper)")
	atomicFlag := flag.Int("atomic", 0, "check that Send changes all 8 lines at once, N trials (needs full loopback)")
	watchFlag := flag.Bool("watch", false, "continuously display the input lines until Enter/Ctrl-C")
	setFlag := flag.String("set", "", "drive one output bitmask and hold it (e.g. 0xAA, 0b10101010, 170)")
	flag.Parse()

	if *listFlag {
		listPorts()
		return
	}

	var mask byte
	if *setFlag != "" {
		v, err := strconv.ParseUint(strings.TrimSpace(*setFlag), 0, 8)
		if err != nil {
			log.Fatalf("-set: cannot parse %q as an 8-bit mask: %v", *setFlag, err)
		}
		mask = byte(v)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	box, port := open(*deviceFlag)
	defer box.Close()

	fmt.Printf("MEG TTL box on %s opened.\n", port)
	reportFirmware(box.Info())

	switch {
	case *loopFlag:
		runLoopback(box)
	case *eventsFlag:
		runEvents(ctx, box)
	case *rtFlag > 0:
		runRTLoop(ctx, box, *rtFlag)
	case *atomicFlag > 0:
		runAtomic(ctx, box, *atomicFlag)
	case *watchFlag:
		runWatch(ctx, box)
	case *setFlag != "":
		runSet(ctx, box, mask)
	default:
		runAuto(ctx, box)
	}

	_ = box.AllLow()
	fmt.Println("\nDone.")
}

// reportFirmware prints the get_info result and warns when the box predates it.
func reportFirmware(info triggers.MEGInfo) {
	fmt.Printf("  firmware: %s\n", info)
	if info.Legacy {
		fmt.Println("  WARNING: this firmware does not answer get_info (opcode 1).")
		fmt.Println("           Reflash arduino/meg_protocol to get version reporting.")
		return
	}
	fmt.Printf("    atomic port writes : %v\n", info.Has(triggers.MEGCapAtomicPort))
	fmt.Printf("    input timestamps   : %v\n", info.Has(triggers.MEGCapTimestamps))
}

func open(device string) (*triggers.MEGTTLBox, string) {
	if device != "" {
		b, err := triggers.NewMEGTTLBox(device)
		if err != nil {
			log.Fatalf("open MEG TTL box: %v", err)
		}
		return b, device
	}
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatalf("enumerate serial ports: %v", err)
	}
	for _, name := range ports {
		if b, err := triggers.NewMEGTTLBox(name); err == nil {
			return b, name
		}
	}
	log.Fatal("no MEG TTL box found — pass -device, or -list to see the available ports")
	return nil, ""
}

func listPorts() {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatalf("enumerate serial ports: %v", err)
	}
	if len(ports) == 0 {
		fmt.Println("no serial ports found")
		return
	}
	fmt.Println("available serial ports:")
	for _, p := range ports {
		fmt.Printf("  %s\n", p)
	}
}

// waitEnter blocks until Enter, EOF, or ctx cancellation. It reports whether it
// was Enter/EOF rather than a signal.
func waitEnter(ctx context.Context, r *bufio.Reader) bool {
	typed := make(chan struct{})
	go func() {
		_, _ = r.ReadString('\n')
		close(typed)
	}()
	select {
	case <-typed:
		return true
	case <-ctx.Done():
		fmt.Println()
		return false
	}
}

// --- Mode: -loopback (automated, needs jumpers) ---

func runLoopback(box *triggers.MEGTTLBox) {
	fmt.Println("\n--- Loopback: Send(mask) then ReadAll(), expecting ^mask ---")
	fmt.Println("Wiring: D30→D22, D31→D23, … D37→D29.")
	fmt.Println("Readings are inverted: inputs are INPUT_PULLUP reported with invert=true,")
	fmt.Println("so a line driven LOW reads 1 and a line driven HIGH reads 0.")

	patterns := []byte{0x00, 0xFF, 0xAA, 0x55, 0x0F, 0xF0, 0x01, 0x80}
	pass, fail := 0, 0
	for _, m := range patterns {
		if err := box.Send(m); err != nil {
			log.Printf("  Send(0x%02X): ERROR: %v", m, err)
			fail++
			continue
		}
		// Let the level settle before reading it back.
		time.Sleep(20 * time.Millisecond)
		got, err := box.ReadAll()
		if err != nil {
			log.Printf("  ReadAll after 0x%02X: ERROR: %v", m, err)
			fail++
			continue
		}
		want := ^m
		status := "OK"
		if got != want {
			status = fmt.Sprintf("MISMATCH (diff 0x%02X)", got^want)
			fail++
		} else {
			pass++
		}
		fmt.Printf("  Send(0x%02X) → ReadAll 0x%02X, want 0x%02X  %s\n", m, got, want, status)
	}
	_ = box.AllLow()
	fmt.Printf("\n  %d passed, %d failed\n", pass, fail)
	if fail > 0 {
		fmt.Println("  A uniform mismatch usually means the jumpers are absent or shifted;")
		fmt.Println("  a single differing bit points at one bad wire.")
	}
}

// --- Mode: -events (live firmware-timestamped events) ---

func runEvents(ctx context.Context, box *triggers.MEGTTLBox) {
	if !box.Info().Has(triggers.MEGCapTimestamps) {
		log.Fatal("this firmware has no timestamped events — reflash arduino/meg_protocol")
	}
	fmt.Println("\n--- Event mode: firmware-timestamped input transitions ---")
	fmt.Println("  Touch a jumper between GND and D22–D29 to generate transitions.")
	fmt.Println("  TS is the instant the *firmware* saw the change, translated to the")
	fmt.Println("  host clock — it does not depend on when this program asked.")
	fmt.Print("  Press Ctrl-C to stop.\n\n")

	if err := box.DrainEvents(); err != nil {
		log.Fatalf("  DrainEvents: %v", err)
	}

	var prev time.Time
	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return
		default:
		}
		ev, ok, err := box.PollEvent()
		if err != nil {
			log.Printf("  PollEvent: ERROR: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if box.EventsDropped() {
			fmt.Println("  ** events were DROPPED (firmware queue overflow) **")
		}
		if !ok {
			time.Sleep(2 * time.Millisecond)
			continue
		}
		kind := "release"
		if ev.Pressed() {
			kind = "PRESS  "
		}
		gap := ""
		if !prev.IsZero() {
			gap = fmt.Sprintf("  (+%v since previous)", ev.TS.Sub(prev).Round(time.Microsecond))
		}
		fmt.Printf("  %s  mask=0x%02X %08b  TS=%s  raw=%d%s\n",
			kind, ev.Mask, ev.Mask, ev.TS.Format("15:04:05.000000"), ev.Raw, gap)
		fmt.Printf("           active: %s\n", buttonList(ev.Mask))
		prev = ev.TS
	}
}

// --- Mode: -rtloop (measure host→firmware-timestamp latency) ---

// runRTLoop drives an output that is jumpered to an input and compares the
// firmware's timestamp of the resulting edge against the host clock just before
// the command was written. That is the one-way host→device latency, and it is
// the part of a reaction time that firmware timestamping cannot remove.
//
// The logic is inverted: inputs are INPUT_PULLUP reported with invert=true, so
// driving D30 LOW pulls D22 low and reads as "pressed".
func runRTLoop(ctx context.Context, box *triggers.MEGTTLBox, n int) {
	if !box.Info().Has(triggers.MEGCapTimestamps) {
		log.Fatal("this firmware has no timestamped events — reflash arduino/meg_protocol")
	}
	fmt.Printf("\n--- RT loop: %d trials, needs a jumper D30 → D22 ---\n", n)
	fmt.Println("  Measures (firmware timestamp of edge) − (host clock before write).")
	fmt.Println("  This is host→device latency; it bounds how well any host-side")
	fmt.Println("  timestamp could ever do, and firmware timestamping removes the rest.")

	var samples []time.Duration
	for i := 0; i < n; i++ {
		select {
		case <-ctx.Done():
			fmt.Println("\n  interrupted")
			return
		default:
		}
		// Release (D30 HIGH → D22 high → not pressed), then clear the queue.
		if err := box.Send(0x01); err != nil {
			log.Printf("  trial %d: Send: %v", i, err)
			continue
		}
		time.Sleep(20 * time.Millisecond)
		if err := box.DrainEvents(); err != nil {
			log.Printf("  trial %d: DrainEvents: %v", i, err)
			continue
		}

		t0 := time.Now()
		if err := box.Send(0x00); err != nil { // D30 LOW → D22 low → "pressed"
			log.Printf("  trial %d: Send: %v", i, err)
			continue
		}
		wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		ev, err := box.WaitForPressTS(wctx)
		cancel()
		if err != nil {
			log.Printf("  trial %d: no press detected: %v (is the D30→D22 jumper in place?)", i, err)
			continue
		}
		samples = append(samples, ev.TS.Sub(t0))
	}
	_ = box.Send(0x01)

	if len(samples) == 0 {
		fmt.Println("\n  no successful trials — check the D30→D22 jumper.")
		return
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	fmt.Printf("\n  n=%d  min=%v  median=%v  max=%v\n",
		len(samples),
		samples[0].Round(time.Microsecond),
		samples[len(samples)/2].Round(time.Microsecond),
		samples[len(samples)-1].Round(time.Microsecond))
	fmt.Println("  Spread here is USB scheduling, not the timestamp: the firmware's")
	fmt.Println("  micros() reading is taken within a loop iteration of the edge.")
}

// --- Mode: -atomic (are all 8 lines really changed at once?) ---

// runAtomic checks the central claim of the atomic port write: that Send moves
// all 8 output lines in a single instruction, so no intermediate value ever
// appears on the pins.
//
// With the full loopback in place the firmware is watching those same lines a
// few microseconds apart, so it is a far better witness than any host-side
// check. Driving 0x00 → 0xFF should therefore produce exactly ONE event, with
// every bit flipping together. A non-atomic write — the legacy set-high then
// set-low pair, ~174 µs apart — would be sampled mid-transition and show up as
// two or more events with a partial mask in between.
func runAtomic(ctx context.Context, box *triggers.MEGTTLBox, n int) {
	if !box.Info().Has(triggers.MEGCapTimestamps) {
		log.Fatal("this firmware has no timestamped events — reflash arduino/meg_protocol")
	}
	if !box.Info().Has(triggers.MEGCapAtomicPort) {
		fmt.Println("NOTE: firmware lacks CAP_ATOMIC_PORT, so Send uses the two-command")
		fmt.Println("      fallback — expect more than one event per transition.")
	}
	fmt.Printf("\n--- Atomicity: %d transitions 0x00 → 0xFF, full loopback required ---\n", n)
	fmt.Println("  Expect exactly 1 event per trial, mask 0x00 (all lines flipped together).")
	fmt.Println("  More than 1 means the port was caught part-way through.")

	counts := map[int]int{}
	var partials []byte
	for i := 0; i < n; i++ {
		select {
		case <-ctx.Done():
			fmt.Println("\n  interrupted")
			return
		default:
		}
		if err := box.Send(0x00); err != nil { // all LOW → all inputs read "pressed"
			log.Printf("  trial %d: Send: %v", i, err)
			continue
		}
		time.Sleep(20 * time.Millisecond)
		if err := box.DrainEvents(); err != nil {
			log.Printf("  trial %d: DrainEvents: %v", i, err)
			continue
		}

		if err := box.Send(0xFF); err != nil { // all HIGH → all inputs read "released"
			log.Printf("  trial %d: Send: %v", i, err)
			continue
		}
		// Collect everything the firmware saw for this transition.
		var evs []triggers.InputEvent
		deadline := time.Now().Add(120 * time.Millisecond)
		for time.Now().Before(deadline) {
			ev, ok, err := box.PollEvent()
			if err != nil {
				log.Printf("  trial %d: PollEvent: %v", i, err)
				break
			}
			if ok {
				evs = append(evs, ev)
				continue
			}
			time.Sleep(2 * time.Millisecond)
		}
		counts[len(evs)]++
		if len(evs) > 1 {
			for _, e := range evs[:len(evs)-1] {
				partials = append(partials, e.Mask)
			}
		}
	}
	_ = box.AllLow()

	fmt.Println("\n  events per transition:")
	for k := 0; k <= 8; k++ {
		if c, ok := counts[k]; ok {
			label := fmt.Sprintf("%d event(s)", k)
			verdict := ""
			switch {
			case k == 1:
				verdict = "  ← atomic"
			case k == 0:
				verdict = "  ← nothing seen (loopback wired?)"
			default:
				verdict = "  ← NOT atomic, port caught mid-write"
			}
			fmt.Printf("    %-12s %3d trials%s\n", label, c, verdict)
		}
	}
	if len(partials) > 0 {
		fmt.Printf("  intermediate masks observed: % 02X\n", partials)
	}
}

// --- Mode: -set (drive one static pattern) ---

func runSet(ctx context.Context, box *triggers.MEGTTLBox, mask byte) {
	fmt.Printf("\n--- Set mode: driving 0x%02X (%08b) on D30–D37 ---\n", mask, mask)
	if err := box.Send(mask); err != nil {
		log.Fatalf("  Send(0x%02X): %v", mask, err)
	}
	for line := 0; line <= 7; line++ {
		level := "LOW  (≈0 V)"
		if mask&(1<<uint(line)) != 0 {
			level = "HIGH (≈5 V)"
		}
		fmt.Printf("  line %d  D%d  %s\n", line, 30+line, level)
	}
	fmt.Print("\n  holding — press Enter or Ctrl-C to drive everything LOW: ")
	waitEnter(ctx, bufio.NewReader(os.Stdin))
}

// --- Mode: -watch (live input display) ---

func runWatch(ctx context.Context, box *triggers.MEGTTLBox) {
	fmt.Println("\n--- Watch mode: live input lines (D22–D29) ---")
	fmt.Println("  Inputs are INPUT_PULLUP, reported inverted: tie a pin to GND to set its bit.")
	fmt.Println("  Press Enter or Ctrl-C to stop.")

	done := make(chan struct{})
	go func() {
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		close(done)
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var last byte
	first := true
	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return
		case <-done:
			return
		case <-ticker.C:
			m, err := box.ReadAll()
			if err != nil {
				log.Printf("  ReadAll: ERROR: %v", err)
				continue
			}
			if !first && m == last {
				continue
			}
			fmt.Printf("\r  0x%02X  %08b   active: %-40s", m, m, buttonList(m))
			last, first = m, false
		}
	}
}

func buttonList(mask byte) string {
	b := triggers.DecodeMask(mask)
	if len(b) == 0 {
		return "(none)"
	}
	names := make([]string, len(b))
	for i, btn := range b {
		names[i] = btn.String()
	}
	return strings.Join(names, ", ")
}

// --- Default: automatic sequence ---

func runAuto(ctx context.Context, box *triggers.MEGTTLBox) {
	fmt.Println("\n--- Send (bitmask → D30–D37) ---")
	for _, v := range []byte{0x00, 0xFF, 0xAA, 0x55, 0x0F, 0xF0, 0x01, 0x80} {
		if err := box.Send(v); err != nil {
			log.Printf("  Send(0x%02X): ERROR: %v", v, err)
			continue
		}
		fmt.Printf("  Send(0x%02X)  OK   %08b\n", v, v)
		time.Sleep(50 * time.Millisecond)
	}
	_ = box.AllLow()

	fmt.Println("\n--- SetHigh / SetLow (individual lines) ---")
	for line := 0; line <= 7; line++ {
		if err := box.SetHigh(line); err != nil {
			log.Printf("  SetHigh(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  SetHigh(%d)  OK   (D%d)\n", line, 30+line)
		time.Sleep(30 * time.Millisecond)
		if err := box.SetLow(line); err != nil {
			log.Printf("  SetLow(%d): ERROR: %v", line, err)
		}
	}

	fmt.Println("\n--- Pulse (device-timed, 5 ms) ---")
	if err := box.Pulse(0, 5*time.Millisecond); err != nil {
		log.Printf("  Pulse: ERROR: %v", err)
	} else {
		fmt.Println("  Pulse(0, 5ms)  OK   (width timed by the Arduino, not the host)")
	}
	if err := box.PulseMask(0b00000011, 5*time.Millisecond); err != nil {
		log.Printf("  PulseMask: ERROR: %v", err)
	} else {
		fmt.Println("  PulseMask(0b11, 5ms)  OK")
	}

	fmt.Println("\n--- ReadAll (input bitmask) ---")
	if m, err := box.ReadAll(); err != nil {
		log.Printf("  ReadAll: ERROR: %v", err)
	} else {
		fmt.Printf("  ReadAll → 0x%02X  %08b   active: %s\n", m, m, buttonList(m))
	}

	fmt.Println("\n--- ReadLine (individual input lines) ---")
	for line := 0; line <= 7; line++ {
		v, err := box.ReadLine(line)
		if err != nil {
			log.Printf("  ReadLine(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  ReadLine(%d) → %d   (D%d)\n", line, v, 22+line)
	}

	fmt.Println("\n--- WaitForInput (2 s window) ---")
	wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if m, rt, err := box.WaitForInput(wctx); err != nil {
		fmt.Printf("  no input detected (timeout or cancel): %v\n", err)
	} else {
		fmt.Printf("  input detected: mask=0x%02X  rt=%v  (%s)\n", m, rt, buttonList(m))
	}
}
