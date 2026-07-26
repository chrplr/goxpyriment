// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_linuxgpio exercises a LinuxGPIOTrigger (Linux GPIO character device v2)
// as an 8-bit TTL trigger device via the goxpyriment triggers package.
//
// It tests:
//   - Output: Send, SetHigh/SetLow, Pulse — on the configured output pins
//   - Input:  ReadAll, ReadLine           — on the configured input pins (optional)
//
// Wiring for a self-loopback smoke-test: connect out[0]→in[0], …, out[7]→in[7].
// Without loopback wiring the output tests still exercise the device; the input
// readings will reflect the undriven (pull-down/floating) state of the pins.
//
// Usage:
//
//	# Raspberry Pi defaults (BCM pin numbers):
//	go run main.go
//
//	# Custom pins:
//	go run main.go -out 17,27,22,5,6,13,19,26 -in 12,16,20,21,4,25,24,23
//
//	# Output only (no input test):
//	go run main.go -out 17,27,22,5,6,13,19,26
//
//	# Different chip:
//	go run main.go -chip /dev/gpiochip4
//
// Prerequisites:
//
//	# User must have rw access to /dev/gpiochip0:
//	sudo usermod -aG gpio $USER   # re-login afterwards
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	chipFlag := flag.String("chip", "/dev/gpiochip0", "GPIO chip device path")
	outFlag := flag.String("out", "17,27,22,5,6,13,19,26", "8 output pin numbers (BCM), comma-separated")
	inFlag := flag.String("in", "", "8 input pin numbers (BCM), comma-separated (omit for output-only test)")
	flag.Parse()

	outPins, err := parsePins(*outFlag)
	if err != nil {
		log.Fatalf("invalid -out: %v", err)
	}

	opts := []triggers.LinuxGPIOOption{
		triggers.WithGPIOChip(*chipFlag),
		triggers.WithGPIOOutputPins(outPins),
	}
	fmt.Printf("Output pins: %v\n", outPins)

	var hasInput bool
	if *inFlag != "" {
		inPins, err := parsePins(*inFlag)
		if err != nil {
			log.Fatalf("invalid -in: %v", err)
		}
		opts = append(opts, triggers.WithGPIOInputPins(inPins))
		fmt.Printf("Input  pins: %v\n", inPins)
		hasInput = true
	} else {
		fmt.Println("Input  pins: (not configured — skipping input tests)")
	}

	dev, err := triggers.NewLinuxGPIOTrigger(opts...)
	if err != nil {
		log.Fatalf("open LinuxGPIOTrigger: %v", err)
	}
	defer dev.Close()
	fmt.Printf("GPIO chip %s opened successfully.\n", *chipFlag)

	// --- Output: Send (bitmask) ---
	fmt.Println("\n--- Send (byte bitmask → output pins) ---")
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

	if !hasInput {
		_ = dev.AllLow()
		fmt.Println("\nDone.")
		return
	}

	// --- Input: ReadAll ---
	fmt.Println("\n--- ReadAll (input bitmask) ---")
	mask, err := dev.ReadAll()
	if err != nil {
		log.Printf("  ReadAll: ERROR: %v", err)
	} else {
		fmt.Printf("  ReadAll → 0x%02X  %08b\n", mask, mask)
	}

	// --- Input: ReadLine ---
	fmt.Println("\n--- ReadLine (individual input lines) ---")
	for line := 0; line <= 7; line++ {
		v, err := dev.ReadLine(line)
		if err != nil {
			log.Printf("  ReadLine(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  ReadLine(%d) → %d\n", line, v)
	}

	// --- Input: WaitForInput (2-second window) ---
	fmt.Println("\n--- WaitForInput (waiting 2 s for any input) ---")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	imask, rt, err := dev.WaitForInput(ctx)
	if err != nil {
		fmt.Printf("  no input detected (timeout or cancel): %v\n", err)
	} else {
		fmt.Printf("  input detected: mask=0x%02X  rt=%v\n", imask, rt)
	}

	_ = dev.AllLow()
	fmt.Println("\nDone.")
}

// parsePins parses a comma-separated list of exactly 8 integer pin numbers.
func parsePins(s string) ([8]int, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 8 {
		return [8]int{}, fmt.Errorf("expected 8 comma-separated pin numbers, got %d", len(parts))
	}
	var pins [8]int
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return [8]int{}, fmt.Errorf("pin %d: %w", i, err)
		}
		pins[i] = n
	}
	return pins, nil
}
