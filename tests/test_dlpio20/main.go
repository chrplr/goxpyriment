// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_dlpio20 exercises a DLP-IO20 as an 8-bit TTL trigger device via the
// goxpyriment triggers package.
//
// It tests:
//   - Output: Send, SetHigh/SetLow, Pulse — on AN0–AN7 by default
//   - Input:  ReadAll, ReadLine, WaitForInput — on AN8–AN13 + RB6/RB7 by default
//   - Channel access outside those windows: SetChannelHigh, ReadChannel
//
// The DLP-IO20's 17 digital channels do not all fit the 8-bit TTL interfaces,
// so lines 0–7 address a window of 8 channels (see triggers.DLPIO20).
//
// Wiring for a self-loopback smoke-test: connect output line N to input line N,
// i.e. AN0→AN8, AN1→AN9, AN2→AN10, AN3→AN11, AN4→AN12, AN5→AN13, AN6→RB6,
// AN7→RB7. Without loopback wiring the output tests still exercise the device;
// the input readings will reflect the undriven state of the input pins.
//
// The automatic run holds each output level for only 30–100 ms — far too
// briefly for a multimeter. Use -hold or -set for static levels.
//
// Usage:
//
//	go run .                              # auto-detect the port
//	go run . -device /dev/ttyUSB0         # explicit port
//	go run . -list                        # list serial ports and exit
//	go run . -hold                        # walk outputs, one at a time
//	go run . -set 0xAA                    # drive one pattern and hold it
//	go run . -watch                       # live input display
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/triggers"
	"go.bug.st/serial"
)

// parseMask accepts a bitmask as decimal, hex (0x..), binary (0b..) or octal.
func parseMask(s string) (byte, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 0, 8)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q as an 8-bit mask (try 0xAA, 0b10101010 or 170): %w", s, err)
	}
	return byte(v), nil
}

func main() {
	deviceFlag := flag.String("device", "", "serial port (e.g. /dev/ttyUSB0); auto-detected if empty")
	listFlag := flag.Bool("list", false, "list available serial ports and exit")
	holdFlag := flag.Bool("hold", false, "walk the 8 output lines, holding each HIGH until you press Enter (for multimeter probing)")
	setFlag := flag.String("set", "", "drive one output bitmask and hold it until Enter/Ctrl-C (e.g. 0xAA, 0b10101010, 170)")
	watchFlag := flag.Bool("watch", false, "continuously display the input lines until Enter/Ctrl-C")
	flag.Parse()

	if *listFlag {
		listPorts()
		return
	}
	nModes := 0
	for _, on := range []bool{*holdFlag, *setFlag != "", *watchFlag} {
		if on {
			nModes++
		}
	}
	if nModes > 1 {
		log.Fatal("-hold, -set and -watch are mutually exclusive")
	}

	// Parse before touching the hardware so a typo fails immediately.
	var mask byte
	if *setFlag != "" {
		var err error
		if mask, err = parseMask(*setFlag); err != nil {
			log.Fatalf("-set: %v", err)
		}
	}

	// Ctrl-C unwinds normally so the deferred Close() drives every line LOW.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dev, portName := open(*deviceFlag)
	defer dev.Close()
	fmt.Printf("DLP-IO20 on %s opened successfully.\n", portName)
	fmt.Printf("  output lines 0–7 → %s\n", channelList(dev.OutputChannels()))
	fmt.Printf("  input  lines 0–7 → %s\n", channelList(dev.InputChannels()))

	switch {
	case *holdFlag:
		runHold(ctx, dev)
	case *setFlag != "":
		runSet(ctx, dev, mask)
	case *watchFlag:
		runWatch(ctx, dev)
	default:
		runAuto(ctx, dev)
	}

	_ = dev.AllLow()
	fmt.Println("\nDone.")
}

