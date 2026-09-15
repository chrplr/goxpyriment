// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Geometric intruder task — Experiment 2 of Sablé-Meyer et al. (2021),
// "Sensitivity to geometric shape regularity in humans and baboons",
// PNAS 118(16). See description.md for the specification this follows.
//
// Six white quadrilaterals on black, in two rows of three: five are the same
// shape (up to a random rotation and size) and one is an intruder whose
// bottom-right vertex was displaced. The participant clicks on the intruder.
//
// -exp 1 runs Experiment 1 instead: black shapes on white, arranged on a
// circle around a fixation mark, canonical displays only (44 trials) and two
// training trials.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Per-trial variations (Materials and Methods, "Variations in Orientation and
// Size"): one random permutation of each list per display, element i to slot
// i, so the six shapes all differ in rotation and in size and every value is
// used exactly once. 0° is deliberately absent.
var (
	rotationsDeg = []float64{-25, -15, -5, 5, 15, 25}
	scales       = []float32{0.875, 0.925, 0.975, 1.025, 1.075, 1.125}
)

const (
	nSlots            = 6
	trainingCriterion = 0.80 // a training block is repeated below this
)

// layout is the geometry of the six-slot display, computed from the draw
// area: two rows of three in experiment 2, a circle in experiment 1.
type layout struct {
	unitPx    float32                // pixels per shape unit
	slots     [nSlots]control.FPoint // slot centres
	hitRadius float32                // nearest-slot fallback for near-miss clicks
	fixation  bool                   // experiment 1 keeps a mark at the centre
}

func newLayout(experiment int, w, h float32) layout {
	l := layout{unitPx: h / 8}
	l.hitRadius = 1.2 * l.unitPx
	if experiment == 1 {
		// "Organized in a circle as big as the screen permitted" (SI), with a
		// fixation mark in the middle (Fig. 1B, left). Slot i sits at angle
		// 60°·i counter-clockwise from the right; the radius leaves room for
		// the largest shape (1.5 units × 1.125) at the top and bottom.
		l.fixation = true
		r := h/2 - 1.0*l.unitPx
		for i := range l.slots {
			a := float64(i) * math.Pi / 3
			l.slots[i] = control.Point(r*float32(math.Cos(a)), r*float32(math.Sin(a)))
		}
		return l
	}
	// Row-major from the top-left.
	xs := []float32{-w / 3.5, 0, w / 3.5}
	ys := []float32{h / 4, -h / 4}
	for r, y := range ys {
		for c, x := range xs {
			l.slots[r*3+c] = control.Point(x, y)
		}
	}
	return l
}

// trial is one display: five copies of common and one of odd.
type trial struct {
	phase        string // trainA, trainB, test
	shape        string // reference shape (or pair id in training)
	deviant      string // shorter / longer / rot_up / rot_down; "" in training
	presentation string // canonical / swapped
	common, odd  figure
	outlierSlot  int
}

// slotShapes are the stimuli of one slot, in pixels relative to the slot
// centre, ready to draw and to hit-test.
type slotShapes struct {
	parts  []part
	shapes []*stimuli.Shape
}

// buildTestTrials returns the test trials in a fully random order, with the
// outlier slot balanced across the block: 11 shapes × 4 deviants ×
// {canonical, swapped} = 88 in experiment 2; canonical only = 44 in
// experiment 1.
func buildTestTrials(experiment int) ([]trial, error) {
	var trials []trial
	for _, s := range referenceShapes {
		ref, err := referenceFigure(s.name)
		if err != nil {
			return nil, err
		}
		for _, d := range deviantNames {
			dev, err := deviantFigure(s.name, d)
			if err != nil {
				return nil, err
			}
			trials = append(trials,
				trial{phase: "test", shape: s.name, deviant: d, presentation: "canonical", common: ref, odd: dev})
			if experiment != 1 {
				trials = append(trials,
					trial{phase: "test", shape: s.name, deviant: d, presentation: "swapped", common: dev, odd: ref})
			}
		}
	}
	design.ShuffleList(trials)
	assignBalancedSlots(trials)
	return trials, nil
}

// buildTrainingA returns the ten picture trials: each pair once, either
// member as the intruder.
func buildTrainingA() []trial {
	var trials []trial
	for _, p := range trainingPicturePairs() {
		t := trial{phase: "trainA", presentation: "canonical", common: p.a, odd: p.b}
		if design.RandInt(0, 1) == 1 {
			t.common, t.odd = p.b, p.a
		}
		t.shape = p.a.name + "|" + p.b.name
		trials = append(trials, t)
	}
	design.ShuffleList(trials)
	assignBalancedSlots(trials)
	return trials
}

