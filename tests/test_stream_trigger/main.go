// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// test_stream_trigger demonstrates and validates the post-flip onset hooks:
// PresentStreamOfStimuliHooks (visual / mixed) and PlayStreamOfSoundsHook
// (audio). An OnsetCallback fires a TTL — here a no-op NullOutputTTLDevice
// standing in for real hardware — at each stimulus onset, and a FrameCallback
// clears the lines at the start of each ISI.
//
// It then checks that the onsetNS delivered to the hook equals the authoritative
// TimingLog.OnsetNS for every element: the whole point of the post-flip hook is
// that the trigger and the logged onset are the SAME SDL-clock instant, not one
// frame apart (as a pre-flip FrameCallback would be).
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	// NewExperiment (not …FromFlags) so no data file is opened — this test writes
	// no behavioural data, it only validates the onset hooks. A window is created
	// so the stream can catch ESC / window-close.
	exp := control.NewExperiment("Stream Trigger Test", 1024, 768, false, control.Black, control.White, 48)
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	// A no-op TTL device stands in for real hardware. In a real setup swap it for
	// triggers.NewParallelPort(...), NewDLPIO8, NewLabJackT4, NewFT232H, etc. — all
	// satisfy triggers.OutputTTLDevice.
	var dev triggers.NullOutputTTLDevice

	pass := true

	// ------------------------------------------------------------------ visual
	words := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	stims := make([]stimuli.Stimulus, len(words))
	for i, w := range words {
		stims[i] = stimuli.NewTextLine(w, 0, 0, control.White)
	}
	elements := stimuli.MakeRegularStream(stims, 400*time.Millisecond, 200*time.Millisecond)

	marks := map[int]uint64{} // onsetNS the hook received, per element
	onset := func(index int, onsetNS uint64) error {
		marks[index] = onsetNS
		return dev.Send(byte(index + 1)) // a per-stimulus code, emitted at the displayed onset
	}
	clearLines := func(ctx stimuli.FrameContext) error {
		if !ctx.OnPhase && ctx.FirstFrame { // drop the lines at the start of each ISI
			return dev.AllLow()
		}
		return nil
	}

	fmt.Printf("Visual: presenting %d words, firing a trigger at each onset...\n", len(words))
	_, logs, err := stimuli.PresentStreamOfStimuliHooks(exp.Screen, elements, 0, 0, clearLines, onset)
	switch {
	case control.IsEndLoop(err):
		fmt.Println("visual stream aborted (ESC) — skipping validation")
		return
	case err != nil:
		log.Fatalf("visual stream failed: %v", err)
	}
	pass = checkHook("visual", logs, marks) && pass

	// ------------------------------------------------------------------- audio
	freqs := []float64{330, 392, 440, 523}
	sounds := make([]stimuli.AudioPlayable, len(freqs))
	for i, f := range freqs {
		t := stimuli.NewTone(f, 200, 0.5)
		if perr := t.PreloadDevice(exp.AudioDevice); perr != nil {
			log.Fatalf("preloading tone %d (%.0f Hz): %v", i, f, perr)
		}
		sounds[i] = t
	}
	soundEls := stimuli.MakeRegularSoundStream(sounds, 200*time.Millisecond, 100*time.Millisecond)

	amarks := map[int]uint64{}
	aOnset := func(index int, onsetNS uint64) error {
		amarks[index] = onsetNS
		return dev.Send(byte(index + 1))
	}
	fmt.Printf("Audio: playing %d tones, firing a trigger at each onset...\n", len(sounds))
	_, aLogs, err := stimuli.PlayStreamOfSoundsHook(soundEls, aOnset)
	switch {
	case control.IsEndLoop(err):
		fmt.Println("audio stream aborted (ESC) — skipping validation")
		return
	case err != nil:
		log.Fatalf("audio stream failed: %v", err)
	}
	pass = checkHook("audio", aLogs, amarks) && pass

	fmt.Println()
	if pass {
		fmt.Println("PASS: every onset hook fired once, and its onsetNS matched TimingLog.OnsetNS exactly")
	} else {
		fmt.Println("FAIL: see mismatches above")
	}
}

// checkHook verifies the hook fired exactly once per logged element and that the
// onsetNS it received equals the authoritative TimingLog.OnsetNS.
func checkHook(label string, logs []stimuli.TimingLog, marks map[int]uint64) bool {
	ok := true
	if len(marks) != len(logs) {
		fmt.Printf("  [%s] FAIL: hook fired %d times for %d elements\n", label, len(marks), len(logs))
		ok = false
	}
	for _, tl := range logs {
		got, fired := marks[tl.Index]
		switch {
		case !fired:
			fmt.Printf("  [%s] element %d: FAIL — hook never fired\n", label, tl.Index)
			ok = false
		case got != tl.OnsetNS:
			fmt.Printf("  [%s] element %d: FAIL — hook onsetNS %d != TimingLog.OnsetNS %d (Δ %d ns)\n",
				label, tl.Index, got, tl.OnsetNS, int64(got)-int64(tl.OnsetNS))
			ok = false
		default:
			fmt.Printf("  [%s] element %d: ok — trigger and log agree at onsetNS %d\n", label, tl.Index, got)
		}
	}
	return ok
}
