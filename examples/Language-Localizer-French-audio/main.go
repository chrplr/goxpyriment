// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Language-Localizer is an auditory language localizer for fMRI.
//
// It is a port of an Expyriment experiment (E. Lin, C. Pallier 2016). On each trial a
// spoken French sentence is played at a precise onset time measured from the
// scanner trigger. Sentences are presented either forward (normal, intelligible
// speech) or temporally reversed as a low-level acoustic control that preserves
// the spectral energy but destroys linguistic content. The block structure and
// the exact onset schedule live in a stimulus table passed as a command-line
// argument.
//
// The participant does nothing but listen while fixating; contrasting the BOLD
// response to forward speech against its reversed control localizes the language
// network.
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
//	go run . -s 2 list.csv     # subject 2, stimulus table list.csv
//	go run . -w -s 2 list.csv  # windowed mode for testing
//
// Flags: -w windowed mode, -d N display index, -s N subject ID.
// The stimulus table is given as a positional argument.
package main

import (
	"embed"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/results"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Sentence audio files are embedded so the binary is self-contained (the
// browser/WASM build has no filesystem, and even on desktop this avoids any
// load jitter mid-run). The stimulus table, in contrast, is read from a file
// path given on the command line.
//
//go:embed sound_files/*.wav
var soundFS embed.FS

// trial is one row of a stimulus table.
type trial struct {
	subj      int
	ntrial    int
	nbloc     int    // block number; 99 marks the terminal silent probe
	langue    string // condition label: "french" (forward) or "control" (reversed)
	sentDur   int    // sentence duration in ms (informational)
	fname     string // WAV file name inside sound_files/
	sentOnset int    // scheduled onset in ms from the scanner trigger
}

func main() {
	exp := control.NewExperimentFromFlags(
		"Language-Localizer-French-Audio",
		control.Black, control.White, 32,
	)
	defer exp.End()

	// The stimulus table is the first positional argument (NewExperimentFromFlags
	// has already called flag.Parse(), so flag.Args() holds the non-flag args).
	args := flag.Args()
	if len(args) < 1 {
		log.Fatalf("usage: %s [-w] [-d N] -s N <stimulus-table.csv>", os.Args[0])
	}
	csvName := args[0]
	trials, err := loadTrials(csvName)
	if err != nil {
		log.Fatalf("cannot read stimulus table %q: %v", csvName, err)
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

	// Every key press during the run is logged to a companion file next to the
	// main results CSV (same directory and basename, "_keypresses" suffix). The
	// per-trial CSV has a fixed sentence-oriented schema, so asynchronous
	// keypresses go to their own tidy file: subject_id, time_ms (from the
	// scanner trigger, same clock as sent_onset), keycode, key.
	kpName := strings.TrimSuffix(exp.Data.Filename, ".csv") + "_keypresses.csv"
	keypresses, err := results.NewOutputFile(exp.Data.Directory, kpName)
	if err != nil {
		log.Fatalf("cannot create keypress log %q: %v", kpName, err)
	}
	keypresses.WriteLine("subject_id,time_ms,keycode,key")
	log.Printf("logging key presses to %s", keypresses.FullPath)

	exp.ShowInstructions(
		"Language localizer\n\n" +
			"Please stay still and keep your eyes on the cross.\n" +
			"Just listen to the sentences.\n\n" +
			"Operator: press SPACE, then wait for the scanner (t) trigger.",
	)

	exp.Run(func() error {
		// Flush the keypress log whichever way the run ends (normal finish, ESC
		// abort, or quit): defers run during panic unwinding too.
		defer func() {
			if serr := keypresses.Save(); serr != nil {
				log.Printf("could not save keypress log: %v", serr)
			}
		}()

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
			// Wait until this sentence's scheduled onset while logging any key
			// pressed in the meantime. Onsets are absolute (ms from the
			// trigger), so the ~1 ms wait granularity does not accumulate into
			// drift.
			if werr := waitLoggingKeys(exp, clk, int64(t.sentOnset), keypresses); werr != nil {
				return werr
			}

			before := clk.NowMillis()
			if perr := sounds[t.fname].Play(); perr != nil {
				log.Printf("trial %d: could not play %q: %v", t.ntrial, t.fname, perr)
			}
			after := clk.NowMillis()

			exp.Data.Add(t.subj, t.nbloc, t.langue, t.sentOnset,
				before, after, t.sentDur, t.fname)
		}
		// Keep recording for 10 s after the last sentence before closing.
		if werr := waitLoggingKeys(exp, clk, clk.NowMillis()+10000, keypresses); werr != nil {
			return werr
		}
		return control.EndLoop
	})
}

// waitLoggingKeys blocks until targetMs (elapsed on clk, i.e. ms from the
// scanner trigger — the same reference as sent_onset) while recording every
// key press to kp. It returns immediately if the target is already in the
// past, and returns control.EndLoop if ESC or a window-close is received so
// the run aborts cleanly.
//
// Key events that arrive during the brief play/record gap between successive
// waits stay queued in SDL and are drained at the start of the next wait, so
// none are lost. The timestamp is taken from clk when the event is dequeued
// (~1 ms polling granularity), keeping it on the same clock as the onsets.
func waitLoggingKeys(exp *control.Experiment, clk *clock.Clock, targetMs int64, kp *results.OutputFile) error {
	for {
		remaining := targetMs - clk.NowMillis()
		if remaining <= 0 {
			return nil
		}
		// keys=nil matches any key; catchMouse=false so only key presses are
		// returned. Returns a zero event (Key==0) on timeout.
		ev, err := exp.WaitAnyEventTS(nil, false, int(remaining))
		if err != nil { // control.EndLoop on ESC / quit
			return err
		}
		if ev.Key != 0 {
			kp.WriteLine(fmt.Sprintf("%d,%d,%d,%s",
				exp.SubjectID, clk.NowMillis(), int(ev.Key), keyLabel(ev.Key)))
		}
	}
}

// keyLabel renders a keycode as a readable name: the character itself for
// printable ASCII keys, otherwise "key_<code>".
func keyLabel(k control.Keycode) string {
	if k >= 32 && k < 127 {
		return string(rune(k))
	}
	return fmt.Sprintf("key_%d", int(k))
}

// loadTrials reads and parses a stimulus table from the given file path.
// Columns: subj, ntrial, nbloc, langue, sent_dur, fname, sent_onset.
func loadTrials(name string) ([]trial, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
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
