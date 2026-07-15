// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Language-Localizer is a bilingual auditory language localizer for fMRI.
//
// It is a port of an Expyriment experiment (E. Lin, 2016). On each trial a
// spoken sentence is played at a precise onset time measured from the scanner
// trigger. Sentences are grouped in three-sentence blocks, each block in one
// language: French (fr), Mandarin Chinese (ch), or Wolof (wol). The block
// structure and the exact onset schedule live in a per-subject stimulus table
// (stim/bilingue_localizer_subj<N>.csv); every subject hears the same sentences
// in a counterbalanced order.
//
// The participant does nothing but listen while fixating; contrasting the BOLD
// response to the native language against unknown languages localizes the
// language network.
//
// Flow:
//
//  1. An instruction screen (operator presses SPACE when ready).
//  2. A GREEN fixation cross while the program waits for the scanner
//     synchronisation pulse, delivered as a 't' keypress (standard for
//     MRI trigger boxes emulating a keyboard).
//  3. A GREY fixation cross for the rest of the run. A clock is started at the
//     trigger; each sentence is played when the clock reaches its scheduled
//     onset (sleep + short busy-wait for sub-millisecond accuracy).
//
// For every trial the scheduled onset and the actual onset (measured just
// before and just after the play call, in ms from the trigger) are written to
// the data file, so onset jitter can be checked offline.
//
// Usage:
//
//	go run . -s 2            # subject 2 -> stim/bilingue_localizer_subj2.csv
//	go run . -w -s 2         # windowed mode for testing
//	go run . -training -s 2  # short shared practice list instead of the table
//
// Flags: -w windowed mode, -d N display index, -s N subject ID,
// -training run the practice list (bilingue_localizer_training.csv).
package main

