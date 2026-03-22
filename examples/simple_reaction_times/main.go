// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"log"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
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

	exp.AddDataVariableNames([]string{"trial", "wait_time", "key", "rt"})

	// 2. Prepare stimuli
	target := stimuli.NewTextLine("+", 0, 0, control.DefaultTextColor)

	instrText := fmt.Sprintf("From time to time, a cross will appear at the center of screen.\n\nYour task is to press the SPACEBAR as quickly as possible when you see it (We measure your reaction-time).\n\nThere will be %d trials in total.\n\nPress the spacebar to start.", NTrials)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		exp.ShowInstructions(instrText)

		// Loop through trials
		for i := 0; i < NTrials; i++ {
			// Blank screen
			exp.Blank(0)

			waitTime := design.RandInt(MinWaitTime, MaxWaitTime-1)
			exp.Wait(waitTime)

			// Target stimulus
			exp.Show(target)

			// Wait for response
			startTime := clock.GetTime()
			key, _ := exp.Keyboard.Wait()
			rt := clock.GetTime() - startTime

			exp.Data.Add(i, waitTime, key, rt)
			fmt.Printf("Trial %d: Wait=%d ms, Key=%d, RT=%d ms\n", i, waitTime, key, rt)

			// Small pause between trials
			exp.Wait(500)
		}

		return control.EndLoop // Graceful exit
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
