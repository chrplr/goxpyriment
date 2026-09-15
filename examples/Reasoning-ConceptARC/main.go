// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Reasoning-ConceptARC — the ConceptARC benchmark (Moskvichev, Odouard &
// Mitchell, 2023) as a self-paced grid-editing task.
//
// Each trial shows the demonstrations of one ARC task (input → output grid
// pairs that all follow the same hidden rule) and one test input. The
// participant paints the corresponding output grid with the mouse — pick a
// colour from the palette, click or drag over cells, resize the grid with the
// −/+ buttons, or start from a copy of the input — and submits it. Three
// attempts are allowed per test input (unlimited on the two training tasks).
//
// The default session follows §5.1 of the paper: two minimal tasks as
// training, then a shuffled mix of three minimal "attention-check" tasks and
// twelve corpus tasks drawn from twelve different concept groups, one random
// test input each. Every palette selection, cell edit, resize, and submission
// is written to the results file, so the whole editing history of each answer
// can be replayed offline.
//
// Usage:
//
//	go run ./examples/Reasoning-ConceptARC [-w] [-d N] [-s <subjectID>]
//	    [-concept NAME] [-n N] [-tests 1|3] [-no-training] [-explain]
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	itiMS            = 600  // blank screen between tasks
	feedbackSolvedMS = 1200 // how long "Correct!" stays on screen
	feedbackFailMS   = 1500 // how long "moving on" stays after the last attempt
	frameSleepMS     = 2    // polling granularity inside the trial loop
)

// ── Results file ─────────────────────────────────────────────────────────────

// eventRow is one line of the results file. Fields that do not apply to a row
// are -1 / "" / false; the summary fields are filled only on the trial_end row.
type eventRow struct {
	kind      string // trial_start | palette | paint | resize | copy_input | reset | submit | click_empty | trial_end
	tMS       int64
	tsNS      uint64
	mouseX    float32
	mouseY    float32
	row, col  int
	oldColor  int
	newColor  int
	rows      int
	cols      int
	attempt   int
	correct   bool
	grid      string
	nAttempts int
	solved    bool
	trialMS   int64
	explain   string
}

func newRow(kind string, tMS int64) eventRow {
	return eventRow{
		kind: kind, tMS: tMS,
		row: -1, col: -1, oldColor: -1, newColor: -1,
		rows: -1, cols: -1, attempt: -1, nAttempts: -1, trialMS: -1,
	}
}

// logRow appends one row to the data file.
func logRow(exp *control.Experiment, trial int, spec TrialSpec, r eventRow) {
	exp.Data.Add(
		trial, spec.Task.Name, spec.Task.Concept, spec.Kind, spec.TestIndex,
		r.kind, r.tMS, r.tsNS, r.mouseX, r.mouseY,
		r.row, r.col, r.oldColor, r.newColor, r.rows, r.cols,
		r.attempt, r.correct, r.grid,
		r.nAttempts, r.solved, r.trialMS, r.explain,
	)
}

// ── Screen text ──────────────────────────────────────────────────────────────

// label is a text line whose content changes occasionally: the texture is
// rebuilt only when the text or colour changes, never per frame.
type label struct {
	x, y  float32
	text  string
	color control.Color
	stim  *stimuli.TextLine
}

func (l *label) set(text string, color control.Color) {
	if text == l.text && color == l.color && l.stim != nil {
		return
	}
	l.unload()
	l.text, l.color = text, color
}

func (l *label) draw(exp *control.Experiment) error {
	if l.text == "" {
		return nil
	}
	if l.stim == nil {
		l.stim = stimuli.NewTextLine(l.text, l.x, l.y, l.color)
	}
	return l.stim.Draw(exp.Screen)
}

func (l *label) unload() {
	if l.stim != nil {
		_ = l.stim.Unload()
		l.stim = nil
	}
}

// ── The editor ───────────────────────────────────────────────────────────────

// Button indices in editor.buttons.
const (
	btnRowsMinus = iota
	btnRowsPlus
	btnColsMinus
	btnColsPlus
	btnCopy
	btnReset
	btnSubmit
)

