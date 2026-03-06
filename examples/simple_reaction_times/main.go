// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/misc"
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

	fullscreen := flag.Bool("F", false, "Launch in fullscreen display mode")
	flag.Parse()

	// 1. Create and initialize the experiment
	exp := control.NewExperiment("Visual Detection", 1368, 1024, *fullscreen)
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
			misc.Wait(waitTime)

			// Target stimulus
			if err := target.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Wait for response
			startTime := misc.GetTime()
			key, err := exp.Keyboard.Wait()
			if err != nil {
				return err
			}
			rt := misc.GetTime() - startTime
			
			exp.Data.Add([]interface{}{i, waitTime, key, rt})
			fmt.Printf("Trial %d: Wait=%d ms, Key=%d, RT=%d ms\n", i, waitTime, key, rt)
			
			// Small pause between trials
			misc.Wait(500)
		}

		return sdl.EndLoop // Graceful exit
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
