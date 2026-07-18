// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// RSVP-MathLang presents sentences word by word (rapid serial visual
// presentation) on a fixed onset schedule started by the scanner trigger, and
// collects a veracity judgement for each one.
//
// The paradigm contrasts nine sentence types (pseudoword lists, word lists,
// meaningless sentences, factual/contextual knowledge, theory of mind,
// arithmetic principles, calculation, geometry), each in a true and a false
// variant, to separate language from mathematical reasoning networks.
//
// The stimulus schedule is not hardcoded: it is read from the per-subject table
// stim_lists/sub-NNN_task-MathLang.csv chosen by -s, with columns
//
//	cond,truth_val,sentence,bloc,order,onset    (onset in ms)
//
// truth_val is the expected answer, in French: its first character is V (vrai)
// or F (faux). Some rows carry a second character (FP, FC, VV, VF, …) encoding
// sub-conditions; it is ignored here.
//
// Each of the 5 blocs is a separate fMRI run with its own timeline (onsets
// restart at 6000 ms in every bloc), so one launch presents one bloc, selected
// with -b, and waits for that run's own 't' trigger.
//
// Flow:
//
//  1. An instruction screen (operator presses SPACE when ready).
//  2. A GREEN fixation cross while the program waits for the scanner
//     synchronisation pulse, delivered as a 't' keypress (standard for
//     MRI trigger boxes emulating a keyboard).
//  3. A GREY fixation cross for the rest of the run. A clock is started at the
//     trigger; each sentence is presented word by word when the clock reaches
//     its scheduled onset.
//
// The participant judges each statement with two buttons: 1 = vrai (true),
// 2 = faux (false). Presses are accepted from the first word of the sentence
// until the next sentence is due; the first press of the trial counts and
// later ones are discarded. Reaction time is measured from the onset of the
// first word (so it includes reading time) using the SDL3 hardware event
// clock. At the end of the bloc a summary of percent correct and median RT,
// overall and per condition, is printed to the console.
//
// For every sentence the scheduled onset and the actual onset/offset (in ms
// from the trigger) are written to the data file, so onset jitter can be
// checked offline.
//
// Usage:
//
//	go run . -s 1            # subject 1, bloc 1
//	go run . -s 1 -b 2       # subject 1, bloc 2
//	go run . -w -s 1         # windowed mode for testing
//
// Flags: -w windowed mode, -d N display index, -s N subject ID, -b N bloc
// (1-5), -on N word duration in ms, -off N blank between words in ms,
// -autostart skip the SPACE prompt and scanner trigger and start the clock
// immediately (for timing audits, not for real scanning). Press ESC (or close
// the window) to abort.
package main

