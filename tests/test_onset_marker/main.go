// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_onset_marker checks the library's onset marker (control.MarkerConfig,
// exp.Marker) on a real display, in software: what the photodiode square looks
// like in every presented frame, and when the TTL codes are written and
// cleared relative to the flips.
//
// The TTL device is a logging stand-in that records each write with its SDL
// timestamp; the square is read back from the finished frame just before each
// present (Screen.SetPrePresentHook — after present SDL has invalidated the
// backbuffer). So this measures the software path only: the latency of a real
// device and the photons need the hardware (a scope, a photodiode).
//
//	go run ./tests/test_onset_marker          # windowed
//	go run ./tests/test_onset_marker -f       # fullscreen
//
// Each check prints PASS or FAIL with the measured value, and is written to
// the data file. The run takes a few seconds and needs no input.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// wedgesGV is a 33-frame, 30 fps .gv clip (a copy of
// examples/demo_play_gv_videos/wedges.gv). At 60 Hz each of its frames is held
// for two refreshes, the case where marking a frame from PlayGvFunc's
// post-flip callback would land a refresh late.
//
//go:embed wedges.gv
var wedgesGV []byte

// write is one TTL write seen by the logging device.
type write struct {
	code byte
	ns   uint64
}

// logTTL records every write instead of driving hardware. The marker writes
// from two goroutines (codes on the flip thread, clears on its timer), hence
// the mutex.
type logTTL struct {
	mu     sync.Mutex
	writes []write
}

func (l *logTTL) Send(code byte) error {
	ns := control.TicksNS()
	l.mu.Lock()
	l.writes = append(l.writes, write{code, ns})
	l.mu.Unlock()
	return nil
}
func (l *logTTL) AllLow() error { return nil }
func (l *logTTL) Close() error  { return nil }

func (l *logTTL) take() []write {
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.writes
	l.writes = nil
	return w
}

// frameLog records, for each presented frame, whether the corner square was
// white.
type frameLog struct {
	on     bool
	whites []bool
}

func (f *frameLog) take() []bool {
	w := f.whites
	f.whites = nil
	return w
}

const (
	frames  = 3
	codeTol = 1 * time.Millisecond // code write after the flip
	clrTol  = 5 * time.Millisecond // timer slack on the clear
)

// whiteAt lists the indices of the white frames, for a readable report.
func whiteAt(frames []bool) []int {
	var idx []int
	for i, w := range frames {
		if w {
			idx = append(idx, i)
		}
	}
	return idx
}