// editor is the state of one trial's screen: the fixed demonstrations and test
// input, the participant's output grid, the selected colour, and the widgets.
type editor struct {
	spec    TrialSpec
	demos   []demoLayout
	test    Grid
	testBox gridBox
	out     Grid
	outBox  gridBox
	color   int
	buttons []*button

	hdrDemos, hdrTest, hdrOut label
	status, feedback          label
}

func newEditor(spec TrialSpec) *editor {
	test := spec.Task.Test[spec.TestIndex].Input
	ed := &editor{
		spec:    spec,
		demos:   layoutDemos(spec.Task.Train),
		test:    test,
		testBox: fitGrid(test.Rows(), test.Cols(), panelLeft, panelRight, testBottom, testTop),
		color:   0,
		buttons: []*button{
			btnRowsMinus: newButton("Rows -", 195, buttonRow1Y, 100, buttonH),
			btnRowsPlus:  newButton("Rows +", 305, buttonRow1Y, 100, buttonH),
			btnColsMinus: newButton("Cols -", 415, buttonRow1Y, 100, buttonH),
			btnColsPlus:  newButton("Cols +", 525, buttonRow1Y, 100, buttonH),
			btnCopy:      newButton("Copy input", 200, buttonRow2Y, 150, buttonH),
			btnReset:     newButton("Reset", 340, buttonRow2Y, 100, buttonH),
			btnSubmit:    newButton("Submit", 500, buttonRow2Y, 180, buttonH),
		},
		hdrDemos: label{x: (demoLeft + demoRight) / 2, y: headerY},
		hdrTest:  label{x: (panelLeft + panelRight) / 2, y: headerY},
		hdrOut:   label{x: (panelLeft + panelRight) / 2, y: outHdrY},
		status:   label{x: (demoLeft + demoRight) / 2, y: statusY},
		feedback: label{x: (panelLeft + panelRight) / 2, y: statusY},
	}
	// The output starts blank at the size of the test input — the participant
	// resizes it when the rule changes the size.
	ed.setOutput(Blank(test.Rows(), test.Cols()))
	ed.hdrDemos.set("Demonstrations", textColor)
	ed.hdrTest.set("Test input", textColor)
	return ed
}

// setOutput replaces the output grid and re-fits its box to the panel.
func (ed *editor) setOutput(g Grid) {
	ed.out = g
	ed.outBox = fitGrid(g.Rows(), g.Cols(), panelLeft, panelRight, outBottom, outTop)
	ed.hdrOut.set(fmt.Sprintf("Your output (%d x %d)", g.Rows(), g.Cols()), textColor)
}

func (ed *editor) unload() {
	for _, b := range ed.buttons {
		_ = b.labelStim.Unload()
	}
	for _, l := range []*label{&ed.hdrDemos, &ed.hdrTest, &ed.hdrOut, &ed.status, &ed.feedback} {
		l.unload()
	}
}

// draw renders one frame. (hoverRow, hoverCol) is the output cell under the
// cursor, or (-1, -1). It clears but does not flip — the caller decides when
// to present.
func (ed *editor) draw(exp *control.Experiment, hoverRow, hoverCol int) error {
	if err := exp.Screen.Clear(); err != nil {
		return err
	}
	for i, d := range ed.demos {
		p := ed.spec.Task.Train[i]
		if err := drawGrid(exp, p.Input, d.in); err != nil {
			return err
		}
		if err := drawArrow(exp, d.arrowX, d.arrowY); err != nil {
			return err
		}
		if err := drawGrid(exp, p.Output, d.out); err != nil {
			return err
		}
	}
	if err := drawGrid(exp, ed.test, ed.testBox); err != nil {
		return err
	}
	if err := drawGrid(exp, ed.out, ed.outBox); err != nil {
		return err
	}
	if hoverRow >= 0 {
		if err := outlineCell(exp, ed.outBox, hoverRow, hoverCol, hoverColor); err != nil {
			return err
		}
	}
	if err := drawPalette(exp, ed.color); err != nil {
		return err
	}
	for _, b := range ed.buttons {
		if err := b.draw(exp); err != nil {
			return err
		}
	}
	for _, l := range []*label{&ed.hdrDemos, &ed.hdrTest, &ed.hdrOut, &ed.status, &ed.feedback} {
		if err := l.draw(exp); err != nil {
			return err
		}
	}
	return nil
}

