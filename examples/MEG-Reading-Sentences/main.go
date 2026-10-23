// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// MEG-Reading-Sentences is an MEG reading paradigm: the participant reads
// 12-word sentences and matched 12-item nonword strings presented one word at
// a time (RSVP, 450 ms per word, no gap), and every word onset is marked on the
// MEG STI channel with an 8-bit code that gives the condition and the word's
// position. A large disc after each item asks for a button press, which keeps
// the participant alert; the reading is the real task.
//
// It is the trial of the EvLab reading localizer
// (examples/Language-Localizer-Reading-English) in an event-related design:
// the 139 sentences and 139 nonword strings are intermixed in random order and
// dealt into -nruns runs (4 by default, about 8 min each). The lists depend
// only on -seed and the subject ID, so the runs of a session are launched one
// at a time with -run 1, -run 2, ... and together present every item once.
//
// Trial, absolute onsets from the start of the run:
//
//	100 ms blank | 12 words x 450 ms = 5400 ms | 400 ms disc probe | 1000-1500 ms fixation
//
// TTL codes (one byte, written right after the flip that shows the event):
//
//	 1-12  sentence word at position 1-12   (-ttl-sentence-base 0)
//	21-32  nonword at position 1-12         (-ttl-nonword-base 20)
//	   50  disc probe                       (-ttl-probe)
//	  100  start and end of the run         (-ttl-sync-code), for a clock-drift check
//
// Multi-bit codes must reach the lines at once, so a device that writes one
// line at a time (dlpio8, dlpio20) is refused; use the parallel port or the
// NeuroSpin megttlbox.
//
// Usage:
//
//	go run . -s 3 -practice                       # practice items, before the MEG
//	go run . -s 3 -run 1 -ttl parallel            # run 1 of 4, codes on the parallel port
//	go run . -w -s 999 -run 1 -autostart -ttl null   # windowed test, no hardware
//
// Press ESC (or close the window) to abort; the events already presented are
// in the data file.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const expName = "MEG-Reading-Sentences"

// Trial timing, in milliseconds.
const (
	preBlankMs  = 100 // blank screen before the first word
	wordMs      = 450 // one word, no gap between words
	probeMs     = 400 // press-probe (the disc)
	itiMinMs    = 1000
	itiMaxMs    = 1500 // fixation after the probe, drawn uniformly per trial
	leadInMs    = 2000 // fixation between the start of the run and the first trial
	trialCoreMs = preBlankMs + wordsPerItem*wordMs + probeMs
)

// Default TTL codes. A word's code is its condition's base plus its position
// (1-12), so the two word ranges are 1-12 and 21-32.
const (
	defaultSentenceBase = 0
	defaultNonwordBase  = 20
	defaultProbeCode    = 50
	defaultSyncCode     = 100
)

// Onset-marker defaults for this paradigm (see control.MarkerFlags): the
// photodiode square is on by default and white for one frame at every word and
// probe onset; the TTL pulse lasts 5 ms, far shorter than a word.
const (
	markerFrames       = 1
	markerTTLMs        = 5
	markerPhotodiodePx = 200
)

// probeRadius is the radius of the press-probe disc, in pixels, as in
// examples/Language-Localizer-Reading-English.
const probeRadius = 110

