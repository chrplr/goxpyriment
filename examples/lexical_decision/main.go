// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	WordResponseKey    = control.K_F
	NonWordResponseKey = control.K_J
	MaxResponseDelay   = 2000
)

type lexicalTrial struct {
	item     string
	category string
	stim     *stimuli.TextLine
}

func main() {
	exp := control.NewExperimentFromFlags("Lexical Decision", control.Black, control.White, 32)
	defer exp.End()

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

	// Prepare event log header and write it as comments in the data file
	evLog := exp.CollectEventLog()
	evLog.SetSubjectID(fmt.Sprintf("%d", exp.SubjectID))
	evLog.SetCSVHeader([]string{"item", "category", "key", "rt"})
	exp.Data.WriteComment("--EVENT LOG")
	exp.Data.WriteComment(evLog.String())
	exp.Data.WriteComment("--TRIAL DATA")

	exp.AddDataVariableNames([]string{"item", "category", "key", "rt"})

	// 4. Shuffle trials
	design.ShuffleList(trials)

	// 5. Prepare common stimuli
	cue := stimuli.NewFixCross(50, 4, control.DefaultTextColor)

	instrText := fmt.Sprintf("When you'll see a stimulus, your task to decide, as quickly as possible, whether it is a word or not.\n\nif it is a word, press 'F'\n\nif it is a non-word, press 'J'\n\nPress the SPACE bar to start.")

	// 6. Run the experiment logic
	err = exp.Run(func() error {
		// Instructions
		exp.ShowInstructions(instrText)

		// Loop through trials
		for _, t := range trials {
			// Blank screen
			exp.Blank(1000)

			// Cue
			exp.Show(cue)
			exp.Wait(500)

			// Stimulus
			onsetNS, _ := exp.ShowNS(t.stim)

			// Wait for response
			key, eventTS, _ := exp.Keyboard.WaitKeysEventRT([]control.Keycode{WordResponseKey, NonWordResponseKey}, MaxResponseDelay)
			rt := int64(eventTS-onsetNS) / 1_000_000

			// RT would be 0 or very large if wait timed out and returned 0,
			// but RT is calculated from startTime.
			// Actually, if key is 0, it means timeout.

			exp.Data.Add(t.item, t.category, key, rt)
			fmt.Printf("Trial: Item=%s, Cat=%s, Key=%d, RT=%d ms\n", t.item, t.category, key, rt)

			// Small pause between trials
			exp.Wait(500)
		}

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment error: %v", err)
	}
}
