// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Change Blindness experiment — Rensink Flicker Paradigm
//
// A 5×5 grid of colored squares flickers between two versions (Image A and
// Image A'), where exactly one cell has changed color. The participant presses
// SPACE as soon as they spot the change.
//
// Timing (Rensink formula):
//   Image A:   240 ms
//   Blank:      80 ms
//   Image A':  240 ms
//   Blank:      80 ms
//   (repeat until response or 10 s timeout)
//
// Usage:
//   go run main.go [-d] [-s <subject_id>]
//
// Data recorded per trial:
//   trial, change_row, change_col, color_before, color_after, rt_ms, detected

package main

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	NTrials    = 20
	GridSize   = 5
	CellSizePx = float32(60)
	CellGapPx  = float32(10)
	StepPx     = CellSizePx + CellGapPx // 70 px between cell centers

	ImageMs = 240
	BlankMs = 80
	MaxMs   = 10_000 // 10 s timeout per trial
)

// palette of 7 visually distinct pastel-saturated colors
var palette = []control.Color{
	control.RGB(210, 50, 50),  // red
	control.RGB(50, 110, 210), // blue
	control.RGB(60, 170, 60),  // green
	control.RGB(210, 175, 50), // yellow
	control.RGB(180, 75, 195), // purple
	control.RGB(45, 190, 190), // cyan
	control.RGB(210, 130, 50), // orange
}

var colorLabels = []string{"red", "blue", "green", "yellow", "purple", "cyan", "orange"}

func colorName(c control.Color) string {
	for i, p := range palette {
		if p == c {
			return colorLabels[i]
		}
	}
	return fmt.Sprintf("rgb(%d,%d,%d)", c.R, c.G, c.B)
}

// cellRect returns the SDL-origin FRect for the cell at (row, col).
// row and col are 0-indexed; (0,0) is the top-left cell.
func cellRect(exp *control.Experiment, row, col int) control.FRect {
	// Convert to center-origin coordinates: col 0..4 → offsets -2..+2 steps
	cx := float32(col-GridSize/2) * StepPx
	cy := float32(row-GridSize/2) * StepPx
	sx, sy := exp.Screen.CenterToSDL(cx, cy)
	half := CellSizePx / 2
	return control.FRect{X: sx - half, Y: sy - half, W: CellSizePx, H: CellSizePx}
}

// drawGrid draws the 5×5 grid.
// colorsA holds the base color for every cell.
// If changeRow >= 0, the cell at (changeRow, changeCol) is drawn with newColor instead.
func drawGrid(exp *control.Experiment,
	colorsA [GridSize][GridSize]control.Color,
	changeRow, changeCol int, newColor control.Color) error {

	for row := 0; row < GridSize; row++ {
		for col := 0; col < GridSize; col++ {
			c := colorsA[row][col]
			if row == changeRow && col == changeCol {
				c = newColor
			}
			if err := exp.Screen.Renderer.SetDrawColor(c.R, c.G, c.B, c.A); err != nil {
				return err
			}
			rect := cellRect(exp, row, col)
			if err := exp.Screen.Renderer.RenderFillRect(&rect); err != nil {
				return err
			}
		}
	}
	return nil
}

// pollMs polls for a SPACE keypress for up to durationMs milliseconds.
// Returns (pressed, absoluteTimeMs, error).
func pollMs(exp *control.Experiment, durationMs int64) (bool, int64, error) {
	deadline := clock.GetTime() + durationMs
	for clock.GetTime() < deadline {
		key, _, err := exp.HandleEvents()
		if err != nil {
			return false, 0, err
		}
		if key == control.K_SPACE {
			return true, clock.GetTime(), nil
		}
		clock.Wait(1)
	}
	return false, 0, nil
}

