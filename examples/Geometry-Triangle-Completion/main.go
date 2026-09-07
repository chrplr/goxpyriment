// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// Geometry triangle completion — implementation of the two-task paradigm of:
// Hart Y., Mahadevan L. & Dillon M. R. (2022). Euclid's Random Walk:
// Developmental Changes in the Use of Simulation for Geometric Reasoning.
// Cognitive Science, 46, e13070. https://doi.org/10.1111/cogs.13070
//
// Participants complete two tasks, always in this order:
//
//  1. REASONING — categorical judgements about the missing third corner of a
//     fragmented scalene triangle after a described change to the two visible
//     corners. 2 blocks × 8 questions.
//  2. LOCALIZATION — click where the missing apex of a fragmented isosceles
//     triangle is. 49 trials over 7 configurations.
//
// The headline measure is the scaling exponent alpha, the slope of log(SD of the
// click's y) on log(triangle side length); see ./analyse.
//
// Usage (from the repo root — go.work resolves the workspace):
//
//	go run ./examples/Geometry-Triangle-Completion -w -s 1
//
// Note "go run ." / "go run ./examples/Geometry-Triangle-Completion", never
// "go run main.go": this package has several files.
//
// Controls:
//
//	mouse       — click an option, a demo button, the response location, the arrow
//	1 / 2 / 3   — pick a reasoning option by keyboard (for the experimenter)
//	S           — revisit the sample-changes display during the reasoning task
//	SPACE       — leave the sample-changes display
//	ESC         — abort the session

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/results"
	"github.com/chrplr/goxpyriment/stimuli"
)

// config holds the display geometry, shared by both tasks.
type config struct {
	baseY        float32 // centre-based y of the triangle base, +Y up
	fragmentFrac float64 // visible arm length, as a fraction of each side
	strokePx     float32
}

func main() {
	// Experiment-specific flags must be registered BEFORE
	// NewExperimentFromFlags, which is what calls flag.Parse; their values must
	// be read only AFTER that call.
	taskFlag := flag.String("task", "both", "which task to run: both, reasoning or localization")
	blocksFlag := flag.Int("blocks", 2, "reasoning task: number of blocks of 8 questions")
	repsFlag := flag.Int("reps", 7, "localization task: repetitions of each of the 7 configurations")
	fracFlag := flag.Float64("fragment-frac", 0.35,
		"visible arm length at each corner, as a fraction of that side")
	strokeFlag := flag.Float64("stroke-px", 4, "line width of the figures, in logical pixels")
	baseFromBottom := flag.Float64("base-y", 80,
		"height of the triangle base above the bottom of the canvas, in logical pixels")
	skipDemo := flag.Bool("skip-demo", false, "skip the introduction and demonstration phase")

	checkTables()

	exp := control.NewExperimentFromFlags("Geometry-Triangle-Completion",
		bgGreen, inkBlack, 26,
		control.InfoField{Name: "age_years", Label: "Age (years)"},
		control.InfoField{Name: "age_months", Label: "Age (extra months)"},
		control.InfoField{Name: "gender", Label: "Gender (M / F / NB)"},
	)
	defer exp.End()

	// Both tasks are mouse-driven. Initialize hides the cursor unless
	// CursorVisible was set, and NewExperimentFromFlags calls Initialize itself,
	// so ask for it back here.
	if err := exp.ShowCursor(); err != nil {
		log.Printf("warning: could not show the mouse cursor: %v", err)
	}

	task := strings.ToLower(strings.TrimSpace(*taskFlag))
	switch task {
	case "both", "reasoning", "localization":
	default:
		exp.Fatal("-task must be both, reasoning or localization (got %q)", task)
	}
	if *blocksFlag < 1 || *repsFlag < 1 {
		exp.Fatal("-blocks and -reps must both be at least 1")
	}
	if *fracFlag <= 0 || *fracFlag > 1 {
		exp.Fatal("-fragment-frac must be in (0, 1]")
	}

	cfg := config{
		baseY:        float32(-logicalHeight/2 + *baseFromBottom),
		fragmentFrac: *fracFlag,
		strokePx:     float32(*strokeFlag),
	}

	if err := exp.SetLogicalSize(logicalWidth, logicalHeight); err != nil {
		log.Printf("warning: SetLogicalSize: %v", err)
	}

	ageYears := strings.TrimSpace(exp.Info["age_years"])
	ageMonths := strings.TrimSpace(exp.Info["age_months"])
	gender := strings.TrimSpace(exp.Info["gender"])

	// Session metadata, into the companion -info.txt.
	exp.Data.WriteComment("--PARADIGM")
	exp.Data.WriteComment("p reference: Hart, Mahadevan & Dillon (2022), Cognitive Science 46, e13070")
	exp.Data.WriteComment(fmt.Sprintf("p task: %s; reasoning blocks=%d; localization reps=%d", task, *blocksFlag, *repsFlag))
	exp.Data.WriteComment(fmt.Sprintf("p canvas: %dx%d logical px, 1 length unit = %.0f px (paper: 1920 x 0.85)",
		logicalWidth, logicalHeight, unitPx))
	exp.Data.WriteComment(fmt.Sprintf("p geometry: base %.0f px above the bottom edge, fragment arms = %.2f of each side, stroke %.1f px",
		*baseFromBottom, cfg.fragmentFrac, cfg.strokePx))
	exp.Data.WriteComment("p coordinates: the CSVs report screen pixels with the origin at the TOP-LEFT and +Y down, matching the paper; the code works in centre-based coordinates with +Y up")
	exp.Data.WriteComment(fmt.Sprintf("p participant: age_years=%s age_months=%s gender=%s", ageYears, ageMonths, gender))

	exp.AddDataVariableNames([]string{
		"age_years", "age_months", "gender",
		"block", "trial", "triangle_id", "base_units", "left_angle_deg", "right_angle_deg",
		"area_units2", "question_type", "dimension", "transformation", "direction",
		"selected_option", "correct_option", "is_correct", "rt_ms", "onset_ns",
	})

	// The localization task gets its own CSV: its columns share almost nothing
	// with the reasoning task's.
	stem := strings.TrimSuffix(exp.Data.Filename, ".csv")
	locFile, err := results.NewOutputFile(exp.Data.Directory, stem+"-localization.csv")
	if err != nil {
		exp.Fatal("creating the localization data file: %v", err)
	}
	locFile.WriteLine("subject_id,age_years,age_months,gender,trial,config_id,base_units,base_angle_deg," +
		"side_length_units,true_vertex_x,true_vertex_y,clicked_x,clicked_y,error_x,error_y,rt_ms,onset_ns")

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	runErr := exp.Run(func() error {
		if task == "both" || task == "reasoning" {
			if err := exp.ShowInstructions(reasoningInstructions(*blocksFlag)); err != nil {
				return err
			}
			if !*skipDemo {
				if err := runIntro(exp, cfg); err != nil {
					return err
				}
				if err := runDemo(exp, cfg); err != nil {
					return err
				}
			}
			trials := buildReasoningTrials(*blocksFlag, rng)
			err := runReasoningTask(exp, cfg, trials, func(t reasoningTrial, n int, r reasoningResponse) {
				correct := correctOption(t.trans, t.dim)
				exp.Data.Add(
					ageYears, ageMonths, gender,
					t.block, n, t.figure.id, t.figure.baseUnits, t.figure.leftDeg, t.figure.rightDeg,
					t.figure.areaUnits(), t.questionType(), t.dim.name(), t.trans.kind(), t.trans.direction(),
					r.selected, correct, r.selected == correct, r.rtMs, r.onsetNS,
				)
				_ = exp.Data.Save()
			})
			if err != nil {
				return err
			}
		}

		if task == "both" || task == "localization" {
			if err := exp.ShowInstructions(localizationInstructions(*repsFlag)); err != nil {
				return err
			}
			if !*skipDemo {
				if err := runIntro(exp, cfg); err != nil {
					return err
				}
			}
			if err := runPracticeTrial(exp, cfg); err != nil {
				return err
			}

			for i, t := range buildLocalizationTrials(*repsFlag, rng) {
				resp, err := runLocalizationTrial(exp, cfg, t)
				if err != nil {
					return err
				}
				writeLocalizationRow(exp, locFile, cfg, i+1, t, resp, ageYears, ageMonths, gender)
				_ = locFile.Save()
			}
		}

		if err := exp.Show(stimuli.NewTextBox(
			"All done — thank you!\n\nPress any key to exit.", 1000, control.FPoint{}, inkBlack)); err != nil {
			return err
		}
		if _, err := exp.Keyboard.Wait(); err != nil && !control.IsEndLoop(err) {
			return err
		}
		return control.EndLoop
	})

	if err := locFile.Finalize(); err != nil {
		log.Printf("warning: saving the localization data: %v", err)
	}
	if runErr != nil && !control.IsEndLoop(runErr) {
		exp.Fatal("experiment error: %v", runErr)
	}
}

