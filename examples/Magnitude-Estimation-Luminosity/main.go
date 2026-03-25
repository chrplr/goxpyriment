// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
// Magnitude Estimation of Luminance
//
// A classic psychophysical experiment (Stevens, 1957) in which participants
// assign numbers to perceived brightness.
//
// Procedure:
//
//	5 blocks × 7 luminance levels = 35 trials.
//	Each trial: fixation (500 ms) → disk (1000 ms) → numeric response → ITI (1000 ms).
//
// Usage:
//
//	go run main.go [-s <subject_id>] [-dist <cm>] [-d]
//
// Flags:
//
//	-s     int     Subject ID (default 0).
//	-dist  float   Viewing distance in cm (default 60). Used to compute disk diameter.
//	-d             Development mode: windowed 1024×768.
//
// NOTE ON GAMMA: Standard monitors apply a power-law transfer function
// L(V) = k·(V/255)^γ (γ ≈ 2.2 for sRGB), so equal steps in the RGB values
// (10, 25, 50, 100, 150, 200, 255) do NOT produce equal steps in physical
// luminance (cd/m²). Use the -gamma flag to enable inverse-gamma correction:
//
//	go run main.go -gamma 2.2
//
// With this flag, each luminance level is treated as a linear luminance target
// (0–255) and mapped to the physical digital value needed to reproduce it on a
// monitor with the given gamma. Measure your monitor's actual gamma with a
// photometer for accurate psychophysics.

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Experiment parameters.
const (
	nBlocks           = 5
	fixDurMs          = 500
	stimDurMs         = 1000
	itiDurMs          = 1000
	bgGray      uint8 = 128 // mid-gray background
	scrW              = 1024
	scrH              = 768
	scrDiagInch       = 17.0 // for visual-angle computation
)

// luminanceLevels are the 7 gray values used as stimuli.
var luminanceLevels = []uint8{10, 25, 50, 100, 150, 200, 255}

// diskRadiusPx computes the pixel radius of the 5° visual-angle disk.
func diskRadiusPx(viewDistCM float64) float32 {
	dpi := math.Sqrt(float64(scrW*scrW+scrH*scrH)) / scrDiagInch
	pitchCM := 2.54 / dpi
	// Disk diameter = 5°, so half-angle = 2.5°.
	radiusCM := viewDistCM * math.Tan(2.5*math.Pi/180.0)
	return float32(radiusCM / pitchCM)
}

// getPositiveNumber shows a text-input box and loops until the participant
// enters a valid positive number. Returns the value and the reaction time
// (ms) measured from the moment startMs was captured (i.e. stimulus offset).
func getPositiveNumber(exp *control.Experiment, startMs int64) (float64, int64, error) {
	prompt := "Assign a number for the brightness you just saw.\n" +
		"If it felt twice as bright as a previous one, give it double the number.\n" +
		"You may use any positive number (integers or decimals).\n\n" +
		"Press ENTER to confirm."

	ti := stimuli.NewTextInput(
		prompt,
		control.Point(0, 0),
		600,
		control.RGB(200, 200, 200), // input-box background
		control.Black,              // input-box frame
		control.Black,              // text color
	)

	for {
		text, err := ti.Get(exp.Screen, exp.Keyboard)
		if err != nil {
			return 0, 0, err
		}
		rt := clock.GetTime() - startMs

		val, parseErr := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if parseErr == nil && val > 0 {
			return val, rt, nil
		}

		// Invalid input: reset and show a brief error.
		ti.UserText = ""
		errBox := stimuli.NewTextBox(
			"Please enter a positive number (e.g. 10, 2.5).",
			600, control.Point(0, -120), control.Red)
		_ = errBox.Present(exp.Screen, true, true)
		clock.Wait(1200)
		_ = errBox.Unload()
	}
}

