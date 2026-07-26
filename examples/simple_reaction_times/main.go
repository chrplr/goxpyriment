// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math"
	"sort"

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

	exp.HideCursor()
	exp.AddDataVariableNames([]string{"trial", "wait_time", "key", "rt"})

	// 2. Prepare stimuli
	target := stimuli.NewFixCross(40, 5, control.DefaultTextColor)

	instrText := fmt.Sprintf("From time to time, a cross will appear at the center of screen.\n\nYour task is to press the SPACEBAR as quickly as possible when you see it (We measure your reaction-time).\n\nThere will be %d trials in total.\n\nPress the spacebar to start.", NTrials)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		exp.ShowInstructions(instrText)

		// Loop through trials
		var rts []float64
		for i := 0; i < NTrials; i++ {
			// Blank screen
			exp.Blank(0)

			waitTime := design.RandInt(MinWaitTime, MaxWaitTime-1)
			exp.Wait(waitTime)

			// Target stimulus — show and measure hardware-precise RT
			key, rtMS, _ := exp.ShowAndGetRT(target, nil, MaxResponseDelay)

			if key != 0 {
				rts = append(rts, float64(rtMS))
			}
			exp.Data.Add(i, waitTime, key, rtMS)
			fmt.Printf("Trial %d: Wait=%d ms, Key=%d, RT=%d ms\n", i, waitTime, key, rtMS)

			// Display RT feedback for 2 seconds
			var feedbackText string
			if key == 0 {
				feedbackText = "Too slow!"
			} else {
				feedbackText = fmt.Sprintf("RT: %d ms", rtMS)
			}
			exp.ShowTimed(stimuli.NewTextLine(feedbackText, 0, 0, control.DefaultTextColor), 2000)
		}

		// Summary screen
		summaryText := "Experiment complete!\n\nNo valid responses recorded.\n\nPress any key to exit."
		if len(rts) > 0 {
			sort.Float64s(rts)
			n := len(rts)
			var median float64
			if n%2 == 0 {
				median = (rts[n/2-1] + rts[n/2]) / 2
			} else {
				median = rts[n/2]
			}
			mean := 0.0
			for _, v := range rts {
				mean += v
			}
			mean /= float64(n)
			variance := 0.0
			for _, v := range rts {
				d := v - mean
				variance += d * d
			}
			stddev := math.Sqrt(variance / float64(n))
			summaryText = fmt.Sprintf("Experiment complete!\n\nMedian RT: %.0f ms\nSD: %.0f ms\n\nPress any key to exit.", median, stddev)
		}
		exp.ShowInstructions(summaryText)

		return control.EndLoop // Graceful exit
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