func main() {
	// Registered before NewExperimentFromFlags, which calls flag.Parse().
	var (
		flagRun       = flag.Int("run", 1, "run to present, 1 to -nruns")
		flagNRuns     = flag.Int("nruns", 4, "number of runs the 278 items are dealt into")
		flagPractice  = flag.Bool("practice", false, "present practice items from the training tables instead of a run")
		flagPracticeN = flag.Int("practice-n", 5, "practice items per condition")
		flagSeed      = flag.Uint64("seed", 1, "random seed for the trial lists and the fixation durations (combined with the subject ID)")
		flagFontSize  = flag.Int("fontsize", 72, "point size of the RSVP words")
		flagRespKey   = flag.String("resp-key", "1", "key sent by the response button (single character)")
		flagWaitTrig  = flag.Bool("wait-trigger", false, "wait for a start key (-trigger) on a green cross before the run")
		flagTrigger   = flag.String("trigger", "t", "start key awaited with -wait-trigger (single character)")
		flagAuto      = flag.Bool("autostart", false, "skip the instruction and result screens (for test runs)")

		flagSentBase = flag.Int("ttl-sentence-base", defaultSentenceBase, "TTL code of a sentence word is this plus its position (1-12)")
		flagNonBase  = flag.Int("ttl-nonword-base", defaultNonwordBase, "TTL code of a nonword is this plus its position (1-12)")
		flagProbe    = flag.Int("ttl-probe", defaultProbeCode, "TTL code at the onset of the press-probe disc (1-255)")
		flagSync     = flag.Int("ttl-sync-code", defaultSyncCode, "TTL code written at the start and at the end of the run, for clock-drift checks; 0 disables")
	)
	// -photodiode (on by default here), -photodiode-size, -marker-frames, -ttl
	// and -ttl-ms mark every word onset and every probe.
	control.MarkerFlags(control.MarkerConfig{
		Photodiode:     true,
		PhotodiodeSize: markerPhotodiodePx,
		Frames:         markerFrames,
		TTLDuration:    markerTTLMs * time.Millisecond,
	})

	exp := control.NewExperimentFromFlags(expName, control.Black, control.White, float32(*flagFontSize))
	defer exp.End()

	// ── Parameter checks ────────────────────────────────────────────────────
	codes := ttlCodes{sentenceBase: *flagSentBase, nonwordBase: *flagNonBase, probe: *flagProbe, sync: *flagSync}
	if err := codes.validate(); err != nil {
		log.Fatalf("TTL codes: %v", err)
	}
	if exp.Marker.TTLEnabled() {
		// Checked here rather than left to the device: a dlpio writes a code
		// line by line, and the STI channel would record the partial values.
		if kind := ttlKind(); kind == "dlpio8" || kind == "dlpio20" {
			log.Fatalf("-ttl %s writes the 8 lines one at a time, so a multi-bit code such as %d "+
				"reaches the STI channel as a series of partial values. Use a device that "+
				"sets all lines at once (parallel, megttlbox).", kind, codes.word("N", wordsPerItem))
		}
		if d := exp.Marker.TTLDuration(); d >= wordMs*time.Millisecond {
			log.Fatalf("-ttl-ms: the %v pulse must be shorter than a word (%d ms)", d, wordMs)
		}
	}
	respKey, err := keyFromFlag(*flagRespKey)
	if err != nil {
		log.Fatalf("-resp-key: %v", err)
	}
	trigger, err := keyFromFlag(*flagTrigger)
	if err != nil {
		log.Fatalf("-trigger: %v", err)
	}

	// ── Trial list ──────────────────────────────────────────────────────────
	rng := rand.New(rand.NewPCG(*flagSeed, uint64(exp.SubjectID)))
	var trials []item
	runLabel := strconv.Itoa(*flagRun)
	if *flagPractice {
		runLabel = "practice"
		ts, terr := loadTable("stimuli/training_sentences_shuffled.csv", "S")
		if terr != nil {
			log.Fatal(terr)
		}
		tn, terr := loadTable("stimuli/training_nonwords_shuffled.csv", "N")
		if terr != nil {
			log.Fatal(terr)
		}
		if trials, err = practiceList(ts, tn, *flagPracticeN, rng); err != nil {
			log.Fatalf("-practice-n: %v", err)
		}
	} else {
		if *flagRun < 1 || *flagRun > *flagNRuns {
			log.Fatalf("-run must be between 1 and -nruns (%d), got %d", *flagNRuns, *flagRun)
		}
		s, serr := loadTable("stimuli/sentences_shuffled.csv", "S")
		if serr != nil {
			log.Fatal(serr)
		}
		n, serr := loadTable("stimuli/nonwords_shuffled.csv", "N")
		if serr != nil {
			log.Fatal(serr)
		}
		runs, berr := buildRuns(s, n, *flagNRuns, rng)
		if berr != nil {
			log.Fatal(berr)
		}
		trials = runs[*flagRun-1]
	}

	// Absolute onsets: drawn after the lists, from the same generator, so they
	// too are reproducible from the seed and the subject ID.
	onsets := make([]int, len(trials))
	itis := make([]int, len(trials))
	next := leadInMs
	for i := range trials {
		itis[i] = itiMinMs + rng.IntN(itiMaxMs-itiMinMs+1)
		onsets[i] = next
		next += trialCoreMs + itis[i]
	}
	runEndMs := next
	log.Printf("%s run %s: %d trials, %.1f min", expName, runLabel, len(trials), float64(runEndMs)/60000)

	// ── Stimuli ─────────────────────────────────────────────────────────────
	fixGreen := stimuli.NewFixCross(45, 5, control.Green)
	fixGrey := stimuli.NewFixCross(45, 3, control.RGB(192, 192, 192))
	blank := stimuli.NewBlankScreen(exp.BackgroundColor)
	probe := stimuli.NewCircle(probeRadius, exp.ForegroundColor)
	stimuli.PreloadVisualOnScreen(exp.Screen, fixGreen)
	stimuli.PreloadVisualOnScreen(exp.Screen, fixGrey)
	stimuli.PreloadVisualOnScreen(exp.Screen, probe)

	// ── Data file ───────────────────────────────────────────────────────────
	exp.Data.WriteComment("--MEG READING SENTENCES")
	exp.Data.WriteComment(fmt.Sprintf("m run: %s of %d  seed: %d", runLabel, *flagNRuns, *flagSeed))
	exp.Data.WriteComment(fmt.Sprintf("m trial: %d ms blank, %d words x %d ms, %d ms probe, %d-%d ms fixation",
		preBlankMs, wordsPerItem, wordMs, probeMs, itiMinMs, itiMaxMs))
	exp.Data.WriteComment(fmt.Sprintf("m ttl_codes: sentence words %d-%d, nonwords %d-%d, probe %d, sync %d",
		codes.word("S", 1), codes.word("S", wordsPerItem), codes.word("N", 1), codes.word("N", wordsPerItem),
		codes.probe, codes.sync))
	exp.Data.WriteComment(fmt.Sprintf("m resp_key: %s", *flagRespKey))
	exp.Data.WriteComment("m times are ms from the start of the run on the SDL clock; word and probe onsets are the flips that showed them")
	exp.AddDataVariableNames([]string{
		"run", "event", "trial", "position", "condition", "item", "text", "code",
		"scheduled_ms", "onset_ms", "rt_ms", "outcome",
	})

	// ── Run ─────────────────────────────────────────────────────────────────
	r := &runner{exp: exp, codes: codes, run: runLabel, respKey: respKey, pending: -1}
	r.probeTrials = make([]probeResult, len(trials))
	runErr := exp.Run(func() error {
		if !*flagAuto {
			if ierr := exp.ShowInstructions(instructions(*flagPractice)); ierr != nil {
				return ierr
			}
		}
		if *flagWaitTrig {
			if serr := exp.Show(fixGreen); serr != nil {
				return serr
			}
			if kerr := exp.Keyboard.WaitKey(trigger); kerr != nil {
				return kerr
			}
		}
		if serr := exp.Show(fixGrey); serr != nil {
			return serr
		}
		exp.Keyboard.Clear()
		// t0 anchors every onset, on the SDL clock that also stamps flips and
		// key events.
		r.t0NS = control.TicksNS()
		if ts := exp.Marker.Fire(byte(codes.sync)); ts != 0 {
			r.addRow("sync_start", 0, 0, "", "", "", codes.sync, "", ts, "", "")
		}

		rerr := r.present(trials, onsets, runEndMs, blank, fixGrey, probe)
		// Sent even when the run was aborted, so the interval can still be
		// compared with the MEG file.
		if ts := exp.Marker.Fire(byte(codes.sync)); ts != 0 {
			r.addRow("sync_end", 0, 0, "", "", "", codes.sync, "", ts, "", "")
		}
		r.save()
		if rerr != nil {
			return rerr
		}
		r.summary()
		if !*flagAuto {
			if eerr := r.showResults(); eerr != nil {
				return eerr
			}
		}
		return control.EndLoop
	})
	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatalf("%s: %v", expName, runErr)
	}
}

