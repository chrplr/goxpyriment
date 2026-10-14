// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Language-Localizer-Reading-English is the EvLab visual language localizer
// for fMRI: the participant reads sentences and matched nonword sequences
// presented word by word (RSVP). The sentences > nonwords contrast localizes
// the high-level language network (Fedorenko et al., 2010).
//
// It is a port of the Psychtoolbox implementation `evlab_langloc_2conds.m`
// (T. Scott, MIT, 2013), and uses that experiment's stimulus tables unchanged.
//
// Structure of a run (358 s = 5 min 58 s, ips = 179 at TR = 2 s):
//
//	FIX  B1 B2 B3 B4  FIX  B5 B6 B7 B8  FIX  B9 B10 B11 B12  FIX  B13 B14 B15 B16  FIX
//
// 16 blocks of 3 trials (48 trials), 8 blocks per condition, each block 18 s;
// 5 fixation periods of 14 s. Every trial lasts exactly 6000 ms:
//
//	100 ms blank | 12 words x 450 ms = 5400 ms | 400 ms press-probe | 100 ms blank
//
// The probe is a large filled disc. Whenever it appears the participant presses
// the response key (default '1'), which keeps them alert; the reading is the
// real task. A press is credited to the most recent probe, up to the next one —
// so a press arriving during the following sentence still counts, as in the
// original.
//
// Trial onsets are absolute, measured from the scanner trigger, exactly as in
// the Psychtoolbox version: nothing accumulates drift, and a slow trial cannot
// push the schedule.
//
// Flow:
//
//  1. An instruction screen (operator presses SPACE when ready).
//  2. A GREEN fixation cross while the program waits for the scanner
//     synchronisation pulse ('t' by default, see -trigger).
//  3. A GREY fixation cross; the clock starts at the trigger and the first
//     trial begins 14 s later.
//
// Usage:
//
//	go run . -s 3                  # subject 3, run 1, set 1
//	go run . -s 3 -run 2           # the second run of the same session
//	go run . -s 3 -set 2           # a subject already scanned on set 1
//	go run . -w -s 999             # windowed mode, for testing
//	go run . -autostart -s 999     # no SPACE, no trigger: start immediately
//	go run . -s 3 -ttl dlpio8      # EEG/MEG: TTL out through a DLP-IO8-G (auto-detected)
//
// Flags: -w windowed mode, -d N display index, -s N subject ID, -run 1|2,
// -set 1..5, -trigger KEY, -autostart, -fontsize N, -ttl KIND[:PORT],
// -ttl-ms N, -photodiode, -photodiode-size N, -marker-frames N,
// -ttl-pin-sentence/-nonword/-probe N. The -photodiode and -ttl flags are the
// library's onset marker (control.MarkerFlags).
//
// -photodiode adds a 200 px white square in the top-left corner, flashed for
// one frame on exactly the frames that carry a word onset and the probe, so a
// photodiode can be compared against the TTL pin on a scope.
//
// With -ttl the experiment marks the recording on three pins of the device
// (a DLP-IO8-G by design): pin 1 is pulsed at every word onset of a sentence
// trial, pin 2 at every word onset of a nonword trial, and pin 3 at the
// press-probe. See the pin constants below for why one pin per condition is
// used rather than 8-bit codes.
package main

