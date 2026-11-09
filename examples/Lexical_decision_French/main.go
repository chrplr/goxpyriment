// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// French lexical decision contrasting word length (4 vs 6 characters) and word
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

// materials has one item per row and no header: item,length,category, where
// category is HF or LF for a word and PW for a pseudoword.
//
//go:embed stimuli/materiel_lexdec.csv
var materials string

const (
	wordKey   = control.K_J
	pseudoKey = control.K_F

	interTrialMs = 2000

	categoryHF     = "HF"
	categoryLF     = "LF"
	categoryPseudo = "PW"

	labelWord   = "word"
	labelPseudo = "pseudoword"

	instructions = "Des suites de lettres vont apparaître une par une au centre de l'écran.\n\n" +
		"Pour chacune, décidez aussi vite et aussi correctement que possible s'il s'agit d'un mot français.\n\n" +
		"C'est un mot : appuyez sur J\n\n" +
		"Ce n'est pas un mot : appuyez sur F\n\n" +
		"Appuyez sur la barre d'espace pour commencer."

	endMessage = "L'expérience est terminée.\n\nMerci de votre participation !\n\nAppuyez sur une touche pour quitter."
)

type trial struct {
	item     string
	length   int
	category string // categoryHF, categoryLF or categoryPseudo
}

// lexicality is the correct response for the trial.
func (t trial) lexicality() string {
	if t.category == categoryPseudo {
		return labelPseudo
	}
	return labelWord
}

// parseTrials reads the materials. A bad row is an error rather than a skipped
// row, since a dropped item would silently unbalance the design.
func parseTrials(data string) ([]trial, error) {
	reader := csv.NewReader(strings.NewReader(data))
	reader.FieldsPerRecord = 3
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading materials: %w", err)
	}

	trials := make([]trial, 0, len(rows))
	for i, row := range rows {
		item, category := row[0], row[2]
		length, err := strconv.Atoi(row[1])
		if err != nil {
			return nil, fmt.Errorf("materials row %d (%q): bad length: %w", i+1, item, err)
		}
		// Count characters, not bytes: "étui" is 4 characters but 5 bytes.
		if n := utf8.RuneCountInString(item); n != length {
			return nil, fmt.Errorf("materials row %d (%q): has %d characters, listed as %d", i+1, item, n, length)
		}
		switch category {
		case categoryHF, categoryLF, categoryPseudo:
		default:
			return nil, fmt.Errorf("materials row %d (%q): unknown category %q", i+1, item, category)
		}
		trials = append(trials, trial{item: item, length: length, category: category})
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

	exp.AddDataVariableNames([]string{"trial", "item", "length", "category", "lexicality", "response", "rt", "correct"})

	responseKeys := []control.Keycode{wordKey, pseudoKey}

	err = exp.Run(func() error {
		if err := exp.ShowInstructions(instructions); err != nil {
			return err
		}

		for i, t := range trials {
			if err := exp.Blank(interTrialMs); err != nil {
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

			response := labelPseudo
			if key == wordKey {
				response = labelWord
			}
			exp.Data.Add(i, t.item, t.length, t.category, t.lexicality(), response, rt, response == t.lexicality())
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
