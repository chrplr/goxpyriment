// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_mmbts exercises a NEUROSPEC MMBT-S interface box through the
// goxpyriment triggers package.
//
// The MMBT-S is output-only and never answers on its serial line, so there is
// nothing to check automatically: this program drives known patterns and the
// operator confirms them. Two ways to do that:
//
//   - No instrument: the green LED beside the D-Sub 25 socket follows bit 1,
//     so it lights on odd codes and stays dark on even ones.
//   - Scope or amplifier: bit N is D-Sub 25 pin N+2 (bit 0 → pin 2, bit 7 →
//     pin 9); ground is any of pins 20-25. 5 V HIGH, 0 V LOW.
//
// The runtime mode matters and cannot be read back. LOOK AT THE P/S SWITCH
// next to the USB-C socket before running, and pass the matching -mode:
//
//	P (factory setting) — the firmware clears the port 8 ms after each byte,
//	                     so every pulse is 8 ms wide whatever is requested,
//	                     and codes sent closer together are delayed.
//	S                  — a byte latches until the next one is written; the
//	                     host controls the width.
//
// Usage:
//
//	go run ./tests/test_mmbts -list                          # list serial ports and exit
//	go run ./tests/test_mmbts -device /dev/ttyACM0           # run the sequence (mode p)
//	go run ./tests/test_mmbts -device /dev/ttyACM0 -mode s   # switch set to "S"
//	go run ./tests/test_mmbts -device /dev/ttyACM0 -set 0xAA # drive one mask and hold
//	go run ./tests/test_mmbts -device /dev/ttyACM0 -cycle    # square wave until Ctrl-C
//
// Prerequisites: read/write access to the port (on Linux the `dialout` group:
// sudo usermod -aG dialout $USER, then log in again).
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

func main() {
	deviceFlag := flag.String("device", "/dev/ttyACM0", "serial port of the MMBT-S (e.g. /dev/ttyACM0, COM4)")
	listFlag := flag.Bool("list", false, "list available serial ports and exit")
	modeFlag := flag.String("mode", "p", "runtime mode the P/S switch is set to: p (pulse) or s (simple)")
	lineFlag := flag.Int("line", 0, "output line used by -cycle and the pulse walk (0-7; bit N is D-Sub 25 pin N+2)")
	setFlag := flag.String("set", "", "drive one output bitmask and hold it (e.g. 0xAA, 0b10101010, 170)")
	cycleFlag := flag.Bool("cycle", false, "square-wave one line until Ctrl-C")
	periodFlag := flag.Duration("period", 500*time.Millisecond, "with -cycle, the period of the square wave")
	widthFlag := flag.Duration("width", 10*time.Millisecond, "pulse width requested (honoured in mode s only; mode p is fixed at 8 ms)")
	noPromptFlag := flag.Bool("no-prompt", false, "do not ask for confirmation before driving the lines")
	flag.Parse()

	if *listFlag {
		listPorts()
		return
	}
	if *lineFlag < 0 || *lineFlag > 7 {
		log.Fatalf("-line: %d is out of range (0-7)", *lineFlag)
	}

	mode, err := parseMode(*modeFlag)
	if err != nil {
		log.Fatalf("-mode: %v", err)
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

	// Everything below puts real edges on the D-Sub 25 socket, which may be
	// wired to an amplifier that is recording. Say what is about to happen and
	// let the operator stop it.
	describe(*deviceFlag, mode, *lineFlag)
	if !*noPromptFlag && !confirm(ctx) {
		fmt.Println("Aborted; nothing was driven.")
		return
	}

	box, err := triggers.NewMMBTS(*deviceFlag, triggers.WithMMBTSMode(mode))
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer box.Close()
	fmt.Printf("\nMMBT-S on %s opened; all lines LOW.\n", *deviceFlag)

	switch {
	case *setFlag != "":
		runSet(ctx, box, mask)
	case *cycleFlag:
		runCycle(ctx, box, *lineFlag, *periodFlag)
	default:
		runSequence(ctx, box, *lineFlag, *widthFlag)
	}

	_ = box.AllLow()
	fmt.Println("\nDone; all lines LOW.")
}

func parseMode(s string) (triggers.MMBTSMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "p", "pulse":
		return triggers.MMBTSPulseMode, nil
	case "s", "simple":
		return triggers.MMBTSSimpleMode, nil
	}
	return 0, fmt.Errorf("%q is neither p (pulse) nor s (simple)", s)
}

func listPorts() {
	ports, err := triggers.AvailablePorts()
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
	fmt.Println("\nThe MMBT-S enumerates as a generic Arduino Micro (USB 2341:8037),")
	fmt.Println("usually /dev/ttyACM0 on Linux, /dev/tty.usbmodemXXXX on macOS, COMx on Windows.")
}

// describe prints the wiring and the assumption the run rests on, before
// anything is opened.
func describe(port string, mode triggers.MMBTSMode, line int) {
	fmt.Printf("About to drive the MMBT-S on %s.\n\n", port)
	fmt.Printf("  runtime mode assumed : %s — CHECK THE P/S SWITCH next to the USB-C socket.\n", mode)
	if mode == triggers.MMBTSPulseMode {
		fmt.Println("                         In pulse mode the firmware clears the port 8 ms after")
		fmt.Println("                         each byte: every pulse is 8 ms wide whatever is asked for.")
	} else {
		fmt.Println("                         In simple mode a byte latches until the next one; the")
		fmt.Println("                         host controls the width.")
	}
	fmt.Printf("  probe                : bit %d = D-Sub 25 pin %d; ground is any of pins 20-25\n", line, line+2)
	fmt.Println("  logic                : 5 V HIGH, 0 V LOW, 8 lines on pins 2-9")
	fmt.Println("  without an instrument: the green LED follows bit 1, so it lights on odd codes")
	fmt.Println("\nIf this box is connected to a recording that is running, these pulses will")
	fmt.Println("appear in it as trigger codes.")
}