func main() {
	fullscreen := flag.Bool("f", false, "fullscreen instead of a 1024x768 window")
	flag.Parse()

	dev := &logTTL{}
	exp := control.NewExperiment("test_onset_marker", 1024, 768, *fullscreen, control.Gray, control.White, 32)
	exp.MarkerConfig = control.MarkerConfig{Photodiode: true, Frames: frames, TTLDevice: dev}
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()
	exp.AddDataVariableNames([]string{"check", "measured", "expected", "pass"})

	fl := &frameLog{}
	exp.Screen.SetPrePresentHook(func() error {
		if !fl.on {
			return nil
		}
		surf, err := exp.Screen.Renderer.ReadPixels(nil)
		if err != nil {
			return err
		}
		defer surf.Destroy()
		r, g, b, _, err := surf.ReadPixel(3, 3)
		if err != nil {
			return err
		}
		fl.whites = append(fl.whites, r == 255 && g == 255 && b == 255)
		return nil
	})

	dur := exp.Marker.TTLDuration()
	log.Printf("marker: %d frames, TTL cleared after %v", frames, dur)

	failed := 0
	check := func(name string, ok bool, measured, expected string) {
		verdict := "PASS"
		if !ok {
			verdict = "FAIL"
			failed++
		}
		fmt.Printf("%s  %-46s measured %-28s expected %s\n", verdict, name, measured, expected)
		exp.Data.Add(name, measured, expected, ok)
	}
	// codeAndClear checks one onset's code write against its flip, and the
	// clear that follows it.
	codeAndClear := func(name string, w []write, code byte, onsetNS uint64) {
		if len(w) < 2 || w[0].code != code || w[1].code != 0 {
			check(name+": writes", false, fmt.Sprint(w), fmt.Sprintf("[%d 0]", code))
			return
		}
		gap := time.Duration(w[0].ns - onsetNS)
		check(name+": code after flip", gap >= 0 && gap < codeTol, gap.String(), "0 ≤ gap < "+codeTol.String())
		width := time.Duration(w[1].ns - w[0].ns)
		check(name+": cleared after", width >= dur && width < dur+clrTol, width.String(),
			fmt.Sprintf("%v .. %v", dur, dur+clrTol))
	}
	settle := func() error { return exp.Wait(int(2*dur/time.Millisecond) + 20) }

	word := stimuli.NewTextLine("marker", 0, 0, control.White)

	err := exp.Run(func() error {
		if err := exp.Blank(200); err != nil {
			return err
		}
		fl.on = true

		// 1. Arm + ShowTS: the square is white on the onset flip and the next
		// two, then black, and ShowTS holds until it is (frames+1 presents).
		dev.take()
		fl.take()
		exp.Marker.Arm(7)
		onset, err := exp.ShowTS(word)
		if err != nil {
			return err
		}
		if err := settle(); err != nil {
			return err
		}
		got := fl.take()
		check("ShowTS: frames presented", len(got) == frames+1, fmt.Sprint(len(got)), fmt.Sprint(frames+1))
		check("ShowTS: square per frame", fmt.Sprint(got) == fmt.Sprint([]bool{true, true, true, false}),
			fmt.Sprint(got), "[true true true false]")
		codeAndClear("ShowTS", dev.take(), 7, onset)

		// 2. ShowFrames never holds: exactly n presents, marker or not.
		exp.Marker.Arm(0)
		if _, err := exp.ShowFrames(word, 2); err != nil {
			return err
		}
		got = fl.take()
		check("ShowFrames(2): frames presented", len(got) == 2, fmt.Sprint(len(got)), "2")
		if _, err := exp.BlankFrames(frames); err != nil {
			return err
		}
		fl.take()
		check("Arm(0): no TTL write", len(dev.take()) == 0, "", "no write")

		// 3. A stream armed from its FrameCallback: one code per element,
		// written right after the flip logged as TimingLog.OnsetNS.
		texts := []stimuli.Stimulus{
			stimuli.NewTextLine("one", 0, 0, control.White),
			stimuli.NewTextLine("two", 0, 0, control.White),
			stimuli.NewTextLine("three", 0, 0, control.White),
		}
		elems := stimuli.MakeRegularStream(texts, 150*time.Millisecond, 250*time.Millisecond)
		cb := func(ctx stimuli.FrameContext) error {
			if ctx.OnPhase && ctx.Frame == 0 {
				exp.Marker.Arm(byte(ctx.Index + 1))
			}
			return nil
		}
		_, timing, err := stimuli.PresentStreamOfStimuliFunc(exp.Screen, elems, 0, 0, cb)
		if err != nil {
			return err
		}
		if err := settle(); err != nil {
			return err
		}
		w := dev.take()
		for i, tl := range timing {
			if 2*i+1 < len(w) {
				codeAndClear(fmt.Sprintf("stream element %d", i), w[2*i:2*i+2], byte(i+1), tl.OnsetNS)
			} else {
				check(fmt.Sprintf("stream element %d: writes", i), false, fmt.Sprint(w), "a code and a clear")
			}
		}
		whites := 0
		for _, white := range fl.take() {
			if white {
				whites++
			}
		}
		check("stream: white frames", whites == frames*len(texts), fmt.Sprint(whites), fmt.Sprint(frames*len(texts)))

		// 4. Fire: a code now, no square.
		before := control.TicksNS()
		ts := exp.Marker.Fire(9)
		if err := exp.Blank(int(2*dur/time.Millisecond) + 20); err != nil {
			return err
		}
		w = dev.take()
		check("Fire: write", len(w) == 2 && w[0].code == 9 && w[1].code == 0, fmt.Sprint(w), "[9 0]")
		check("Fire: timestamp", ts >= before && ts <= control.TicksNS(), fmt.Sprint(ts), "between the calls")
		whites = 0
		for _, white := range fl.take() {
			if white {
				whites++
			}
		}
		check("Fire: no square", whites == 0, fmt.Sprint(whites), "0 white frames")

		// 5. Two onsets one frame apart: the flashes merge, and the second
		// code goes out while the first is still on the lines.
		st0 := exp.Marker.Stats()
		exp.Marker.Arm(1)
		if _, err := exp.ShowFrames(word, 1); err != nil {
			return err
		}
		exp.Marker.Arm(2)
		if _, err := exp.ShowTS(word); err != nil {
			return err
		}
		if err := settle(); err != nil {
			return err
		}
		st := exp.Marker.Stats()
		check("back-to-back: merged", st.Merged-st0.Merged == 1, fmt.Sprint(st.Merged-st0.Merged), "1")
		check("back-to-back: overlaps", st.Overlaps-st0.Overlaps == 1, fmt.Sprint(st.Overlaps-st0.Overlaps), "1")
		w = dev.take()
		check("back-to-back: one clear, after the second code",
			len(w) == 3 && w[0].code == 1 && w[1].code == 2 && w[2].code == 0, fmt.Sprint(w), "[1 2 0]")
		// 6. A .gv video with frames marked through PlayGvFunc: the square and
		// the code land on each marked frame's own first refresh, and a mark
		// on the last frame is completed after the video ends.
		gvPath := filepath.Join(os.TempDir(), fmt.Sprintf("test_onset_marker_%d.gv", os.Getpid()))
		if err := os.WriteFile(gvPath, wedgesGV, 0o600); err != nil {
			return err
		}
		defer os.Remove(gvPath)
		const gvFrames = 33
		marked := []stimuli.GvMark{{Frame: 0, Code: 1}, {Frame: 10, Code: 2}, {Frame: gvFrames - 1, Code: 3}}
		dev.take()
		fl.take()
		_, logs, err := stimuli.PlayGvFunc(exp.Screen, gvPath, 0, 0, nil, marked...)
		if err != nil {
			return err
		}
		if err := settle(); err != nil {
			return err
		}
		hold := int(math.Round(float64(time.Second) / float64(exp.Screen.FrameDuration()) / 30))
		w = dev.take()
		got = fl.take()
		check("gv: frames logged", len(logs) == gvFrames, fmt.Sprint(len(logs)), fmt.Sprint(gvFrames))
		for k, mk := range marked {
			if 2*k+1 < len(w) && mk.Frame < len(logs) {
				codeAndClear(fmt.Sprintf("gv frame %d (hold %d)", mk.Frame, hold), w[2*k:2*k+2], mk.Code, logs[mk.Frame].FlipNS)
			} else {
				check(fmt.Sprintf("gv frame %d: writes", mk.Frame), false, fmt.Sprint(w), "a code and a clear")
			}
		}
		// Expected square per presented refresh: white for `frames` refreshes
		// from the first refresh of each marked frame, then black; the video
		// is followed by just enough refreshes of its last frame to blacken
		// the square.
		want := make([]bool, gvFrames*hold)
		for _, mk := range marked {
			for r := 0; r < frames; r++ {
				if i := mk.Frame*hold + r; i < len(want) {
					want[i] = true
				} else {
					want = append(want, true)
				}
			}
		}
		want = append(want, false)
		check("gv: square per refresh", fmt.Sprint(got) == fmt.Sprint(want),
			fmt.Sprintf("%d refreshes, white at %v", len(got), whiteAt(got)),
			fmt.Sprintf("%d refreshes, white at %v", len(want), whiteAt(want)))

		fl.on = false
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("test_onset_marker: %v", err)
	}
	if failed > 0 {
		fmt.Printf("\n%d check(s) FAILED\n", failed)
		exp.End()
		os.Exit(1)
	}
	fmt.Println("\nall checks passed")
}
