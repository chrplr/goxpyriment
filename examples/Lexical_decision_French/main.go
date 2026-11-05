// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// French lexical decision crossing word length (4 vs 6 characters) with
// frequency (HF vs LF). Each item stays on screen until the participant
// responds: J for a word, F for a pseudoword.
package main

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// materials holds one item per row, without a header: item,length,code, where
// code is HF or LF for a word and PW for a pseudoword.
//
//go:embed stimuli/materiel_lexdec.csv
var materials string

const (
	wordKey   = control.K_J
	pseudoKey = control.K_F

	itiMs = 2000

	wordLabel   = "word"
	pseudoLabel = "pseudoword"

	instructions = "Des suites de lettres vont apparaître une par une au centre de l'écran.\n\n" +
		"Pour chacune, décidez le plus vite et le plus exactement possible s'il s'agit d'un mot français.\n\n" +
		"Mot français : appuyez sur J\n\n" +
		"Pas un mot français : appuyez sur F\n\n" +
		"Appuyez sur la barre d'espace pour commencer."

	endMessage = "L'expérience est terminée.\n\nMerci de votre participation !\n\nAppuyez sur une touche pour quitter."
)

type lexTrial struct {
	item   string
	length int
	freq   string // HF, LF or PW
}

// lexicality returns the correct response for the trial.
func (t lexTrial) lexicality() string {
	if t.freq == "PW" {
		return pseudoLabel
	}
	return wordLabel
}

// parseTrials reads the materials. A malformed row is an error rather than a
// skipped row, since a dropped item would silently unbalance the design.
func parseTrials(data string) ([]lexTrial, error) {
	reader := csv.NewReader(strings.NewReader(data))
	reader.FieldsPerRecord = 3
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading materials: %w", err)
	}

	trials := make([]lexTrial, 0, len(records))
	for i, record := range records {
		item, freq := record[0], record[2]
		length, err := strconv.Atoi(record[1])
		if err != nil {
			return nil, fmt.Errorf("materials row %d (%q): bad length: %w", i+1, item, err)
		}
		// Count characters, not bytes: "étui" has 4 characters but 5 bytes.
		if n := utf8.RuneCountInString(item); n != length {
			return nil, fmt.Errorf("materials row %d (%q): has %d characters, listed as %d", i+1, item, n, length)
		}
		if freq != "HF" && freq != "LF" && freq != "PW" {
			return nil, fmt.Errorf("materials row %d (%q): unknown code %q", i+1, item, freq)
		}
		trials = append(trials, lexTrial{item: item, length: length, freq: freq})
	}
	return trials, nil
}

func main() {
	trials, err := parseTrials(materials)
	if err != nil {
		log.Fatal(err)
	}
	design.ShuffleList(trials)

	exp := control.NewExperimentFromFlags("Lexical Decision French", control.Black, control.White, 32)
	defer exp.End()

	exp.AddDataVariableNames([]string{"trial", "item", "length", "freq", "lexicality", "response", "rt", "correct"})

	responseKeys := []control.Keycode{wordKey, pseudoKey}

	err = exp.Run(func() error {
		if err := exp.ShowInstructions(instructions); err != nil {
			return err
		}

		for i, t := range trials {
			if err := exp.Blank(itiMs); err != nil {
				return err
			}

			stim := stimuli.NewTextLine(t.item, 0, 0, exp.ForegroundColor)
			key, rt, err := exp.ShowAndGetRT(stim, responseKeys, -1)
			if err != nil {
				return err
			}
			if err := stim.Unload(); err != nil {
				return err
			}

			response := pseudoLabel
			if key == wordKey {
				response = wordLabel
			}
			exp.Data.Add(i, t.item, t.length, t.freq, t.lexicality(), response, rt, response == t.lexicality())
		}

		if err := exp.ShowEndMessage(endMessage); err != nil {
			return err
		}
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
