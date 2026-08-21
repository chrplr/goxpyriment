// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_parallel_port exercises read and write on a Linux LPT parallel port
// via the ppdev kernel interface, and helps find which physical DB25 pin a
// logical line comes out on.
//
// Usage:
//
//	go run ./tests/test_parallel_port /dev/parport0             # self-test
//	go run ./tests/test_parallel_port -find /dev/parport0       # walk D0..D7 slowly
//	go run ./tests/test_parallel_port -line 0 /dev/parport0     # hold one line HIGH
//	go run ./tests/test_parallel_port -blink 0 /dev/parport0    # toggle one line at 1 Hz
//
// # Finding the pin with a multimeter
//
// The self-test flips lines every 50 ms, which a meter reads as an average of
// nothing useful. The three modes above are slow on purpose.
//
// Put the black probe on any ground pin (18-25) and the red probe on a data pin.
// The data lines D0-D7 are DB25 pins 2-9 on a standard port, but the mapping is
// worth confirming rather than assuming — that is what -line is for: hold one
// line HIGH, then probe pins 2 through 9 until one reads about 5 V (3.3 V on
// some chipsets; either is a valid TTL high).
//
// -find is the other way round: leave the probe on one pin and watch the console
// as each line is held HIGH in turn.
//
// Prerequisites:
//
//	sudo modprobe ppdev
//	sudo usermod -aG lp $USER   # re-login afterwards
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chrplr/goxpyriment/tests/internal/safeexit"
	"github.com/chrplr/goxpyriment/triggers"
)

// db25 names the DB25 pin each data line is expected on, for a standard port.
// Printed as a hint, never as an assertion — confirming it against a meter is
// the whole point of these modes.
var db25 = [8]int{2, 3, 4, 5, 6, 7, 8, 9}

