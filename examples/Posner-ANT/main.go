// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Attention Network Task (vertical variant) — Go translation of ant-v.py.
//
// Targets (flanker arrows) appear above or below a central fixation cross,
// eliminating the Simon response-compatibility component present in the
// left/right ANT-R variant.
//
// A graphical setup dialog collects participant info and session type
// (Training / Main experiment) before the experiment window opens.
//
// Usage:
//
//	go run . [-w] [-d N]
package main

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

//go:embed assets/trials.csv
var trialsCSV []byte

//go:embed assets/training.csv
var trainingCSV []byte

// Timing constants (milliseconds), matching ant-v.py defaults.
const (
	ITIBaseMS           = 1500
	ITIJitterMS         = 2000
	CueDisplayMS        = 100
	CueTargetIntervalMS = 400
	TargetDisplayMS     = 200
	MaxResponseMS       = 1700 // response window starting at target onset
	FeedbackMS          = 400

	// Visual layout (pixels in logical 1920×1080 space)
	BoxW    = 400 // frame width around flanker arrows
	BoxH    = 80  // frame height
	BoxShift = 100 // vertical distance from center to box center
	BorderW  = 3   // outline thickness (simulated via nested filled rectangles)

	LeftKey  = control.K_F
	RightKey = control.K_J
)

var (
	grey  = control.RGB(80, 80, 80)
	green = control.RGB(0, 255, 0)
	red   = control.RGB(255, 0, 0)
)

type trial struct {
	arrowDirection    string // "left" or "right"
	flankerCongruency string // "cong" or "incong"
	alerting          string // "no_cue", "dbl_cue", "spatial_cue"
	cueValidity       string // "NA", "valid", "invalid"
	cueUp             bool
	cueDown           bool
	targetPosition    string // "up" or "down"
}

