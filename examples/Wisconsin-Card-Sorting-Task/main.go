// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Wisconsin Card Sorting Task (WCST).
//
// Four key cards are shown in a row at the top of the screen; a test card is
// shown below. The participant sorts the test card under one of the key cards
// by pressing 1-4 or by clicking the card. The only feedback is "Correct" or
// "Wrong": the sorting rule (colour, shape or number) is never announced, and
// it changes without warning after a run of 6-10 consecutive correct sorts.
//
// Reference: Berg (1948); Grant & Berg (1948); Heaton et al. (1993).
package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

//go:embed stimuli/*.png
var cardFiles embed.FS

// ─── Card material ────────────────────────────────────────────────────────────

// The three sorting dimensions. Index i of each list is the value carried by
// key card i, so the four key cards are 1-red-triangle, 2-green-star,
// 3-yellow-cross and 4-blue-circle — the classic WCST set.
var (
	colorNames = []string{"red", "green", "yellow", "blue"}
	shapeNames = []string{"triangle", "star", "cross", "circle"}
	ruleNames  = []string{"number", "color", "shape"}
)

const (
	ruleNumber = 0
	ruleColor  = 1
	ruleShape  = 2
)

// card holds the three attribute indices (0-3). The printed number is num+1.
type card struct {
	num, color, shape int
}

func (c card) name() string {
	return fmt.Sprintf("%d-%s-%s", c.num+1, colorNames[c.color], shapeNames[c.shape])
}

func (c card) file() string { return "stimuli/" + c.name() + ".png" }

// match returns the index (0-3) of the key card this card belongs under
// according to the given rule. It works because the key cards exhaust the four
// values of every dimension.
func (c card) match(rule int) int {
	switch rule {
	case ruleNumber:
		return c.num
	case ruleColor:
		return c.color
	default:
		return c.shape
	}
}

// keyCards are the four cards displayed in the top row.
func keyCards() []card {
	cs := make([]card, 4)
	for i := range cs {
		cs[i] = card{num: i, color: i, shape: i}
	}
	return cs
}

// buildDeck returns the pool of test cards.
//
// By default only *unambiguous* cards are used: cards whose number, colour and
// shape point at three different key cards (24 of the 64). Every response is
// then diagnostic of exactly one rule, which is what makes perseveration
// readable in the data. With -all-cards the pool is instead the 60 cards that
// are not themselves key cards, as in the clinical deck, where a single card
// can match one key card on two dimensions at once.
func buildDeck(all bool) []card {
	var deck []card
	for n := 0; n < 4; n++ {
		for c := 0; c < 4; c++ {
			for s := 0; s < 4; s++ {
				cd := card{num: n, color: c, shape: s}
				if n == c && c == s {
					continue // a key card
				}
				if !all && (n == c || c == s || n == s) {
					continue // ambiguous
				}
				deck = append(deck, cd)
			}
		}
	}
	return deck
}

// ─── Layout ───────────────────────────────────────────────────────────────────

// minRespWindowMS is the shortest response window a fixed SOA may leave. A
// card the participant cannot answer in time is not a trial, so too small a
// -soa is rejected rather than run.
const minRespWindowMS = 500

const (
	logicalW, logicalH = 1024, 768

	cardH   = 190                            // on-screen card height, px
	cardW   = cardH * 361.0 / 511.0          // the PNGs are 361x511
	keyGapX = 190                            // horizontal spacing of the key cards
	keyY    = 170                            // centre height of the key-card row
	testY   = -150                           // centre height of the test card
	labelY  = keyY - cardH/2 - 26            // the "1".."4" labels under the key cards
	fbY     = (labelY + testY + cardH/2) / 2 // feedback line, between the rows
)

func keyX(i int) float32 { return keyGapX * float32(2*i-3) / 2 } // -285 … +285

// hitKeyCard returns the index of the key card under (mx, my), or -1.
func hitKeyCard(mx, my float32) int {
	for i := 0; i < 4; i++ {
		dx, dy := mx-keyX(i), my-float32(keyY)
		if dx > -cardW/2 && dx < cardW/2 && dy > -cardH/2 && dy < cardH/2 {
			return i
		}
	}
	return -1
}

// ─── Picture cache ────────────────────────────────────────────────────────────

