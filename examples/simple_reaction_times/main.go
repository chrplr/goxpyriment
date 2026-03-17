// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
	"math/rand"
)

const (
	NTrials          = 20
	MinWaitTime      = 1000
	MaxWaitTime      = 2000
	MaxResponseDelay = 2000
)

func main() {
	exp := control.NewExperimentFromFlags("Visual Detection", control.Black, control.White, 32)
	defer exp.End()

	exp.Data.AddVariableNames([]string{"trial", "wait_time", "key", "rt"})

	// 2. Prepare stimuli
	target := stimuli.NewTextLine("+", 0, 0, control.DefaultTextColor)

	instrText := fmt.Sprintf("From time to time, a cross will appear at the center of screen.\n\nYour task is to press the SPACEBAR as quickly as possible when you see it (We measure your reaction-time).\n\nThere will be %d trials in total.\n\nPress the spacebar to start.", NTrials)
	instructions := stimuli.NewTextBox(instrText, 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		if err := exp.Show(instructions); err != nil {
			return err
		}
		if err := exp.Keyboard.WaitKey(control.K_SPACE); err != nil {
			return err
		}

		// Loop through trials
		for i := 0; i < NTrials; i++ {
			// Blank screen
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := exp.Screen.Update(); err != nil {
				return err
			}

			waitTime := rand.Intn(MaxWaitTime-MinWaitTime) + MinWaitTime
			clock.Wait(waitTime)

			// Target stimulus
			if err := exp.Show(target); err != nil {
				return err
			}

			// Wait for response
			startTime := clock.GetTime()
			key, err := exp.Keyboard.Wait()
			if err != nil {
				return err
			}
			rt := clock.GetTime() - startTime

			exp.Data.Add(i, waitTime, key, rt)
			fmt.Printf("Trial %d: Wait=%d ms, Key=%d, RT=%d ms\n", i, waitTime, key, rt)

			// Small pause between trials
			clock.Wait(500)
		}

		return control.EndLoop // Graceful exit
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
