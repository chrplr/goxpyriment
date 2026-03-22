// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Constants
const (
	NShapeTotal       = 24
	NShapesPerColor   = 12
	NTripletsPerColor = 4
	NRepetitions      = 24  // Number of cover task repetitions per color
	NFamiliarization  = 624 // Total shapes in interleaved stream
)

type Triplet []int // Indices into the 24 shapes

type ExperimentType string

const (
	Exp1A ExperimentType = "1A"
	Exp1B ExperimentType = "1B"
	Exp2A ExperimentType = "2A"
	Exp2B ExperimentType = "2B"
	Exp3  ExperimentType = "3"
)

type shapeInfo struct {
	shape         *stimuli.Shape
	originalColor sdl.Color
}

func main() {
	// register custom flags first (before NewExperimentFromFlags which calls flag.Parse)
	expTypeFlag := flag.String("exp", "1B", "Experiment type (1A, 1B, 2A, 2B, 3)")

	exp := control.NewExperimentFromFlags("Visual Statistical Learning", control.White, control.Black, 24)
	defer exp.End()

	expType := ExperimentType(*expTypeFlag)

	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("Warning: failed to set logical size: %v", err)
	}

	// 2. Generate 24 novel shapes
	shapes := generateNovelShapes(NShapeTotal)

	// 3. Assign colors and triplets
	indices := rand.Perm(NShapeTotal)
	redIndices := indices[:NShapesPerColor]
	greenIndices := indices[NShapesPerColor:]

	for _, idx := range redIndices {
		shapes[idx].originalColor = control.Red
	}
	for _, idx := range greenIndices {
		shapes[idx].originalColor = control.Green
	}

	redTriplets := makeTriplets(redIndices)
	greenTriplets := makeTriplets(greenIndices)

	// 4. Generate familiarization stream
	attendedColorName := "red"
	if exp.SubjectID%2 == 1 {
		attendedColorName = "green"
	}

	soa := 1000
	stimDuration := 800
	if expType == Exp1A {
		soa = 400
		stimDuration = 200
	}

	stream := generateInterleavedStream(redTriplets, greenTriplets)

	// 5. Run the experiment
	err := exp.Run(func() error {
		// Instructions
		instr := fmt.Sprintf("Welcome to the Visual Statistical Learning experiment.\n\n"+
			"You will see a sequence of Red and Green shapes.\n"+
			"Your task is to attend to the %s shapes.\n"+
			"Press SPACEBAR whenever you see a %s shape repeat immediately.\n\n"+
			"Press SPACEBAR to start.", attendedColorName, attendedColorName)
		instructions := stimuli.NewTextBox(instr, 1000, control.Point(0, 0), control.Black)
		if err := exp.Show(instructions); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}

		// Familiarization Phase
		exp.AddDataVariableNames([]string{"phase", "trial", "shape_idx", "color", "is_repetition", "attended", "response_key", "rt", "hit"})

		for i, item := range stream {
			info := shapes[item.shapeIdx]
			colorName := "red"
			if item.isGreen {
				colorName = "green"
			}
			info.shape.Color = info.originalColor

			if err := exp.Show(info.shape); err != nil {
				return err
			}

			// Stim phase — wait up to stimDuration for SPACE
			var responseKey control.Keycode
			var rt int64
			spaceKey := []control.Keycode{control.K_SPACE}
			k, rtMs, err := exp.Keyboard.WaitKeysRT(spaceKey, stimDuration)
			if err != nil {
				return err
			}
			responded := k == control.K_SPACE
			if responded {
				responseKey = k
				rt = rtMs
			}

			// Blank screen
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := exp.Screen.Update(); err != nil {
				return err
			}

			// Blank phase — collect response if not yet given
			blankMs := soa - stimDuration
			if !responded && blankMs > 0 {
				k, rtMs, err = exp.Keyboard.WaitKeysRT(spaceKey, blankMs)
				if err != nil {
					return err
				}
				if k == control.K_SPACE {
					responded = true
					responseKey = k
					rt = int64(stimDuration) + rtMs
				}
			} else if blankMs > 0 {
				clock.Wait(blankMs)
			}

			isAttended := colorName == attendedColorName
			hit := responded && item.isRepetition && isAttended
			exp.Data.Add("familiarization", i, item.shapeIdx, colorName, item.isRepetition, isAttended, responseKey, rt, hit)
		}

		// Test Phase
		if expType == Exp3 {
			return runExp3Test(exp, shapes, redTriplets, greenTriplets)
		} else {
			return run2IFCTest(exp, expType, shapes, redTriplets, greenTriplets)
		}
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}

type streamItem struct {
	shapeIdx     int
	isGreen      bool
	isRepetition bool
}