import (
	"bytes"
	"embed"
	"encoding/csv"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"strconv"
	"strings"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Sentence audio files and stimulus tables are embedded so the binary is
// self-contained (the browser/WASM build has no filesystem, and even on
// desktop this avoids any load jitter mid-run).
//
//go:embed sound_files/*.wav
var soundFS embed.FS

//go:embed stim/*.csv
var stimFS embed.FS

// The short practice run shares the sound pool and column layout with the main
// experiment; it is selected with -training.
//
//go:embed bilingue_localizer_training.csv
var trainingCSV []byte

// trial is one row of a per-subject stimulus table.
type trial struct {
	subj      int
	ntrial    int
	nbloc     int    // block number; 99 marks the terminal silent probe
	langue    string // "fr", "ch", "wol", or "" (silent row)
	sentDur   int    // sentence duration in ms (informational)
	fname     string // WAV file name inside sound_files/
	sentOnset int    // scheduled onset in ms from the scanner trigger
}

func main() {
	// Register experiment-specific flags before NewExperimentFromFlags, which
	// calls flag.Parse().
	training := flag.Bool("training", false,
		"run the short practice list (bilingue_localizer_training.csv) instead of the subject's table")

	exp := control.NewExperimentFromFlags(
		"bilingue_localizer",
		control.Black, control.White, 32,
	)
	defer exp.End()

	// Select the stimulus table: the shared practice list with -training,
	// otherwise the per-subject table chosen by -s.
	var (
		csvName string
		raw     []byte
		err     error
	)
	if *training {
		csvName = "bilingue_localizer_training.csv"
		raw = trainingCSV
	} else {
		csvName = fmt.Sprintf("stim/bilingue_localizer_subj%d.csv", exp.SubjectID)
		raw, err = fs.ReadFile(stimFS, csvName)
		if err != nil {
			log.Fatalf("cannot read stimulus table %q: %v\n"+
				"(available subjects are the numbers N in stim/bilingue_localizer_subjN.csv)",
				csvName, err)
		}
	}

	trials, err := loadTrials(csvName, raw)
	if err != nil {
		log.Fatalf("cannot parse stimulus table %q: %v", csvName, err)
	}
	log.Printf("loaded %d trials from %s", len(trials), csvName)

	// Preload every referenced sound once, binding a stream per file. Doing
	// this before the trigger guarantees no decoding happens during the run.
	sounds := make(map[string]*stimuli.Sound)
	for _, t := range trials {
		if _, ok := sounds[t.fname]; ok {
			continue
		}
		data, rerr := soundFS.ReadFile("sound_files/" + t.fname)
		if rerr != nil {
			log.Fatalf("missing embedded sound %q: %v", t.fname, rerr)
		}
		s := stimuli.NewSoundFromMemory(data)
		if perr := s.PreloadDevice(exp.AudioDevice); perr != nil {
			log.Fatalf("cannot preload sound %q: %v", t.fname, perr)
		}
		sounds[t.fname] = s
	}
	log.Printf("preloaded %d distinct sound files", len(sounds))

	// Fixation crosses (sizes/colours match the original Expyriment version).
	fixGreen := stimuli.NewFixCross(45, 5, control.Green)             // waiting for trigger
	fixGrey := stimuli.NewFixCross(45, 3, control.RGB(192, 192, 192)) // during the run
	stimuli.PreloadVisualOnScreen(exp.Screen, fixGreen)
	stimuli.PreloadVisualOnScreen(exp.Screen, fixGrey)

	exp.AddDataVariableNames([]string{
		"subj", "nbloc", "langue", "sent_onset",
		"real_sentence_onset_before", "real_sentence_onset_after",
		"sent_dur", "filename",
	})

	exp.ShowInstructions(
		"Bilingual language localizer\n\n" +
			"Please stay still and keep your eyes on the cross.\n" +
			"Just listen to the sentences.\n\n" +
			"Operator: press SPACE, then wait for the scanner (t) trigger.",
	)

	exp.Run(func() error {
		// Green cross: wait for the scanner synchronisation pulse ('t').
		if serr := exp.Show(fixGreen); serr != nil {
			return serr
		}
		if kerr := exp.Keyboard.WaitKey(control.K_T); kerr != nil {
			return kerr
		}

		// Grey cross for the remainder of the run, then start the clock at the
		// trigger so every onset is measured from t=0 (matches the original).
		if serr := exp.Show(fixGrey); serr != nil {
			return serr
		}
		clk := clock.NewClock()

		for _, t := range trials {
			// Wait until this sentence's scheduled onset. exp.Wait pumps SDL
			// events every frame, so the window stays responsive and ESC (or
			// window-close) aborts the run gracefully. Onsets are recomputed
			// against the absolute schedule each trial, so the ~1 ms wait
			// granularity does not accumulate into drift.
			if remaining := int(t.sentOnset) - int(clk.NowMillis()); remaining > 0 {
				if werr := exp.Wait(remaining); werr != nil {
					return werr
				}
			}

			before := clk.NowMillis()
			if perr := sounds[t.fname].Play(); perr != nil {
				log.Printf("trial %d: could not play %q: %v", t.ntrial, t.fname, perr)
			}
			after := clk.NowMillis()

			exp.Data.Add(t.subj, t.nbloc, t.langue, t.sentOnset,
				before, after, t.sentDur, t.fname)
		}
		return control.EndLoop
	})
}

// loadTrials parses a stimulus table (raw is its file contents; name is used
// only for error messages).
// Columns: subj, ntrial, nbloc, langue, sent_dur, fname, sent_onset.
func loadTrials(name string, raw []byte) ([]trial, error) {
	// Normalise line endings — the practice table uses old-Mac bare CR, which
	// encoding/csv does not treat as a record terminator.
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	raw = bytes.ReplaceAll(raw, []byte("\r"), []byte("\n"))

	r := csv.NewReader(bytes.NewReader(raw))
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("no data rows in %s", name)
	}

	// Map header names to column indices so the code is robust to column order.
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.TrimSpace(h)] = i
	}
	for _, required := range []string{"subj", "ntrial", "nbloc", "langue", "sent_dur", "fname", "sent_onset"} {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("missing column %q in %s", required, name)
		}
	}

	atoi := func(rec []string, key string) (int, error) {
		return strconv.Atoi(strings.TrimSpace(rec[col[key]]))
	}

	var trials []trial
	for lineno, rec := range rows[1:] {
		var t trial
		var e error
		if t.subj, e = atoi(rec, "subj"); e != nil {
			return nil, fmt.Errorf("%s line %d: subj: %w", name, lineno+2, e)
		}
		if t.ntrial, e = atoi(rec, "ntrial"); e != nil {
			return nil, fmt.Errorf("%s line %d: ntrial: %w", name, lineno+2, e)
		}
		if t.nbloc, e = atoi(rec, "nbloc"); e != nil {
			return nil, fmt.Errorf("%s line %d: nbloc: %w", name, lineno+2, e)
		}
		if t.sentDur, e = atoi(rec, "sent_dur"); e != nil {
			return nil, fmt.Errorf("%s line %d: sent_dur: %w", name, lineno+2, e)
		}
		if t.sentOnset, e = atoi(rec, "sent_onset"); e != nil {
			return nil, fmt.Errorf("%s line %d: sent_onset: %w", name, lineno+2, e)
		}
		t.langue = strings.TrimSpace(rec[col["langue"]])
		t.fname = strings.TrimSpace(rec[col["fname"]])
		trials = append(trials, t)
	}
	return trials, nil
}
