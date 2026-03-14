// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"
	"log"
	"math/rand"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

const (
	NTrials          = 20
	MinWaitTime      = 1000
	MaxWaitTime      = 2000
	MaxResponseDelay = 2000
)

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	// 1. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("Visual Detection", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	exp.Data.AddVariableNames([]string{"trial", "wait_time", "key", "rt"})

	// 2. Prepare stimuli
	target := stimuli.NewTextLine("+", 0, 0, control.DefaultTextColor)
	
	instrText := fmt.Sprintf("From time to time, a cross will appear at the center of screen.\n\nYour task is to press the SPACEBAR as quickly as possible when you see it (We measure your reaction-time).\n\nThere will be %d trials in total.\n\nPress the spacebar to start.", NTrials)
	instructions := stimuli.NewTextBox(instrText, 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)

	// 3. Run the experiment logic
	err := exp.Run(func() error {
		// Instructions
		if err := instructions.Present(exp.Screen, true, true); err != nil {
			return err
		}
		for {
			key, err := exp.Keyboard.Wait()
			if err != nil {
				return err
			}
			if key == sdl.K_SPACE {
				break
			}
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
			if err := target.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Wait for response
			startTime := clock.GetTime()
			key, err := exp.Keyboard.Wait()
			if err != nil {
				return err
			}
			rt := clock.GetTime() - startTime
			
			exp.Data.Add([]interface{}{i, waitTime, key, rt})
			fmt.Printf("Trial %d: Wait=%d ms, Key=%d, RT=%d ms\n", i, waitTime, key, rt)
			
			// Small pause between trials
			clock.Wait(500)
		}

		return sdl.EndLoop // Graceful exit
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
