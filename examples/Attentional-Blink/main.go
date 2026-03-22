// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"log"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
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

	instrBox := stimuli.NewTextBox(text, 650, control.FPoint{X: 0, Y: 0}, control.White)
	if err := exp.Show(instrBox); err != nil {
		return err
	}
	_, err := exp.Keyboard.Wait()
	return err
}

func main() {
	exp := control.NewExperimentFromFlags("Attentional-Blink", control.Gray, control.White, 32)
	defer exp.End()

	exp.AddDataVariableNames([]string{"trial_idx", "has_j", "has_k", "lag", "response", "is_correct", "rt"})

	if err := showInstructions(exp); err != nil {
		if control.IsEndLoop(err) {
			return
		}
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
		if err := exp.Show(fixation); err != nil {
			return "", false, 0, err
		}
		clock.Wait(FixationDuration)

		// B. RSVP Stream
		for _, char := range items {
			txt := stimuli.NewTextLine(char, 0, 0, control.Black)
			if err := exp.Show(txt); err != nil {
				return "", false, 0, err
			}
			clock.Wait(ItemDuration)
		}

		// C. Response Screen
		prompt := stimuli.NewTextLine("What did you see? (J, K, B=Both, N=Neither)", 0, 0, control.Black)
		if err := exp.Show(prompt); err != nil {
			return "", false, 0, err
		}

		startTime := clock.GetTime()
		key, err := exp.Keyboard.WaitKeys([]control.Keycode{control.K_J, control.K_K, control.K_B, control.K_N, control.K_ESCAPE}, -1)
		if err != nil {
			return "", false, 0, err
		}
		rt := clock.GetTime() - startTime

		if key == control.K_ESCAPE {
			return "", false, rt, control.EndLoop
		}

		// Evaluate response
		response := ""
		isCorrect := false
		switch key {
		case control.K_J:
			response = "j"
			isCorrect = config.HasJ && !config.HasK
		case control.K_K:
			response = "k"
			isCorrect = !config.HasJ && config.HasK
		case control.K_B:
			response = "both"
			isCorrect = config.HasJ && config.HasK
		case control.K_N:
			response = "neither"
			isCorrect = !config.HasJ && !config.HasK
		}

		// Feedback
		if !isCorrect {
			_ = stimuli.PlayBuzzer(exp.AudioDevice)
		}

		// ITI
		if err := exp.Blank(1000); err != nil {
			return response, isCorrect, rt, err
		}

		return response, isCorrect, rt, nil
	}

	// 2. Training Loop (8 trials, feedback, not logged).
	for _, config := range trainingConfigs {
		if _, _, _, err := runOne(config); err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatalf("training trial error: %v", err)
		}
	}

	// Training finished screen.
	trainDone := stimuli.NewTextBox(
		"Training finished.\n\nPress a key to go on to the main experiment.",
		650,
		control.FPoint{X: 0, Y: 0},
		control.White,
	)
	if err := exp.Show(trainDone); err != nil {
		log.Fatalf("training-finished screen error: %v", err)
	}
	if _, err := exp.Keyboard.Wait(); err != nil && !control.IsEndLoop(err) {
		log.Fatalf("training-finished wait error: %v", err)
	}

	// 3. Main Trial Loop (logged).
	for i, config := range trialConfigs {
		response, isCorrect, rt, err := runOne(config)
		if err != nil {
			if control.IsEndLoop(err) {
				return
			}
			log.Fatalf("trial error: %v", err)
		}

		// Log data
		exp.Data.Add(
			i+1, config.HasJ, config.HasK, config.Lag, response, isCorrect, rt,
		)
	}
}
