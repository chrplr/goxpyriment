// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Rush Hour — a sliding-block puzzle used as a problem-solving task.
//
// Each trial presents a different 6×6 Rush Hour configuration. The participant
// frees the red car by sliding the other vehicles out of the way; a trial ends
// only when the red car reaches the exit on the right wall. Every mouse action
// is written to the results file, so the full solution path (including dead
// ends and hesitations) can be reconstructed offline.
//
// Vehicles can be moved in two ways:
//   - drag: press on a vehicle and drag along its axis;
//   - click: click a vehicle to select it, then click the cell to slide it to.
//
// In both cases a vehicle slides as far as the target allows and parks flush
// against the first wall or vehicle in its way.
//
// Output columns: trial, puzzle, min_moves, event, t_ms, event_ts_ns, mouse_x,
// mouse_y, car, orientation, from_row, from_col, to_row, to_col, n_moves,
// solved, trial_ms.
//
// Usage:
//
//	go run ./examples/Rush-Hour [-w] [-d N] [-s <subjectID>] [-n <nPuzzles>]
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
)

//go:embed puzzles.txt
var puzzleFile string

const (
	itiMS          = 800  // blank screen between puzzles
	solvedFeedback = 1200 // how long the solved board stays on screen (ms)
	frameSleepMS   = 2    // polling granularity inside the trial loop
)

// eventRow is one line of the results file. The positional fields are -1 on
// rows where they do not apply, and the summary fields are filled only on the
// trial_end row.
type eventRow struct {
	kind    string // trial_start | press | drag_move | click_move | release | trial_end
	tMS     int64
	tsNS    uint64
	mouseX  float32
	mouseY  float32
	car     string
	orient  string
	fromR   int
	fromC   int
	toR     int
	toC     int
	nMoves  int
	solved  bool
	trialMS int64
}

func newRow(kind string, tMS int64) eventRow {
	return eventRow{
		kind: kind, tMS: tMS,
		fromR: -1, fromC: -1, toR: -1, toC: -1,
		nMoves: -1, trialMS: -1,
	}
}

// logRow appends one row to the data file.
func logRow(exp *control.Experiment, trial int, puzzle Puzzle, r eventRow) {
	exp.Data.Add(
		trial, puzzle.Name, puzzle.MinMoves, r.kind, r.tMS, r.tsNS,
		r.mouseX, r.mouseY, r.car, r.orient,
		r.fromR, r.fromC, r.toR, r.toC,
		r.nMoves, r.solved, r.trialMS,
	)
}

// runTrial presents one puzzle and returns when it is solved (or when the
// participant quits, in which case it returns control.EndLoop).
func runTrial(exp *control.Experiment, trial int, p Puzzle, nTrials int) error {
	b := p.Fresh()
	status := fmt.Sprintf("Puzzle %d/%d - free the RED car", trial, nTrials)

	onset := clock.GetTime()
	nMoves := 0
	logRow(exp, trial, p, newRow("trial_start", 0))

	var selected *Car // highlighted vehicle: drag subject and click-move subject
	dragging := false
	movedThisPress := false

	for {
		state := exp.PollEvents(nil)
		if state.QuitRequested {
			return control.EndLoop
		}
		mx, my := exp.Screen.MousePosition()
		now := clock.GetTime() - onset

		// ── Button down: start a drag, or complete a click-move ───────────
		if state.LastMouseButton == control.BUTTON_LEFT {
			row, col, onBoard := cellAt(mx, my)
			var hit *Car
			if onBoard {
				hit = b.CarAt(row, col)
			}

			switch {
			case hit != nil:
				selected = hit
				dragging = true
				movedThisPress = false
				r := newRow("press", now)
				r.tsNS = state.LastMouseTimestamp
				r.mouseX, r.mouseY = mx, my
				r.car, r.orient = string(hit.Label), hit.Orientation()
				r.fromR, r.fromC = hit.Row, hit.Col
				r.toR, r.toC = hit.Row, hit.Col
				logRow(exp, trial, p, r)

			case onBoard && selected != nil:
				// Click-destination fallback: slide the selected vehicle
				// toward the clicked cell.
				car := selected
				fromR, fromC := car.Row, car.Col
				target := destinationFor(car, row, col)
				if b.TryMove(car, target[0], target[1]) {
					nMoves++
					r := newRow("click_move", now)
					r.tsNS = state.LastMouseTimestamp
					r.mouseX, r.mouseY = mx, my
					r.car, r.orient = string(car.Label), car.Orientation()
					r.fromR, r.fromC = fromR, fromC
					r.toR, r.toC = car.Row, car.Col
					logRow(exp, trial, p, r)
				}
				selected = nil
			}
		}

		// ── Drag: follow the cursor while the button is physically held ───
		if dragging && selected != nil {
			if exp.Mouse.IsPressed(control.BUTTON_LEFT) {
				row, col := clampedCellAt(mx, my)
				fromR, fromC := selected.Row, selected.Col
				dest := destinationFor(selected, row, col)
				if b.TryMove(selected, dest[0], dest[1]) {
					nMoves++
					movedThisPress = true
					r := newRow("drag_move", now)
					r.mouseX, r.mouseY = mx, my
					r.car, r.orient = string(selected.Label), selected.Orientation()
					r.fromR, r.fromC = fromR, fromC
					r.toR, r.toC = selected.Row, selected.Col
					logRow(exp, trial, p, r)
				}
			} else {
				dragging = false
			}
		}

		// ── Button up: end the drag ───────────────────────────────────────
		if state.LastMouseButtonUp == control.BUTTON_LEFT {
			if selected != nil {
				r := newRow("release", now)
				r.tsNS = state.LastMouseButtonUpTimestamp
				r.mouseX, r.mouseY = mx, my
				r.car, r.orient = string(selected.Label), selected.Orientation()
				r.fromR, r.fromC = selected.Row, selected.Col
				r.toR, r.toC = selected.Row, selected.Col
				logRow(exp, trial, p, r)
			}
			dragging = false
			// A press that moved nothing is treated as a click-select: keep
			// the vehicle highlighted so the click-destination path can
			// complete it.
			if movedThisPress {
				selected = nil
			}
			movedThisPress = false
		}

		// ── Solved? ───────────────────────────────────────────────────────
		if b.Solved() {
			trialMS := clock.GetTime() - onset
			r := newRow("trial_end", trialMS)
			r.nMoves = nMoves
			r.solved = true
			r.trialMS = trialMS
			logRow(exp, trial, p, r)

			if err := drawBoard(exp, b, nil, "PUZZLE SOLVED!"); err != nil {
				return err
			}
			if err := exp.Screen.PacedFlip(); err != nil {
				return err
			}
			exp.Audio.PlayCorrect()
			exp.Wait(solvedFeedback)
			fmt.Printf("Puzzle %2d (%s) solved in %d moves (optimum %d), %.1f s\n",
				trial, p.Name, nMoves, p.MinMoves, float64(trialMS)/1000)
			return nil
		}

		if err := drawBoard(exp, b, selected, status); err != nil {
			return err
		}
		if err := exp.Screen.PacedFlip(); err != nil {
			return err
		}
		time.Sleep(frameSleepMS * time.Millisecond)
	}
}

