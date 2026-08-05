// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Pictures-RSVP is a one-back task using Rapid Serial Visual Presentation.
//
// Every image in the images/ subfolder is shown once, in a fresh random order,
// for 200 ms each, separated by 100 ms of blank screen (a 300 ms SOA). About
// 10% of the presentations are immediate repeats of the preceding image (the
// one-back targets); the participant presses SPACE whenever an image is
// identical to the one just before it. When a repeat is missed (no SPACE
// within the response window), a buzzer sounds as soon as the window lapses.
//
// The presentation loop is VSYNC-locked with GC disabled, so image durations
// are rounded to the nearest whole frame. Per-image onset/offset and response
// timing are written to the data file, and a hit/false-alarm summary is printed.
//
// Usage:
//
//	go run main.go -w -s 1
//
// Flags: -w windowed mode, -d N display index, -s N subject ID, -size N
//
//	bounding-box side in px, -repeat F target proportion (0–1),
//	-window N response window in ms.
//
// Press ESC (or close the window) to abort.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	durationOn  = 200 * time.Millisecond // image visible
	durationOff = 100 * time.Millisecond // blank screen between images
)

func main() {
	boxSize := flag.Float64("size", 400, "bounding-box side in px; images are scaled to fit, preserving aspect ratio")
	repeatProp := flag.Float64("repeat", 0.10, "proportion of presentations that are one-back repeats (0–1)")
	windowMs := flag.Int("window", 1000, "response window in ms after a repeat's onset")

	exp := control.NewExperimentFromFlags(
		"Pictures RSVP (one-back)",
		control.Black, control.White, 32,
	)
	defer exp.End()

	// Collect every image in the images/ subfolder.
	var files []string
	for _, pattern := range []string{"*.jpg", "*.jpeg", "*.png", "*.bmp"} {
		matches, err := filepath.Glob(filepath.Join("images", pattern))
		if err != nil {
			log.Fatalf("globbing %s: %v", pattern, err)
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		log.Fatal("no images found in images/ (expected .jpg/.jpeg/.png/.bmp)")
	}

	// Present them in a fresh random order.
	rand.Shuffle(len(files), func(i, j int) {
		files[i], files[j] = files[j], files[i]
	})

	// Build the one-back sequence: walk the unique images in their shuffled
	// order and, for a random ~repeatProp fraction of them, show the image a
	// second time in a row — that duplicate is a one-back target. Targets are
	// chosen among non-first images so each repeat has a preceding image.
	nTargets := int(math.Round(*repeatProp * float64(len(files))))
	if nTargets < 1 && *repeatProp > 0 {
		nTargets = 1
	}
	if nTargets > len(files)-1 {
		nTargets = len(files) - 1
	}
	isTargetAfter := make([]bool, len(files)) // duplicate the image at this index?
	for _, idx := range rand.Perm(len(files) - 1)[:nTargets] {
		isTargetAfter[idx+1] = true // skip index 0 so the stream never opens on a repeat
	}

	var seqFiles []string
	var seqIsRepeat []bool
	for i, f := range files {
		seqFiles = append(seqFiles, f)
		seqIsRepeat = append(seqIsRepeat, false)
		if isTargetAfter[i] {
			seqFiles = append(seqFiles, f)
			seqIsRepeat = append(seqIsRepeat, true)
		}
	}

	// Load each picture and scale it to fit inside a common boxSize×boxSize
	// square, preserving aspect ratio so nothing is distorted. Preloading here
	// populates each picture's native Width/Height, which we then override, and
	// uploads the texture to the GPU so the loop never stalls on first draw.
	pics := make([]stimuli.VisualStimulus, len(seqFiles))
	for i, path := range seqFiles {
		pic := stimuli.NewPicture(path, 0, 0)
		if err := stimuli.PreloadVisualOnScreen(exp.Screen, pic); err != nil {
			log.Fatalf("loading %s: %v", path, err)
		}
		scale := float32(*boxSize) / max(pic.Width, pic.Height)
		pic.Width *= scale
		pic.Height *= scale
		pics[i] = pic
	}

	// Preload the buzzer once so it can be fired (non-blocking) on a miss.
	buzzer := stimuli.NewSoundFromMemory(assets_embed.BuzzerWav)
	if err := buzzer.PreloadDevice(exp.AudioDevice); err != nil {
		log.Fatalf("loading buzzer: %v", err)
	}

	exp.AddDataVariableNames([]string{"position", "filename", "is_repeat", "responded", "rt_ms"})

	exp.ShowInstructions(fmt.Sprintf(
		"One-Back Picture Task\n\n"+
			"%d pictures will flash by, one every %d ms.\n\n"+
			"Press SPACE whenever a picture is IDENTICAL\n"+
			"to the one shown immediately before it.\n"+
			"A buzzer sounds if you miss a repeat.\n\n"+
			"Press SPACE to begin (ESC to abort).",
		len(seqFiles), (durationOn + durationOff).Milliseconds(),
	))

	res := runOneBack(exp.Screen, pics, seqIsRepeat, buzzer, time.Duration(*windowMs)*time.Millisecond)

	var hits, misses, falseAlarms, nShown int
	for i := range res.onsetNS {
		exp.Data.Add(i+1, filepath.Base(seqFiles[i]), seqIsRepeat[i], res.responded[i], res.rtMs[i])
		switch {
		case seqIsRepeat[i]:
			nShown++
			if res.responded[i] {
				hits++
			} else {
				misses++
			}
		case res.responded[i]:
			falseAlarms++
		}
	}

	log.Printf("presented %d images (%d one-back targets)", len(res.onsetNS), nShown)
	log.Printf("hits %d/%d, misses %d, false alarms %d", hits, nShown, misses, falseAlarms)
}

// oneBackResult holds the per-position outcome of the presentation loop.
type oneBackResult struct {
	onsetNS   []uint64 // SDL flip timestamp of each image's onset
	responded []bool   // a SPACE landed in this image's response window
	rtMs      []int64  // ms from onset to the response, or -1 if none
}

// runOneBack presents the stimuli VSYNC-locked, one image at a time, scoring
// SPACE presses against the one-back targets and sounding the buzzer the moment
// a target's response window lapses without a response. The buzzer is fired via
// the non-blocking Sound.Play() so it never stalls the presentation loop.
func runOneBack(screen *apparatus.Screen, pics []stimuli.VisualStimulus, isRepeat []bool, buzzer *stimuli.Sound, window time.Duration) oneBackResult {
	n := len(pics)
	res := oneBackResult{
		onsetNS:   make([]uint64, n),
		responded: make([]bool, n),
		rtMs:      make([]int64, n),
	}
	for i := range res.rtMs {
		res.rtMs[i] = -1
	}
	windowNS := uint64(window.Nanoseconds())

	// Frame timing from the display's actual refresh rate.
	refreshRate := float32(60.0)
	displayID := sdl.GetDisplayForWindow(screen.Window)
	if mode, err := displayID.CurrentDisplayMode(); err == nil && mode != nil && mode.RefreshRate > 0 {
		refreshRate = mode.RefreshRate
	}
	frameDuration := time.Duration(float64(time.Second) / float64(refreshRate))
	framesOn := frameCount(durationOn, frameDuration)
	framesOff := frameCount(durationOff, frameDuration)

	// Disable GC during the timed loop to avoid jitter (restored on return).
	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	// nextEval is the index of the earliest target not yet finalised as
	// hit-or-miss; targets are finalised in order once "nowNS" passes their
	// deadline, which lets the buzzer fire as soon as the window lapses.
	nextEval := 0

	// finalise checks every already-presented target whose response window has
	// closed by nowNS, sounding the buzzer for any that went unanswered.
	// presentedUpTo bounds the scan to images that have actually been shown —
	// future images still have onsetNS == 0, so they must not be evaluated yet.
	finalise := func(nowNS uint64, presentedUpTo int) {
		for nextEval <= presentedUpTo {
			t := nextEval
			if !isRepeat[t] {
				nextEval++
				continue
			}
			if nowNS <= res.onsetNS[t]+windowNS {
				break // window still open for this target; stop (targets are ordered)
			}
			if !res.responded[t] {
				_ = buzzer.Play()
			}
			nextEval++
		}
	}

	// registerPress scores a SPACE press. It first tries to credit the press as
	// a hit on the earliest open, unanswered target whose window contains it.
	// Failing that, the press is a false alarm: it is attributed to the image on
	// screen at press time — the most recently onset non-repeat image — so stray
	// presses are logged (responded=true, rt_ms set) instead of discarded.
	registerPress := func(pressNS uint64) {
		// 1. Hit: an open, unanswered target window containing the press.
		for t := 0; t < n; t++ {
			if !isRepeat[t] || res.responded[t] {
				continue
			}
			if pressNS >= res.onsetNS[t] && pressNS <= res.onsetNS[t]+windowNS {
				res.responded[t] = true
				res.rtMs[t] = int64(pressNS-res.onsetNS[t]) / 1_000_000
				return
			}
		}
		// 2. False alarm: attribute to the image currently on screen — the
		// highest-index image already presented whose onset precedes the press
		// (images are presented in order, so onsets increase with index).
		// Unpresented images still have onsetNS == 0 and are skipped.
		cur := -1
		for i := 0; i < n; i++ {
			if res.onsetNS[i] != 0 && res.onsetNS[i] <= pressNS {
				cur = i
			}
		}
		if cur >= 0 && !isRepeat[cur] && !res.responded[cur] {
			res.responded[cur] = true
			res.rtMs[cur] = int64(pressNS-res.onsetNS[cur]) / 1_000_000
		}
	}

	for i, pic := range pics {
		pic.SetPosition(sdl.FPoint{X: 0, Y: 0})

		// --- IMAGE ON ---
		for f := 0; f < framesOn; f++ {
			if err := screen.Clear(); err != nil {
				return res
			}
			if err := pic.Draw(screen); err != nil {
				return res
			}
			if f == 0 {
				ts, err := screen.FlipTS()
				if err != nil {
					return res
				}
				res.onsetNS[i] = ts
				finalise(ts, i) // any earlier target whose window closed before this onset
			} else if err := screen.Flip(); err != nil {
				return res
			}
			if drainPresses(registerPress) {
				return res // ESC / quit
			}
		}

		// --- BLANK (ISI) ---
		for f := 0; f < framesOff; f++ {
			if err := screen.Clear(); err != nil {
				return res
			}
			if err := screen.Flip(); err != nil {
				return res
			}
			if drainPresses(registerPress) {
				return res
			}
		}
	}

	// Tail: the last targets' windows may extend past the final image. Keep
	// polling (blank screen) until every window has closed so late hits are
	// caught and misses still buzz.
	for nextEval < n {
		nowNS := sdl.TicksNS()
		if drainPresses(registerPress) {
			break
		}
		finalise(nowNS, n-1) // all images presented; evaluate remaining targets
		time.Sleep(2 * time.Millisecond)
	}

	return res
}

// frameCount rounds a duration to the nearest whole number of frames, never
// dropping a positive duration to zero frames.
func frameCount(d, frame time.Duration) int {
	frames := int((d + frame/2) / frame)
	if frames == 0 && d > 0 {
		frames = 1
	}
	return frames
}

// drainPresses polls the SDL event queue, calling onPress with the SDL
// nanosecond timestamp of each SPACE key-down. It returns true if ESC was
// pressed or the window was closed (signal to abort).
func drainPresses(onPress func(uint64)) bool {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		switch event.Type {
		case sdl.EVENT_QUIT:
			return true
		case sdl.EVENT_KEY_DOWN:
			ke := event.KeyboardEvent()
			switch ke.Key {
			case sdl.K_ESCAPE:
				return true
			case control.K_SPACE:
				onPress(ke.Timestamp)
			}
		}
	}
	return false
}