func main() {
	distCM := flag.Float64("dist", 60.0, "Viewing distance in cm")
	gammaVal := flag.Float64("gamma", 0, "Monitor gamma for inverse-gamma correction (0 = disabled, typical 2.2)")
	bg := control.RGB(bgGray, bgGray, bgGray) // mid-gray background
	exp := control.NewExperimentFromFlags("Magnitude Estimation – Luminance", bg, control.Black, 24)
	defer exp.End()

	if *gammaVal > 0 {
		exp.SetGamma(*gammaVal)
		log.Printf("Gamma correction enabled: γ=%.2f", *gammaVal)
	}

	if err := exp.SetLogicalSize(scrW, scrH); err != nil {
		log.Printf("warning: set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"participant_id", "trial_number", "block",
		"stimulus_luminance", "participant_response", "reaction_time_ms",
	})

	radius := diskRadiusPx(*distCM)
	log.Printf("Disk radius: %.1f px (viewing distance %.0f cm)", radius, *distCM)

	err := exp.Run(func() error {
		// ── Instructions ──────────────────────────────────────────────────────
		instrText :=
			"Magnitude Estimation of Brightness\n\n" +
				"In each trial you will briefly see a gray disk on the screen.\n" +
				"Your task is to judge how BRIGHT the disk appears\n" +
				"by assigning it a number.\n\n" +
				"Rules:\n" +
				"  • There is no right or wrong answer.\n" +
				"  • Choose any positive number that feels right.\n" +
				"  • If one disk seems twice as bright as another,\n" +
				"    give it a number twice as large.\n" +
				"  • You may use integers or decimals (e.g. 10, 25.5).\n\n" +
				"Press SPACE to begin."
		instr := stimuli.NewTextBox(instrText, 900, control.Point(0, 0), control.Black)
		if err := exp.Show(instr); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}
		if err := instr.Unload(); err != nil {
			return err
		}

		// ── Pre-create one Circle per luminance level ─────────────────────────
		// exp.CorrectColor is a no-op when gamma correction is disabled.
		disks := make(map[uint8]*stimuli.Circle, len(luminanceLevels))
		for _, lum := range luminanceLevels {
			disks[lum] = stimuli.NewCircle(radius, exp.CorrectColor(control.RGB(lum, lum, lum)))
		}

		fix := stimuli.NewFixCross(20, 2, control.Black)

		trialNum := 0

		// ── Block loop ────────────────────────────────────────────────────────
		for block := 1; block <= nBlocks; block++ {
			// Show block start message (except before the very first block).
			if block > 1 {
				msg := stimuli.NewTextBox(
					fmt.Sprintf("Block %d of %d.\n\nPress SPACE to continue.", block, nBlocks),
					600, control.Point(0, 0), control.Black)
				if err := exp.Show(msg); err != nil {
					return err
				}
				if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
					return err
				}
				if err := msg.Unload(); err != nil {
					return err
				}
			}

			// Shuffle luminance levels for this block ("shuffled deck").
			order := make([]uint8, len(luminanceLevels))
			copy(order, luminanceLevels)
			rand.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })

			// ── Trial loop ────────────────────────────────────────────────────
			for _, lum := range order {
				trialNum++

				// 1. Fixation cross 500 ms.
				if err := exp.Show(fix); err != nil {
					return err
				}
				clock.Wait(fixDurMs)

				// 2. Disk 1000 ms.
				if err := exp.Show(disks[lum]); err != nil {
					return err
				}
				clock.Wait(stimDurMs)

				// 3. Clear screen → response.
				if err := exp.Screen.ClearAndUpdate(); err != nil {
					return err
				}
				stimOffset := clock.GetTime()

				response, rtMs, err := getPositiveNumber(exp, stimOffset)
				if err != nil {
					return err
				}

				exp.Data.Add(
					exp.SubjectID,
					trialNum,
					block,
					lum,
					response,
					rtMs,
				)

				// 4. ITI 1000 ms of blank gray.
				if err := exp.Blank(itiDurMs); err != nil {
					return err
				}
			}
		}

		// ── End screen ────────────────────────────────────────────────────────
		end := stimuli.NewTextBox(
			"The experiment is complete.\nThank you for your participation!\n\nPress SPACE to exit.",
			700, control.Point(0, 0), control.Black)
		if err := exp.Show(end); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