// pictures keeps one Picture (hence one GPU texture) per card image; cards
// recur throughout the session, so they are loaded once and repositioned
// before each draw.
type pictures map[string]*stimuli.Picture

func (p pictures) get(c card) *stimuli.Picture {
	if pic, ok := p[c.name()]; ok {
		return pic
	}
	data, err := cardFiles.ReadFile(c.file())
	if err != nil {
		log.Fatalf("reading embedded card %s: %v", c.file(), err)
	}
	pic := stimuli.NewPictureFromMemory(data, 0, 0)
	pic.Width, pic.Height = cardW, cardH // set before the first Draw: scales the texture
	p[c.name()] = pic
	return pic
}

// ─── Trial display ────────────────────────────────────────────────────────────

// drawScene draws the four key cards, their labels and the test card.
// highlight (0-3) frames the chosen key card; pass -1 for none.
// The caller flips.
func drawScene(exp *control.Experiment, pics pictures, keys []card, labels []*stimuli.TextLine,
	test card, highlight int) error {
	screen := exp.Screen
	if err := screen.Clear(); err != nil {
		return err
	}
	if highlight >= 0 {
		// A black slab behind the chosen card thickens its own black border.
		// Deliberately not a colour: colour is one of the sorting dimensions.
		frame := stimuli.NewRectangle(keyX(highlight), keyY, cardW+16, cardH+16, control.Black)
		if err := frame.Draw(screen); err != nil {
			return err
		}
	}
	for i, kc := range keys {
		pic := pics.get(kc)
		pic.SetPosition(control.Point(keyX(i), keyY))
		if err := pic.Draw(screen); err != nil {
			return err
		}
		if err := labels[i].Draw(screen); err != nil {
			return err
		}
	}
	pic := pics.get(test)
	pic.SetPosition(control.Point(0, testY))
	return pic.Draw(screen)
}

// waitUntil blocks until the given offset (ms) on the session clock. Every
// phase of a fixed-SOA trial is timed against that single zero rather than
// against the phase before it, so a late frame costs its own phase and nothing
// downstream: onsets cannot drift over a scanner run.
func waitUntil(exp *control.Experiment, sessionClock *clock.Clock, targetMS int64) error {
	if remaining := targetMS - sessionClock.NowMillis(); remaining > 0 {
		return exp.Wait(int(remaining))
	}
	return nil
}

