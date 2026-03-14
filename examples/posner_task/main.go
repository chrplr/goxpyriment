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

//go:embed assets/star.png
var starImg []byte

//go:embed assets/left_arrow.png
var leftArrowImg []byte

//go:embed assets/right_arrow.png
var rightArrowImg []byte

const MaxResponseDuration = 2000

type trial struct {
	congruency string
	side       string
}

func main() {
	rand.Seed(time.Now().UnixNano())

	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	// 1. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("Posner-task", width, height, fullscreen, control.Gray, control.White, 32)
	exp.SubjectID = *subject
	exp.ForegroundColor = control.Black
	
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// 2. Prepare design
	var trials []trial
	for i := 0; i < 40; i++ {
		trials = append(trials, trial{"congruent", "left"})
		trials = append(trials, trial{"congruent", "right"})
	}
	for i := 0; i < 10; i++ {
		trials = append(trials, trial{"incongruent", "left"})
		trials = append(trials, trial{"incongruent", "right"})
	}
	rand.Shuffle(len(trials), func(i, j int) {
		trials[i], trials[j] = trials[j], trials[i]
	})

	// 3. Prepare stimuli
	cross := stimuli.NewFixCross(40, 6, control.Black)
	
	targetLeft := stimuli.NewPictureFromMemory(starImg, -150, 0)
	targetRight := stimuli.NewPictureFromMemory(starImg, 150, 0)
	cueLeft := stimuli.NewPictureFromMemory(leftArrowImg, 0, 0)
	cueRight := stimuli.NewPictureFromMemory(rightArrowImg, 0, 0)

	instrText := "Keep your eyes fixated on the central cross.\n\nA cue will appear followed by a star.\nPress the spacebar as quickly as possible when you see the *STAR*.\n\nNote that the cue indicates the most probable location of the star.\n\nPress space to start."
	instructions := stimuli.NewTextBox(instrText, 600, sdl.FPoint{X: 0, Y: 100}, control.Black)

	// 4. Run the experiment logic
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

		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}
		clock.Wait(1000)

		for _, t := range trials {
			clock.Wait(1000)
			if err := cross.Present(exp.Screen, true, true); err != nil {
				return err
			}
			clock.Wait(1000)

			// Show cue
			var cue *stimuli.Picture
			if (t.congruency == "congruent" && t.side == "left") || (t.congruency == "incongruent" && t.side == "right") {
				cue = cueLeft
			} else {
				cue = cueRight
			}
			if err := cue.Present(exp.Screen, true, true); err != nil {
				return err
			}
			clock.Wait(2000)

			// Show target
			var target *stimuli.Picture
			if t.side == "left" {
				target = targetLeft
			} else {
				target = targetRight
			}
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

			fmt.Printf("Trial: %s, Side: %s, Key: %d, RT: %d ms\n", t.congruency, t.side, key, rt)

			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := exp.Screen.Update(); err != nil {
				return err
			}
		}

		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