func loadTrials(data []byte) ([]trial, error) {
	r := csv.NewReader(bytes.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	var trials []trial
	for i, rec := range records {
		if i == 0 {
			continue // skip header
		}
		if len(rec) < 7 {
			continue
		}
		trials = append(trials, trial{
			arrowDirection:    rec[0],
			flankerCongruency: rec[1],
			alerting:          rec[2],
			cueValidity:       rec[3],
			cueUp:             strings.EqualFold(rec[4], "TRUE"),
			cueDown:           strings.EqualFold(rec[5], "TRUE"),
			targetPosition:    rec[6],
		})
	}
	return trials, nil
}

func main() {
	// ── Participant info dialog ───────────────────────────────────────────────
	// Must be called before exp.Initialize() (i.e. before NewExperiment+Initialize).
	fields := append(
		control.ParticipantFields,
		control.InfoField{
			Name:    "session",
			Label:   "Session",
			Default: "Training",
			Type:    control.FieldSelect,
			Options: []string{"Training", "Main experiment"},
		},
		control.FullscreenField,
	)

	info, err := control.GetParticipantInfo("ANT — Attention Network Task", fields)
	if errors.Is(err, control.ErrCancelled) {
		log.Fatal("Setup cancelled.")
	}
	if err != nil {
		log.Fatalf("GetParticipantInfo: %v", err)
	}

	// ── Experiment initialisation ─────────────────────────────────────────────
	fullscreen := info["fullscreen"] == "true"
	width, height := 0, 0
	if !fullscreen {
		width, height = 1024, 768
	}

	exp := control.NewExperiment("ANT-vertical", width, height, fullscreen, grey, control.White, 32)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("Initialize: %v", err)
	}
	defer exp.End()
	exp.Info = info

	if err := exp.SetLogicalSize(1920, 1080); err != nil {
		log.Printf("Warning: SetLogicalSize: %v", err)
	}

	// ── Trial list ────────────────────────────────────────────────────────────
	training := info["session"] == "Training"
	var csvData []byte
	if training {
		csvData = trainingCSV
		fmt.Println("TRAINING mode")
	} else {
		csvData = trialsCSV
	}

	trials, err := loadTrials(csvData)
	if err != nil {
		log.Fatalf("failed to load trials: %v", err)
	}
	design.ShuffleList(trials)

	// ── Stimuli ──────────────────────────────────────────────────────────────
	arrowFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 50)
	if err != nil {
		log.Fatalf("failed to load arrow font: %v", err)
	}
	defer arrowFont.Close()

	makeArrow := func(text string) *stimuli.TextLine {
		s := stimuli.NewTextLine(text, 0, 0, control.White)
		s.Font = arrowFont
		return s
	}

	arrowCongLeft    := makeArrow(" < < < < < ")
	arrowCongRight   := makeArrow(" > > > > > ")
	arrowIncongLeft  := makeArrow(" > > < > > ")
	arrowIncongRight := makeArrow(" < < > < < ")

	crossBlack := stimuli.NewFixCross(30, 4, control.Black)
	crossGreen  := stimuli.NewFixCross(30, 4, green)
	crossRed    := stimuli.NewFixCross(30, 4, red)

	// Outlined frames: outer filled rect (border color) + inner filled rect
	// (background color) to simulate a BorderW-pixel outline.
	iw := float32(BoxW - 2*BorderW)
	ih := float32(BoxH - 2*BorderW)

	boxTopOuter    := stimuli.NewRectangle(0,  BoxShift, BoxW, BoxH, control.Black)
	boxTopInner    := stimuli.NewRectangle(0,  BoxShift, iw, ih, grey)
	boxBottomOuter := stimuli.NewRectangle(0, -BoxShift, BoxW, BoxH, control.Black)
	boxBottomInner := stimuli.NewRectangle(0, -BoxShift, iw, ih, grey)

	cueTopOuter    := stimuli.NewRectangle(0,  BoxShift, BoxW, BoxH, control.White)
	cueTopInner    := stimuli.NewRectangle(0,  BoxShift, iw, ih, grey)
	cueBottomOuter := stimuli.NewRectangle(0, -BoxShift, BoxW, BoxH, control.White)
	cueBottomInner := stimuli.NewRectangle(0, -BoxShift, iw, ih, grey)

	// drawScene clears the screen and draws the fixation cross + two frames.
	// topCued / bottomCued select white (cued) vs black (uncued) border.
	drawScene := func(cross *stimuli.FixCross, topCued, bottomCued bool) {
		_ = exp.Screen.Clear()
		_ = cross.Draw(exp.Screen)
		if topCued {
			_ = cueTopOuter.Draw(exp.Screen)
			_ = cueTopInner.Draw(exp.Screen)
		} else {
			_ = boxTopOuter.Draw(exp.Screen)
			_ = boxTopInner.Draw(exp.Screen)
		}
		if bottomCued {
			_ = cueBottomOuter.Draw(exp.Screen)
			_ = cueBottomInner.Draw(exp.Screen)
		} else {
			_ = boxBottomOuter.Draw(exp.Screen)
			_ = boxBottomInner.Draw(exp.Screen)
		}
	}

	exp.AddDataVariableNames([]string{
		"arrow_direction", "flanker_congruency", "alerting", "cue_validity",
		"cue_up", "cue_down", "target_position",
		"response_key", "reaction_time_ms", "correct",
	})

	// ── Instructions ─────────────────────────────────────────────────────────
	instrText := "Keep your eyes fixated on the central cross.\n\n" +
		"Flanker arrows will appear above or below the cross.\n" +
		"Your task: identify the direction of the CENTRAL arrow.\n\n" +
		"  Press 'F' if the central arrow points LEFT  (<)\n" +
		"  Press 'J' if the central arrow points RIGHT (>)\n\n" +
		"Sometimes a frame will brighten before the arrows appear —\n" +
		"it indicates the most likely target location.\n\n" +
		"Keep your eyes on the cross at all times.\n\n" +
		"Press SPACE to start."

	// ── Run ──────────────────────────────────────────────────────────────────
	runErr := exp.Run(func() error {
		exp.ShowInstructions(instrText)

		for i, t := range trials {
			// ITI: fixation + empty frames
			drawScene(crossBlack, false, false)
			_ = exp.Screen.Update()
			exp.Wait(ITIBaseMS + design.RandInt(0, ITIJitterMS))

			// Cue phase
			drawScene(crossBlack, t.cueUp, t.cueDown)
			_ = exp.Screen.Update()
			exp.Wait(CueDisplayMS)

			// Cue-target interval (no cue)
			drawScene(crossBlack, false, false)
			_ = exp.Screen.Update()
			exp.Wait(CueTargetIntervalMS)

			// Select and position target arrows
			var target *stimuli.TextLine
			if t.arrowDirection == "left" {
				if t.flankerCongruency == "cong" {
					target = arrowCongLeft
				} else {
					target = arrowIncongLeft
				}
			} else {
				if t.flankerCongruency == "cong" {
					target = arrowCongRight
				} else {
					target = arrowIncongRight
				}
			}

			var targetY float32
			if t.targetPosition == "up" {
				targetY = BoxShift
			} else {
				targetY = -BoxShift
			}
			target.SetPosition(control.FPoint{X: 0, Y: targetY})

			// Show target; capture VSYNC-aligned onset timestamp
			drawScene(crossBlack, false, false)
			_ = target.Draw(exp.Screen)
			onsetNS, _ := exp.Screen.FlipNS()

			// Response window spans TargetDisplayMS + MaxResponseMS from onset
			key, eventTS, kErr := exp.Keyboard.WaitKeysEventRT(
				[]control.Keycode{LeftKey, RightKey},
				TargetDisplayMS+MaxResponseMS,
			)
			if kErr != nil {
				return kErr
			}

			// Hide target
			drawScene(crossBlack, false, false)
			_ = exp.Screen.Update()

			// Compute RT and correctness
			var rtMS int64
			if eventTS != 0 {
				rtMS = int64(eventTS-onsetNS) / 1_000_000
			}
			correct := key == LeftKey && t.arrowDirection == "left" ||
				key == RightKey && t.arrowDirection == "right"

			exp.Data.Add(
				t.arrowDirection, t.flankerCongruency, t.alerting, t.cueValidity,
				t.cueUp, t.cueDown, t.targetPosition,
				key, rtMS, correct,
			)
			fmt.Printf("Trial %3d: dir=%-5s flanker=%-6s alerting=%-12s pos=%-4s key=%d rt=%4d ms correct=%v\n",
				i+1, t.arrowDirection, t.flankerCongruency, t.alerting,
				t.targetPosition, key, rtMS, correct)

			// Feedback: green cross = correct, red cross = incorrect
			if correct {
				drawScene(crossGreen, false, false)
			} else {
				drawScene(crossRed, false, false)
			}
			_ = exp.Screen.Update()
			exp.Wait(FeedbackMS)
		}

		_ = exp.Data.Save()
		exp.ShowInstructions("Experiment complete.\n\nThank you for your participation.\n\nPress SPACE to exit.")
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("experiment error: %v", runErr)
	}
}