// destinationFor projects a clicked/dragged cell onto the vehicle's own axis:
// a horizontal car only takes the column, a vertical car only the row. Without
// this, TryMove would reject every off-axis cursor position.
func destinationFor(c *Car, row, col int) [2]int {
	if c.Horizontal {
		return [2]int{c.Row, col}
	}
	return [2]int{row, c.Col}
}

func main() {
	puzzles, err := ParsePuzzleFile(puzzleFile)
	if err != nil {
		log.Fatalf("Rush-Hour: %v", err)
	}

	// Registered before NewExperimentFromFlags, which calls flag.Parse().
	// The library holds 49 puzzles from 3 to 51 moves — far more than one
	// session usually needs, so a run normally takes a prefix of the ramp.
	nPuzzles := flag.Int("n", 12, "number of puzzles to present, from the easiest (0 = all)")

	exp := control.NewExperimentFromFlags("Rush Hour", bgColor, textColor, 28)
	defer exp.End()

	if *nPuzzles > 0 && *nPuzzles < len(puzzles) {
		puzzles = puzzles[:*nPuzzles]
	}

	if err := exp.SetLogicalSize(logicalW, logicalH); err != nil {
		log.Printf("Warning: could not set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"trial", "puzzle", "min_moves", "event", "t_ms", "event_ts_ns",
		"mouse_x", "mouse_y", "car", "orientation",
		"from_row", "from_col", "to_row", "to_col",
		"n_moves", "solved", "trial_ms",
	})

	instructions := fmt.Sprintf(
		"RUSH HOUR\n\n"+
			"On each puzzle, get the RED car out through the opening\n"+
			"on the right wall.\n\n"+
			"Vehicles only slide along their own axis, and cannot pass\n"+
			"through each other.\n\n"+
			"To move a vehicle, either drag it with the mouse, or click it\n"+
			"once (it turns white) and then click where it should go.\n\n"+
			"There are %d puzzles, in increasing order of difficulty.\n"+
			"Take all the time you need.\n\n"+
			"Press SPACE to begin.",
		len(puzzles))

	err = exp.Run(func() error {
		if err := exp.Mouse.ShowCursor(true); err != nil {
			log.Printf("Warning: could not show cursor: %v", err)
		}
		if err := exp.ShowInstructions(instructions); err != nil {
			return err
		}

		for i, p := range puzzles {
			exp.Blank(itiMS)
			if err := runTrial(exp, i+1, p, len(puzzles)); err != nil {
				return err
			}
			exp.Data.Save() // flush after every puzzle — ESC must not cost data
		}

		exp.ShowEndMessage("All puzzles completed!\n\nThank you for your participation.\n\nPress any key to exit.")
		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
