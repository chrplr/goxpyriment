// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// Visual Angle Calibration
//
// Asks the user for the monitor's physical width and viewing distance, then
// draws three concentric ring outlines whose radii correspond to 2°, 5°, and
// 10° of visual angle. This is a quick sanity-check that the units.Monitor
// conversion is calibrated correctly for the current display and seating
// distance.
//
// Usage:
//
//	go run main.go [-d]
//
// Flags:
//
//	-d  Developer mode: windowed 1024×768.

package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/units"
)

// ─── Ring parameters ─────────────────────────────────────────────────────────

var (
	ringAngles = []float64{2, 5, 10} // degrees of visual angle
	ringColors = []control.Color{
		{R: 0, G: 210, B: 255, A: 255}, // cyan  — 2°
		{R: 255, G: 210, B: 0, A: 255}, // gold  — 5°
		{R: 255, G: 90, B: 50, A: 255}, // coral — 10°
	}
)

// ─── Drawing helpers ──────────────────────────────────────────────────────────

// drawRing renders a 3-pixel-thick circle outline (radius in SDL pixels)
// centred at the SDL point (cx, cy), using short line segments.
func drawRing(rend *sdl.Renderer, cx, cy, radius float32, col sdl.Color) error {
	if err := rend.SetDrawColor(col.R, col.G, col.B, col.A); err != nil {
		return err
	}
	// Sample ~1 point per pixel of arc for a smooth outline.
	n := int(2*math.Pi*float64(radius)) + 1
	if n < 32 {
		n = 32
	}
	for _, w := range []float32{-1, 0, 1} { // three concentric passes → 3 px thick
		r := radius + w
		for i := 0; i < n; i++ {
			a1 := 2 * math.Pi * float64(i) / float64(n)
			a2 := 2 * math.Pi * float64(i+1) / float64(n)
			rend.RenderLine(
				cx+r*float32(math.Cos(a1)), cy+r*float32(math.Sin(a1)),
				cx+r*float32(math.Cos(a2)), cy+r*float32(math.Sin(a2)),
			)
		}
	}
	return nil
}

// ─── Input helpers ────────────────────────────────────────────────────────────

// askFloat shows a TextInput prompt and keeps re-prompting until the user
// enters a number in [min, max]. Returns control.EndLoop on ESC.
func askFloat(exp *control.Experiment, prompt string, min, max float64) (float64, error) {
	msg := prompt
	for {
		ti := stimuli.NewTextInput(
			msg,
			control.Point(0, -120),
			500,
			control.Color{R: 20, G: 20, B: 20, A: 255}, // input bg
			control.LightGray,                          // frame
			control.White,                              // text
		)
		str, err := ti.Get(exp.Screen, exp.Keyboard)
		if err != nil {
			return 0, err
		}
		v, parseErr := strconv.ParseFloat(strings.TrimSpace(str), 64)
		if parseErr != nil || v < min || v > max {
			msg = fmt.Sprintf(
				"%s\n\nInvalid input %q — please enter a number between %.0f and %.0f.",
				prompt, str, min, max)
			continue
		}
		return v, nil
	}
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	exp := control.NewExperimentFromFlags("Visual Angle Calibration", control.Black, control.White, 24)
	defer exp.End()

	runErr := exp.Run(func() error {

		// ── Step 1: collect monitor parameters via TextInput ─────────────────

		widthCm, err := askFloat(exp,
			"Monitor physical width (cm)\n\n"+
				"Measure only the screen surface, not the bezel.\n"+
				"A 24\" monitor is approximately 53 cm wide.\n\n"+
				"Press Enter to confirm.",
			10, 300)
		if err != nil {
			return err
		}

		distanceCm, err := askFloat(exp,
			fmt.Sprintf(
				"Viewing distance (cm)\n\n"+
					"Measure from your eyes to the screen surface.\n"+
					"A typical lab distance is 57–70 cm.\n"+
					"(Monitor width: %.1f cm)\n\n"+
					"Press Enter to confirm.", widthCm),
			20, 500)
		if err != nil {
			return err
		}

		// ── Step 2: build Monitor (pixel resolution from the live window) ────

		widthPx := exp.Screen.Width
		heightPx := exp.Screen.Height
		// Derive physical height assuming square pixels.
		heightCm := widthCm * float64(heightPx) / float64(widthPx)
		mon := units.NewMonitor(widthCm, heightCm, widthPx, heightPx, distanceCm)

		// ── Step 3: draw the concentric rings ────────────────────────────────

		exp.Screen.Clear()

		// Fixation cross at screen centre.
		fix := stimuli.NewFixCross(24, 2, control.White)
		if err := fix.Draw(exp.Screen); err != nil {
			return err
		}

		// SDL coordinates of the screen centre.
		cx, cy := exp.Screen.CenterToSDL(0, 0)

		for i, deg := range ringAngles {
			radius := float32(mon.DegToPx(deg))
			col := ringColors[i]

			if err := drawRing(exp.Screen.Renderer, cx, cy, radius, col); err != nil {
				return err
			}

			// Label positioned at the upper-right of the ring (45°).
			// In center-based coords y+ = UP, so sin(π/4) > 0 is up the screen.
			const labelAngle = math.Pi / 4
			lx := float32(float64(radius)*math.Cos(labelAngle)) + 8
			ly := float32(float64(radius) * math.Sin(labelAngle))
			label := stimuli.NewTextLine(fmt.Sprintf("%.0f°", deg), lx, ly, col)
			if err := label.Draw(exp.Screen); err != nil {
				return err
			}
		}

		// Monitor summary at the bottom of the screen.
		bottomY := -float32(heightPx)/2 + 22
		info := stimuli.NewTextLine(mon.String(), 0, bottomY, control.LightGray)
		if err := info.Draw(exp.Screen); err != nil {
			return err
		}

		// Title at the top.
		topY := float32(heightPx)/2 - 22
		title := stimuli.NewTextLine("Concentric circles at 2°, 5°, 10° of visual angle", 0, topY, control.LightGray)
		if err := title.Draw(exp.Screen); err != nil {
			return err
		}

		exp.Screen.Update()

		// Wait for any key then exit.
		if _, err := exp.Keyboard.Wait(); err != nil {
			return err
		}
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("experiment error: %v", runErr)
	}
}
