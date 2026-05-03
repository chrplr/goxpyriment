// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// test_av_sync — verify PlaySyncedWithFlip aligns audio onset with the VSYNC flip.
//
// Every second an icon image is shown for 3 display frames (~50 ms at 60 Hz)
// and a 50 ms 1 kHz tone is started synchronised to the first frame flip via
// PlaySyncedWithFlip.  Timing of each cycle is printed to stdout.
//
// With a photodiode on the screen and a microphone (or BBTK), the audio onset
// should lag the visual onset by at most one audio callback period.
//
// Controls:
//
//	ESC / Q — quit early
//
// Flags:
//
//	-w         windowed mode (1024×768)
//	-d int     display index (-1 = primary, default -1)
//	-s string  subject ID (default "0")
//	-cycles N  number of AV cycles (default 30)
//	-freq Hz   tone frequency in Hz (default 1000)
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	fCycles := flag.Int("cycles", 30, "number of AV cycles")
	fFreq := flag.Float64("freq", 1000, "tone frequency in Hz")

	exp := control.NewExperimentFromFlags("AV Sync Test", control.Black, control.White, 32)
	defer exp.End()

	// Inform user about audio pipeline latency.
	if spec, frames, err := exp.AudioDevice.Format(); err == nil && spec != nil {
		latencyMs := float64(frames) / float64(spec.Freq) * 1000
		fmt.Printf("audio device: %d Hz  %d ch  %d frames/callback  max audio lag ≤ %.1f ms\n",
			spec.Freq, spec.Channels, frames, latencyMs)
		fmt.Printf("  (call exp.SetAudioSampleFrames(128) before Initialize() to reduce to ≤ %.1f ms)\n",
			128.0/float64(spec.Freq)*1000)
	}

	// 50 ms tone with 5 ms linear ramp to avoid clicks.
	tone := stimuli.NewComplexTone([]float64{*fFreq}, 50, 5, 0.8)
	if err := tone.PreloadDevice(exp.AudioDevice); err != nil {
		log.Fatalf("preload tone: %v", err)
	}
	defer tone.Unload()

	// Icon image centred on screen.
	img := stimuli.NewPictureFromMemory(assets_embed.IconPNG, 0, 0)
	if err := stimuli.PreloadVisualOnScreen(exp.Screen, img); err != nil {
		log.Fatalf("preload image: %v", err)
	}
	defer img.Unload()

	exp.AddDataVariableNames([]string{"cycle", "flip_ns", "cycle_ms"})

	fmt.Printf("\n%-8s  %-20s  %-10s\n", "cycle", "flip_ns", "cycle_ms")

	cycle := 0
	err := exp.Run(func() error {
		if cycle >= *fCycles {
			return control.EndLoop
		}

		t0 := time.Now()

		// Frame 1: draw image into backbuffer, then pause audio / fill / flip / resume.
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := img.Draw(exp.Screen); err != nil {
			return err
		}
		flipNS, err := tone.PlaySyncedWithFlip(exp.Screen)
		if err != nil {
			return fmt.Errorf("PlaySyncedWithFlip: %w", err)
		}

		// Frames 2–3: keep image on screen for a total of 3 frames (~50 ms at 60 Hz).
		for f := 0; f < 2; f++ {
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := img.Draw(exp.Screen); err != nil {
				return err
			}
			if err := exp.Screen.Flip(); err != nil {
				return err
			}
		}

		// Blank screen for the remainder of the 1-second cycle.
		if err := exp.Screen.ClearAndUpdate(); err != nil {
			return err
		}

		cycleMs := float64(time.Since(t0).Microseconds()) / 1000
		fmt.Printf("%-8d  %-20d  %-10.1f\n", cycle, flipNS, cycleMs)
		exp.Data.Add(cycle, flipNS, fmt.Sprintf("%.1f", cycleMs))

		// Sleep until 1 second from cycle start, then poll for ESC.
		if remaining := time.Until(t0.Add(time.Second)); remaining > 0 {
			time.Sleep(remaining)
		}

		state := exp.PollEvents(nil)
		if state.QuitRequested {
			return control.EndLoop
		}

		cycle++
		return nil
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
	fmt.Printf("\n%d cycles complete.\n", cycle)
}