// ── TTL codes ──────────────────────────────────────────────────────────────

type ttlCodes struct {
	sentenceBase, nonwordBase, probe, sync int
}

// word is the code of the word at position (1-12) of an item of condition cond.
func (c ttlCodes) word(cond string, position int) int {
	if cond == "S" {
		return c.sentenceBase + position
	}
	return c.nonwordBase + position
}

// validate checks that every code fits in a byte, that the two word ranges do
// not overlap, and that the probe and sync codes fall in neither.
func (c ttlCodes) validate() error {
	inWords := func(code int) bool {
		return (code > c.sentenceBase && code <= c.sentenceBase+wordsPerItem) ||
			(code > c.nonwordBase && code <= c.nonwordBase+wordsPerItem)
	}
	switch {
	case c.sentenceBase < 0 || c.sentenceBase+wordsPerItem > 255:
		return fmt.Errorf("-ttl-sentence-base %d: codes %d-%d do not fit in 1-255",
			c.sentenceBase, c.sentenceBase+1, c.sentenceBase+wordsPerItem)
	case c.nonwordBase < 0 || c.nonwordBase+wordsPerItem > 255:
		return fmt.Errorf("-ttl-nonword-base %d: codes %d-%d do not fit in 1-255",
			c.nonwordBase, c.nonwordBase+1, c.nonwordBase+wordsPerItem)
	case c.nonwordBase-c.sentenceBase < wordsPerItem && c.sentenceBase-c.nonwordBase < wordsPerItem:
		return fmt.Errorf("sentence codes %d-%d and nonword codes %d-%d overlap",
			c.sentenceBase+1, c.sentenceBase+wordsPerItem, c.nonwordBase+1, c.nonwordBase+wordsPerItem)
	case c.probe < 1 || c.probe > 255 || inWords(c.probe):
		return fmt.Errorf("-ttl-probe %d must be in 1-255 and outside the word codes", c.probe)
	case c.sync < 0 || c.sync > 255 || inWords(c.sync) || c.sync == c.probe:
		return fmt.Errorf("-ttl-sync-code %d must be 0 or in 1-255, distinct from the word and probe codes", c.sync)
	}
	return nil
}