func generateNovelShapes(n int) []shapeInfo {
	res := make([]shapeInfo, n)
	for i := 0; i < n; i++ {
		points := make([]sdl.FPoint, 10)
		for j := 0; j < 10; j++ {
			angle := float64(j) * 2 * math.Pi / 10
			radius := 30.0 + rand.Float64()*40.0
			if j%2 == 1 {
				radius = 15.0 + rand.Float64()*15.0
			}
			points[j] = sdl.FPoint{
				X: float32(radius * math.Cos(angle)),
				Y: float32(radius * math.Sin(angle)),
			}
		}
		res[i] = shapeInfo{
			shape: stimuli.NewShape(points, control.Black),
		}
	}
	return res
}

func makeTriplets(indices []int) []Triplet {
	res := make([]Triplet, NTripletsPerColor)
	for i := 0; i < NTripletsPerColor; i++ {
		res[i] = Triplet{indices[i*3], indices[i*3+1], indices[i*3+2]}
	}
	return res
}

func generateInterleavedStream(redTriplets, greenTriplets []Triplet) []streamItem {
	redStream := generateColorStream(redTriplets, false)
	greenStream := generateColorStream(greenTriplets, true)

	total := len(redStream) + len(greenStream)
	res := make([]streamItem, 0, total)

	rIdx, gIdx := 0, 0
	for rIdx < len(redStream) || gIdx < len(greenStream) {
		useGreen := false
		if rIdx >= len(redStream) {
			useGreen = true
		} else if gIdx < len(greenStream) {
			diff := rIdx - gIdx
			if diff > 6 {
				useGreen = true
			} else if diff < -6 {
				useGreen = false
			} else {
				useGreen = rand.Float64() < 0.5
			}
		}

		if useGreen {
			res = append(res, greenStream[gIdx])
			gIdx++
		} else {
			res = append(res, redStream[rIdx])
			rIdx++
		}
	}
	return res
}

func generateColorStream(triplets []Triplet, isGreen bool) []streamItem {
	stream := make([]streamItem, 0, 312)
	allTriplets := make([]Triplet, 0, 96)
	for i := 0; i < 24; i++ {
		for _, t := range triplets {
			allTriplets = append(allTriplets, t)
		}
	}
	rand.Shuffle(len(allTriplets), func(i, j int) {
		allTriplets[i], allTriplets[j] = allTriplets[j], allTriplets[i]
	})

	repIndices := rand.Perm(96)[:24]
	isRep := make(map[int]bool)
	for _, idx := range repIndices {
		isRep[idx] = true
	}

	for i, t := range allTriplets {
		stream = append(stream, streamItem{t[0], isGreen, false})
		stream = append(stream, streamItem{t[1], isGreen, false})
		stream = append(stream, streamItem{t[2], isGreen, false})
		if isRep[i] {
			stream = append(stream, streamItem{t[2], isGreen, true})
		}
	}
	return stream
}

func run2IFCTest(exp *control.Experiment, expType ExperimentType, shapes []shapeInfo, redTriplets, greenTriplets []Triplet) error {
	exp.AddDataVariableNames([]string{"phase", "trial", "triplet_type", "choice", "correct"})

	instr := "Now we will test your memory of the shapes.\n\n" +
		"In each trial, you will see two sequences of 3 shapes.\n" +
		"One sequence appeared more often than the other.\n" +
		"Your task is to choose which sequence feels more FAMILIAR.\n\n" +
		"Press '1' for the first sequence, '2' for the second.\n\n" +
		"Press SPACEBAR to start."
	instructions := stimuli.NewTextBox(instr, 1000, control.Point(0, 0), control.Black)
	if err := exp.Show(instructions); err != nil {
		return err
	}
	if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
		return err
	}

	foils := makeFoils(redTriplets, greenTriplets)

	type testTrial struct {
		triplet Triplet
		foil    Triplet
		isGreen bool
	}
	var trials []testTrial
	for i, t := range redTriplets {
		for j := 0; j < 4; j++ {
			trials = append(trials, testTrial{t, foils[0][(i+j)%4], false})
		}
	}
	for i, t := range greenTriplets {
		for j := 0; j < 4; j++ {
			trials = append(trials, testTrial{t, foils[1][(i+j)%4], true})
		}
	}
	rand.Shuffle(len(trials), func(i, j int) {
		trials[i], trials[j] = trials[j], trials[i]
	})

	for i, t := range trials {
		firstIsTriplet := rand.Float64() < 0.5

		if err := presentSequence(exp, expType, shapes, t.triplet, t.foil, firstIsTriplet, true); err != nil {
			return err
		}
		if err := presentSequence(exp, expType, shapes, t.triplet, t.foil, !firstIsTriplet, false); err != nil {
			return err
		}

		key, _, err := exp.Keyboard.WaitKeysRT([]control.Keycode{control.K_1, control.K_2}, -1)
		if err != nil {
			return err
		}
		var choice int
		if key == control.K_1 {
			choice = 1
		} else {
			choice = 2
		}

		correct := (choice == 1 && firstIsTriplet) || (choice == 2 && !firstIsTriplet)
		tripletType := "red"
		if t.isGreen {
			tripletType = "green"
		}
		exp.Data.Add("test_2ifc", i, tripletType, choice, correct)
		clock.Wait(500)
	}
	return nil
}

