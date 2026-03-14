// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
)

const (
	NumItems         = 19
	ItemDuration     = 100 // ms
	FixationDuration = 500 // ms
)

type TrialConfig struct {
	HasJ bool
	HasK bool
	Lag  int // 1 means K is immediately after J
}

func generateLetters(config TrialConfig) []string {
	alphabet := "ABCDEFGHILMNOPQRSTUVWXZ" // Exclude J, K, Y (to avoid confusion)
	items := make([]string, NumItems)
	for i := range items {
		items[i] = string(alphabet[design.RandInt(0, len(alphabet)-1)])
	}

	if config.HasJ && !config.HasK {
		posJ := design.RandInt(3, 10)
		items[posJ] = "J"
	} else if !config.HasJ && config.HasK {
		posK := design.RandInt(3, 15)
		items[posK] = "K"
	} else if config.HasJ && config.HasK {
		posJ := design.RandInt(3, 7)
		posK := posJ + config.Lag
		if posK >= NumItems {
			posK = NumItems - 1
		}
		items[posJ] = "J"
		items[posK] = "K"
	}

	return items
}

func showInstructions(exp *control.Experiment) error {
	text := "Attentional Blink Experiment\n\n" +
		"A fast stream of letters will appear in the center.\n" +
		"Your task is to detect the letters 'J' and 'K'.\n\n" +
		"After the stream, report what you saw:\n" +
		" - Press 'J' if you only saw J\n" +
		" - Press 'K' if you only saw K\n" +
		" - Press 'B' if you saw BOTH\n" +
		" - Press 'N' if you saw NEITHER\n\n" +
		"Press any key to begin."

	instrBox := stimuli.NewTextBox(text, 650, sdl.FPoint{X: 0, Y: 0}, control.White)
	if err := instrBox.Present(exp.Screen, true, true); err != nil {
		return err
	}
	_, err := exp.Keyboard.Wait()
	return err
}

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 800x600)")
	subjectID := flag.Int("s", 1, "Subject ID")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 800, 600, false
	}

	exp := control.NewExperiment("Attentional-Blink", width, height, fullscreen, control.Gray, control.White, 32)
	exp.SubjectID = *subjectID

	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	exp.Data.AddVariableNames([]string{"trial_idx", "has_j", "has_k", "lag", "response", "is_correct", "rt"})

	if err := showInstructions(exp); err != nil {
		if err == sdl.EndLoop { return }
		log.Fatalf("instruction error: %v", err)
	}

	// 1. Create Design
	var trialConfigs []TrialConfig
	// 5 reps of lags 1-8 = 40 trials
	for lag := 1; lag <= 8; lag++ {
		for i := 0; i < 5; i++ {
			trialConfigs = append(trialConfigs, TrialConfig{HasJ: true, HasK: true, Lag: lag})
		}
	}
	// 10 J only
	for i := 0; i < 10; i++ {
		trialConfigs = append(trialConfigs, TrialConfig{HasJ: true, HasK: false, Lag: 0})
	}
	// 5 K only
	for i := 0; i < 5; i++ {
		trialConfigs = append(trialConfigs, TrialConfig{HasJ: false, HasK: true, Lag: 0})
	}
	// 5 Neither
	for i := 0; i < 5; i++ {
		trialConfigs = append(trialConfigs, TrialConfig{HasJ: false, HasK: false, Lag: 0})
	}
	design.ShuffleList(trialConfigs)

	// 8 training trials (not logged) with the same response/feedback logic.
	var trainingConfigs []TrialConfig
	for i := 0; i < 8; i++ {
		trainingConfigs = append(trainingConfigs, TrialConfig{
			HasJ: design.CoinFlip(0.5),
			HasK: design.CoinFlip(0.5),
			Lag:  design.RandInt(1, 8),
		})
	}
	design.ShuffleList(trainingConfigs)

	fixation := stimuli.NewFixCross(20, 2, control.Black)

	runOne := func(config TrialConfig) (string, bool, int64, error) {
		items := generateLetters(config)

		// A. Fixation
		if err := fixation.Present(exp.Screen, true, true); err != nil { return "", false, 0, err }
		clock.Wait(FixationDuration)

		// B. RSVP Stream
		for _, char := range items {
			txt := stimuli.NewTextLine(char, 0, 0, control.Black)
			if err := txt.Present(exp.Screen, true, true); err != nil { return "", false, 0, err }
			clock.Wait(ItemDuration)
		}

		// C. Response Screen
		prompt := stimuli.NewTextLine("What did you see? (J, K, B=Both, N=Neither)", 0, 0, control.Black)
		if err := prompt.Present(exp.Screen, true, true); err != nil { return "", false, 0, err }

		startTime := clock.GetTime()
		key, err := exp.Keyboard.WaitKeys([]sdl.Keycode{sdl.K_J, sdl.K_K, sdl.K_B, sdl.K_N, sdl.K_ESCAPE}, -1)
		if err != nil {
			return "", false, 0, err
		}
		rt := clock.GetTime() - startTime

		if key == sdl.K_ESCAPE {
			return "", false, rt, sdl.EndLoop
		}

		// Evaluate response
		response := ""
		isCorrect := false
		switch key {
		case sdl.K_J:
			response = "j"
			isCorrect = config.HasJ && !config.HasK
		case sdl.K_K:
			response = "k"
			isCorrect = !config.HasJ && config.HasK
		case sdl.K_B:
			response = "both"
			isCorrect = config.HasJ && config.HasK
		case sdl.K_N:
			response = "neither"
			isCorrect = !config.HasJ && !config.HasK
		}

		// Feedback
		if !isCorrect {
			_ = stimuli.PlayBuzzer(exp.AudioDevice)
		}

		// ITI
		if err := exp.Screen.Clear(); err != nil { return response, isCorrect, rt, err }
		exp.Screen.Update()
		clock.Wait(1000)

		return response, isCorrect, rt, nil
	}

	// 2. Training Loop (8 trials, feedback, not logged).
	for _, config := range trainingConfigs {
		if _, _, _, err := runOne(config); err != nil {
			if err == sdl.EndLoop {
				return
			}
			log.Fatalf("training trial error: %v", err)
		}
	}

	// Training finished screen.
	trainDone := stimuli.NewTextBox(
		"Training finished.\n\nPress a key to go on to the main experiment.",
		650,
		sdl.FPoint{X: 0, Y: 0},
		control.White,
	)
	if err := trainDone.Present(exp.Screen, true, true); err != nil {
		log.Fatalf("training-finished screen error: %v", err)
	}
	if _, err := exp.Keyboard.Wait(); err != nil && err != sdl.EndLoop {
		log.Fatalf("training-finished wait error: %v", err)
	}

	// 3. Main Trial Loop (logged).
	for i, config := range trialConfigs {
		response, isCorrect, rt, err := runOne(config)
		if err != nil {
			if err == sdl.EndLoop { return }
			log.Fatalf("trial error: %v", err)
		}

		// Log data
		exp.Data.Add([]interface{}{
			i + 1, config.HasJ, config.HasK, config.Lag, response, isCorrect, rt,
		})
	}
}
