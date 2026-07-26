// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_videorecorder exercises the BEL_video networked video recorder over
// TCP/IP via the goxpyriment triggers package.
//
// It connects, starts recording, sets the subject id, sends a train of
// trial/condition overlay labels, then stops and disconnects. There is no
// automated pass/fail: watch the recorder's preview window (the labels should
// appear burned into the video) and check the saved AVI afterwards.
//
// Usage:
//
//	go run ./tests/test_videorecorder -host 192.168.8.212
//	go run ./tests/test_videorecorder -host 192.168.8.212 -port 55113 -subject bb0012025 -n 5
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	hostFlag := flag.String("host", "", "video recorder host IP address (required, e.g. 192.168.8.212)")
	portFlag := flag.Int("port", 55113, "recorder TCP port")
	subjFlag := flag.String("subject", "test0001", "subject id (NIP) — names the output file")
	nFlag := flag.Int("n", 5, "number of trial labels to send")
	isiFlag := flag.Int("isi", 2000, "interval between trials in ms")
	flag.Parse()

	if *hostFlag == "" {
		log.Fatal("usage: go run ./tests/test_videorecorder -host <ip>")
	}

	addr := fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
	fmt.Printf("Connecting to video recorder at %s ...\n", addr)
	vr, err := triggers.NewVideoRecorder(addr)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer vr.Close()
	fmt.Println("  connected")

	// --- Start recording ---
	fmt.Println("\n--- Start ---")
	if err := vr.Start(); err != nil {
		log.Fatalf("  Start: %v", err)
	}
	fmt.Printf("  recording = %v\n", vr.Recording())

	// --- Subject id (names the file) ---
	fmt.Printf("\n--- SetSubject(%q) ---\n", *subjFlag)
	if err := vr.SetSubject(*subjFlag); err != nil {
		log.Printf("  SetSubject: ERROR: %v", err)
	} else {
		fmt.Println("  subject set")
	}

	// --- A train of trial/condition labels ---
	fmt.Printf("\n--- %d trials, %d ms apart ---\n", *nFlag, *isiFlag)
	isi := time.Duration(*isiFlag) * time.Millisecond
	for i := 1; i <= *nFlag; i++ {
		trl := fmt.Sprintf("%03d", i)
		cnd := fmt.Sprintf("%03d", (i%9)+1)
		if err := vr.Label("TRL", trl); err != nil {
			log.Printf("  TRL %s: ERROR: %v", trl, err)
		}
		if err := vr.Label("CND", cnd); err != nil {
			log.Printf("  CND %s: ERROR: %v", cnd, err)
		}
		fmt.Printf("  trial %d: TRL:%s CND:%s\n", i, trl, cnd)
		time.Sleep(isi)
	}

	// --- Stop recording ---
	fmt.Println("\n--- Stop ---")
	if err := vr.Stop(); err != nil {
		log.Printf("  Stop: ERROR: %v", err)
	}
	fmt.Printf("  recording = %v\n", vr.Recording())

	fmt.Println("\nDone. (Close() disconnects.)")
}