// ttlKind is the device kind of the -ttl spec ("dlpio8" for "dlpio8:COM3").
func ttlKind() string {
	f := flag.Lookup("ttl")
	if f == nil {
		return ""
	}
	kind := strings.ToLower(strings.TrimSpace(f.Value.String()))
	if i := strings.IndexAny(kind, ":,"); i >= 0 {
		kind = kind[:i]
	}
	return kind
}

// ── Presentation ───────────────────────────────────────────────────────────

// probeResult is the outcome of one trial's probe.
type probeResult struct {
	cond      string
	onsetNS   uint64
	responded bool
	rtMs      float64
}

// runner holds the state of a run: the response bookkeeping and the data rows.
type runner struct {
	exp     *control.Experiment
	codes   ttlCodes
	run     string
	respKey control.Keycode

	t0NS uint64 // start of the run, on the SDL clock

	probeTrials []probeResult
	presented   int // trials whose probe has been shown
	// pending is the trial whose probe is awaiting a press, -1 when there is
	// none. A press is credited to it however late it comes, up to the next
	// probe, as in the EvLab localizer.
	pending                   int
	falseAlarms, extraPresses int
}

// present runs the trials on their absolute schedule, then holds the
// fixation cross until runEndMs.
func (r *runner) present(trials []item, onsets []int, runEndMs int, blank, fix, probe stimuli.VisualStimulus) error {
	exp := r.exp

	// The frame callback arms the marker on the first frame of every word, so
	// the photodiode square is white in the flip that shows the word and its
	// code goes out right after it. wordCond is set before each stream.
	wordCond := "S"
	onFrame := func(ctx stimuli.FrameContext) error {
		if ctx.OnPhase && ctx.Frame == 0 {
			exp.Marker.Arm(byte(r.codes.word(wordCond, ctx.Index+1)))
		}
		return nil
	}

	for i, t := range trials {
		if err := r.waitUntil(onsets[i]); err != nil {
			return err
		}
		if err := exp.Show(blank); err != nil {
			return err
		}
		wordCond = t.cond
		stims := make([]stimuli.Stimulus, len(t.words))
		for j, w := range t.words {
			stims[j] = stimuli.NewTextLine(w, 0, 0, exp.ForegroundColor)
		}
		elements := stimuli.MakeRegularStream(stims, wordMs*time.Millisecond, 0)
		if err := r.waitUntil(onsets[i] + preBlankMs); err != nil {
			return err
		}

		events, timing, err := stimuli.PresentStreamOfStimuliFunc(exp.Screen, elements, 0, 0, onFrame)
		for j, tl := range timing {
			if tl.OnsetNS == 0 {
				continue // not reached: ESC during the stream
			}
			r.addRow("word", i+1, j+1, t.cond, strconv.Itoa(t.index), t.words[j], r.codes.word(t.cond, j+1),
				strconv.Itoa(onsets[i]+preBlankMs+j*wordMs), tl.OnsetNS, "", "")
		}
		for _, ev := range events {
			if r.isResponse(ev.Event) {
				r.press(ev.TimestampNS)
			}
		}
		if err != nil {
			return err // includes EndLoop on ESC, handled by exp.Run
		}

		// The probe opens this trial's response window and closes the
		// previous one.
		exp.Marker.Arm(byte(r.codes.probe))
		probeNS, err := exp.ShowTS(probe)
		if err != nil {
			return err
		}
		r.probeTrials[i] = probeResult{cond: t.cond, onsetNS: probeNS}
		r.presented = i + 1
		r.pending = i
		r.addRow("probe", i+1, 0, t.cond, strconv.Itoa(t.index), "", r.codes.probe,
			strconv.Itoa(onsets[i]+preBlankMs+wordsPerItem*wordMs), probeNS, "", "")
		if err := r.waitUntil(onsets[i] + trialCoreMs); err != nil {
			return err
		}
		if err := exp.Show(fix); err != nil {
			return err
		}
		// The fixation period is at least a second long: write this trial's
		// rows to disk now, so a run that is killed keeps them.
		r.save()
	}
	return r.waitUntil(runEndMs)
}