// present draws the current state and flips.
func (ed *editor) present(exp *control.Experiment, hoverRow, hoverCol int) error {
	if err := ed.draw(exp, hoverRow, hoverCol); err != nil {
		return err
	}
	return exp.Screen.Flip()
}

// ── Trial ────────────────────────────────────────────────────────────────────

// runTrial presents one test input and returns when it is solved or the
// attempts are exhausted (or control.EndLoop when the participant quits).
func runTrial(exp *control.Experiment, trial, nTrials int, spec TrialSpec, explain bool) error {
	ed := newEditor(spec)
	defer ed.unload()

	attemptsLeft := func(used int) string {
		if spec.MaxAttempts == 0 {
			return "unlimited attempts"
		}
		return fmt.Sprintf("%d attempt(s) left", spec.MaxAttempts-used)
	}
	setStatus := func(used int) {
		ed.status.set(fmt.Sprintf("Task %d/%d  -  %s", trial, nTrials, attemptsLeft(used)), textColor)
	}

	onset := clock.GetTime()
	start := newRow("trial_start", 0)
	start.rows, start.cols = ed.out.Rows(), ed.out.Cols()
	start.grid = ed.out.String()
	logRow(exp, trial, spec, start)
	setStatus(0)

	attempts := 0
	solved := false
	var trialMS int64
	painting := false // left button went down on the output grid and is still held

	// paint recolours one cell with the selected colour, logging the change.
	paint := func(r, c int, now int64, tsNS uint64, mx, my float32) {
		old := ed.out[r][c]
		if old == ed.color {
			return
		}
		ed.out[r][c] = ed.color
		row := newRow("paint", now)
		row.tsNS, row.mouseX, row.mouseY = tsNS, mx, my
		row.row, row.col, row.oldColor, row.newColor = r, c, old, ed.color
		logRow(exp, trial, spec, row)
	}

	for {
		state := exp.PollEvents(nil)
		if state.QuitRequested {
			return control.EndLoop
		}
		mx, my := exp.Screen.MousePosition()
		now := clock.GetTime() - onset

		// Digit keys select the palette colour, as in the ARC interface.
		if k := state.LastKey; k >= control.K_0 && k <= control.K_9 {
			ed.color = int(k - control.K_0)
			row := newRow("palette", now)
			row.tsNS, row.newColor = state.LastKeyTimestamp, ed.color
			logRow(exp, trial, spec, row)
		}

		hit := updateHover(ed.buttons, mx, my)
		hoverRow, hoverCol, onOut := ed.outBox.cellAt(mx, my)
		if !onOut {
			hoverRow, hoverCol = -1, -1
		}

		if state.LastMouseButton == control.BUTTON_LEFT {
			ts := state.LastMouseTimestamp
			switch {
			case swatchAt(mx, my) >= 0:
				ed.color = swatchAt(mx, my)
				row := newRow("palette", now)
				row.tsNS, row.mouseX, row.mouseY, row.newColor = ts, mx, my, ed.color
				logRow(exp, trial, spec, row)

			case onOut:
				painting = true
				paint(hoverRow, hoverCol, now, ts, mx, my)

			case hit == btnRowsMinus || hit == btnRowsPlus || hit == btnColsMinus || hit == btnColsPlus:
				rows, cols := ed.out.Rows(), ed.out.Cols()
				switch hit {
				case btnRowsMinus:
					rows--
				case btnRowsPlus:
					rows++
				case btnColsMinus:
					cols--
				case btnColsPlus:
					cols++
				}
				if rows >= 1 && rows <= MaxGridSide && cols >= 1 && cols <= MaxGridSide {
					ed.setOutput(ed.out.Resized(rows, cols))
					row := newRow("resize", now)
					row.tsNS, row.mouseX, row.mouseY = ts, mx, my
					row.rows, row.cols, row.grid = rows, cols, ed.out.String()
					logRow(exp, trial, spec, row)
				}

			case hit == btnCopy:
				ed.setOutput(ed.test.Clone())
				row := newRow("copy_input", now)
				row.tsNS, row.mouseX, row.mouseY = ts, mx, my
				row.rows, row.cols, row.grid = ed.out.Rows(), ed.out.Cols(), ed.out.String()
				logRow(exp, trial, spec, row)

			case hit == btnReset:
				ed.setOutput(Blank(ed.out.Rows(), ed.out.Cols()))
				row := newRow("reset", now)
				row.tsNS, row.mouseX, row.mouseY = ts, mx, my
				row.rows, row.cols, row.grid = ed.out.Rows(), ed.out.Cols(), ed.out.String()
				logRow(exp, trial, spec, row)

			case hit == btnSubmit:
				attempts++
				solved = ed.out.Equal(spec.Task.Test[spec.TestIndex].Output)
				row := newRow("submit", now)
				row.tsNS, row.mouseX, row.mouseY = ts, mx, my
				row.rows, row.cols, row.grid = ed.out.Rows(), ed.out.Cols(), ed.out.String()
				row.attempt, row.correct = attempts, solved
				logRow(exp, trial, spec, row)
				exhausted := spec.MaxAttempts > 0 && attempts >= spec.MaxAttempts
				setStatus(attempts)
				switch {
				case solved:
					trialMS = now
					ed.feedback.set("Correct!", okColor)
					if err := ed.present(exp, -1, -1); err != nil {
						return err
					}
					_ = exp.Audio.PlayCorrect()
					exp.Wait(feedbackSolvedMS)
				case exhausted:
					trialMS = now
					ed.feedback.set("Not correct - moving on", errColor)
					if err := ed.present(exp, -1, -1); err != nil {
						return err
					}
					_ = exp.Audio.PlayBuzzer()
					exp.Wait(feedbackFailMS)
				default:
					ed.feedback.set("Not correct - try again", errColor)
					_ = exp.Audio.PlayBuzzer()
				}
				if solved || exhausted {
					return endTrial(exp, trial, spec, ed, attempts, solved, trialMS, explain)
				}

			default:
				row := newRow("click_empty", now)
				row.tsNS, row.mouseX, row.mouseY = ts, mx, my
				logRow(exp, trial, spec, row)
			}
		} else if painting && exp.Mouse.IsPressed(control.BUTTON_LEFT) {
			// Dragging with the button held paints every cell the cursor
			// crosses. The timestamp is the frame's, not a hardware one.
			if onOut {
				paint(hoverRow, hoverCol, now, control.TicksNS(), mx, my)
			}
		}
		if !exp.Mouse.IsPressed(control.BUTTON_LEFT) {
			painting = false
		}

		if err := ed.present(exp, hoverRow, hoverCol); err != nil {
			return err
		}
		time.Sleep(frameSleepMS * time.Millisecond)
	}
}

