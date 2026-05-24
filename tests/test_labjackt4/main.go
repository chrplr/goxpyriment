// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// test_labjackt4 exercises a LabJack T4 as an 8-bit TTL trigger device via
// the goxpyriment triggers package.
//
// It tests:
//   - Output: Send, SetHigh/SetLow, Pulse — on FIO0–FIO7
//   - Input:  ReadAll, ReadLine, WaitForInput — on EIO0–EIO7
//
// Wiring for a self-loopback smoke-test: connect FIO0→EIO0, …, FIO7→EIO7.
// Without loopback wiring the output tests still exercise the device; the
// input readings will reflect the undriven state of the EIO pins.
//
// Usage:
//
//	go run main.go -host 192.168.1.100
//	go run main.go -host 192.168.1.100 -port 502
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	hostFlag := flag.String("host", "", "LabJack T4 IP address (required, e.g. 192.168.1.100)")
	portFlag := flag.Int("port", 502, "Modbus TCP port")
	flag.Parse()

	if *hostFlag == "" {
		log.Fatal("usage: go run main.go -host <ip>")
	}

	addr := fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
	dev, err := triggers.NewLabJackT4(addr)
	if err != nil {
		log.Fatalf("open LabJackT4: %v", err)
	}
	defer dev.Close()
	fmt.Printf("LabJack T4 at %s opened successfully.\n", addr)

	// --- Output: Send (bitmask) ---
	fmt.Println("\n--- Send (byte bitmask → FIO0–FIO7) ---")
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
	fmt.Println("\n--- ReadAll (EIO0–EIO7 input bitmask) ---")
	mask, err := dev.ReadAll()
	if err != nil {
		log.Printf("  ReadAll: ERROR: %v", err)
	} else {
		fmt.Printf("  ReadAll → 0x%02X  %08b\n", mask, mask)
	}

	// --- Input: ReadLine ---
	fmt.Println("\n--- ReadLine (individual EIO0–EIO7 lines) ---")
	for line := 0; line <= 7; line++ {
		v, err := dev.ReadLine(line)
		if err != nil {
			log.Printf("  ReadLine(%d): ERROR: %v", line, err)
			continue
		}
		fmt.Printf("  ReadLine(%d) → %d\n", line, v)
	}

	// --- Input: WaitForInput (2-second window) ---
	fmt.Println("\n--- WaitForInput (waiting 2 s for any EIO0–EIO7 input) ---")
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
