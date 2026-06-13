// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// test_menu demonstrates the stimuli.Menu widget. Run with:
//
//	go run examples/test_menu/main.go -w
package main

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func show(exp *control.Experiment, msg string) {
	box := stimuli.NewTextBox(msg+"\n\n[SPACE to continue]", 900, control.FPoint{}, control.White)
	exp.Show(box)
	exp.Keyboard.WaitKey(control.K_SPACE)
}

func main() {
	exp := control.NewExperimentFromFlags("Menu Demo", control.Black, control.White, 32)
	defer exp.End()

	err := exp.Run(func() error {

		// ----------------------------------------------------------------
		// 1. Basic usage — defaults, no pre-selection
		// ----------------------------------------------------------------
		m1 := stimuli.NewMenu([]string{
			"Option A",
			"Option B",
			"Option C",
		})
		idx, err := m1.Get(exp.Screen, exp.Keyboard, 0)
		if err != nil {
			return err
		}
		show(exp, fmt.Sprintf(
			"1. Basic menu\n\nYou selected item %d (0-based index).\n"+
				"(Number key or arrow+ENTER both work.)",
			idx))

		// ----------------------------------------------------------------
		// 2. Pre-selected item — starts with a non-zero highlight
		// ----------------------------------------------------------------
		m2 := stimuli.NewMenu([]string{
			"Easy",
			"Medium",
			"Hard",
			"Expert",
		})
		// Start with "Hard" (index 2) pre-highlighted.
		idx, err = m2.Get(exp.Screen, exp.Keyboard, 2)
		if err != nil {
			return err
		}
		difficulties := []string{"Easy", "Medium", "Hard", "Expert"}
		show(exp, fmt.Sprintf(
			"2. Pre-selected item\n\nMenu opened with \"Hard\" highlighted.\n"+
				"You chose: %s (index %d).",
			difficulties[idx], idx))

		// ----------------------------------------------------------------
		// 3. Custom colors and position
		// ----------------------------------------------------------------
		m3 := stimuli.NewMenu([]string{
			"Salmon",
			"Cyan",
			"Lime",
			"Magenta",
			"Orange",
		})
		m3.TextColor = sdl.Color{R: 160, G: 160, B: 160, A: 255}
		m3.HighlightColor = sdl.Color{R: 255, G: 100, B: 50, A: 255} // orange-red
		m3.Pos = sdl.FPoint{X: -200, Y: 0}                           // shifted left

		idx, err = m3.Get(exp.Screen, exp.Keyboard, 0)
		if err != nil {
			return err
		}
		colors := []string{"Salmon", "Cyan", "Lime", "Magenta", "Orange"}
		show(exp, fmt.Sprintf(
			"3. Custom colors and position\n\n"+
				"Highlight color was orange-red; menu was left-shifted.\n"+
				"You chose: %s (index %d).",
			colors[idx], idx))

		// ----------------------------------------------------------------
		// 4. Many items — demonstrates wrapping number keys (0 = 10th item)
		// ----------------------------------------------------------------
		items := make([]string, 12)
		for i := range items {
			items[i] = fmt.Sprintf("Item %d", i+1)
		}
		m4 := stimuli.NewMenu(items)
		idx, err = m4.Get(exp.Screen, exp.Keyboard, 0)
		if err != nil {
			return err
		}
		show(exp, fmt.Sprintf(
			"4. Many items (12 total)\n\n"+
				"Keys 1–9 select items 1–9 directly.\n"+
				"Key 0 selects item 10. Items 11–12 require arrows + ENTER.\n\n"+
				"You chose: %s (index %d).",
			items[idx], idx))

		// ----------------------------------------------------------------
		// Done
		// ----------------------------------------------------------------
		show(exp,
			"Menu demo complete.\n\n"+
				"Summary of Menu API:\n"+
				"  stimuli.NewMenu(items)         — create with defaults\n"+
				"  m.Pos                          — reposition (center-based)\n"+
				"  m.TextColor / HighlightColor   — color customisation\n"+
				"  m.LineSpacing                  — vertical item spacing\n"+
				"  m.Font                         — override font (nil = screen default)\n"+
				"  m.Get(screen, kb, initialSel)  — display and collect (0-based index)")

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