// confirm asks before the first edge. It reports whether to go ahead.
func confirm(ctx context.Context) bool {
	fmt.Print("\nProceed? [y/N] ")
	answer := make(chan string, 1)
	go func() {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			answer <- ""
			return
		}
		answer <- strings.ToLower(strings.TrimSpace(line))
	}()
	select {
	case a := <-answer:
		return a == "y" || a == "yes"
	case <-ctx.Done():
		fmt.Println()
		return false
	}
}

// runSequence is the default run: whole-port patterns, then one line at a
// time, then a pulse.
func runSequence(ctx context.Context, box *triggers.MMBTS, line int, width time.Duration) {
	fmt.Println("\n--- Send: whole-port patterns (1 s each) ---")
	for _, m := range []byte{0x00, 0xFF, 0xAA, 0x55, 0x0F, 0xF0, 0x01, 0x80} {
		if ctx.Err() != nil {
			return
		}
		odd := "LED off"
		if m&0x01 != 0 {
			odd = "LED on"
		}
		fmt.Printf("  Send(0x%02X)  %08b  %s\n", m, m, odd)
		if err := box.Send(m); err != nil {
			log.Fatalf("Send(0x%02X): %v", m, err)
		}
		sleepCtx(ctx, time.Second)
	}

	fmt.Println("\n--- SetHigh / SetLow: one line at a time (0.5 s each) ---")
	if box.Mode() == triggers.MMBTSPulseMode {
		fmt.Println("  (in pulse mode the firmware drops each line after 8 ms, so the SetLow")
		fmt.Println("   that follows only confirms a line that is already LOW)")
	}
	for l := range 8 {
		if ctx.Err() != nil {
			return
		}
		fmt.Printf("  line %d (D-Sub 25 pin %d) HIGH", l, l+2)
		if err := box.SetHigh(l); err != nil {
			log.Fatalf("SetHigh(%d): %v", l, err)
		}
		sleepCtx(ctx, 500*time.Millisecond)
		if err := box.SetLow(l); err != nil {
			log.Fatalf("SetLow(%d): %v", l, err)
		}
		fmt.Println(" → LOW")
	}

	fmt.Printf("\n--- Pulse: 10 pulses on line %d, requested width %v ---\n", line, width)
	if box.Mode() == triggers.MMBTSPulseMode {
		fmt.Printf("  the firmware fixes the electrical width at %v; the requested width only\n", box.PulseWidth())
		fmt.Println("  sets how long the call blocks")
	}
	for i := range 10 {
		if ctx.Err() != nil {
			return
		}
		if err := box.Pulse(line, width); err != nil {
			log.Fatalf("Pulse(%d): %v", line, err)
		}
		fmt.Printf("  pulse %d/10\n", i+1)
		sleepCtx(ctx, 200*time.Millisecond)
	}
}

// runSet drives one mask and holds it until Ctrl-C.
func runSet(ctx context.Context, box *triggers.MMBTS, mask byte) {
	fmt.Printf("\nSend(0x%02X)  %08b — holding until Ctrl-C.\n", mask, mask)
	if box.Mode() == triggers.MMBTSPulseMode && mask != 0 {
		fmt.Printf("WARNING: in pulse mode the firmware clears the port after %v, so this mask\n", box.PulseWidth())
		fmt.Println("         will NOT be held. Set the switch to \"S\" and pass -mode s to latch it.")
	}
	if err := box.Send(mask); err != nil {
		log.Fatalf("Send(0x%02X): %v", mask, err)
	}
	<-ctx.Done()
}

// runCycle square-waves one line until Ctrl-C — the pattern to put a scope or
// a photodiode-latency capture on.
func runCycle(ctx context.Context, box *triggers.MMBTS, line int, period time.Duration) {
	half := period / 2
	fmt.Printf("\nSquare wave on line %d (D-Sub 25 pin %d), period %v — Ctrl-C to stop.\n", line, line+2, period)
	if box.Mode() == triggers.MMBTSPulseMode {
		fmt.Printf("WARNING: in pulse mode the firmware drops the line after %v, so the HIGH half\n", box.PulseWidth())
		fmt.Printf("         will be %v long, not %v. Use -mode s (switch at \"S\") for a real\n", box.PulseWidth(), half)
		fmt.Println("         square wave.")
	}
	n := 0
	for ctx.Err() == nil {
		if err := box.SetHigh(line); err != nil {
			log.Fatalf("SetHigh(%d): %v", line, err)
		}
		sleepCtx(ctx, half)
		if err := box.SetLow(line); err != nil {
			log.Fatalf("SetLow(%d): %v", line, err)
		}
		sleepCtx(ctx, half)
		n++
		if n%10 == 0 {
			fmt.Printf("  %d cycles\n", n)
		}
	}
	fmt.Printf("  stopped after %d cycles\n", n)
}

// sleepCtx sleeps for d, or returns early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}