import (
	"embed"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// The ten stimulus tables (2 runs x 5 sets) are embedded so the binary is
// self-contained and no file is read once the run has started.
//
//go:embed stim/*.csv
var stimFS embed.FS

// Trial and block timing, in milliseconds. These reproduce the Psychtoolbox
// script; changing any of them changes the fMRI design matrix.
const (
	nTrials      = 48  // 16 blocks x 3 trials
	wordsPerItem = 12  // every sentence and nonword string is 12 items long
	preBlankMs   = 100 // blank screen before the first word
	wordMs       = 450 // one word, no gap between words
	probeMs      = 400 // press-probe (the disc)
	postBlankMs  = 100 // blank screen after the probe
	trialMs      = preBlankMs + wordsPerItem*wordMs + probeMs + postBlankMs
	fixationMs   = 14000 // each of the 5 fixation periods
	blockTrials  = 12    // a fixation period follows every 12 trials
)

// probeRadius is the radius of the press-probe disc, in pixels. The original
// showed a 480x480 photograph of a hand pressing a button; a disc is the same
// "press now" signal without a third-party image.
const probeRadius = 110

// TTL output, when -ttl selects a device (e.g. -ttl dlpio8). The condition is
// carried by WHICH pin pulses:
//
//	pin 1  pulsed at each word onset in a sentence trial
//	pin 2  pulsed at each word onset in a nonword trial
//	pin 3  pulsed at the onset of the press-probe, which ends the trial
//
// A trial is therefore 12 pulses on pin 1 or on pin 2, followed by one on
// pin 3. These are defaults; change them with -ttl-pin-sentence,
// -ttl-pin-nonword and -ttl-pin-probe.
//
// Pins are numbered 1-8 as they are labelled on the board (and as
// tests/test_dlpio8 numbers them); pin N is the code with bit N-1 set.
//
// Only ever ONE pin changes at a time. The DLP-IO8-G has no write-all
// command — triggers.DLPIO8.Send writes one byte per line, so an 8-bit code is
// eight sequential edges over the serial link, and a recording system latching
// on any edge would see the intermediate values. One pin per condition avoids
// that entirely: the onset marker writes a single-bit code as one SetHigh.
const (
	defaultSentencePin = 1
	defaultNonwordPin  = 2
	defaultProbePin    = 3
)

// Onset-marker defaults for this paradigm (see control.MarkerFlags). The TTL
// width is a duration because it is about the recording system; the
// photodiode square is a frame count because it is about what is on screen.
// Clearing the line on a frame count instead produced 0.087 ms pulses
// (measured on an AD3, 2026-08-19: the render loop runs a frame ahead of
// scan-out) — too short for most EEG/MEG inputs to latch.
const (
	markerTTLMs        = 10  // TTL pulse width, words and probe
	markerFrames       = 1   // frames the photodiode square is white
	markerPhotodiodePx = 200 // side of the photodiode square
)

// pinCode is the TTL code that raises board pin (1-8) alone; 0 for pin 0.
func pinCode(pin int) byte {
	if pin < 1 {
		return 0
	}
	return 1 << uint(pin-1)
}

// trial is one row of a stimulus table.
type trial struct {
	item    int      // item number within the set (column stim1)
	words   []string // the 12 words or nonwords (columns stim2..stim13)
	cond    string   // "S" (sentence) or "N" (nonwords), column stim14
	onsetMs int      // scheduled onset in ms from the scanner trigger
}

// record is what is known about a trial. Responses are resolved after the fact
// — a press may arrive during the *next* trial — so a row is only final once
// the following probe has appeared, and that is when it is written out.
type record struct {
	t              trial
	displayedOnset int64 // real onset of the first word, ms from the trigger
	probeOnsetNS   uint64
	probeOnsetMs   int64 // real onset of the probe, ms from the trigger
	responded      bool
	rtMs           int64 // from the probe onset
}

func main() {
	// Experiment-specific flags must be registered before
	// NewExperimentFromFlags, which calls flag.Parse().
	run := flag.Int("run", 1, "run number: 1 or 2 (the two counterbalanced condition orders)")
	set := flag.Int("set", 1, "stimulus set: 1..5 (use a set the participant has not seen)")
	triggerKey := flag.String("trigger", "t", "key sent by the scanner to start the run (single character)")
	autostart := flag.Bool("autostart", false, "skip the SPACE prompt and the scanner trigger; start the clock immediately (for timing audits)")
	fontSize := flag.Int("fontsize", 72, "point size of the RSVP words")
	sentencePin := flag.Int("ttl-pin-sentence", defaultSentencePin,
		"TTL pin pulsed at each word onset of a SENTENCE trial (1-8, as labelled on the board)")
	nonwordPin := flag.Int("ttl-pin-nonword", defaultNonwordPin,
		"TTL pin pulsed at each word onset of a NONWORD trial (1-8)")
	probePin := flag.Int("ttl-pin-probe", defaultProbePin,
		"TTL pin pulsed at the press-probe (1-8); 0 to send no probe marker")
	// -photodiode, -photodiode-size, -marker-frames, -ttl (e.g. -ttl dlpio8,
	// -ttl dlpio8:/dev/ttyUSB0) and -ttl-ms mark every word onset and the probe.
	control.MarkerFlags(control.MarkerConfig{
		PhotodiodeSize: markerPhotodiodePx,
		Frames:         markerFrames,
		TTLDuration:    markerTTLMs * time.Millisecond,
	})

	exp := control.NewExperimentFromFlags(
		"langloc_reading_english",
		control.Black, control.White, float32(*fontSize),
	)
	defer exp.End()

	if *run < 1 || *run > 2 {
		log.Fatalf("-run must be 1 or 2, got %d", *run)
	}
	if *set < 1 || *set > 5 {
		log.Fatalf("-set must be between 1 and 5, got %d", *set)
	}
	trigger, err := triggerKeycode(*triggerKey)
	if err != nil {
		log.Fatalf("-trigger: %v", err)
	}

	csvName := fmt.Sprintf("stim/langloc_fmri_run%d_stim_set%d.csv", *run, *set)
	raw, err := stimFS.ReadFile(csvName)
	if err != nil {
		log.Fatalf("cannot read stimulus table %q: %v", csvName, err)
	}
	trials, err := loadTrials(csvName, raw)
	if err != nil {
		log.Fatalf("cannot parse stimulus table %q: %v", csvName, err)
	}
	log.Printf("loaded %d trials from %s (%s)", len(trials), csvName, conditionOrder(trials))

	// Pin checks. (A device that cannot be opened has already stopped the
	// program, in NewExperimentFromFlags.)
	if exp.Marker.TTLEnabled() {
		for _, p := range []struct {
			name string
			pin  int
		}{{"-ttl-pin-sentence", *sentencePin}, {"-ttl-pin-nonword", *nonwordPin}} {
			if p.pin < 1 || p.pin > 8 {
				log.Fatalf("%s: pin %d out of range (1-8)", p.name, p.pin)
			}
		}
		if *probePin < 0 || *probePin > 8 {
			log.Fatalf("-ttl-pin-probe: pin %d out of range (1-8, or 0 for none)", *probePin)
		}
		if *sentencePin == *nonwordPin {
			log.Fatalf("-ttl-pin-sentence and -ttl-pin-nonword are both pin %d: the two conditions would be indistinguishable",
				*sentencePin)
		}
		probeDesc := fmt.Sprintf("pin %d = probe", *probePin)
		if *probePin == 0 {
			probeDesc = "no probe marker"
		}
		log.Printf("TTL triggers on %s: pin %d = word onset (sentences), pin %d = word onset (nonwords), %s",
			exp.Marker.OutputDesc(), *sentencePin, *nonwordPin, probeDesc)
		exp.AddExperimentInfo(fmt.Sprintf("ttl pins: %d = word onset (sentences), %d = word onset (nonwords), %s",
			*sentencePin, *nonwordPin, probeDesc))
	}

	// Fixation crosses, matching the other localizers in this repository:
	// green while waiting for the scanner, grey for the run itself.
	fixGreen := stimuli.NewFixCross(45, 5, control.Green)
	fixGrey := stimuli.NewFixCross(45, 3, control.RGB(192, 192, 192))
	probe := stimuli.NewCircle(probeRadius, exp.ForegroundColor)
	blank := stimuli.NewBlankScreen(exp.BackgroundColor)
	stimuli.PreloadVisualOnScreen(exp.Screen, fixGreen)
	stimuli.PreloadVisualOnScreen(exp.Screen, fixGrey)
	stimuli.PreloadVisualOnScreen(exp.Screen, probe)

	exp.AddDataVariableNames([]string{
		"run", "set", "trial", "block", "item", "condition", "words",
		"scheduled_onset", "displayed_onset", "probe_onset", "responded", "rt",
	})

	if !*autostart {
		exp.ShowInstructions(
			"Language localizer — reading\n\n" +
				"You will read sentences, and sequences of nonwords such as\n" +
				"BLICKET or FLORP, one word at a time.\n\n" +
				"Read them attentively and silently, as you would read a book.\n" +
				"Do not worry if they seem fast at first — you will get used to it.\n\n" +
				"At the end of each sequence a large disc appears:\n" +
				"press the response key whenever you see it.\n\n" +
				"Please stay still and keep your eyes on the centre of the screen.\n\n" +
				"Operator: press SPACE, then wait for the scanner (" +
				strings.ToLower(*triggerKey) + ") trigger.")
	}

	respKeys := []control.Keycode{control.K_1}
	records := make([]record, 0, len(trials))

	// Rows are written one trial behind the presentation: a trial's response
	// window only closes when the next probe appears, so its row is final only
	// then. flushed is the number of records already handed to exp.Data.
	flushed := 0
	flushUpTo := func(n int) {
		if flushed >= n {
			return
		}
		// exp.Data.Add only appends to an in-memory buffer; Save appends it to
		// the file. Without the Save a run that is killed rather than ended
		// with ESC loses every row it had "written". It is called from the
		// 400 ms probe window, where an append of a few hundred bytes is free.
		defer func() {
			if serr := exp.Data.Save(); serr != nil {
				log.Printf("WARNING: could not write the data file: %v", serr)
			}
		}()
		for ; flushed < n; flushed++ {
			r := records[flushed]
			responded, rt := 0, "n/a"
			if r.responded {
				responded = 1
				rt = strconv.FormatInt(r.rtMs, 10)
			}
			exp.Data.Add(*run, *set, flushed+1, flushed/3+1, r.t.item, r.t.cond,
				strings.Join(r.t.words, " "),
				r.t.onsetMs, r.displayedOnset, r.probeOnsetMs, responded, rt)
		}
	}

	exp.Run(func() error {
		if !*autostart {
			if serr := exp.Show(fixGreen); serr != nil {
				return serr
			}
			if kerr := exp.Keyboard.WaitKey(trigger); kerr != nil {
				return kerr
			}
		}

		// The clock and the SDL hardware clock are both anchored at the
		// trigger: clk drives the schedule, triggerNS converts the stream's
		// VSYNC-flip timestamps into ms from the trigger.
		if serr := exp.Show(fixGrey); serr != nil {
			return serr
		}
		clk := clock.NewClock()
		triggerNS := sdl.TicksNS()
		exp.Keyboard.Clear()

		// pending is the index of the trial whose probe is awaiting a press;
		// -1 when the last probe has already been answered. A press is credited
		// to that trial however late it arrives, up to the next probe — the
		// attribution rule of the original script.
		pending := -1
		credit := func(ts uint64) {
			if pending < 0 {
				return
			}
			records[pending].responded = true
			records[pending].rtMs = int64(ts-records[pending].probeOnsetNS) / 1_000_000
			pending = -1
		}

		// waitUntil holds until targetMs on the trigger clock, crediting any
		// response key pressed meanwhile. ESC propagates as EndLoop.
		waitUntil := func(targetMs int) error {
			for {
				remaining := targetMs - int(clk.NowMillis())
				if remaining <= 0 {
					return nil
				}
				key, ts, kerr := exp.Keyboard.GetKeyEventTS(respKeys, remaining)
				if kerr != nil {
					return kerr
				}
				if key == 0 {
					return nil // timed out: the deadline has been reached
				}
				credit(ts)
			}
		}

		// Onset marking. onFrame runs just before each flip; on the first frame
		// of a word it arms the marker, so the photodiode square turns white in
		// the flip that puts the word on screen and the trial's condition pin
		// is raised right after it, on the same SDL clock instant as the logged
		// TimingLog.OnsetNS. The marker lowers the pin markerTTLMs later.
		//
		// wordCode is set once per trial, before the stream, to the pin that
		// codes this trial's condition.
		wordCode := pinCode(*sentencePin)
		onFrame := func(ctx stimuli.FrameContext) error {
			if ctx.OnPhase && ctx.Frame == 0 {
				exp.Marker.Arm(wordCode)
			}
			return nil
		}

		for i, t := range trials {
			records = append(records, record{t: t})

			// Blank screen for 100 ms, starting at the scheduled trial onset.
			if werr := waitUntil(t.onsetMs); werr != nil {
				return werr
			}
			if serr := exp.Show(blank); serr != nil {
				return serr
			}

			// Which pin this trial's word onsets will pulse. Chosen here, in
			// the blank before the first word, so the frame callback has
			// nothing to decide.
			wordCode = pinCode(*sentencePin)
			if t.cond == "N" {
				wordCode = pinCode(*nonwordPin)
			}

			if werr := waitUntil(t.onsetMs + preBlankMs); werr != nil {
				return werr
			}

			// 12 words at 450 ms each, no gap. The stream is VSYNC-locked and
			// records every key event with a hardware timestamp, so presses
			// made while reading are not lost. The frame callback marks each
			// word onset; without -photodiode or -ttl the marker does nothing.
			stims := make([]stimuli.Stimulus, len(t.words))
			for j, w := range t.words {
				stims[j] = stimuli.NewTextLine(w, 0, 0, exp.ForegroundColor)
			}
			elements := stimuli.MakeRegularStream(stims, wordMs*time.Millisecond, 0)
			events, timing, perr := stimuli.PresentStreamOfStimuliFunc(
				exp.Screen, elements, 0, 0, onFrame)
			if perr != nil {
				return perr // includes EndLoop on ESC, handled by exp.Run
			}
			for _, ev := range events {
				if isResponse(ev, respKeys) {
					credit(ev.TimestampNS)
				}
			}
			records[i].displayedOnset = int64(timing[0].OnsetNS-triggerNS) / 1_000_000

			// Press-probe for 400 ms. It opens this trial's response window and
			// closes the previous one: an unanswered probe stays unanswered.
			// Its onset is marked like a word's: the square in its first flip,
			// the probe pin right after it (none with -ttl-pin-probe 0).
			exp.Marker.Arm(pinCode(*probePin))
			probeNS, serr := exp.ShowTS(probe)
			if serr != nil {
				return serr
			}
			records[i].probeOnsetNS = probeNS
			records[i].probeOnsetMs = int64(probeNS-triggerNS) / 1_000_000
			pending = i

			// The previous trial can no longer receive a response: its row is
			// final and is written now, so a run cut short by anything other
			// than ESC still leaves everything but the last trial on disk.
			flushUpTo(i)
			if werr := waitUntil(t.onsetMs + preBlankMs + wordsPerItem*wordMs + probeMs); werr != nil {
				return werr
			}

			// Blank screen for 100 ms, closing the 6000 ms trial.
			if serr := exp.Show(blank); serr != nil {
				return serr
			}
			if werr := waitUntil(t.onsetMs + trialMs); werr != nil {
				return werr
			}

			// A 14 s fixation period follows every 12th trial, including the
			// last one (that trailing period is part of the 358 s run).
			if (i+1)%blockTrials == 0 {
				if serr := exp.Show(fixGrey); serr != nil {
					return serr
				}
				if werr := waitUntil(t.onsetMs + trialMs + fixationMs); werr != nil {
					return werr
				}
			}
		}
		return control.EndLoop
	})

	// Whatever the run did not flush — the last trial, or every trial after the
	// one where ESC was pressed.
	flushUpTo(len(records))

	printSummary(records)
}

// isResponse reports whether a stream event is a press of one of keys.
func isResponse(ev stimuli.UserEvent, keys []sdl.Keycode) bool {
	if ev.Event.Type != sdl.EVENT_KEY_DOWN {
		return false
	}
	pressed := ev.Event.KeyboardEvent().Key
	for _, k := range keys {
		if pressed == k {
			return true
		}
	}
	return false
}

// triggerKeycode maps the -trigger flag (a single character) to a keycode.
func triggerKeycode(s string) (sdl.Keycode, error) {
	r := []rune(strings.ToLower(strings.TrimSpace(s)))
	if len(r) != 1 || r[0] > 127 {
		return 0, fmt.Errorf("expected a single ASCII character, got %q", s)
	}
	return sdl.Keycode(r[0]), nil
}

// loadTrials parses a stimulus table and computes the absolute onset schedule.
//
// Columns are those of the Psychtoolbox tables: stim1 is the item number,
// stim2..stim13 are the 12 words, stim14 is the condition ("S" or "N").
func loadTrials(name string, raw []byte) ([]trial, error) {
	rows, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) != nTrials+1 {
		return nil, fmt.Errorf("%s: expected %d data rows plus a header, got %d rows",
			name, nTrials, len(rows))
	}

	trials := make([]trial, 0, nTrials)
	for i, rec := range rows[1:] {
		lineno := i + 2
		if len(rec) != wordsPerItem+2 {
			return nil, fmt.Errorf("%s line %d: expected %d columns, got %d",
				name, lineno, wordsPerItem+2, len(rec))
		}
		item, cerr := strconv.Atoi(strings.TrimSpace(rec[0]))
		if cerr != nil {
			return nil, fmt.Errorf("%s line %d: item number: %w", name, lineno, cerr)
		}
		words := make([]string, wordsPerItem)
		for j := range words {
			words[j] = strings.TrimSpace(rec[j+1])
			if words[j] == "" {
				return nil, fmt.Errorf("%s line %d: empty word in column stim%d", name, lineno, j+2)
			}
		}
		cond := strings.ToUpper(strings.TrimSpace(rec[wordsPerItem+1]))
		if cond != "S" && cond != "N" {
			return nil, fmt.Errorf("%s line %d: condition must be S or N, got %q", name, lineno, cond)
		}

		// Onsets are absolute: the run opens with a fixation period, and one
		// more is inserted after every 12 trials.
		onset := fixationMs + i*trialMs + (i/blockTrials)*fixationMs
		trials = append(trials, trial{item: item, words: words, cond: cond, onsetMs: onset})
	}
	return trials, nil
}

