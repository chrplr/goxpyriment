// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_netstation exercises an EGI/NetStation EEG host over TCP/IP (the ECI
// protocol) via the goxpyriment triggers package.
//
// It walks the full session: connect + handshake, clock synchronize, start
// recording, send several event markers (plain, with an explicit onset time,
// and with key/value payloads), then stop recording and disconnect. There is
// no automated pass/fail — watch the NetStation host: the recording should
// start, the STIM/RESP/T<n> events should appear on the timeline, and it should
// stop cleanly.
//
// Usage:
//
//	go run ./tests/test_netstation -host 134.225.198.12
//	go run ./tests/test_netstation -host 134.225.198.12 -port 55513 -n 20
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	hostFlag := flag.String("host", "", "NetStation host IP address (required, e.g. 134.225.198.12)")
	portFlag := flag.Int("port", 55513, "ECI TCP port")
	nFlag := flag.Int("n", 10, "number of numbered T<n> events to send")
	isiFlag := flag.Int("isi", 500, "inter-event interval in ms")
	flag.Parse()

	if *hostFlag == "" {
		log.Fatal("usage: go run ./tests/test_netstation -host <ip>")
	}

	addr := fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
	fmt.Printf("Connecting to NetStation at %s ...\n", addr)
	ns, err := triggers.NewNetStation(addr)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer ns.Close()
	fmt.Println("  connected (ECI handshake OK)")

	// --- Synchronize ---
	fmt.Println("\n--- Synchronize (align host clock) ---")
	if err := ns.Synchronize(); err != nil {
		log.Fatalf("  Synchronize: %v", err)
	}
	fmt.Println("  Synchronize OK")

	// --- Start recording ---
	fmt.Println("\n--- StartRecording ---")
	if err := ns.StartRecording(); err != nil {
		log.Fatalf("  StartRecording: %v", err)
	}
	fmt.Printf("  recording = %v\n", ns.Recording())
	time.Sleep(200 * time.Millisecond)

	// --- Plain event (now, 1 ms, no keys) ---
	fmt.Println("\n--- SendEvent(\"STIM\") ---")
	if err := ns.SendEvent("STIM"); err != nil {
		log.Printf("  SendEvent: ERROR: %v", err)
	} else {
		fmt.Println("  STIM sent")
	}

	// --- Event with explicit onset and key/value payloads ---
	fmt.Println("\n--- SendEventFull(\"RESP\", keys corr=1 rt=423) ---")
	respErr := ns.SendEventFull(triggers.Event{
		Code:     "RESP",
		Start:    time.Now(),
		Duration: 2 * time.Millisecond,
		Keys: []triggers.EventKey{
			{Code: "corr", Value: 1},
			{Code: "rt", Value: 423},
		},
	})
	if respErr != nil {
		log.Printf("  SendEventFull: ERROR: %v", respErr)
	} else {
		fmt.Println("  RESP sent")
	}

	// --- A train of numbered events ---
	fmt.Printf("\n--- %d numbered events, %d ms apart ---\n", *nFlag, *isiFlag)
	isi := time.Duration(*isiFlag) * time.Millisecond
	for i := 1; i <= *nFlag; i++ {
		code := fmt.Sprintf("T%d", i) // padded/truncated to 4 chars by the driver
		if err := ns.SendEvent(code); err != nil {
			log.Printf("  event %d (%q): ERROR: %v", i, code, err)
		} else {
			fmt.Printf("  event %d: %q\n", i, code)
		}
		time.Sleep(isi)
	}

	// --- Stop recording ---
	fmt.Println("\n--- StopRecording ---")
	if err := ns.StopRecording(); err != nil {
		log.Printf("  StopRecording: ERROR: %v", err)
	}
	fmt.Printf("  recording = %v\n", ns.Recording())

	fmt.Println("\nDone. (Close() will disconnect the ECI session.)")
}
