// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Command MEG-left-right is a two-button task for MEG: a dark-grey fixation
// cross on a mid-grey background is replaced, every 3-8 s, for 800 ms by a '<'
// (press the LEFT button) or a '>' (press the RIGHT button). Exactly half the
// arrows ask for each button, in random order. Every arrow onset is marked by
// the photodiode square and a TTL pulse on a line of its own side. A sync code
// is also written at the start and at the end of the run, so that the
// interval between them on the stimulus PC's clock can be compared with the
// same interval on the MEG acquisition's clock, to check for drift.
//
// It is the visual task of examples/MEG-localizer3 without the retinotopic
// stimulation and the sounds.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	expName = "MEG-left-right"

	// Default run duration, in seconds.
	durationSec = 180.0

	// Duration of an arrow replacing the fixation cross, in seconds.
	targetDurSec = 0.800
	// Bounds on the gap between the end of one arrow and the start of the
	// next.
	targetGapMinSec = 1.5
	targetGapMaxSec = 4.0

	// A press is attributed to an arrow only within respWindowSec of its
	// onset. The window must not exceed the arrow plus the shortest gap, or a
	// press could belong to two arrows.
	respWindowSec = 1.5

	fixCrossSizePx = 32
	fixCrossLinePx = 3

	// Onset-marker defaults for this paradigm (see control.MarkerFlags): the
	// photodiode square is white for one frame at every arrow onset, in the
	// flip whose timestamp is recorded as onset_ms, and the TTL pulse is 5 ms.
	markerFrames = 3
	markerTTLMs  = 5
)

// fixColor is the colour of the fixation cross and of the arrows that replace
// it: darker than the mid-grey background, so they stay visible. The arrows
// differ from the cross in shape only, not in colour or luminance.
var fixColor = control.DarkGray

// arrowHalfWidthPx is half the width of a '<' or '>'; its height is the
// cross's, fixCrossSizePx.
const arrowHalfWidthPx = 10

// newArrow returns a '<' (side left) or '>' (side right) centred on the
// fixation point, drawn with the cross's line width. Points are +Y up.
func newArrow(side int) *stimuli.PolyLine {
	w, h := float32(arrowHalfWidthPx), float32(fixCrossSizePx)/2
	if side == right {
		w = -w
	}
	return stimuli.NewPolyLine([]control.FPoint{{X: w, Y: h}, {X: -w, Y: 0}, {X: w, Y: -h}},
		false, fixCrossLinePx, fixColor)
}