// endTrial collects the optional explanation and writes the trial_end row.
func endTrial(exp *control.Experiment, trial int, spec TrialSpec, ed *editor, attempts int, solved bool, trialMS int64, explain bool) error {
	explanation := ""
	if explain {
		text, err := askExplanation(exp, solved)
		if err != nil {
			return err
		}
		explanation = text
	}
	end := newRow("trial_end", trialMS)
	end.rows, end.cols, end.grid = ed.out.Rows(), ed.out.Cols(), ed.out.String()
	end.nAttempts, end.solved, end.trialMS, end.explain = attempts, solved, trialMS, explanation
	logRow(exp, trial, spec, end)
	fmt.Printf("Task %2d %-28s test %d: %-8s in %d attempt(s), %.1f s\n",
		trial, spec.Task.Name, spec.TestIndex, map[bool]string{true: "solved", false: "failed"}[solved],
		attempts, float64(trialMS)/1000)
	return nil
}

// askExplanation shows a text-entry screen and returns what was typed. Line
// breaks and commas are kept; the results writer quotes the field.
func askExplanation(exp *control.Experiment, solved bool) (string, error) {
	prompt := "In one or two sentences, describe the rule you used to produce the output.\n\nType your answer, then press ENTER."
	if !solved {
		prompt = "In one or two sentences, describe the rule you think transforms the input into the output.\n\nType your answer, then press ENTER."
	}
	box := stimuli.NewTextInput(prompt, control.Point(0, -40), 1100, control.White, gridFrame, textColor)
	text, err := box.Get(exp.Screen, exp.Keyboard)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	tasks, err := LoadTasks()
	if err != nil {
		log.Fatalf("Reasoning-ConceptARC: %v", err)
	}

	// Registered before NewExperimentFromFlags, which calls flag.Parse().
	defaults := DefaultSessionOptions()
	concept := flag.String("concept", "", "present every task of one concept group (e.g. Center) instead of the mixed draw; one of: "+strings.Join(Concepts(tasks), ", "))
	nCorpus := flag.Int("n", defaults.NCorpus, "number of corpus tasks in the mixed session")
	nTests := flag.Int("tests", defaults.TestsPerTask, "test inputs per task: 1 (drawn at random) or 3 (all)")
	noTraining := flag.Bool("no-training", false, "skip the two training tasks")
	explain := flag.Bool("explain", false, "ask for a typed explanation after each task")

	exp := control.NewExperimentFromFlags("ConceptARC", bgColor, textColor, 22)
	defer exp.End()

	opts := defaults
	opts.Concept, opts.NCorpus, opts.TestsPerTask = *concept, *nCorpus, *nTests
	if *noTraining {
		opts.NTraining = 0
	}
	// The draw depends only on the subject ID, so a session can be rebuilt
	// from the results file alone.
	session, err := BuildSession(tasks, opts, rand.New(rand.NewSource(int64(exp.SubjectID))))
	if err != nil {
		exp.Fatal("Reasoning-ConceptARC: %v", err)
	}

	if err := exp.SetLogicalSize(logicalW, logicalH); err != nil {
		log.Printf("Warning: could not set logical size: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"trial", "task", "concept", "kind", "test_index",
		"event", "t_ms", "event_ts_ns", "mouse_x", "mouse_y",
		"row", "col", "old_color", "new_color", "rows", "cols",
		"attempt", "correct", "grid",
		"n_attempts", "solved", "trial_ms", "explanation",
	})

	instructions := fmt.Sprintf(
		"CONCEPT ARC\n\n"+
			"On each screen, the left side shows a few DEMONSTRATIONS: pairs of\n"+
			"grids, an input and the output it turns into. Every pair in a task\n"+
			"follows the same rule. Work out the rule, then apply it to the\n"+
			"TEST INPUT on the right by painting the output grid yourself.\n\n"+
			"Pick a colour in the palette (or press its number key), then click\n"+
			"or drag over the cells. 'Rows'/'Cols' change the size of your grid,\n"+
			"'Copy input' starts from a copy of the test input, 'Reset' clears it.\n"+
			"Press 'Submit' when you are done: you have %d attempts per task.\n\n"+
			"The first %d tasks are for practice and allow unlimited attempts.\n"+
			"There are %d tasks in total. Take all the time you need.\n\n"+
			"Press SPACE to begin.",
		defaults.MaxAttempts, opts.NTraining, len(session))

	err = exp.Run(func() error {
		if err := exp.ShowCursor(); err != nil {
			log.Printf("Warning: could not show cursor: %v", err)
		}
		if err := exp.ShowInstructions(instructions); err != nil {
			return err
		}
		for i, spec := range session {
			exp.Blank(itiMS)
			if err := runTrial(exp, i+1, len(session), spec, *explain); err != nil {
				return err
			}
			exp.Data.Save() // flush after every task — ESC must not cost data
		}
		exp.ShowEndMessage("All tasks completed!\n\nThank you for your participation.\n\nPress any key to exit.")
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