// buildTrainingExp1 returns the two training trials of experiment 1: "two
// training pairs of geometric shapes, randomly selected from the 3" (SI),
// either member as the intruder.
func buildTrainingExp1() []trial {
	pairs := trainingPolygonPairs()
	design.ShuffleList(pairs)
	var trials []trial
	for _, p := range pairs[:2] {
		t := trial{phase: "train", shape: p.a.name + "|" + p.b.name, presentation: "canonical", common: p.a, odd: p.b}
		if design.RandInt(0, 1) == 1 {
			t.common, t.odd = p.b, p.a
		}
		trials = append(trials, t)
	}
	assignBalancedSlots(trials)
	return trials
}

// buildTrainingB returns the six polygon trials: each pair twice, each
// member once as the intruder.
func buildTrainingB() []trial {
	var trials []trial
	for _, p := range trainingPolygonPairs() {
		id := p.a.name + "|" + p.b.name
		trials = append(trials,
			trial{phase: "trainB", shape: id, presentation: "canonical", common: p.a, odd: p.b},
			trial{phase: "trainB", shape: id, presentation: "canonical", common: p.b, odd: p.a})
	}
	design.ShuffleList(trials)
	assignBalancedSlots(trials)
	return trials
}

// assignBalancedSlots gives each trial an outlier slot such that the six
// slots are used as evenly as possible over the list (Table S2 treats the
// slot as a factor).
func assignBalancedSlots(trials []trial) {
	slots := make([]int, 0, len(trials)+nSlots)
	for len(slots) < len(trials) {
		slots = append(slots, design.RandIntSequence(0, nSlots-1)...)
	}
	for i := range trials {
		trials[i].outlierSlot = slots[i]
	}
}

// sweepTone is a pure tone whose frequency glides linearly from f0 to f1
// (rising = correct, falling = error: "ascending / descending pitch").
func sweepTone(f0, f1 float64, durationMs int, amplitude float32) *stimuli.Tone {
	const rate = 44100
	n := rate * durationMs / 1000
	ramp := rate * 5 / 1000
	data := make([]byte, n*4)
	phase := 0.0
	for i := 0; i < n; i++ {
		f := f0 + (f1-f0)*float64(i)/float64(n)
		phase += 2 * math.Pi * f / rate
		v := float32(math.Sin(phase)) * amplitude
		if i < ramp {
			v *= float32(i) / float32(ramp)
		} else if i >= n-ramp {
			v *= float32(n-1-i) / float32(ramp)
		}
		bits := math.Float32bits(v)
		data[i*4], data[i*4+1], data[i*4+2], data[i*4+3] = byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24)
	}
	return &stimuli.Tone{Frequency: f0, Duration: durationMs, Amplitude: amplitude, Data: data}
}

func joinFloats[T float32 | float64](xs []T) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = fmt.Sprint(x)
	}
	return strings.Join(s, ";")
}