// waitUntil holds until targetMs after the start of the run, scoring any
// response key pressed meanwhile. ESC propagates as EndLoop.
func (r *runner) waitUntil(targetMs int) error {
	for {
		remaining := targetMs - int((control.TicksNS()-r.t0NS)/1_000_000)
		if remaining <= 0 {
			return nil
		}
		key, ts, err := r.exp.Keyboard.GetKeyEventTS([]control.Keycode{r.respKey}, remaining)
		if err != nil {
			return err
		}
		if key == 0 {
			return nil // timed out: the deadline has been reached
		}
		r.press(ts)
	}
}

// press scores one press of the response key against the pending probe.
func (r *runner) press(ts uint64) {
	trial, rt, outcome := 0, "", ""
	switch {
	case r.pending >= 0:
		p := &r.probeTrials[r.pending]
		p.responded = true
		p.rtMs = float64(ts-p.onsetNS) / 1e6
		trial, rt, outcome = r.pending+1, strconv.FormatFloat(p.rtMs, 'f', 2, 64), "hit"
		r.pending = -1
	case r.presented == 0:
		outcome = "false_alarm" // before the first probe
		r.falseAlarms++
	default:
		outcome = "extra" // the last probe was already answered
		r.extraPresses++
	}
	r.addRow("response", trial, 0, "", "", "", 0, "", ts, rt, outcome)
}