// getResponse waits for a key (1-4) or a click on a key card and returns the
// chosen card index, the response modality and the reaction time in ms.
// Clicks that land outside the key cards are ignored.
//
// deadline is the moment (on sessionClock, ms) at which the response window
// closes; pass a negative value to wait indefinitely. On timeout it returns
// (-1, "none", 0, nil).
func getResponse(exp *control.Experiment, onsetNS uint64,
	sessionClock *clock.Clock, deadline int64) (int, string, int64, error) {
	respKeys := []control.Keycode{
		control.K_1, control.K_2, control.K_3, control.K_4,
		control.K_KP_1, control.K_KP_2, control.K_KP_3, control.K_KP_4,
	}
	for {
		// Recomputed each time round, so the window still closes on schedule
		// when a stray click sends us back here.
		timeout := -1
		if deadline >= 0 {
			timeout = int(deadline - sessionClock.NowMillis())
			if timeout <= 0 {
				return -1, "none", 0, nil
			}
		}
		ev, err := exp.WaitAnyEventTS(respKeys, true, timeout)
		if err != nil {
			return 0, "", 0, err
		}
		if ev.Key == 0 && ev.Button == 0 {
			// WaitAnyEventTS returns a zero event on timeout.
			return -1, "none", 0, nil
		}
		rt := int64(ev.TimestampNS-onsetNS) / 1_000_000
		switch ev.Device {
		case apparatus.DeviceKeyboard:
			for i, k := range respKeys {
				if ev.Key == k {
					return i % 4, "key", rt, nil
				}
			}
		case apparatus.DeviceMouse:
			if ev.Button != control.BUTTON_LEFT {
				continue
			}
			// The click position is read here rather than carried by the
			// event: a fraction of a millisecond later, and the cards are
			// large.
			mx, my := exp.Screen.MousePosition()
			if i := hitKeyCard(mx, my); i >= 0 {
				return i, "mouse", rt, nil
			}
		}
	}
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	nTrials := flag.Int("n", 128, "Number of trials")
	minRun := flag.Int("minrun", 6, "Minimum number of consecutive correct sorts before the rule changes")
	maxRun := flag.Int("maxrun", 10, "Maximum number of consecutive correct sorts before the rule changes")
	feedbackMS := flag.Int("fb", 800, "Feedback duration (ms)")
	itiMS := flag.Int("iti", 400, "Inter-trial interval (ms)")
	soaMS := flag.Int("soa", 0, "Fixed stimulus-onset asynchrony (ms): every trial lasts exactly this long, "+
		"so the session lasts n*soa (scanner mode). 0 = self-paced trials")
	waitTrigger := flag.Bool("trigger", false,
		"Wait for the scanner trigger — the first 't' keypress after the instructions — before starting; "+
			"trial onsets are then counted from that pulse")
	allCards := flag.Bool("all-cards", false, "Draw test cards from the full 60-card deck (includes cards matching one key card on two dimensions)")

	exp := control.NewExperimentFromFlags("Wisconsin-Card-Sorting-Task", control.Gray, control.Black, 32)
	defer exp.End()

	if *minRun < 1 || *maxRun < *minRun {
		log.Fatalf("-minrun/-maxrun: need 1 <= minrun (%d) <= maxrun (%d)", *minRun, *maxRun)
	}

	// Scanner mode: the trial is a fixed grid — card, then feedback at a fixed
	// latency, then blank — so that trial k starts exactly k*soa after the
	// first one and the run lasts n*soa whatever the participant does.
	// The response window is what is left of the SOA once the feedback and the
	// inter-trial blank are taken out.
	respWindow := 0
	if *soaMS > 0 {
		respWindow = *soaMS - *feedbackMS - *itiMS
		if respWindow < minRespWindowMS {
			log.Fatalf("-soa %d ms leaves only %d ms to respond (soa - fb %d - iti %d); "+
				"raise -soa or lower -fb/-iti so at least %d ms remain",
				*soaMS, respWindow, *feedbackMS, *itiMS, minRespWindowMS)
		}
	}

	if err := exp.SetLogicalSize(logicalW, logicalH); err != nil {
		log.Printf("warning: set logical size: %v", err)
	}
	// Mouse responses are accepted, so the participant needs to see the cursor.
	// Some display back-ends (KMSDRM) cannot create a cursor shape; the
	// keyboard route still works there, so this is a warning, not a failure.
	if err := exp.ShowCursor(); err != nil {
		log.Printf("warning: show cursor: %v", err)
	}

	exp.AddExperimentInfo(fmt.Sprintf("deck: %d cards (%s)", len(buildDeck(*allCards)),
		map[bool]string{true: "full", false: "unambiguous only"}[*allCards]))
	exp.AddExperimentInfo(fmt.Sprintf("rule changes after %d-%d consecutive correct sorts", *minRun, *maxRun))
	if *soaMS > 0 {
		total := float64(*nTrials) * float64(*soaMS) / 1000
		exp.AddExperimentInfo(fmt.Sprintf(
			"fixed SOA %d ms = card %d ms + feedback %d ms + blank %d ms; %d trials = %.1f s",
			*soaMS, respWindow, *feedbackMS, *itiMS, *nTrials, total))
		log.Printf("fixed SOA %d ms (card %d ms, feedback %d ms, blank %d ms): "+
			"%d trials last %.1f s (%.2f min) after the instructions",
			*soaMS, respWindow, *feedbackMS, *itiMS, *nTrials, total, total/60)
	} else {
		exp.AddExperimentInfo("self-paced trials (no fixed SOA)")
	}
	exp.AddDataVariableNames([]string{
		"trial", "category", "rule", "trial_in_category", "run_correct",
		"test_card", "number", "color", "shape",
		"match_number", "match_color", "match_shape",
		"response", "modality", "rt_ms", "correct",
		"matched_dims", "perseverative", "rule_changed_after",
	})

	keys := keyCards()
	deck := buildDeck(*allCards)
	pics := pictures{}

	labels := make([]*stimuli.TextLine, 4)
	for i := range labels {
		labels[i] = stimuli.NewTextLine(fmt.Sprintf("%d", i+1), keyX(i), labelY, control.Black)
	}

	correctFB := stimuli.NewTextLine("Correct", 0, fbY, control.Green)
	wrongFB := stimuli.NewTextLine("Wrong", 0, fbY, control.Red)
	missedFB := stimuli.NewTextLine("Too slow", 0, fbY, control.White)

	// Session state.
	rule := rand.Intn(3)
	prevRule := -1 // the rule in force before the last change; -1 before any change
	runTarget := *minRun + rand.Intn(*maxRun-*minRun+1)
	runCorrect := 0
	category := 1
	trialInCategory := 0
	nCorrect, nPerseverative, nMissed, categoriesCompleted := 0, 0, 0, 0

	// The pool is drawn from without replacement and reshuffled when exhausted,
	// so no card recurs until every other one has been seen.
	pool := append([]card(nil), deck...)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	poolIdx := 0
	nextCard := func() card {
		if poolIdx >= len(pool) {
			last := pool[len(pool)-1]
			for {
				rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
				if pool[0] != last {
					break
				}
			}
			poolIdx = 0
		}
		c := pool[poolIdx]
		poolIdx++
		return c
	}

	err := exp.Run(func() error {
		// Preload every texture now: no first-draw hitch inside a trial.
		var all []stimuli.VisualStimulus
		for _, c := range append(append([]card(nil), keys...), deck...) {
			all = append(all, pics.get(c))
		}
		for _, l := range labels {
			all = append(all, l)
		}
		all = append(all, correctFB, wrongFB, missedFB)
		if err := stimuli.PreloadAllVisual(exp.Screen, all); err != nil {
			return err
		}

		if err := exp.ShowInstructions(
			"WISCONSIN CARD SORTING TASK\n\n" +
				"Four cards are shown at the top of the screen.\n" +
				"On each trial a new card appears below them.\n\n" +
				"Decide which of the four cards it goes with, and say so\n" +
				"by pressing 1, 2, 3 or 4 — or by clicking on that card.\n\n" +
				"You are never told the sorting rule: you have to work it out\n" +
				"from the feedback, which is only 'Correct' or 'Wrong'.\n" +
				"The rule changes from time to time without warning.\n\n" +
				"Press SPACE to start."); err != nil {
			return err
		}

		// In scanner mode every trial onset is measured from this zero, so a
		// slow frame cannot push the following trials off the grid. origin is
		// where the grid starts on that clock: 0 normally, and the moment the
		// scanner trigger arrived when waiting for one (a small negative
		// number, since the clock is reset just after the trigger event).
		sessionClock := clock.NewClock()
		origin := int64(0)

		if *waitTrigger {
			tb := exp.FittedTextBox("Waiting for scanner TTL…\n\n(the first 't' pulse starts the run)")
			if err := exp.Show(tb); err != nil {
				tb.Unload()
				return err
			}
			_, triggerNS, err := exp.Keyboard.GetKeyEventTS([]control.Keycode{control.K_T}, -1)
			tb.Unload()
			if err != nil {
				return err
			}
			// The trigger is detected by a 1 ms polling loop, so the clock is
			// reset a millisecond or two after the pulse itself. The SDL event
			// timestamp says by how much, and the grid is shifted back by that
			// lag so trial onsets are counted from the pulse, not from its
			// detection.
			lagMS := int64(control.TicksNS()-triggerNS) / 1_000_000
			if err := exp.Screen.ClearAndUpdate(); err != nil {
				return err
			}
			sessionClock.Reset()
			origin = -lagMS
			exp.AddExperimentInfo(fmt.Sprintf("started on scanner trigger ('t'); detection lag %d ms", lagMS))
			log.Printf("scanner trigger received (detection lag %d ms) — starting", lagMS)
		} else {
			if err := exp.Screen.ClearAndUpdate(); err != nil {
				return err
			}
			sessionClock.Reset()
		}

		for trial := 1; trial <= *nTrials; trial++ {
			// Onset, response deadline and feedback onset of this trial on the
			// session clock. Unused when self-paced.
			trialStart := origin + int64(trial-1)*int64(*soaMS)
			respDeadline := trialStart + int64(respWindow)
			fbEnd := respDeadline + int64(*feedbackMS)

			if *soaMS > 0 {
				if err := waitUntil(exp, sessionClock, trialStart); err != nil {
					return err
				}
			} else if err := exp.Blank(*itiMS); err != nil {
				return err
			}
			test := nextCard()
			trialInCategory++

			exp.Keyboard.Clear() // drop stale presses/clicks from the last trial
			if err := drawScene(exp, pics, keys, labels, test, -1); err != nil {
				return err
			}
			onsetNS, err := exp.Screen.FlipTS()
			if err != nil {
				return err
			}

			deadline := int64(-1)
			if *soaMS > 0 {
				deadline = respDeadline
			}
			resp, modality, rt, err := getResponse(exp, onsetNS, sessionClock, deadline)
			if err != nil {
				return err
			}
			// Fixed SOA: the card stays up for the whole window whether or not
			// the participant has answered, so feedback onset is on the grid
			// and does not inherit the jitter of the reaction time.
			if *soaMS > 0 {
				if err := waitUntil(exp, sessionClock, respDeadline); err != nil {
					return err
				}
			}

			correct := resp >= 0 && resp == test.match(rule)
			if correct {
				nCorrect++
			}
			// A perseverative error repeats the rule that has just been
			// abandoned (Heaton's "perseveration to the previous principle").
			perseverative := resp >= 0 && !correct && prevRule >= 0 && resp == test.match(prevRule)
			if perseverative {
				nPerseverative++
			}
			if resp < 0 {
				nMissed++
			}

			// Which dimensions the chosen card actually shared with the test
			// card — "color+number" and so on; empty when it shared none.
			var dims []string
			for r := 0; resp >= 0 && r < 3; r++ {
				if resp == test.match(r) {
					dims = append(dims, ruleNames[r])
				}
			}

			// Feedback: the same display, with the chosen card framed.
			if err := drawScene(exp, pics, keys, labels, test, resp); err != nil {
				return err
			}
			fb := wrongFB
			switch {
			case correct:
				fb = correctFB
			case resp < 0:
				fb = missedFB
			}
			if err := fb.Draw(exp.Screen); err != nil {
				return err
			}
			if err := exp.Screen.Update(); err != nil {
				return err
			}

			// Rule bookkeeping. The row describes the trial, so the rule,
			// category and position are the ones that were in force while the
			// participant was responding — captured before any change below.
			trialRule, trialCategory, trialPos := rule, category, trialInCategory
			ruleChanged := false
			if correct {
				runCorrect++
				if runCorrect >= runTarget {
					prevRule = rule
					rule = (rule + 1 + rand.Intn(2)) % 3 // any of the other two
					runCorrect = 0
					runTarget = *minRun + rand.Intn(*maxRun-*minRun+1)
					category++
					trialInCategory = 0
					categoriesCompleted++
					ruleChanged = true
				}
			} else {
				runCorrect = 0
			}

			exp.Data.Add(trial, trialCategory, ruleNames[trialRule], trialPos, runCorrect,
				test.name(), test.num+1, colorNames[test.color], shapeNames[test.shape],
				test.match(ruleNumber)+1, test.match(ruleColor)+1, test.match(ruleShape)+1,
				resp+1, modality, rt, correct,
				strings.Join(dims, "+"), perseverative, ruleChanged)

			if *soaMS > 0 {
				// Feedback off, then blank to the end of the trial. Both ends
				// come off the session grid, so the next onset is at k*soa.
				if err := waitUntil(exp, sessionClock, fbEnd); err != nil {
					return err
				}
				if err := exp.Screen.ClearAndUpdate(); err != nil {
					return err
				}
			} else if err := exp.Wait(*feedbackMS); err != nil {
				return err
			}
		}
		if *soaMS > 0 { // let the last trial's inter-trial blank run its course
			if err := waitUntil(exp, sessionClock, origin+int64(*nTrials)*int64(*soaMS)); err != nil {
				return err
			}
		}

		summary := fmt.Sprintf(
			"Finished.\n\n%d/%d correct — %d categories completed,\n%d perseverative errors.",
			nCorrect, *nTrials, categoriesCompleted, nPerseverative)
		if *soaMS > 0 {
			summary += fmt.Sprintf("\n%d trials with no response.", nMissed)
		}
		return exp.ShowEndMessage(summary + "\n\nThank you!\n\nPress any key to quit.")
	})
	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
