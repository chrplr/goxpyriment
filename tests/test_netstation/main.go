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
// Whatever happens — an error mid-run, or Ctrl-C — the recording is stopped and
// the ECI session ended before the program exits. A recording left open is what
// produces an .mff that cannot be reopened (Acquiring.xml present, info.xml
// missing).
//
// Usage:
//
//	go run ./tests/test_netstation -host 134.225.198.12
//	go run ./tests/test_netstation -host 134.225.198.12 -port 55513 -n 20
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
	hostFlag := flag.String("host", "", "NetStation host IP address (required, e.g. 134.225.198.12)")
	portFlag := flag.Int("port", 55513, "ECI TCP port")
	nFlag := flag.Int("n", 10, "number of numbered T<n> events to send")
	isiFlag := flag.Int("isi", 500, "inter-event interval in ms")
	flag.Parse()

	if *hostFlag == "" {
		log.Fatal("usage: go run ./tests/test_netstation -host <ip>")
	}

	// Ctrl-C unwinds normally so the deferred teardown still stops the
	// recording — never leave NetStation acquiring.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	addr := fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
	if err := run(ctx, addr, *nFlag, time.Duration(*isiFlag)*time.Millisecond); err != nil {
		log.Fatalf("test_netstation: %v", err)
	}
	fmt.Println("\nDone.")
}

// run owns the session. Every step returns an error rather than calling
// log.Fatal, because log.Fatal exits without running deferred functions — which
// would skip the teardown and leave the recording open on the host.
func run(ctx context.Context, addr string, nEvents int, isi time.Duration) (err error) {
	fmt.Printf("Connecting to NetStation at %s ...\n", addr)
	ns, err := triggers.NewNetStation(addr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() {
		fmt.Println("\n--- Close (stop recording if needed, end ECI session) ---")
		if cerr := ns.Close(); cerr != nil {
			// Report it even when the run itself succeeded: a failed stop
			// means the .mff on the host may be unusable.
			fmt.Printf("  Close: ERROR: %v\n", cerr)
			if err == nil {
				err = cerr
			}
			return
		}
		fmt.Println("  closed cleanly")
	}()
	fmt.Printf("  connected (ECI handshake OK, protocol version %d)\n", ns.ECIVersion())

	// --- Synchronize ---
	fmt.Println("\n--- Synchronize (align host clock) ---")
	if err := ns.Synchronize(); err != nil {
		return fmt.Errorf("Synchronize: %w", err)
	}
	fmt.Println("  Synchronize OK")

	// --- Start recording ---
	fmt.Println("\n--- StartRecording ---")
	if err := ns.StartRecording(); err != nil {
		return fmt.Errorf("StartRecording: %w", err)
	}
	fmt.Printf("  recording = %v\n", ns.Recording())
	time.Sleep(200 * time.Millisecond)

	// --- Plain event (now, 1 ms, no keys) ---
	fmt.Println("\n--- SendEvent(\"STIM\") ---")
	if err := ns.SendEvent("STIM"); err != nil {
		return fmt.Errorf("SendEvent: %w", err)
	}
	fmt.Println("  STIM sent")

	// --- Event with explicit onset and key/value payloads ---
	fmt.Println("\n--- SendEventFull(\"RESP\", keys corr=1 rt=423) ---")
	if err := ns.SendEventFull(triggers.Event{
		Code:     "RESP",
		Start:    time.Now(),
		Duration: 2 * time.Millisecond,
		Keys: []triggers.EventKey{
			{Code: "corr", Value: 1},
			{Code: "rt", Value: 423},
		},
	}); err != nil {
		return fmt.Errorf("SendEventFull: %w", err)
	}
	fmt.Println("  RESP sent")

	// --- A train of numbered events ---
	fmt.Printf("\n--- %d numbered events, %v apart ---\n", nEvents, isi)
	for i := 1; i <= nEvents; i++ {
		if ctx.Err() != nil {
			fmt.Println("\n  interrupted — stopping the recording cleanly")
			break
		}
		code := fmt.Sprintf("T%d", i) // padded/truncated to 4 chars by the driver
		if err := ns.SendEvent(code); err != nil {
			return fmt.Errorf("event %d (%q): %w", i, code, err)
		}
		fmt.Printf("  event %d: %q\n", i, code)
		select {
		case <-ctx.Done():
		case <-time.After(isi):
		}
	}

	// --- Stop recording ---
	fmt.Println("\n--- StopRecording ---")
	if err := ns.StopRecording(); err != nil {
		return fmt.Errorf("StopRecording: %w", err)
	}
	fmt.Printf("  recording = %v\n", ns.Recording())
	return nil
}