// writeLocalizationRow converts the trial to the paper's screen-pixel frame
// (origin top-left, +Y down) and appends one row.
func writeLocalizationRow(exp *control.Experiment, f *results.OutputFile, cfg config,
	trial int, t tri, r localizationResponse, ageYears, ageMonths, gender string) {

	_, _, apex := t.corners(cfg.baseY)
	trueX, trueY := exp.Screen.CenterToSDL(apex.X, apex.Y)
	clickX, clickY := exp.Screen.CenterToSDL(r.clickX, r.clickY)
	leg, _ := t.legUnits()

	// error_y follows the paper's sign convention: y_true − y_clicked in the
	// screen frame, so a POSITIVE value means the click fell short of the true
	// vertex — the underestimation the paper reports.
	f.WriteLine(fmt.Sprintf("%d,%s,%s,%s,%d,%s,%g,%g,%.6f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%d,%d",
		exp.SubjectID, quoteCSV(ageYears), quoteCSV(ageMonths), quoteCSV(gender),
		trial, t.id, t.baseUnits, t.leftDeg, leg,
		trueX, trueY, clickX, clickY,
		clickX-trueX, trueY-clickY, r.rtMs, r.onsetNS))
}

// quoteCSV quotes a free-text field only when it needs it (RFC 4180).
func quoteCSV(s string) string {
	if !strings.ContainsAny(s, `",`+"\n") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func reasoningInstructions(blocks int) string {
	return fmt.Sprintf(
		"Part 1 of 2 — reasoning about triangles\n\n"+
			"You will see partial triangles with only two corners showing,\n"+
			"and be asked what happens to the missing top corner when the\n"+
			"two visible corners change.\n\n"+
			"There are %d questions. There are no right-or-wrong messages —\n"+
			"just say what you think.\n\n"+
			"[Experimenter] click the answer, or press 1, 2 or 3.\n"+
			"Press S at any time to show the sample changes again.\n\n"+
			"Press SPACE to begin.", blocks*8)
}

func localizationInstructions(reps int) string {
	return fmt.Sprintf(
		"Part 2 of 2 — finding the missing corner\n\n"+
			"You will see more partial triangles. Each time, click with the\n"+
			"mouse where you think the missing top corner is, then click the\n"+
			"arrow to go on.\n\n"+
			"There is one practice trial, then %d test trials.\n\n"+
			"Press SPACE to begin.", reps*len(localizationTriangles))
}
