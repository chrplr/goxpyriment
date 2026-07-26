// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_parallel_port exercises read and write on a Linux LPT parallel port
// via the ppdev kernel interface.
//
// Usage:
//
//	go run main.go /dev/parport0
//
// Prerequisites:
//
//	sudo modprobe ppdev
//	sudo usermod -aG lp $USER   # re-login afterwards
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <device>\n  e.g. %s /dev/parport0\n",
			os.Args[0], os.Args[0])
		fmt.Fprintf(os.Stderr, "\nAvailable ports: %v\n", triggers.AvailableParallelPorts())
		os.Exit(1)
	}
	device := os.Args[1]

	pp := triggers.NewParallelPort(device)
	if err := pp.Open(); err != nil {
		log.Fatalf("open: %v", err)
	}
	defer pp.Close()
	fmt.Printf("Opened %s\n", device)

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