import (
	"bytes"
	"embed"
	"encoding/csv"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Stimulus tables are embedded so the binary is self-contained (the
// browser/WASM build has no filesystem, and even on desktop this avoids any
// load jitter mid-run).
//
//go:embed stim_lists/sub-*_task-MathLang.csv
var stimFS embed.FS

// trial is one row of a per-subject stimulus table.
type trial struct {
	cond     string // condition label, e.g. "08-Calculation"
	truthVal string // "V", "F", "FP", "FC"
	sentence string
	bloc     int
	order    int // presentation rank within the bloc
	onsetMs  int // scheduled onset in ms from the scanner trigger
}

func main() {
	// Register experiment-specific flags before NewExperimentFromFlags, which
	// calls flag.Parse().
	bloc := flag.Int("b", 1, "bloc (fMRI run) to present, 1-5")
	onMs := flag.Int("on", 300, "duration of each word in ms")
	offMs := flag.Int("off", 0, "blank between words in ms")
	autostart := flag.Bool("autostart", false, "skip the SPACE prompt and scanner trigger; start the clock immediately (for timing audits)")

	exp := control.NewExperimentFromFlags(
		"RSVP-MathLang",
		control.Black, control.White, 32,
	)
	defer exp.End()

	csvName := fmt.Sprintf("stim_lists/sub-%03d_task-MathLang.csv", exp.SubjectID)
	raw, err := fs.ReadFile(stimFS, csvName)
	if err != nil {
		log.Fatalf("cannot read stimulus table %q: %v\n"+
			"(available subjects are the numbers N in stim_lists/sub-NNN_task-MathLang.csv)",
			csvName, err)
	}

	trials, err := loadTrials(csvName, raw, *bloc)
	if err != nil {
		log.Fatalf("cannot parse stimulus table %q: %v", csvName, err)
	}
	if len(trials) == 0 {
		log.Fatalf("%s contains no trials for bloc %d", csvName, *bloc)
	}
	log.Printf("loaded %d trials for bloc %d from %s", len(trials), *bloc, csvName)

	// Warn if a sentence cannot finish before the next one is due: at 300/0 ms
	// per word the longest sentence (11 words) takes 3.3 s against a minimum SOA
	// of 5.6 s, but that margin disappears if -on/-off are raised.
	soa := *onMs + *offMs
	for i, t := range trials {
		dur := len(strings.Fields(t.sentence)) * soa
		if i+1 < len(trials) && t.onsetMs+dur > trials[i+1].onsetMs {
			log.Printf("warning: trial %d (%s) lasts %d ms but the next onset is %d ms away; "+
				"the schedule will run late",
				t.order, t.cond, dur, trials[i+1].onsetMs-t.onsetMs)
		}
	}

	// Fixation crosses, mirroring the Language-Localizer convention: green
	// while waiting for the trigger, grey during the run.
	fixGreen := stimuli.NewFixCross(45, 5, control.Green)
	fixGrey := stimuli.NewFixCross(45, 3, control.RGB(192, 192, 192))
	stimuli.PreloadVisualOnScreen(exp.Screen, fixGreen)
	stimuli.PreloadVisualOnScreen(exp.Screen, fixGrey)

	exp.AddDataVariableNames([]string{
		"bloc", "order", "cond", "truth_val", "sentence", "nwords",
		"scheduled_onset", "actual_onset", "actual_offset",
		"expected", "response", "correct", "rt",
	})

	if !*autostart {
		exp.ShowInstructions(fmt.Sprintf(
			"MathLang — bloc %d\n\n"+
				"Please stay still and keep your eyes on the cross.\n"+
				"Sentences will appear word by word.\n"+
				"Decide whether each statement is true or false:\n\n"+
				"    1 = VRAI        2 = FAUX\n\n"+
				"Answer as soon as you have made up your mind.\n\n"+
				"Operator: press SPACE, then wait for the scanner (t) trigger.",
			*bloc))
	}

	wordOn := time.Duration(*onMs) * time.Millisecond
	wordOff := time.Duration(*offMs) * time.Millisecond
	respKeys := []control.Keycode{control.K_1, control.K_2}

	// Results accumulate outside the run closure so the summary can still be
	// printed for a run aborted early with ESC.
	results := make([]response, 0, len(trials))

	exp.Run(func() error {
		// Green cross: wait for the scanner synchronisation pulse ('t').
		// -autostart skips both, starting the clock immediately (timing audits).
		if !*autostart {
			if serr := exp.Show(fixGreen); serr != nil {
				return serr
			}
			if kerr := exp.Keyboard.WaitKey(control.K_T); kerr != nil {
				return kerr
			}
		}

		// Grey cross for the remainder of the run, then start the clock at the
		// trigger so every onset is measured from t=0. triggerNS anchors the SDL
		// hardware clock at the same instant: the stream reports each word's real
		// VSYNC-flip time as TimingLog.OnsetNS on that clock, so subtracting
		// triggerNS yields the displayed onset in ms from the trigger.
		if serr := exp.Show(fixGrey); serr != nil {
			return serr
		}
		clk := clock.NewClock()
		triggerNS := sdl.TicksNS()

		for i, t := range trials {
			// Wait until this sentence's scheduled onset. exp.Wait pumps SDL
			// events every frame, so the window stays responsive and ESC (or
			// window-close) aborts the run gracefully. Onsets are recomputed
			// against the absolute schedule each trial, so the ~1 ms wait
			// granularity does not accumulate into drift.
			if remaining := t.onsetMs - int(clk.NowMillis()); remaining > 0 {
				if werr := exp.Wait(remaining); werr != nil {
					return werr
				}
			}

			// Drop anything still queued: the previous trial's response window
			// closed at this onset, so a late press must not leak into this one.
			exp.Keyboard.Clear()

			words := strings.Fields(t.sentence)
			events, timing, perr := stimuli.PresentStreamOfText(
				exp.Screen, words, wordOn, wordOff, 0, 0, exp.ForegroundColor)
			after := clk.NowMillis()
			if perr != nil {
				return perr // includes EndLoop on ESC, handled by exp.Run
			}

			// Back to the fixation cross for the inter-sentence interval.
			if serr := exp.Show(fixGrey); serr != nil {
				return serr
			}

			// RT is measured from the onset of the first word, using the SDL3
			// hardware event clock (the same clock as TimingLog.OnsetNS), never
			// a wall-clock delta.
			firstWordNS := timing[0].OnsetNS

			// The logged onset is the achieved VSYNC flip of the first word, not
			// the pre-call clock reading `before`: timing[0].OnsetNS is the SDL
			// hardware timestamp of the flip that put the word on screen, so
			// (OnsetNS - triggerNS) is the displayed onset in ms from the trigger.
			// This is ~one frame later than `before`, the true photon onset.
			displayedOnset := int64(timing[0].OnsetNS-triggerNS) / 1_000_000

			// The response window spans the sentence and the gap that follows.
			// First look for a press captured during the RSVP stream itself.
			key, pressNS := firstResponse(events, respKeys)

			// Nothing yet: keep waiting, up to the moment the next sentence is
			// due. The last trial of the bloc gets a window of one start-delay
			// (6 s), matching the schedule's own lead-in.
			windowEnd := t.onsetMs + lastTrialWindowMs
			if i+1 < len(trials) {
				windowEnd = trials[i+1].onsetMs
			}
			if key == 0 {
				if remaining := windowEnd - int(clk.NowMillis()); remaining > 0 {
					k, ts, kerr := exp.Keyboard.GetKeyEventTS(respKeys, remaining)
					if kerr != nil {
						return kerr // EndLoop on ESC
					}
					key, pressNS = k, ts
				}
			}

			r := response{cond: t.cond, expected: expectedAnswer(t.truthVal)}
			if key != 0 {
				r.answered = true
				r.given = "V"
				if key == control.K_2 {
					r.given = "F"
				}
				r.correct = r.given == r.expected
				r.rtMs = int64(pressNS-firstWordNS) / 1_000_000
			}
			results = append(results, r)

			given, correct, rt := "n/a", "n/a", "n/a"
			if r.answered {
				given = r.given
				correct = "0"
				if r.correct {
					correct = "1"
				}
				rt = strconv.FormatInt(r.rtMs, 10)
			}
			exp.Data.Add(t.bloc, t.order, t.cond, t.truthVal, t.sentence, len(words),
				t.onsetMs, displayedOnset, after, r.expected, given, correct, rt)
		}
		return control.EndLoop
	})

	printSummary(results, *bloc)
}

// lastTrialWindowMs is the response window granted to the final sentence of a
// bloc, which has no following onset to bound it. It matches the schedule's
// own 6 s start delay.
const lastTrialWindowMs = 6000

// response is one trial's scored judgement, kept for the end-of-bloc summary.
type response struct {
	cond     string
	expected string // "V" or "F"
	given    string // "V" or "F"; meaningful only when answered
	answered bool
	correct  bool
	rtMs     int64
}

// expectedAnswer maps a truth_val field to the expected button. The values are
// French: the first character is V (vrai) or F (faux). A second character
// (FP, FC, VV, VF, …) encodes a sub-condition and is ignored.
func expectedAnswer(truthVal string) string {
	if truthVal == "" {
		return ""
	}
	return strings.ToUpper(truthVal[:1])
}

// printSummary reports percent correct and median RT for the bloc, overall and
// broken down by condition, on the console.
//
// Percent correct is computed over every sentence presented, so an unanswered
// trial counts as an error; the "miss" column reports how many those were.
// The median RT is taken over answered trials only (correct and incorrect
// alike) — an unanswered trial has no RT to contribute.
func printSummary(results []response, bloc int) {
	if len(results) == 0 {
		return
	}

	// Group by condition, preserving first-seen order for a stable table.
	byCond := map[string][]response{}
	var conds []string
	for _, r := range results {
		if _, seen := byCond[r.cond]; !seen {
			conds = append(conds, r.cond)
		}
		byCond[r.cond] = append(byCond[r.cond], r)
	}
	sort.Strings(conds) // condition labels are numbered (01-…, 02-…), so this is the design order

	line := func(label string, rs []response) {
		n, correct, misses := len(rs), 0, 0
		var rts []int64
		for _, r := range rs {
			if !r.answered {
				misses++
				continue
			}
			if r.correct {
				correct++
			}
			rts = append(rts, r.rtMs)
		}
		pct := 100 * float64(correct) / float64(n)
		med := "n/a"
		if len(rts) > 0 {
			med = strconv.FormatInt(medianInt64(rts), 10)
		}
		log.Printf("  %-24s %4d %8.1f %10s %6d", label, n, pct, med, misses)
	}

	log.Printf("bloc %d summary", bloc)
	log.Printf("  %-24s %4s %8s %10s %6s", "condition", "n", "%correct", "median RT", "missed")
	for _, c := range conds {
		line(c, byCond[c])
	}
	log.Printf("  %-24s %4s %8s %10s %6s", strings.Repeat("-", 24), "----", "--------", "----------", "------")
	line("ALL", results)
}

// medianInt64 returns the median of xs, which must be non-empty. It sorts a
// copy, leaving the caller's slice untouched.
func medianInt64(xs []int64) int64 {
	s := append([]int64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}

// firstResponse returns the first press of one of keys found in the stream's
// captured events, with its SDL3 hardware timestamp. It returns (0, 0) when the
// participant did not answer during the sentence itself.
func firstResponse(events []stimuli.UserEvent, keys []control.Keycode) (control.Keycode, uint64) {
	for _, ev := range events {
		if ev.Event.Type != sdl.EVENT_KEY_DOWN {
			continue
		}
		k := ev.Event.KeyboardEvent().Key
		for _, want := range keys {
			if k == want {
				return k, ev.TimestampNS
			}
		}
	}
	return 0, 0
}

// loadTrials parses a stimulus table (raw is its file contents; name is used
// only for error messages) and returns the rows of the requested bloc, in
// presentation order. Columns: cond, truth_val, sentence, bloc, order, onset.
func loadTrials(name string, raw []byte, bloc int) ([]trial, error) {
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
	for _, required := range []string{"cond", "truth_val", "sentence", "bloc", "order", "onset"} {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("missing column %q in %s", required, name)
		}
	}

	var trials []trial
	for i, row := range rows[1:] {
		line := i + 2 // 1-based, past the header

		b, cerr := strconv.Atoi(strings.TrimSpace(row[col["bloc"]]))
		if cerr != nil {
			return nil, fmt.Errorf("%s line %d: bad bloc: %v", name, line, cerr)
		}
		if b != bloc {
			continue
		}
		order, cerr := strconv.Atoi(strings.TrimSpace(row[col["order"]]))
		if cerr != nil {
			return nil, fmt.Errorf("%s line %d: bad order: %v", name, line, cerr)
		}
		// onset is written by the generator as a float ("6000.0").
		onset, cerr := strconv.ParseFloat(strings.TrimSpace(row[col["onset"]]), 64)
		if cerr != nil {
			return nil, fmt.Errorf("%s line %d: bad onset: %v", name, line, cerr)
		}

		trials = append(trials, trial{
			cond:     row[col["cond"]],
			truthVal: row[col["truth_val"]],
			sentence: row[col["sentence"]],
			bloc:     b,
			order:    order,
			onsetMs:  int(onset + 0.5),
		})
	}

	// The table is already sorted by (bloc, order), but the schedule depends on
	// onsets increasing, so sort defensively rather than trusting the file.
	for i := 1; i < len(trials); i++ {
		if trials[i].onsetMs < trials[i-1].onsetMs {
			return nil, fmt.Errorf("%s: bloc %d onsets are not in increasing order "+
				"(trial %d starts before trial %d)", name, bloc, trials[i].order, trials[i-1].order)
		}
	}
	return trials, nil
}
