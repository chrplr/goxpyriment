// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_labjackt4 exercises a LabJack T4 as an 8-bit TTL trigger device via
// the goxpyriment triggers package.
//
// It tests:
//   - Output: Send, SetHigh/SetLow, Pulse — on DIO4–DIO11 (FIO4–FIO7, EIO0–EIO3)
//   - Input:  ReadAll, ReadLine, WaitForInput — on DIO12–DIO19 (EIO4–EIO7, CIO0–CIO3)
//
// (DIO0–DIO3 are the T4's dedicated analog inputs AIN0–AIN3 and cannot be
// used as digital lines.)
//
// Wiring for a self-loopback smoke-test: connect output line N to input line N,
// i.e. FIO4→EIO4, FIO5→EIO5, FIO6→EIO6, FIO7→EIO7, EIO0→CIO0, EIO1→CIO1,
// EIO2→CIO2, EIO3→CIO3. Without loopback wiring the output tests still
// exercise the device; the input readings will reflect the undriven state of
// the input pins.
//
// The default (automatic) run holds each output level for only 30–100 ms —
// far too briefly for a multimeter. Use -hold or -set for static levels.
//
// Usage:
//
//	go run . -host 192.168.1.100              # automatic sequence
//	go run . -host 192.168.1.100 -port 502
//	go run . -host 192.168.1.100 -hold        # walk outputs, one at a time
//	go run . -host 192.168.1.100 -set 0xAA    # drive one pattern and hold it
//	go run . -host 192.168.1.100 -watch       # live input display
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
)

// Default line groups of triggers.LabJackT4 on a T4.
const (
	outputBaseDIO = 4  // output line 0 → DIO4
	inputBaseDIO  = 12 // input line 0  → DIO12
)

// dioName returns the LabJack terminal name of a DIO number, e.g. 4 → "FIO4",
// 8 → "EIO0", 16 → "CIO0".
func dioName(dio int) string {
	switch {
	case dio >= 0 && dio <= 7:
		return fmt.Sprintf("FIO%d", dio)
	case dio <= 15:
		return fmt.Sprintf("EIO%d", dio-8)
	case dio <= 19:
		return fmt.Sprintf("CIO%d", dio-16)
	case dio <= 22:
		return fmt.Sprintf("MIO%d", dio-20)
	}
	return fmt.Sprintf("DIO%d", dio)
}

func outputPin(line int) string { return dioName(outputBaseDIO + line) }
func inputPin(line int) string  { return dioName(inputBaseDIO + line) }

// parseMask accepts a bitmask as decimal, hex (0x..), binary (0b..) or octal.
func parseMask(s string) (byte, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 0, 8)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q as an 8-bit mask (try 0xAA, 0b10101010 or 170): %w", s, err)
	}
	return byte(v), nil
}

