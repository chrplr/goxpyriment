// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"

)

const (
	WordResponseKey   = control.K_F
	NonWordResponseKey = control.K_J
	MaxResponseDelay  = 2000
)

type lexicalTrial struct {
	item     string
	category string
	stim     *stimuli.TextLine
}

func main() {
	rand.Seed(time.Now().UnixNano())

	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	subject := flag.Int("s", 0, "Subject ID")
	flag.Parse()

	// 1. Get CSV file from command line
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: lexical_decision [-F] CSVFILE")
		os.Exit(1)
	}
	stimFile := args[0]

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
		if len(record) < 2 {
			log.Printf("skipping malformed CSV line %d: %#v", i+1, record)
			continue
		}
		item := record[0]
		category := record[1]
		stim := stimuli.NewTextLine(item, 0, 0, control.DefaultTextColor)
		trials = append(trials, lexicalTrial{item: item, category: category, stim: stim})
	}

	// 3. Create and initialize the experiment
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	exp := control.NewExperiment("Lexical Decision", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = *subject
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Prepare event log header and write it as comments in the data file
	evLog := exp.CollectEventLog()
	evLog.SetSubjectID(fmt.Sprintf("%d", exp.SubjectID))
	evLog.SetCSVHeader([]string{"item", "category", "key", "rt"})
	exp.Data.WriteComment("--EVENT LOG")
	exp.Data.WriteComment(evLog.String())
	exp.Data.WriteComment("--TRIAL DATA")

	exp.Data.AddVariableNames([]string{"item", "category", "key", "rt"})

	// 4. Shuffle trials
	rand.Shuffle(len(trials), func(i, j int) {
		trials[i], trials[j] = trials[j], trials[i]
	})

	// 5. Prepare common stimuli
	cue := stimuli.NewFixCross(50, 4, control.DefaultTextColor)
	
	instrText := fmt.Sprintf("When you'll see a stimulus, your task to decide, as quickly as possible, whether it is a word or not.\n\nif it is a word, press 'F'\n\nif it is a non-word, press 'J'\n\nPress the SPACE bar to start.")
	instructions := stimuli.NewTextBox(instrText, 600, control.FPoint{X: 0, Y: 100}, control.DefaultTextColor)

	// 6. Run the experiment logic
	err = exp.Run(func() error {
		// Instructions
		if err := instructions.Present(exp.Screen, true, true); err != nil {
			return err
		}
		if _, err := exp.Keyboard.WaitKeys([]control.Keycode{control.K_SPACE}, -1); err != nil {
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
			clock.Wait(1000)

			// Cue
			if err := cue.Present(exp.Screen, true, true); err != nil {
				return err
			}
			clock.Wait(500)

			// Stimulus
			if err := t.stim.Present(exp.Screen, true, true); err != nil {
				return err
			}

			// Wait for response
			startTime := clock.GetTime()
			key, err := exp.Keyboard.WaitKeys([]control.Keycode{WordResponseKey, NonWordResponseKey}, MaxResponseDelay)
			if err != nil {
				return err
			}
			rt := clock.GetTime() - startTime
			
			// RT would be 0 or very large if wait timed out and returned 0,
			// but RT is calculated from startTime.
			// Actually, if key is 0, it means timeout.
			
			exp.Data.Add([]interface{}{t.item, t.category, key, rt})
			fmt.Printf("Trial: Item=%s, Cat=%s, Key=%d, RT=%d ms\n", t.item, t.category, key, rt)

			// Small pause between trials
			clock.Wait(500)
		}

		return control.EndLoop
	})

	if err != nil && err != control.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