// open returns a ready device, auto-detecting the port when none is given.
func open(device string) (*triggers.DLPIO20, string) {
	if device != "" {
		d, err := triggers.NewDLPIO20(device)
		if err != nil {
			log.Fatalf("open DLP-IO20: %v", err)
		}
		return d, device
	}
	out, name, err := triggers.AutoDetectDLPIO20()
	if err != nil {
		log.Fatalf("auto-detect DLP-IO20: %v", err)
	}
	d, ok := out.(*triggers.DLPIO20)
	if !ok {
		log.Fatal("no DLP-IO20 found — pass -device, or -list to see the available ports")
	}
	return d, name
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

func channelList(chans [8]triggers.IO20Channel) string {
	names := make([]string, len(chans))
	for i, c := range chans {
		names[i] = c.String()
	}
	return strings.Join(names, ", ")
}

// waitEnter blocks until the user presses Enter, stdin reaches EOF, or ctx is
// cancelled (Ctrl-C). It reports whether it was Enter/EOF rather than a signal.
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

// --- Mode: -hold (walk the output lines, one static level at a time) ---

func runHold(ctx context.Context, dev *triggers.DLPIO20) {
	fmt.Println("\n--- Hold mode: probe each output line with a multimeter ---")
	fmt.Println("Measure between the named terminal and GND: HIGH ≈ 5 V, LOW ≈ 0 V.")
	fmt.Println("Press Enter to advance to the next line, Ctrl-C to stop.")

	chans := dev.OutputChannels()
	r := bufio.NewReader(os.Stdin)
	for line := 0; line <= 7; line++ {
		if err := dev.SetHigh(line); err != nil {
			log.Printf("  SetHigh(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  line %d = %s HIGH — measure, then press Enter: ", line, chans[line])
		cont := waitEnter(ctx, r)
		if err := dev.SetLow(line); err != nil {
			log.Printf("  SetLow(%d): ERROR: %v", line, err)
		}
		if !cont {
			fmt.Println("  interrupted — all lines LOW.")
			return
		}
	}
	fmt.Println("  all 8 output lines probed.")
}

// --- Mode: -set (drive one static pattern) ---

func runSet(ctx context.Context, dev *triggers.DLPIO20, mask byte) {
	fmt.Printf("\n--- Set mode: driving 0x%02X (%08b) ---\n", mask, mask)
	if err := dev.Send(mask); err != nil {
		log.Fatalf("  Send(0x%02X): %v", mask, err)
	}
	chans := dev.OutputChannels()
	fmt.Println("  line  terminal  expected")
	for line := 0; line <= 7; line++ {
		level := "LOW  (≈0 V)"
		if mask&(1<<uint(line)) != 0 {
			level = "HIGH (≈5 V)"
		}
		fmt.Printf("  %-5d %-9s %s\n", line, chans[line], level)
	}
	fmt.Print("\n  holding — press Enter or Ctrl-C to drive everything LOW: ")
	waitEnter(ctx, bufio.NewReader(os.Stdin))
}

// --- Mode: -watch (live input display) ---

func runWatch(ctx context.Context, dev *triggers.DLPIO20) {
	fmt.Println("\n--- Watch mode: live input lines ---")
	fmt.Printf("  lines 0–7 = %s\n", channelList(dev.InputChannels()))
	fmt.Print("  Patch a pin to +5V or GND to see its bit change.\n")
	fmt.Print("  Press Enter or Ctrl-C to stop.\n\n")

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
			m, err := dev.ReadAll()
			if err != nil {
				log.Printf("  ReadAll: ERROR: %v", err)
				continue
			}
			if !first && m == last {
				continue
			}
			fmt.Printf("\r  0x%02X  %08b   high: %s          ", m, m, highLines(dev, m))
			last, first = m, false
		}
	}
}

// highLines names the input lines currently reading HIGH.
func highLines(dev *triggers.DLPIO20, mask byte) string {
	chans := dev.InputChannels()
	var names []string
	for line := 0; line <= 7; line++ {
		if mask&(1<<uint(line)) != 0 {
			names = append(names, fmt.Sprintf("%d=%s", line, chans[line]))
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, " ")
}

// --- Default: automatic sequence ---

func runAuto(ctx context.Context, dev *triggers.DLPIO20) {
	// --- Output: Send (bitmask) ---
	fmt.Println("\n--- Send (byte bitmask → output window) ---")
	testValues := []byte{0x00, 0xFF, 0xAA, 0x55, 0x0F, 0xF0, 0x01, 0x80}
	for _, v := range testValues {
		if err := dev.Send(v); err != nil {
			log.Printf("  Send(0x%02X): ERROR: %v", v, err)
			continue
		}
		fmt.Printf("  Send(0x%02X)  OK   %08b\n", v, v)
		time.Sleep(50 * time.Millisecond)
	}
	_ = dev.AllLow()

	// --- Output: SetHigh / SetLow ---
	fmt.Println("\n--- SetHigh / SetLow (individual lines) ---")
	chans := dev.OutputChannels()
	for line := 0; line <= 7; line++ {
		if err := dev.SetHigh(line); err != nil {
			log.Printf("  SetHigh(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  SetHigh(%d)  OK   (%s)\n", line, chans[line])
		time.Sleep(30 * time.Millisecond)
		if err := dev.SetLow(line); err != nil {
			log.Printf("  SetLow(%d): ERROR: %v", line, err)
		}
	}

	// --- Output: Pulse ---
	fmt.Printf("\n--- Pulse (line 0 = %s, 100 ms) ---\n", chans[0])
	if err := dev.Pulse(0, 100*time.Millisecond); err != nil {
		log.Printf("  Pulse: ERROR: %v", err)
	} else {
		fmt.Println("  Pulse(0, 100ms)  OK")
	}

	// --- Channel access outside the 8-line windows ---
	fmt.Println("\n--- Channel access beyond the windows (RA4) ---")
	if err := dev.SetChannelHigh(triggers.IO20_RA4); err != nil {
		log.Printf("  SetChannelHigh(RA4): ERROR: %v", err)
	} else {
		fmt.Println("  SetChannelHigh(RA4)  OK")
		time.Sleep(50 * time.Millisecond)
		if err := dev.SetChannelLow(triggers.IO20_RA4); err != nil {
			log.Printf("  SetChannelLow(RA4): ERROR: %v", err)
		}
	}

	// --- Input: ReadAll ---
	fmt.Println("\n--- ReadAll (input window bitmask) ---")
	mask, err := dev.ReadAll()
	if err != nil {
		log.Printf("  ReadAll: ERROR: %v", err)
	} else {
		fmt.Printf("  ReadAll → 0x%02X  %08b\n", mask, mask)
	}

	// --- Input: ReadLine ---
	fmt.Println("\n--- ReadLine (individual input lines) ---")
	inChans := dev.InputChannels()
	for line := 0; line <= 7; line++ {
		v, err := dev.ReadLine(line)
		if err != nil {
			log.Printf("  ReadLine(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  ReadLine(%d) → %d   (%s)\n", line, v, inChans[line])
	}

	// --- Input: WaitForInput (2-second window) ---
	fmt.Println("\n--- WaitForInput (waiting 2 s for any input line) ---")
	wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	imask, rt, err := dev.WaitForInput(wctx)
	if err != nil {
		fmt.Printf("  no input detected (timeout or cancel): %v\n", err)
	} else {
		fmt.Printf("  input detected: mask=0x%02X  rt=%v\n", imask, rt)
	}
}
