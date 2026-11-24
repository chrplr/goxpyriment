// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Rush Hour — a sliding-block puzzle used as a problem-solving task.
//
// Each trial presents a different 6×6 Rush Hour configuration. The participant
// frees the red car by sliding the other vehicles out of the way; a trial ends
// only when the red car reaches the exit on the right wall. Every mouse action
// is written to the results file, so the full solution path (including dead
// ends and hesitations) can be reconstructed offline.
//
// Moving is one click, one cell: a click on the half of a vehicle lying on one
// side of its midline slides it one cell that way. There is no selection state
// and no dragging — a click either moves a vehicle by exactly one cell, or
// (wall, neighbour, or no vehicle under the cursor) moves nothing.
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
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/control"
)

const (
	itiMS          = 800  // blank screen between puzzles
	solvedFeedback = 1200 // how long the solved board stays on screen (ms)
	frameSleepMS   = 2    // polling granularity inside the trial loop
)

// eventRow is one line of the results file. The positional fields are -1 on
// rows where they do not apply, and the summary fields are filled only on the
// trial_end row.
type eventRow struct {
	kind    string // trial_start | click_move | click_blocked | click_empty | trial_end
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

	// The puzzle's onset is the flip that first shows it, and every time in the
	// trial is an SDL event timestamp minus that flip's — one clock throughout.
	// t_ms used to come from the Go clock, read before the board was even drawn,
	// and a click's t_ms was read when the loop got round to it, up to a frame
	// after the click.
	//
	// Clicks queued before the board appears (on the solved screen or during the
	// ITI) are discarded: they were made on a different screen, and were
	// otherwise applied as moves on this puzzle's first frame.
	exp.Keyboard.Clear() // drains the whole SDL queue, mouse included
	if err := drawBoard(exp, b, nil, status); err != nil {
		return err
	}
	onsetNS, err := exp.Screen.FlipTS()
	if err != nil {
		return err
	}
	msSinceOnset := func(ts uint64) int64 { return int64(ts-onsetNS) / 1_000_000 }

	start := newRow("trial_start", 0)
	start.tsNS = onsetNS
	logRow(exp, trial, p, start)

	// slides counts *slides*, not clicks: consecutive one-cell steps of the
	// same vehicle in the same direction are one slide, which is the metric
	// puzzles.txt uses for min_moves. The raw clicks remain one row each.
	var slides SlideCounter

	for {
		state := exp.PollEvents(nil)
		if state.QuitRequested {
			return control.EndLoop
		}
		mx, my := exp.Screen.MousePosition()

		// Vehicle under the cursor, outlined in white as a hover cue.
		var hover *Car
		if row, col, onBoard := cellAt(mx, my); onBoard {
			hover = b.CarAt(row, col)
		}

		// ── Click: slide the clicked vehicle one cell ─────────────────────
		// Every click is logged, including the ones that move nothing, so
		// hesitations and blocked attempts stay in the record.
		if state.LastMouseButton == control.BUTTON_LEFT {
			r := newRow("click_empty", msSinceOnset(state.LastMouseTimestamp))
			r.tsNS = state.LastMouseTimestamp
			r.mouseX, r.mouseY = mx, my

			row, col, onBoard := cellAt(mx, my)
			var car *Car
			if onBoard {
				car = b.CarAt(row, col)
			}
			if car != nil {
				r.kind = "click_blocked"
				r.car, r.orient = string(car.Label), car.Orientation()
				r.fromR, r.fromC = car.Row, car.Col
				r.toR, r.toC = car.Row, car.Col

				if step := stepForPoint(car, mx, my); step != 0 {
					if b.Step(car, step) {
						slides.Add(car.ID, step)
						r.kind = "click_move"
						r.toR, r.toC = car.Row, car.Col
					}
				}
			}
			logRow(exp, trial, p, r)
		}

		// ── Solved? ───────────────────────────────────────────────────────
		// Only a click can solve the board, so the trial ends at the click
		// handled just above: its event time, not the moment it was noticed.
		if b.Solved() {
			trialMS := msSinceOnset(state.LastMouseTimestamp)
			r := newRow("trial_end", trialMS)
			r.tsNS = state.LastMouseTimestamp
			r.nMoves = slides.N()
			r.solved = true
			r.trialMS = trialMS
			logRow(exp, trial, p, r)

			if err := drawBoard(exp, b, nil, "PUZZLE SOLVED!"); err != nil {
				return err
			}
			if err := exp.Screen.Flip(); err != nil {
				return err
			}
			exp.Audio.PlayCorrect()
			exp.Wait(solvedFeedback)
			fmt.Printf("Puzzle %2d (%s) solved in %d moves (optimum %d), %.1f s\n",
				trial, p.Name, slides.N(), p.MinMoves, float64(trialMS)/1000)
			return nil
		}

		if err := drawBoard(exp, b, hover, status); err != nil {
			return err
		}
		if err := exp.Screen.Flip(); err != nil {
			return err
		}
		time.Sleep(frameSleepMS * time.Millisecond)
	}
}

func main() {
	puzzles, err := DefaultPuzzles()
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
			"To move a vehicle one cell, click on the side of it that points\n"+
			"the way you want it to go.\n\n"+
			"There are %d puzzles, in increasing order of difficulty.\n"+
			"Take all the time you need.\n\n"+
			"Press SPACE to begin.",
		len(puzzles))

	err = exp.Run(func() error {
		if err := exp.ShowCursor(); err != nil {
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