func main() {
	// Experiment-specific flags must be declared before
	// NewExperimentFromFlags, which calls flag.Parse.
	expFlag := flag.Int("exp", 2, "which experiment of the paper to run: 1 (black on white, circular layout, canonical only) or 2")
	demoFlag := flag.Bool("demo", false, "show the 11 reference shapes with their four deviants and exit")
	skipTraining := flag.Bool("skip-training", false, "go straight to the test trials")
	itiFlag := flag.Int("iti", 500, "inter-trial blank in ms")
	feedbackFlag := flag.Int("feedback", 700, "feedback display duration in ms")

	exp := control.NewExperimentFromFlags("Geometry-LoT1", control.Black, control.White, 32)
	defer exp.End()

	switch *expFlag {
	case 1:
		// Experiment 1 showed black shapes on white. The colours were fixed
		// above, before the flag was parsed, so swap them here: the screen
		// reads BgColor on every Clear and the text screens read
		// ForegroundColor on every draw.
		palette.shape, palette.background = control.Black, control.White
		exp.BackgroundColor, exp.ForegroundColor = palette.background, palette.shape
		exp.Screen.BgColor = palette.background
	case 2:
	default:
		exp.Fatal("-exp must be 1 or 2, got %d", *expFlag)
	}
	exp.Data.WriteComment(fmt.Sprintf("experiment: %d", *expFlag))

	// The participant points at a shape, so the cursor must be visible;
	// Initialize() hides it by default.
	if err := exp.ShowCursor(); err != nil {
		log.Printf("warning: could not show cursor: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"experiment", "trial", "phase", "block_repeat", "shape", "deviant", "presentation",
		"outlier_slot", "outlier_rotation", "outlier_scale", "rotations", "scales",
		"response_slot", "correct", "rt", "onset_ts",
	})

	w, h := exp.DrawArea()
	lay := newLayout(*expFlag, w, h)
	fixation := stimuli.NewFixCross(0.12*lay.unitPx, 2, palette.shape)

	correctTone := sweepTone(440, 880, 200, 0.5)
	errorTone := sweepTone(880, 440, 200, 0.5)
	// A silent tone played right after the start keypress wakes an idle audio
	// sink, so that the first real feedback tone is not swallowed.
	silence := sweepTone(440, 440, 50, 0)
	for _, t := range []*stimuli.Tone{correctTone, errorTone, silence} {
		if err := t.PreloadDevice(exp.AudioDevice); err != nil {
			exp.Fatal("preparing feedback tone: %v", err)
		}
		defer t.Unload()
	}

	// buildSlots lays out one trial: the permutations are drawn here and
	// returned so that they can be logged.
	buildSlots := func(t trial) ([nSlots]slotShapes, []float64, []float32) {
		rots := append([]float64(nil), rotationsDeg...)
		scs := append([]float32(nil), scales...)
		design.ShuffleList(rots)
		design.ShuffleList(scs)
		var slots [nSlots]slotShapes
		for i := range slots {
			f := t.common
			if i == t.outlierSlot {
				f = t.odd
			}
			slots[i].parts = f.transform(lay.unitPx, scs[i], rots[i])
			for _, p := range slots[i].parts {
				sh := stimuli.NewShape(p.points, p.color)
				sh.SetPosition(lay.slots[i])
				slots[i].shapes = append(slots[i].shapes, sh)
			}
		}
		return slots, rots, scs
	}

	drawSlots := func(slots [nSlots]slotShapes) error {
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		for _, s := range slots {
			for _, sh := range s.shapes {
				if err := sh.Draw(exp.Screen); err != nil {
					return err
				}
			}
		}
		if lay.fixation {
			return fixation.Draw(exp.Screen)
		}
		return nil
	}

	// hitSlot maps a click (centre-relative, y up) to a slot: point-in-polygon
	// on the transformed parts, else the nearest slot centre within hitRadius,
	// else -1 (a click on the background is ignored).
	hitSlot := func(slots [nSlots]slotShapes, mx, my float32) int {
		for i, s := range slots {
			for _, p := range s.parts {
				if pointInPolygon(mx-lay.slots[i].X, my-lay.slots[i].Y, p.points) {
					return i
				}
			}
		}
		best, bestD := -1, lay.hitRadius
		for i, c := range lay.slots {
			d := float32(math.Hypot(float64(mx-c.X), float64(my-c.Y)))
			if d < bestD {
				best, bestD = i, d
			}
		}
		return best
	}

	// recolour repaints a slot's non-background parts for the feedback frame
	// (SI: "filled in green for geometric shapes").
	recolour := func(s slotShapes, c control.Color) {
		for _, sh := range s.shapes {
			if sh.Color != palette.background {
				sh.Color = c
			}
		}
	}

	// runTrial presents one display, waits for the click, gives feedback and
	// logs the row. It returns whether the response was correct.
	runTrial := func(index int, t trial, blockRepeat int) (bool, error) {
		if err := exp.Blank(*itiFlag); err != nil {
			return false, err
		}
		slots, rots, scs := buildSlots(t)
		if err := drawSlots(slots); err != nil {
			return false, err
		}
		exp.PollEvents(nil) // discard clicks made during the blank
		onset, err := exp.Screen.FlipTS()
		if err != nil {
			return false, err
		}

		var response int
		var pressTS uint64
		for {
			btn, ts, err := exp.Mouse.GetPressEventTS(-1)
			if err != nil {
				return false, err
			}
			if btn != control.BUTTON_LEFT {
				continue
			}
			mx, my := exp.Screen.MousePosition()
			if response = hitSlot(slots, mx, my); response >= 0 {
				pressTS = ts
				break
			}
		}
		rt := float64(pressTS-onset) / 1e6
		correct := response == t.outlierSlot

		// Feedback: the clicked shape green if correct, red otherwise, and the
		// intruder green in both cases; rising or falling tone.
		recolour(slots[response], control.Red)
		recolour(slots[t.outlierSlot], control.Green)
		if err := drawSlots(slots); err != nil {
			return false, err
		}
		if err := exp.Screen.Update(); err != nil {
			return false, err
		}
		tone := errorTone
		if correct {
			tone = correctTone
		}
		if err := tone.Play(); err != nil {
			return false, err
		}
		if err := exp.Wait(*feedbackFlag); err != nil {
			return false, err
		}

		exp.Data.Add(*expFlag, index, t.phase, blockRepeat, t.shape, t.deviant, t.presentation,
			t.outlierSlot, rots[t.outlierSlot], scs[t.outlierSlot],
			joinFloats(rots), joinFloats(scs),
			response, correct, rt, onset)
		return correct, nil
	}

	// runTrainingBlock repeats a block until the criterion is met (a
	// criterion of 0 runs it once).
	runTrainingBlock := func(build func() []trial, criterion float64, message string) error {
		for repeat := 0; ; repeat++ {
			if err := exp.ShowInstructions(message); err != nil {
				return err
			}
			trials := build()
			nCorrect := 0
			for i, t := range trials {
				ok, err := runTrial(i+1, t, repeat)
				if err != nil {
					return err
				}
				if ok {
					nCorrect++
				}
			}
			if float64(nCorrect) >= criterion*float64(len(trials)) {
				return nil
			}
			message = fmt.Sprintf("%d correct out of %d.\n\nLet's practise this once more.\n\nPress SPACE to continue.", nCorrect, len(trials))
		}
	}

	err := exp.Run(func() error {
		if *demoFlag {
			return showDemo(exp, lay)
		}

		if err := exp.ShowInstructions(
			"On each screen you will see six shapes.\n" +
				"Five of them are identical, apart from their size and orientation,\n" +
				"and one is different.\n\n" +
				"Click on the shape that is different from the others,\n" +
				"as fast and as accurately as you can.\n\n" +
				"The shape you click turns green if you are right and red if you are wrong,\n" +
				"and a rising or falling sound tells you the same thing.\n\n" +
				"Press SPACE to start."); err != nil {
			return err
		}
		if err := silence.Play(); err != nil {
			return err
		}

		switch {
		case *skipTraining:
		case *expFlag == 1:
			if err := runTrainingBlock(buildTrainingExp1, 0,
				"First, two practice trials.\n\nPress SPACE to start."); err != nil {
				return err
			}
		default:
			if err := runTrainingBlock(buildTrainingA, trainingCriterion,
				"First, a short practice with pictures.\n\nPress SPACE to start."); err != nil {
				return err
			}
			if err := runTrainingBlock(buildTrainingB, trainingCriterion,
				"Now a short practice with simple shapes.\n\nPress SPACE to start."); err != nil {
				return err
			}
		}

		if err := exp.ShowInstructions("The experiment itself starts now.\n\nPress SPACE when you are ready."); err != nil {
			return err
		}
		trials, err := buildTestTrials(*expFlag)
		if err != nil {
			return err
		}
		for i, t := range trials {
			if _, err := runTrial(i+1, t, 0); err != nil {
				return err
			}
		}
		if err := exp.ShowEndMessage("Thank you, the experiment is over."); err != nil {
			return err
		}
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}

// showDemo draws the 11 reference shapes (leftmost column) and their four
// deviants in the 0° orientation, the quickest check of the coordinate table
// and of the deviant geometry. Any key or click exits.
func showDemo(exp *control.Experiment, lay layout) error {
	w, h := exp.DrawArea()
	unit := h / float32(len(referenceShapes)) / 2.4
	cols := []string{"", "shorter", "longer", "rot_up", "rot_down"}
	if err := exp.Screen.Clear(); err != nil {
		return err
	}
	for r, s := range referenceShapes {
		y := h/2 - (float32(r)+0.7)*h/float32(len(referenceShapes))
		label := stimuli.NewTextLine(s.name, -w/2+w*0.1, y, palette.shape)
		if err := label.Draw(exp.Screen); err != nil {
			return err
		}
		for c, d := range cols {
			var f figure
			var err error
			if d == "" {
				f, err = referenceFigure(s.name)
			} else {
				f, err = deviantFigure(s.name, d)
			}
			if err != nil {
				return err
			}
			for _, p := range f.transform(unit, 1, 0) {
				sh := stimuli.NewShape(p.points, p.color)
				sh.SetPosition(control.Point(-w/2+w*0.28+float32(c)*w*0.14, y))
				if err := sh.Draw(exp.Screen); err != nil {
					return err
				}
			}
		}
	}
	for c, d := range cols {
		if d == "" {
			d = "reference"
		}
		hdr := stimuli.NewTextLine(d, -w/2+w*0.28+float32(c)*w*0.14, h/2-20, control.Gray)
		if err := hdr.Draw(exp.Screen); err != nil {
			return err
		}
	}
	if err := exp.Screen.Update(); err != nil {
		return err
	}
	if _, err := exp.WaitAnyEventTS(nil, true, -1); err != nil {
		return err
	}
	return control.EndLoop
}