func makeFoils(redTriplets, greenTriplets []Triplet) [][]Triplet {
	foils := make([][]Triplet, 2)
	foils[0] = make([]Triplet, 4)
	foils[1] = make([]Triplet, 4)
	for i := 0; i < 4; i++ {
		foils[0][i] = Triplet{redTriplets[i][0], redTriplets[(i+1)%4][1], redTriplets[(i+2)%4][2]}
		foils[1][i] = Triplet{greenTriplets[i][0], greenTriplets[(i+1)%4][1], greenTriplets[(i+2)%4][2]}
	}
	return foils
}

func presentSequence(exp *control.Experiment, expType ExperimentType, shapes []shapeInfo, triplet, foil Triplet, isTriplet bool, first bool) error {
	seq := triplet
	if !isTriplet {
		seq = foil
	}

	label := "First sequence"
	if !first {
		label = "Second sequence"
	}
	text := stimuli.NewTextLine(label, 0, 200, control.Black)

	for _, shapeIdx := range seq {
		info := shapes[shapeIdx]

		color := control.Black
		if expType == Exp2A {
			color = info.originalColor
		} else if expType == Exp2B {
			// Swap colors
			if info.originalColor == control.Red {
				color = control.Green
			} else {
				color = control.Red
			}
		}
		info.shape.Color = color

		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := text.Draw(exp.Screen); err != nil {
			return err
		}
		if err := info.shape.Draw(exp.Screen); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}
		clock.Wait(800)
		if err := exp.Blank(200); err != nil {
			return err
		}
	}
	clock.Wait(500)
	return nil
}

func runExp3Test(exp *control.Experiment, shapes []shapeInfo, redTriplets, greenTriplets []Triplet) error {
	exp.AddDataVariableNames([]string{"phase", "trial", "target_idx", "pos_in_triplet", "rt", "hit"})

	instr := "Now we will test your reaction speed.\n\n" +
		"In each trial, you will first see a TARGET shape.\n" +
		"Then, a fast stream of shapes will appear.\n" +
		"Press SPACEBAR as fast as possible when you see the TARGET.\n\n" +
		"Press SPACEBAR to start."
	instructions := stimuli.NewTextBox(instr, 1000, control.Point(0, 0), control.Black)
	if err := exp.Show(instructions); err != nil {
		return err
	}
	if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
		return err
	}

	for i := 0; i < 96; i++ {
		targetIdx := rand.Intn(NShapeTotal)
		info := shapes[targetIdx]
		info.shape.Color = control.Black

		msg := stimuli.NewTextLine("Target for this trial:", 0, 150, control.Black)
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := msg.Draw(exp.Screen); err != nil {
			return err
		}
		if err := info.shape.Draw(exp.Screen); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}
		clock.Wait(1500)

		streamIndices := rand.Perm(NShapeTotal)
		found := false
		for _, idx := range streamIndices {
			if idx == targetIdx {
				found = true
				break
			}
		}
		if !found {
			streamIndices[rand.Intn(24)] = targetIdx
		}

		responded := false
		var rt int64
		for _, idx := range streamIndices {
			sInfo := shapes[idx]
			sInfo.shape.Color = control.Black
			if err := exp.Show(sInfo.shape); err != nil {
				return err
			}

			k, rtMs, err := exp.Keyboard.WaitKeysRT([]control.Keycode{control.K_SPACE}, 200)
			if err != nil {
				return err
			}
			if k == control.K_SPACE && !responded && idx == targetIdx {
				responded = true
				rt = rtMs
			}
			if err := exp.Blank(200); err != nil {
				return err
			}
		}

		posInTriplet := -1
		// Search in red triplets
		for _, t := range redTriplets {
			for p, idx := range t {
				if idx == targetIdx {
					posInTriplet = p + 1
				}
			}
		}
		// Search in green triplets
		if posInTriplet == -1 {
			for _, t := range greenTriplets {
				for p, idx := range t {
					if idx == targetIdx {
						posInTriplet = p + 1
					}
				}
			}
		}

		exp.Data.Add("test_rt", i, targetIdx, posInTriplet, rt, responded)
		clock.Wait(1000)
	}
	return nil
}