// conditionOrder renders the block sequence ("SNNS NSNS …") for the log line,
// so the operator can see at a glance which counterbalancing was loaded.
func conditionOrder(trials []trial) string {
	var b strings.Builder
	for i := 0; i < len(trials); i += 3 {
		if i > 0 && i%blockTrials == 0 {
			b.WriteByte(' ')
		}
		b.WriteString(trials[i].cond)
	}
	return b.String()
}

// printSummary reports probe detection per condition — the only online check
// that the participant stayed awake.
func printSummary(records []record) {
	if len(records) == 0 {
		return
	}
	type acc struct{ n, hits int }
	byCond := map[string]*acc{"S": {}, "N": {}}
	var rts []int64
	for _, r := range records {
		a := byCond[r.t.cond]
		a.n++
		if r.responded {
			a.hits++
			rts = append(rts, r.rtMs)
		}
	}
	fmt.Printf("\n--- probe detection (%d trials presented) ---\n", len(records))
	for _, c := range []string{"S", "N"} {
		a := byCond[c]
		if a.n == 0 {
			continue
		}
		fmt.Printf("  %s: %3d/%3d responses (%.0f%%)\n",
			conditionName(c), a.hits, a.n, 100*float64(a.hits)/float64(a.n))
	}
	if len(rts) > 0 {
		var sum int64
		for _, rt := range rts {
			sum += rt
		}
		fmt.Printf("  mean RT from probe onset: %d ms\n", sum/int64(len(rts)))
	}
}

func conditionName(c string) string {
	if c == "S" {
		return "sentences"
	}
	return "nonwords "
}