func (r *runner) isResponse(e control.Event) bool {
	if e.Type != control.EVENT_KEY_DOWN {
		return false
	}
	ke := e.KeyboardEvent()
	return !ke.Repeat && ke.Key == r.respKey
}

// addRow writes one line of the data file. Fields that do not apply are left
// empty rather than given a sentinel value.
func (r *runner) addRow(event string, trial, position int, cond, item, text string, code int,
	scheduled string, tsNS uint64, rt, outcome string) {
	field := func(v int) string {
		if v == 0 {
			return ""
		}
		return strconv.Itoa(v)
	}
	onset := ""
	if tsNS >= r.t0NS {
		onset = strconv.FormatFloat(float64(tsNS-r.t0NS)/1e6, 'f', 3, 64)
	}
	r.exp.Data.Add(r.run, event, field(trial), field(position), cond, item, text, field(code),
		scheduled, onset, rt, outcome)
}

func (r *runner) save() {
	if err := r.exp.Data.Save(); err != nil {
		log.Printf("WARNING: could not write the data file: %v", err)
	}
}

// ── Results ────────────────────────────────────────────────────────────────

// tally returns, per condition, the probes shown and those answered, and the
// mean RT over answered probes.
func (r *runner) tally() (shown, hits map[string]int, meanRT float64) {
	shown, hits = map[string]int{}, map[string]int{}
	var sum float64
	n := 0
	for _, p := range r.probeTrials[:r.presented] {
		shown[p.cond]++
		if p.responded {
			hits[p.cond]++
			sum += p.rtMs
			n++
		}
	}
	if n > 0 {
		meanRT = sum / float64(n)
	}
	return shown, hits, meanRT
}

func (r *runner) summary() {
	shown, hits, meanRT := r.tally()
	for _, c := range []string{"S", "N"} {
		line := fmt.Sprintf("%s: %d/%d probes answered", condName(c), hits[c], shown[c])
		log.Print(line)
		r.exp.Data.WriteComment("m " + line)
	}
	line := fmt.Sprintf("mean RT from probe onset %.0f ms; false alarms %d, extra presses %d",
		meanRT, r.falseAlarms, r.extraPresses)
	log.Print(line)
	r.exp.Data.WriteComment("m " + line)
}

// showResults displays the probe detection and holds until SPACE, so a late
// press on the response button cannot dismiss it.
func (r *runner) showResults() error {
	shown, hits, _ := r.tally()
	msg := fmt.Sprintf("Run finished.\n\nDiscs answered:\n\nafter sentences:   %d / %d\nafter nonwords:   %d / %d\n\nThank you!",
		hits["S"], shown["S"], hits["N"], shown["N"])
	tb := r.exp.FittedTextBox(msg)
	defer tb.Unload()
	if err := r.exp.Show(tb); err != nil {
		return err
	}
	return r.exp.Keyboard.WaitKey(control.K_SPACE)
}

func condName(c string) string {
	if c == "S" {
		return "sentences"
	}
	return "nonwords"
}

// ── Helpers ────────────────────────────────────────────────────────────────

func instructions(practice bool) string {
	msg := "You will read sentences, and sequences of nonwords such as\n" +
		"BLICKET or FLORP, one word at a time.\n\n" +
		"Read them attentively and silently, as you would read a book.\n" +
		"Do not worry if they seem fast at first: you will get used to it.\n\n" +
		"At the end of each sequence a large disc appears:\n" +
		"press the button whenever you see it.\n\n" +
		"Please stay still and keep your eyes on the centre of the screen.\n\n"
	if practice {
		msg = "Practice\n\n" + msg
	}
	return msg + "Press SPACE to start."
}

// keyFromFlag maps a single-character flag to a key code. SDL key codes for
// letters and digits are their lower-case ASCII codes.
func keyFromFlag(s string) (control.Keycode, error) {
	r := []rune(strings.ToLower(strings.TrimSpace(s)))
	if len(r) != 1 || r[0] > 127 {
		return 0, fmt.Errorf("expected a single ASCII character, got %q", s)
	}
	return control.Keycode(r[0]), nil
}