func main() {
	fFind := flag.Bool("find", false, "walk D0..D7, holding each HIGH in turn (probe one pin, watch the console)")
	fLine := flag.Int("line", -1, "hold this line (0-7) HIGH until interrupted (probe pins 2-9 to find it)")
	fBlink := flag.Int("blink", -1, "toggle this line (0-7) at 1 Hz until interrupted")
	fAll := flag.Bool("all", false, "drive ALL data lines HIGH — safe to probe with a clumsy meter, since bridging two pins shorts high to high")
	fHold := flag.Duration("hold", 3*time.Second, "how long -find holds each line HIGH")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <device>\n  e.g. %s /dev/parport0\n",
			os.Args[0], os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAvailable ports: %v\n", triggers.AvailableParallelPorts())
		os.Exit(1)
	}
	device := flag.Arg(0)

	pp := triggers.NewParallelPort(device)
	if err := pp.Open(); err != nil {
		log.Fatalf("open: %v", err)
	}
	defer pp.Close()
	fmt.Printf("Opened %s\n", device)

	// Leave the port LOW however the program exits, including Ctrl-C: a data
	// line left HIGH keeps whatever is downstream latched, and on a trigger rig
	// that means the next recording starts with a stuck trigger.
	// Through safeexit, not signal.Notify directly: AllLow and Close are ioctls
	// on the port, and a port that has stopped answering is exactly when the
	// operator reaches for Ctrl-C. Catching the signal and then blocking in the
	// driver leaves no way to stop the program at all — see safeexit's comment.
	safeexit.OnSignal(0, func() {
		_ = pp.AllLow()
		_ = pp.Close()
		fmt.Println("\nall lines LOW, port closed.")
	})

	if *fAll {
		if err := pp.Send(0xFF); err != nil {
			log.Fatalf("Send(0xFF): %v", err)
		}
		fmt.Println("\nAll eight data lines are HIGH (DB25 pins 2-9 on a standard port).")
		fmt.Println("Ground is any of pins 18-25. Every data pin should read the same,")
		fmt.Println("so bridging two of them with a wide probe is harmless here — use this")
		fmt.Println("to find the data block, then -find to identify a single line.")
		fmt.Println("Press Ctrl-C to stop; all lines are driven LOW on exit.")
		select {}
	}

	if *fFind || *fLine >= 0 || *fBlink >= 0 {
		fmt.Println("\nCAUTION: only one line is driven HIGH at a time here, so a probe that")
		fmt.Println("bridges two adjacent pins shorts an output high into an output low.")
		fmt.Println("Use -all while hunting for physical contact, then come back to this.")
		fmt.Println("\nGround is any of DB25 pins 18-25. A TTL high reads ~5 V (3.3 V on some chipsets).")
		fmt.Println("Press Ctrl-C to stop; all lines are driven LOW on exit.")
		_ = pp.AllLow()
		switch {
		case *fLine >= 0:
			if *fLine > 7 {
				log.Fatalf("-line %d out of range (0-7)", *fLine)
			}
			if err := pp.SetHigh(*fLine); err != nil {
				log.Fatalf("SetHigh(%d): %v", *fLine, err)
			}
			fmt.Printf("\nD%d is HIGH and will stay HIGH — expected on DB25 pin %d.\n",
				*fLine, db25[*fLine])
			fmt.Println("Probe pins 2,3,4,5,6,7,8,9 in turn; exactly one should read high.")
			select {} // until Ctrl-C
		case *fBlink >= 0:
			if *fBlink > 7 {
				log.Fatalf("-blink %d out of range (0-7)", *fBlink)
			}
			fmt.Printf("\nD%d toggling at 1 Hz — expected on DB25 pin %d.\n", *fBlink, db25[*fBlink])
			for high := true; ; high = !high {
				if high {
					_ = pp.SetHigh(*fBlink)
				} else {
					_ = pp.SetLow(*fBlink)
				}
				fmt.Printf("\r  D%d %-4s ", *fBlink, map[bool]string{true: "HIGH", false: "low"}[high])
				time.Sleep(500 * time.Millisecond)
			}
		default:
			fmt.Printf("\nWalking D0..D7, %v each. Leave the probe on ONE pin and note when it goes high.\n\n", *fHold)
			for {
				for line := 0; line <= 7; line++ {
					_ = pp.AllLow()
					if err := pp.SetHigh(line); err != nil {
						log.Printf("SetHigh(%d): %v", line, err)
						continue
					}
					fmt.Printf("  D%d HIGH   (expected DB25 pin %d)\n", line, db25[line])
					time.Sleep(*fHold)
				}
				fmt.Println("  ---- repeating ----")
			}
		}
	}

	// --- Data register write tests ---
	fmt.Println("\n--- Data register write tests ---")
	testValues := []byte{0x00, 0xFF, 0xAA, 0x55, 0x0F, 0xF0, 0x01, 0x80}
	for _, v := range testValues {
		if err := pp.Send(v); err != nil {
			log.Printf("Send(0x%02X): ERROR: %v", v, err)
			continue
		}
		fmt.Printf("Send(0x%02X)  OK   (binary: %08b)\n", v, v)
		time.Sleep(50 * time.Millisecond)
	}

	// --- Individual line tests ---
	fmt.Println("\n--- Individual line (SetHigh/SetLow) tests ---")
	if err := pp.AllLow(); err != nil {
		log.Printf("AllLow: %v", err)
	}
	for line := 0; line <= 7; line++ {
		if err := pp.SetHigh(line); err != nil {
			log.Printf("SetHigh(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("SetHigh(%d)  OK\n", line)
		time.Sleep(50 * time.Millisecond)
		if err := pp.SetLow(line); err != nil {
			log.Printf("SetLow(%d): ERROR: %v", line, err)
		}
	}

	// --- Pulse test ---
	fmt.Println("\n--- Pulse test (line 0, 100 ms) ---")
	if err := pp.Pulse(0, 100*time.Millisecond); err != nil {
		log.Printf("Pulse: ERROR: %v", err)
	} else {
		fmt.Println("Pulse(0, 100ms)  OK")
	}

	// --- Status register read ---
	fmt.Println("\n--- Status register read ---")
	status, err := pp.ReadStatus()
	if err != nil {
		log.Printf("ReadStatus: ERROR: %v", err)
	} else {
		fmt.Printf("Status register: 0x%02X  (binary: %08b)\n", status, status)
		fmt.Printf("  nACK      (bit 6): %d\n", (status>>6)&1)
		fmt.Printf("  BUSY      (bit 7): %d\n", (status>>7)&1)
		fmt.Printf("  PAPER-OUT (bit 5): %d\n", (status>>5)&1)
		fmt.Printf("  SELECT    (bit 4): %d\n", (status>>4)&1)
		fmt.Printf("  nERROR    (bit 3): %d\n", (status>>3)&1)
	}

	// Leave all lines LOW on exit.
	if err := pp.AllLow(); err != nil {
		log.Printf("AllLow on exit: %v", err)
	}
	fmt.Println("\nDone.")
}