func main() {
	hostFlag := flag.String("host", "", "LabJack T4 IP address (required, e.g. 192.168.1.100)")
	portFlag := flag.Int("port", 502, "Modbus TCP port")
	holdFlag := flag.Bool("hold", false, "walk the 8 output lines, holding each HIGH until you press Enter (for multimeter probing)")
	setFlag := flag.String("set", "", "drive one output bitmask and hold it until Enter/Ctrl-C (e.g. 0xAA, 0b10101010, 170)")
	watchFlag := flag.Bool("watch", false, "continuously display the input lines until Enter/Ctrl-C")
	flag.Parse()

	if *hostFlag == "" {
		log.Fatal("usage: go run . -host <ip> [-hold | -set <mask> | -watch]")
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

	addr := fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
	dev, err := triggers.NewLabJackT4(addr)
	if err != nil {
		log.Fatalf("open LabJackT4: %v", err)
	}
	defer dev.Close()
	fmt.Printf("LabJack T4 at %s opened successfully.\n", addr)

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

func runHold(ctx context.Context, dev *triggers.LabJackT4) {
	fmt.Println("\n--- Hold mode: probe each output line with a multimeter ---")
	fmt.Println("Measure between the named terminal and GND: HIGH ≈ 3.3 V, LOW ≈ 0 V.")
	fmt.Println("Press Enter to advance to the next line, Ctrl-C to stop.")

	r := bufio.NewReader(os.Stdin)
	for line := 0; line <= 7; line++ {
		if err := dev.SetHigh(line); err != nil {
			log.Printf("  SetHigh(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  line %d = %s (DIO%d) HIGH — measure, then press Enter: ",
			line, outputPin(line), outputBaseDIO+line)
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

func runSet(ctx context.Context, dev *triggers.LabJackT4, mask byte) {
	fmt.Printf("\n--- Set mode: driving 0x%02X (%08b) ---\n", mask, mask)
	if err := dev.Send(mask); err != nil {
		log.Fatalf("  Send(0x%02X): %v", mask, err)
	}
	fmt.Println("  line  terminal  DIO   expected")
	for line := 0; line <= 7; line++ {
		level := "LOW  (≈0 V)"
		if mask&(1<<uint(line)) != 0 {
			level = "HIGH (≈3.3 V)"
		}
		fmt.Printf("  %-5d %-9s DIO%-3d %s\n", line, outputPin(line), outputBaseDIO+line, level)
	}
	fmt.Print("\n  holding — press Enter or Ctrl-C to drive everything LOW: ")
	waitEnter(ctx, bufio.NewReader(os.Stdin))
}

// --- Mode: -watch (live input display, for patching a pin to GND or 3.3 V) ---

func runWatch(ctx context.Context, dev *triggers.LabJackT4) {
	fmt.Println("\n--- Watch mode: live input lines ---")
	fmt.Printf("  lines 0–7 = %s…%s (DIO%d–DIO%d). They idle HIGH on internal\n",
		inputPin(0), inputPin(7), inputBaseDIO, inputBaseDIO+7)
	fmt.Println("  pull-ups, so patch a pin to GND to see its bit drop to 0.")
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
			fmt.Printf("\r  0x%02X  %08b   low: %s          ", m, m, lowLines(m))
			last, first = m, false
		}
	}
}

// lowLines names the input lines currently pulled LOW, i.e. the pins wired to GND.
func lowLines(mask byte) string {
	var names []string
	for line := 0; line <= 7; line++ {
		if mask&(1<<uint(line)) == 0 {
			names = append(names, fmt.Sprintf("%d=%s", line, inputPin(line)))
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, " ")
}

// --- Default: automatic sequence ---

func runAuto(ctx context.Context, dev *triggers.LabJackT4) {
	// --- Output: Send (bitmask) ---
	fmt.Println("\n--- Send (byte bitmask → DIO4–DIO11) ---")
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
	for line := 0; line <= 7; line++ {
		if err := dev.SetHigh(line); err != nil {
			log.Printf("  SetHigh(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  SetHigh(%d)  OK\n", line)
		time.Sleep(30 * time.Millisecond)
		if err := dev.SetLow(line); err != nil {
			log.Printf("  SetLow(%d): ERROR: %v", line, err)
		}
	}

	// --- Output: Pulse ---
	fmt.Println("\n--- Pulse (line 0, 100 ms) ---")
	if err := dev.Pulse(0, 100*time.Millisecond); err != nil {
		log.Printf("  Pulse: ERROR: %v", err)
	} else {
		fmt.Println("  Pulse(0, 100ms)  OK")
	}

	// --- Input: ReadAll ---
	fmt.Println("\n--- ReadAll (DIO12–DIO19 input bitmask) ---")
	mask, err := dev.ReadAll()
	if err != nil {
		log.Printf("  ReadAll: ERROR: %v", err)
	} else {
		fmt.Printf("  ReadAll → 0x%02X  %08b\n", mask, mask)
	}

	// --- Input: ReadLine ---
	fmt.Println("\n--- ReadLine (individual DIO12–DIO19 lines) ---")
	for line := 0; line <= 7; line++ {
		v, err := dev.ReadLine(line)
		if err != nil {
			log.Printf("  ReadLine(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  ReadLine(%d) → %d\n", line, v)
	}

	// --- Input: WaitForInput (2-second window) ---
	fmt.Println("\n--- WaitForInput (waiting 2 s for any DIO12–DIO19 input) ---")
	wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	imask, rt, err := dev.WaitForInput(wctx)
	if err != nil {
		fmt.Printf("  no input detected (timeout or cancel): %v\n", err)
	} else {
		fmt.Printf("  input detected: mask=0x%02X  rt=%v\n", imask, rt)
	}
}
