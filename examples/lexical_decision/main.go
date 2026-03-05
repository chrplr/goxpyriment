// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"encoding/csv"
	"flag"
	"fmt"
	"goxpyriment/control"
	"goxpyriment/misc"
	"goxpyriment/stimuli"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

//go:embed assets/Inconsolata.ttf
var inconsolataFont []byte

const (
	WordResponseKey   = sdl.K_F
	NonWordResponseKey = sdl.K_J
	MaxResponseDelay  = 2000
)

type lexicalTrial struct {
	item     string
	category string
	stim     *stimuli.TextLine
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fullscreen := flag.Bool("F", false, "Launch in fullscreen display mode")
	flag.Parse()

	// 1. Get CSV file from command line
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go CSVFILE")
		os.Exit(1)
	}
	stimFile := os.Args[1]

	// 2. Load stimuli from CSV
	file, err := os.Open(stimFile)
	if err != nil {
		log.Fatalf("failed to open stimuli file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("failed to read stimuli: %v", err)
	}

	// Assume first line is header: item,category
	var trials []lexicalTrial
	for i, record := range records {
		if i == 0 {
			continue // skip header
		}
		item := record[0]
		category := record[1]
		stim := stimuli.NewTextLine(item, 0, 0, control.DefaultTextColor)
		trials = append(trials, lexicalTrial{item: item, category: category, stim: stim})
	}

	// 3. Create and initialize the experiment
	exp := control.NewExperiment("Lexical Decision", 1368, 1024, *fullscreen)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	if err := exp.LoadFontFromMemory(inconsolataFont, 32); err != nil {
		log.Printf("Warning: failed to load font: %v. Using fallback.", err)
	}

	exp.Data.AddVariableNames([]string{"item", "category", "key", "rt"})

	// 4. Shuffle trials
	rand.Shuffle(len(trials), func(i, j int) {
		trials[i], trials[j] = trials[j], trials[i]
	})

	// 5. Prepare common stimuli
	cue := stimuli.NewFixCross(50, 4, control.DefaultTextColor)
	
	instrText := fmt.Sprintf("When you'll see a stimulus, your task to decide, as quickly as possible, whether it is a word or not.\n\nif it is a word, press 'F'\n\nif it is a non-word, press 'J'\n\nPress the SPACE bar to start.")
	instructions := stimuli.NewTextBox(instrText, 600, sdl.FPoint{X: 0, Y: 100}, control.DefaultTextColor)

	// 6. Run the experiment logic
	err = exp.Run(func() error {
		// Instructions
		if err := instructions.Present(exp.Screen, true, true); err != nil {
			return err
		}
		if _, err := exp.Keyboard.WaitKeys([]sdl.Keycode{sdl.K_SPACE}, -1); err != nil {
			return err
		}

		// Loop through trials
		for _, t := range trials {
			// Blank screen
			if err := exp.Screen.Clear(); err != nil {
				return err
			}
			if err := exp.Screen.Update(); err != nil {
				return err
			}
			misc.Wait(1000)

			// Cue
			if err := cue.Present(exp.Screen, true, true); err != nil {
				return err
			}
			misc.Wait(500)

			// Stimulus
			if err := t.stim.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Wait for response
			startTime := misc.GetTime()
			key, err := exp.Keyboard.WaitKeys([]sdl.Keycode{WordResponseKey, NonWordResponseKey}, MaxResponseDelay)
			if err != nil {
				return err
			}
			rt := misc.GetTime() - startTime
			
			// RT would be 0 or very large if wait timed out and returned 0,
			// but RT is calculated from startTime.
			// Actually, if key is 0, it means timeout.
			
			exp.Data.Add([]interface{}{t.item, t.category, key, rt})
			fmt.Printf("Trial: Item=%s, Cat=%s, Key=%d, RT=%d ms\n", t.item, t.category, key, rt)

			// Small pause between trials
			misc.Wait(500)
		}

		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