func main() {
	// Registered before NewExperimentFromFlags, which calls flag.Parse().
	var (
		flagDuration = flag.Float64("duration", durationSec, "Run duration in seconds")
		flagAuto     = flag.Bool("autostart", false, "Skip the instruction and result screens (for test runs)")
		flagSeed     = flag.Uint64("seed", 1, "Random seed for the arrow schedule")

		flagLeftKey  = flag.String("left-key", "left", "Key sent by the LEFT response button (left, right, up, down, space, a-z, 0-9)")
		flagRightKey = flag.String("right-key", "right", "Key sent by the RIGHT response button")

		flagWaitTrig  = flag.Bool("wait-trigger", false, "Wait for the scanner pulse (key 't') before starting the run")
		flagTTLArrowL = flag.Int("ttl-arrow-left", 0, "TTL line (0-7) pulsed at the onset of a '<'")
		flagTTLArrowR = flag.Int("ttl-arrow-right", 1, "TTL line (0-7) pulsed at the onset of a '>'")
		flagTTLSync   = flag.Int("ttl-sync-code", 4, "TTL code (1-255) written at the start and at the end of the run, for clock-drift checks; 0 disables")
	)
	// -photodiode (on by default here), -photodiode-size, -marker-frames, -ttl
	// and -ttl-ms mark every arrow onset for the recording.
	control.MarkerFlags(control.MarkerConfig{
		Photodiode:  true,
		Frames:      markerFrames,
		TTLDuration: markerTTLMs * time.Millisecond,
	})

	exp := control.NewExperimentFromFlags(expName, control.Gray, control.White, 24)
	defer exp.End()

	for _, l := range []struct {
		name string
		line int
	}{{"-ttl-arrow-left", *flagTTLArrowL}, {"-ttl-arrow-right", *flagTTLArrowR}} {
		if l.line < 0 || l.line > 7 {
			log.Fatalf("%s=%d out of range (0-7)", l.name, l.line)
		}
	}
	if *flagTTLSync < 0 || *flagTTLSync > 255 {
		log.Fatalf("-ttl-sync-code=%d out of range (0-255)", *flagTTLSync)
	}
	if *flagTTLArrowL == *flagTTLArrowR {
		log.Fatalf("-ttl-arrow-left and -ttl-arrow-right are both line %d; each side needs its own line",
			*flagTTLArrowL)
	}
	// One onset's pulse must be over well before the next onset.
	if d := exp.Marker.TTLDuration(); exp.Marker.TTLEnabled() && d >= time.Duration(targetDurSec*float64(time.Second)) {
		log.Fatalf("-ttl-ms: the %v TTL pulse must be shorter than an arrow (%.0f ms)",
			d, targetDurSec*1000)
	}

	leftKey, err := parseKey(*flagLeftKey)
	if err != nil {
		log.Fatalf("-left-key: %v", err)
	}
	rightKey, err := parseKey(*flagRightKey)
	if err != nil {
		log.Fatalf("-right-key: %v", err)
	}
	if leftKey == rightKey {
		log.Fatalf("-left-key and -right-key are both %q", *flagLeftKey)
	}

	if herr := exp.HideCursor(); herr != nil {
		log.Printf("Warning: could not hide cursor: %v", herr)
	}

	runErr := exp.Run(func() error {
		// ── Frame-locked timing ─────────────────────────────────────────────
		refresh := float64(exp.Screen.RefreshRate())
		if refresh <= 0 {
			refresh = 60
			log.Print("Warning: refresh rate unknown, assuming 60 Hz")
		}
		frameMs := 1000.0 / refresh
		secToFrames := func(s float64) int { return maxInt(1, int(math.Round(s*refresh))) }
		totalFrames := secToFrames(*flagDuration)

		r := &runner{
			exp:          exp,
			fix:          stimuli.NewFixCross(fixCrossSizePx, fixCrossLinePx, fixColor),
			arrows:       [2]*stimuli.PolyLine{newArrow(left), newArrow(right)},
			keys:         [2]control.Keycode{leftKey, rightKey},
			arrowCodes:   [2]byte{byte(1) << uint(*flagTTLArrowL), byte(1) << uint(*flagTTLArrowR)},
			targetFrames: secToFrames(targetDurSec),
			respWindowNS: uint64(respWindowSec * 1e9),
		}

		// ── Arrow schedule ──────────────────────────────────────────────────
		rng := rand.New(rand.NewPCG(*flagSeed, uint64(exp.SubjectID)))
		plan, perr := planArrows(totalFrames, scheduleParams{
			TargetFrames: r.targetFrames,
			GapMinFrames: secToFrames(targetGapMinSec),
			GapMaxFrames: maxInt(2, secToFrames(targetGapMaxSec)),
			RespFrames:   secToFrames(respWindowSec),
		}, rng)
		if perr != nil {
			return perr
		}
		if len(plan) < 2 {
			return fmt.Errorf("-duration %.1f s is too short for one '<' and one '>'", *flagDuration)
		}

		// ── Data file header ────────────────────────────────────────────────
		exp.Data.WriteComment("--MEG LEFT-RIGHT")
		exp.Data.WriteComment("m task: '<' replaces the cross -> LEFT, '>' -> RIGHT")
		exp.Data.WriteComment(fmt.Sprintf("m refresh_hz: %.4f  frame_ms: %.4f", refresh, frameMs))
		exp.Data.WriteComment(fmt.Sprintf("m duration_s: %.1f (%d frames)", *flagDuration, totalFrames))
		exp.Data.WriteComment(fmt.Sprintf("m seed: %d", *flagSeed))
		exp.Data.WriteComment(fmt.Sprintf("m keys: left=%s right=%s", *flagLeftKey, *flagRightKey))
		exp.Data.WriteComment(fmt.Sprintf("m fix_target: %.0f ms, gap %.1f-%.1f s after its end",
			targetDurSec*1000, targetGapMinSec, targetGapMaxSec))
		exp.Data.WriteComment(fmt.Sprintf("m planned_arrows: %d (half left, half right, in random order)", len(plan)))
		exp.Data.WriteComment(fmt.Sprintf("m response_window_s: %.1f", respWindowSec))
		syncCode := byte(*flagTTLSync)
		if exp.Marker.TTLEnabled() {
			exp.Data.WriteComment(fmt.Sprintf("m ttl_lines: arrow_left=%d arrow_right=%d",
				*flagTTLArrowL, *flagTTLArrowR))
			exp.Data.WriteComment(fmt.Sprintf("m ttl_sync_code: %d (start and end of the run)", syncCode))
		}
		exp.AddDataVariableNames([]string{"event_type", "onset_ms", "side", "button", "outcome", "rt_ms"})
		log.Printf("planned arrows: %d, exactly half left/half right, over %.1f s", len(plan), *flagDuration)

		// ── Start ───────────────────────────────────────────────────────────
		if !*flagAuto {
			msg := "Keep your eyes on the central cross at all times.\n\n" +
				"When the cross becomes  <  press the LEFT button.\n" +
				"When the cross becomes  >  press the RIGHT button.\n\n" +
				"Press SPACE to continue."
			if ierr := exp.ShowInstructions(msg); ierr != nil {
				return ierr
			}
		}
		if *flagWaitTrig {
			if serr := exp.Screen.Clear(); serr != nil {
				return serr
			}
			if serr := r.fix.Draw(exp.Screen); serr != nil {
				return serr
			}
			if serr := exp.Screen.Flip(); serr != nil {
				return serr
			}
			if kerr := exp.Keyboard.WaitKey(control.K_T); kerr != nil {
				return kerr
			}
		}
		// t0 anchors every onset. It is on the SDL clock, the same one that
		// stamps flips (FlipTS) and key events, so onsets and responses are
		// directly comparable without crossing clocks.
		r.t0NS = control.TicksNS()
		// Clock-drift check: the sync code goes out at the start and at the
		// end of the run, outside any flip, and Fire returns the SDL time
		// taken just before each write (0 when there is no TTL output). The
		// first arrow is GapMin seconds away, so the pulses cannot overlap.
		startNS := r.exp.Marker.Fire(syncCode)
		if startNS != 0 {
			r.addRow(row{event: "sync_start", tsNS: startNS})
		}

		rerr := r.run(plan, totalFrames)
		// Sent even when the run was aborted (ESC), so the interval can
		// still be compared with the MEG file.
		if endNS := r.exp.Marker.Fire(syncCode); startNS != 0 && endNS != 0 {
			r.addRow(row{event: "sync_end", tsNS: endNS})
			exp.Data.WriteComment(fmt.Sprintf("m sync_start_ns: %d  sync_end_ns: %d  (SDL clock)", startNS, endNS))
			exp.Data.WriteComment(fmt.Sprintf("m sync_interval_ms: %.3f", float64(endNS-startNS)/1e6))
			log.Printf("sync code %d: start-to-end interval %.3f ms on the stimulus PC's clock",
				syncCode, float64(endNS-startNS)/1e6)
		}
		if rerr != nil {
			return rerr
		}
		// An arrow still inside its window when the run ends is scored now.
		r.expire(math.MaxUint64)

		// Return to a uniform grey field before the end message.
		if berr := exp.Screen.ClearAndUpdate(); berr != nil {
			return berr
		}
		for side, sname := range sideNames {
			s := r.score[side]
			exp.Data.WriteComment(fmt.Sprintf("m arrow %s: events %d  correct %d  wrong %d  misses %d",
				sname, s.events, s.correct, s.wrong, s.misses))
			log.Printf("arrow %s: events %d, correct %d, wrong %d, misses %d",
				sname, s.events, s.correct, s.wrong, s.misses)
		}
		exp.Data.WriteComment(fmt.Sprintf("m false_alarms: %d  extra_presses: %d", r.falseAlarms, r.extraPresses))
		log.Printf("false alarms %d, extra presses %d", r.falseAlarms, r.extraPresses)

		// The result screen waits for a keypress, which would hang an
		// unattended run, so -autostart skips it as well as the instructions.
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

// ── the task ───────────────────────────────────────────────────────────────

var (
	sideNames = [2]string{"left", "right"}
	// eventNames[side] is the event_type of an arrow's row.
	eventNames = [2]string{"arrow_left", "arrow_right"}
)

// taskEvent is one arrow, from its onset flip until its response window
// closes.
type taskEvent struct {
	side      int
	onsetNS   uint64
	responded bool
}

// tally counts the outcomes of the arrows of one side. An arrow is correct if
// its first press in the window was the button it asked for; wrong and misses
// are both errors.
type tally struct {
	events, correct, wrong, misses int
}

// ── the frame loop ─────────────────────────────────────────────────────────

// runner holds the state the frame loop mutates.
type runner struct {
	exp        *control.Experiment
	fix        *stimuli.FixCross
	arrows     [2]*stimuli.PolyLine // '<', '>'
	keys       [2]control.Keycode   // by side
	arrowCodes [2]byte              // '<', '>'

	targetFrames int
	respWindowNS uint64

	t0NS  uint64
	frame int // frames since the run started
	quit  bool

	targetEndFrame int // the arrow replaces the cross until this frame
	targetSide     int // which arrow

	cur *taskEvent // the last arrow, while its response window is open

	score                     [2]tally // by side
	falseAlarms, extraPresses int
}

// run presents totalFrames frames, showing each planned arrow from its onset
// frame for targetFrames frames and the fixation cross otherwise.
func (r *runner) run(plan []arrow, totalFrames int) error {
	next := 0
	for r.frame = 0; r.frame < totalFrames; {
		var ev *taskEvent
		if next < len(plan) && plan[next].OnsetFrame == r.frame {
			ev = &taskEvent{side: plan[next].Side}
			r.targetEndFrame = r.frame + r.targetFrames
			r.targetSide = ev.side
			next++
		}

		if err := r.exp.Screen.Clear(); err != nil {
			return err
		}
		var center interface{ Draw(*apparatus.Screen) error } = r.fix
		if r.frame < r.targetEndFrame {
			center = r.arrows[r.targetSide]
		}
		if err := center.Draw(r.exp.Screen); err != nil {
			return err
		}
		// An arrow onset: the marker's square turns white in this flip, and
		// its TTL code is written right after it, tens of microseconds after
		// the flip returns. The photons come one to three frames later,
		// depending on the display stack; measure that lag with the
		// photodiode and subtract it in analysis.
		if ev != nil {
			r.exp.Marker.Arm(r.arrowCodes[ev.side])
		}
		ts, err := r.exp.Screen.FlipTS()
		if err != nil {
			return err
		}
		if ev != nil {
			// The previous arrow's window closed at least a gap ago.
			r.expire(math.MaxUint64)
			ev.onsetNS = ts
			r.cur = ev
			r.score[ev.side].events++
			r.addRow(row{event: eventNames[ev.side], tsNS: ts, ev: ev})
		}

		r.frame++
		if err := r.pollResponses(); err != nil {
			return err
		}
		r.expire(ts)
		if r.quit {
			return control.EndLoop
		}
	}
	return nil
}

// expire closes the current arrow's response window if it ended before
// nowNS, logging a miss when no button was pressed in it.
func (r *runner) expire(nowNS uint64) {
	ev := r.cur
	if ev == nil || nowNS <= ev.onsetNS+r.respWindowNS {
		return
	}
	if !ev.responded {
		r.score[ev.side].misses++
		r.addRow(row{event: "miss", tsNS: ev.onsetNS, ev: ev, outcome: "miss"})
	}
	r.cur = nil
}

// pollResponses drains SDL events for this frame, scoring presses of the two
// response buttons and noticing ESC or a window close. Presses are read from
// the event stream rather than from EventState so that auto-repeat from a
// held button is ignored and two presses in one frame are both kept.
func (r *runner) pollResponses() error {
	type press struct {
		side int
		ts   uint64
	}
	var presses []press
	st := r.exp.PollEvents(func(e control.Event) bool {
		if e.Type != control.EVENT_KEY_DOWN {
			return false
		}
		ke := e.KeyboardEvent()
		if ke.Repeat {
			return false
		}
		for side, k := range r.keys {
			if ke.Key == k {
				presses = append(presses, press{side, ke.Timestamp})
			}
		}
		return false
	})
	if st.QuitRequested {
		r.quit = true
	}
	for _, p := range presses {
		r.scorePress(p.side, p.ts)
	}
	return nil
}

// scorePress classifies one button press against the current arrow.
func (r *runner) scorePress(side int, tsNS uint64) {
	ev := r.cur
	var rt string
	var outcome string
	switch {
	case ev == nil || tsNS < ev.onsetNS || tsNS > ev.onsetNS+r.respWindowNS:
		outcome = "false_alarm"
		r.falseAlarms++
		ev = nil
	case ev.responded:
		outcome = "extra"
		r.extraPresses++
	case side == ev.side:
		outcome = "correct"
		r.score[ev.side].correct++
	default:
		outcome = "wrong"
		r.score[ev.side].wrong++
	}
	if ev != nil {
		ev.responded = true
		rt = strconv.FormatFloat(float64(tsNS-ev.onsetNS)/1e6, 'f', 2, 64)
	}
	r.addRow(row{event: "response", tsNS: tsNS, ev: ev, button: sideNames[side], outcome: outcome, rtMs: rt})
}

// showResults displays correct/total for each side and holds until SPACE.
// Only SPACE ends it, so a late press on a response button cannot dismiss it
// before the experimenter has read it.
func (r *runner) showResults() error {
	var b strings.Builder
	b.WriteString("Run finished.\n\nCorrect responses / total:\n\n")
	labels := [2]string{"Arrow  <", "Arrow  >"}
	for side, s := range r.score {
		fmt.Fprintf(&b, "%s:   %d / %d\n", labels[side], s.correct, s.events)
	}
	b.WriteString("\nThank you!")
	tb := r.exp.FittedTextBox(b.String())
	defer tb.Unload()
	if err := r.exp.Show(tb); err != nil {
		return err
	}
	return r.exp.Keyboard.WaitKey(control.K_SPACE)
}

// row is one line of the data file. Zero fields are written empty.
type row struct {
	event   string
	tsNS    uint64
	ev      *taskEvent // side, for arrows and their responses
	button  string
	outcome string
	rtMs    string
}

// addRow writes one line of the data file. Fields that do not apply to the
// event are left empty rather than filled with a sentinel that analysis code
// might mistake for a measurement.
func (r *runner) addRow(w row) {
	onsetMs := ""
	if w.tsNS != 0 && w.tsNS >= r.t0NS {
		onsetMs = strconv.FormatFloat(float64(w.tsNS-r.t0NS)/1e6, 'f', 3, 64)
	}
	side := ""
	if w.ev != nil {
		side = sideNames[w.ev.side]
	}
	r.exp.Data.Add(w.event, onsetMs, side, w.button, w.outcome, w.rtMs)
}

// ── helpers ────────────────────────────────────────────────────────────────

// parseKey resolves a -left-key/-right-key value to a key code.
func parseKey(name string) (control.Keycode, error) {
	switch n := strings.ToLower(strings.TrimSpace(name)); {
	case n == "left":
		return control.K_LEFT, nil
	case n == "right":
		return control.K_RIGHT, nil
	case n == "up":
		return control.K_UP, nil
	case n == "down":
		return control.K_DOWN, nil
	case n == "space":
		return control.K_SPACE, nil
	// SDL key codes for letters and digits are their ASCII codes, so the
	// K_A..K_Z and K_0..K_9 ranges are contiguous.
	case len(n) == 1 && n[0] >= 'a' && n[0] <= 'z':
		return control.K_A + control.Keycode(n[0]-'a'), nil
	case len(n) == 1 && n[0] >= '0' && n[0] <= '9':
		return control.K_0 + control.Keycode(n[0]-'0'), nil
	}
	return 0, fmt.Errorf("unknown key %q (use left, right, up, down, space, a-z or 0-9)", name)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