func main() {
	exp := control.NewExperimentFromFlags("Change Blindness", control.Black, control.White, 32)
	defer exp.End()

	if err := exp.SetLogicalSize(1024, 768); err != nil {
		log.Printf("warning: set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"trial", "change_row", "change_col",
		"color_before", "color_after",
		"rt_ms", "detected",
	})

	instrText := fmt.Sprintf(
		"CHANGE BLINDNESS\n\n"+
			"A 5×5 grid of colored squares will flicker.\n"+
			"Between flickers, one square silently changes color.\n\n"+
			"Press SPACE as soon as you spot the changing square.\n"+
			"Each trial lasts up to 10 seconds.\n\n"+
			"There will be %d trials.\n\n"+
			"Press SPACE to begin.", NTrials)
	instructions := stimuli.NewTextBox(instrText, 1024, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)

	err := exp.Run(func() error {
		// Instructions screen
		if err := exp.Show(instructions); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}

		for trial := 0; trial < NTrials; trial++ {
			// Build Image A: random color for every cell
			var colorsA [GridSize][GridSize]control.Color
			for r := 0; r < GridSize; r++ {
				for c := 0; c < GridSize; c++ {
					colorsA[r][c] = palette[rand.Intn(len(palette))]
				}
			}

			// Choose one cell to change
			changeRow := rand.Intn(GridSize)
			changeCol := rand.Intn(GridSize)
			origColor := colorsA[changeRow][changeCol]

			// Pick a new color different from the original
			var newColor control.Color
			for {
				newColor = palette[rand.Intn(len(palette))]
				if newColor != origColor {
					break
				}
			}

			// Inter-trial interval
			if err := exp.Blank(500); err != nil {
				return err
			}
			exp.Keyboard.Clear()

			// Flicker loop
			trialStart := clock.GetTime()
			detected := false
			var rtMs int64

		flickerLoop:
			for {
				// ── Image A ────────────────────────────────────────────────
				exp.Screen.Clear()
				if err := drawGrid(exp, colorsA, -1, -1, origColor); err != nil {
					return err
				}
				exp.Screen.Update()
				clock.Wait(ImageMs)

				// ── Blank ──────────────────────────────────────────────────
				exp.Screen.Clear()
				exp.Screen.Update()
				pressed, t, err := pollMs(exp, BlankMs)
				if err != nil {
					return err
				}
				if pressed {
					rtMs = t - trialStart
					detected = true
					break flickerLoop
				}

				// ── Image A' (with change) ─────────────────────────────────
				exp.Screen.Clear()
				if err := drawGrid(exp, colorsA, changeRow, changeCol, newColor); err != nil {
					return err
				}
				exp.Screen.Update()
				clock.Wait(ImageMs)

				// ── Blank ──────────────────────────────────────────────────
				exp.Screen.Clear()
				exp.Screen.Update()
				pressed, t, err = pollMs(exp, BlankMs)
				if err != nil {
					return err
				}
				if pressed {
					rtMs = t - trialStart
					detected = true
					break flickerLoop
				}

				// ── Timeout check ──────────────────────────────────────────
				if clock.GetTime()-trialStart >= MaxMs {
					rtMs = MaxMs
					break flickerLoop
				}
			}

			// Brief end-of-trial signal: green dot = detected, gray = timeout
			dotColor := control.Gray
			if detected {
				dotColor = control.RGB(80, 200, 80)
			}
			if err := exp.Screen.Renderer.SetDrawColor(dotColor.R, dotColor.G, dotColor.B, dotColor.A); err != nil {
				return err
			}
			sx, sy := exp.Screen.CenterToSDL(0, 0)
			dot := control.FRect{X: sx - 10, Y: sy - 10, W: 20, H: 20}
			exp.Screen.Clear()
			if err := exp.Screen.Renderer.RenderFillRect(&dot); err != nil {
				return err
			}
			exp.Screen.Update()
			clock.Wait(400)

			// Log trial data
			exp.Data.Add(
				trial+1,
				changeRow, changeCol,
				colorName(origColor), colorName(newColor),
				rtMs, detected,
			)
			fmt.Printf("Trial %2d: cell (%d,%d)  %s→%s  RT=%d ms  detected=%v\n",
				trial+1, changeRow, changeCol,
				colorName(origColor), colorName(newColor),
				rtMs, detected)
		}

		// End screen
		endMsg := stimuli.NewTextBox(
			"Experiment complete.\nThank you for your participation!\n\nPress SPACE to exit.",
			700, control.FPoint{X: 0, Y: 0}, control.DefaultTextColor)
		if err := exp.Show(endMsg); err != nil {
			return err
		}
		return exp.Keyboard.WaitKey(control.K_SPACE)
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
