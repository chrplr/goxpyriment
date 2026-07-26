// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_ft232h exercises the Adafruit FT232H breakout as an 8-bit TTL trigger
// device via the goxpyriment triggers package.
//
// It tests:
//   - Output: Send, SetHigh/SetLow, Pulse — on AD0–AD7 (D-bus)
//   - Input:  ReadAll, ReadLine           — on AC0–AC7 (C-bus)
//
// Wiring for a self-loopback smoke-test: connect AD0→AC0, AD1→AC1, …, AD7→AC7.
// Without any wiring the output tests still exercise the device; the input
// readings will simply reflect the undriven (floating) state of the C-bus pins.
//
// Usage:
//
//	go run main.go
//
// Prerequisites:
//
//	sudo rmmod ftdi_sio          # detach kernel UART driver if loaded
//	sudo usermod -aG plugdev $USER  # grant access to /dev/bus/usb; re-login
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	period := flag.Duration("period", 100*time.Millisecond, "square wave period")
	duty := flag.Float64("duty", 10.0, "duty cycle in percent (0–100)")
	flag.Parse()

	dev, err := triggers.NewFT232H()
	if err != nil {
		log.Fatalf("open FT232H: %v", err)
	}
	defer dev.Close()
	fmt.Println("FT232H opened successfully.")

	// --- Output: Send (bitmask) ---
	fmt.Println("\n--- Send (byte bitmask → AD0–AD7) ---")
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
	fmt.Println("\n--- ReadAll (AC0–AC7 input bitmask) ---")
	mask, err := dev.ReadAll()
	if err != nil {
		log.Printf("  ReadAll: ERROR: %v", err)
	} else {
		fmt.Printf("  ReadAll → 0x%02X  %08b\n", mask, mask)
	}

	// --- Input: ReadLine ---
	fmt.Println("\n--- ReadLine (individual AC0–AC7 lines) ---")
	for line := 0; line <= 7; line++ {
		v, err := dev.ReadLine(line)
		if err != nil {
			log.Printf("  ReadLine(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  ReadLine(%d) → %d\n", line, v)
	}

	// --- Input: WaitForInput (2-second window) ---
	fmt.Println("\n--- WaitForInput (waiting 2 s for any AC0–AC7 input) ---")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	imask, rt, err := dev.WaitForInput(ctx)
	if err != nil {
		fmt.Printf("  no input detected (timeout or cancel): %v\n", err)
	} else {
		fmt.Printf("  input detected: mask=0x%02X  rt=%v\n", imask, rt)
	}

	// --- Square wave ---
	highDur := time.Duration(float64(*period) * *duty / 100.0)
	fmt.Printf("\n--- Square wave on AD0: period=%v duty=%.1f%% high=%v low=%v (Ctrl-C to stop) ---\n",
		*period, *duty, highDur, *period-highDur)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)

	_ = dev.AllLow()
	ticker := time.NewTicker(*period)
	defer ticker.Stop()
squareWave:
	for {
		select {
		case <-stop:
			break squareWave
		case <-ticker.C:
			if err := dev.Pulse(0, highDur); err != nil {
				log.Printf("Pulse: %v", err)
				break squareWave
			}
		}
	}

	_ = dev.AllLow()
	fmt.Println("\nDone.")
}
