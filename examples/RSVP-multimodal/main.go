// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// RSVP-multimodal presents a multimodal stimulation script — rapid serial
// visual presentation of words, flashing checkerboards, and spoken sentences —
// on a fixed absolute onset schedule read from a stimulus table.
//
// The bundled protocols implement the French version of the *Pinel functional
// localizer* (Pinel et al., 2007), a five-minute fMRI run that contrasts four
// tasks, each delivered in both a visual and an auditory modality:
//
//   - reading / listening to sentences        (phraseVideo, phraseAudio)
//   - reading / listening to subtractions,    (calculvideo, calculaudio)
//     solved silently
//   - reading / hearing "press the left/right (clicGvideo, clicGaudio,
//     button three times", then doing so       clicDvideo, clicDaudio)
//   - passively viewing flashing checkerboards (CheckBoardH, CheckBoardV)
//
// Contrasting the conditions isolates the language, number, motor, and visual
// networks in a single short run.
//
// # The stimulus table
//
// Nothing about the paradigm is hardcoded: a run is a tab-separated table in
// protocols/, with the columns
//
//	onset_time	duration	type	cond	stimuli
//
// onset_time is in ms from the start of the run and duration is in ms. cond is
// optional (the instructions protocol omits it) and is carried through to the
// data file as the condition label. The supported types are:
//
//	TEXT          a line of text
//	BOX           a block of text; literal "\n" in the field starts a new line
//	IMAGE         an image file, centered
//	SOUND         a .wav file
//	TEXT_STREAM   words shown one after another (RSVP), "~"-separated
//	IMAGE_STREAM  images shown one after another, "~"-separated
//	SOUND_STREAM  sounds played one after another, "~"-separated
//
// In a *_STREAM row, duration is the default duration of each element, and an
// element may override it as path:duration or path:duration:gap (ms), as in
//
//	0	400	TEXT_STREAM	calculvideo	calculez~seize~moins~huit
//	8700	400	IMAGE_STREAM	CheckboardH	checkerboard_hpb.bmp~checkerboard_hnb.bmp
//
// Stimulus files are looked up by name in stimuli/. Both directories are
// embedded in the binary, so it is self-contained and nothing is decoded or
// loaded from disk mid-run.
//
// # Timing
//
// Every row is presented by a single stimuli.PresentStreamOfStimuli call —
// VSYNC-locked, GC disabled — preceded by a fixation-cross element that fills
// the gap until the row's scheduled onset. That gap is recomputed against the
// absolute schedule on every row, so the ~1-frame rounding of each slot cannot
// accumulate into drift over the run: onset error stays bounded at roughly half
// a frame instead of growing with time. Both the scheduled and the achieved
// onset are written to the data file, and the console prints the onset error at
// the end of the run so it can be checked.
//
// Because rows are presented sequentially, a row must finish before the next
// one is due; the loader rejects a table whose rows overlap.
//
// # Flow
//
//  1. An instruction screen (operator presses SPACE when ready).
//  2. For a run: a GREEN fixation cross while the program waits for the scanner
//     synchronisation pulse, delivered as a 't' keypress (standard for MRI
//     trigger boxes emulating a keyboard). The clock starts at the pulse, so
//     every onset is measured from it. The instructions protocol is shown
//     outside the scanner and starts immediately instead.
//  3. A WHITE fixation cross whenever no stimulus is on screen.
//
// Button presses are recorded throughout with their time and the condition in
// force when they occurred, which is what scores the left/right button task.
//
// Usage:
//
//	go run . -p instructions   # the spoken/written instructions, once, before the scanner
//	go run . -p run1 -s 1      # subject 1, run 1
//	go run . -w -p run1        # windowed mode for testing
//
// Flags: -p protocol (instructions, run1…run4), -w windowed mode, -d N display
// index, -s N subject ID, -skip-wait to start immediately (no instruction
// screen, no trigger). Press ESC (or close the window) to abort.
//
// Reference: Pinel, P., Thirion, B., Meriaux, S., Jobert, A., Serres, J.,
// Le Bihan, D., Poline, J.-B., & Dehaene, S. (2007). Fast reproducible
// identification and large-scale databasing of individual functional cognitive
// networks. BMC Neuroscience, 8, 91. https://doi.org/10.1186/1471-2202-8-91
//
// The stimuli were designed by Philippe Pinel (INSERM U562, Cognitive
// Neuroimaging Unit). This is a port of the gostim2 implementation at
// https://github.com/chrplr/Pinel-localizer-go.
package main

import (
	"bytes"
	"embed"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Protocols and stimuli are embedded so the binary is self-contained (the
// browser/WASM build has no filesystem, and on desktop this keeps every decode
// and texture upload out of the run).
//
//go:embed protocols/*.tsv
var protocolFS embed.FS

//go:embed stimuli/*.wav stimuli/*.bmp
var assetFS embed.FS

// gracePeriodMs is how long the fixation cross is held after the last row, so
// the final stimulus is not cut off by the window closing. It matches the grace
// period of the gostim2 implementation this is ported from.
const gracePeriodMs = 500

// element is one item of a row's stimulus sequence: a single stimulus for the
// scalar types, one of the "~"-separated items for the *_STREAM types.
type element struct {
	spec  string // file name, or the text itself for TEXT/BOX
	durMs int
	gapMs int // blank/silence after this item, before the next
}

// row is one line of a stimulus table.
type row struct {
	line     int    // line number in the table, for error messages
	onsetMs  int    // scheduled onset, in ms from the start of the run
	durMs    int    // the row's duration field: per-element for *_STREAM types
	stype    string // TEXT, BOX, IMAGE, SOUND, TEXT_STREAM, IMAGE_STREAM, SOUND_STREAM
	cond     string // condition label, may be empty
	stimuli  string // the stimuli field verbatim, for the data file
	elements []element
}

// totalMs is how long the whole row occupies the screen/speakers.
func (r *row) totalMs() int {
	total := 0
	for _, e := range r.elements {
		total += e.durMs + e.gapMs
	}
	return total
}

// isVisual reports whether the row puts something on screen. Sound rows do not:
// the fixation cross stays up while they play.
func (r *row) isVisual() bool { return !strings.HasPrefix(r.stype, "SOUND") }

// logEntry is one line of the data file. Entries are buffered and sorted by
// time so the file reads chronologically, like the gostim2 event log.
type logEntry struct {
	intendedMs int // scheduled time; negative for a response, which has none
	actualMs   int
	event      string // e.g. TEXT_STREAM_ONSET, SOUND_ONSET, RESPONSE
	cond       string
	stim       string // the row's stimuli field, or the key name for a response
}

func main() {
	// Register experiment-specific flags before NewExperimentFromFlags, which
	// calls flag.Parse().
	protocol := flag.String("p", "run1", "protocol to present: instructions, run1, run2, run3, run4")
	skipWait := flag.Bool("skip-wait", false, "start immediately, without the instruction screen or the scanner trigger")

	// Font size 50 and a white-on-black screen reproduce the defaults of the
	// gostim2 implementation, which used the same Inconsolata font.
	exp := control.NewExperimentFromFlags("RSVP-multimodal", control.Black, control.White, 50)
	defer exp.End()

	name := "protocols/" + *protocol + ".tsv"
	raw, err := protocolFS.ReadFile(name)
	if err != nil {
		names, _ := protocolFS.ReadDir("protocols")
		var avail []string
		for _, n := range names {
			avail = append(avail, strings.TrimSuffix(n.Name(), ".tsv"))
		}
		log.Fatalf("unknown protocol %q: no embedded %s (available: %s)",
			*protocol, name, strings.Join(avail, ", "))
	}

	rows, err := loadProtocol(name, raw)
	if err != nil {
		log.Fatalf("cannot parse %s: %v", name, err)
	}
	lastMs := rows[len(rows)-1].onsetMs + rows[len(rows)-1].totalMs()
	log.Printf("loaded %d rows from %s (%.1f s)", len(rows), name, float64(lastMs)/1000)

	// Build and preload every stimulus up front: identical items (the four
	// checkerboards, a repeated instruction word) are built once and shared, so
	// each distinct file costs one texture or one audio stream, and no decoding
	// happens once the run has started.
	res := newResources(exp)
	streams := make([][]stimuli.StreamElement, len(rows))
	for i := range rows {
		if streams[i], err = res.build(&rows[i]); err != nil {
			log.Fatalf("%s line %d: %v", name, rows[i].line, err)
		}
	}
	log.Printf("preloaded %d images, %d sounds, %d texts",
		len(res.pictures), len(res.sounds), len(res.texts)+len(res.boxes))

	// The fixation cross fills every gap between stimuli. Size and line width
	// match the gostim2 original (arms of 20 px either side of centre).
	fix := stimuli.NewFixCross(40, 2, exp.ForegroundColor)
	fixGreen := stimuli.NewFixCross(40, 4, control.Green) // waiting for the trigger
	if err := stimuli.PreloadVisualOnScreen(exp.Screen, fix); err != nil {
		log.Fatalf("cannot preload fixation cross: %v", err)
	}
	if err := stimuli.PreloadVisualOnScreen(exp.Screen, fixGreen); err != nil {
		log.Fatalf("cannot preload fixation cross: %v", err)
	}

	exp.AddDataVariableNames([]string{"intended_ms", "actual_ms", "event", "cond", "stimuli"})
	exp.AddExperimentInfo("protocol: " + *protocol)

	isRun := *protocol != "instructions"

	// Entries accumulate outside the run closure so a run aborted with ESC
	// still writes everything recorded up to that point.
	var entries []logEntry

	exp.Run(func() error {
		switch {
		case *skipWait:
			// Nothing to wait for: the clock starts below, right away.
		case isRun:
			if ierr := exp.ShowInstructions(
				"Localizer — " + *protocol + "\n\n" +
					"Please stay still and keep your eyes on the cross.\n\n" +
					"Operator: press SPACE, then wait for the scanner (t) trigger."); ierr != nil {
				return ierr
			}
			// Green cross: wait for the scanner synchronisation pulse.
			if serr := exp.Show(fixGreen); serr != nil {
				return serr
			}
			if kerr := exp.Keyboard.WaitKey(control.K_T); kerr != nil {
				return kerr
			}
		default:
			if ierr := exp.ShowInstructions(
				"Instructions\n\n" +
					"They will be presented automatically.\n\n" +
					"Press SPACE to start."); ierr != nil {
				return ierr
			}
		}

		// Hold GC off for the whole run rather than letting each
		// PresentStreamOfStimuli call restore it between rows: a collection
		// falling on a row boundary could otherwise cost a frame at an onset.
		// The per-call disable nests harmlessly inside this one.
		defer debug.SetGCPercent(debug.SetGCPercent(-1))

		if serr := exp.Show(fix); serr != nil {
			return serr
		}
		// The clock starts at the trigger, so every onset is measured from it.
		// triggerNS anchors the SDL hardware clock at the same instant: the stream
		// reports each stimulus's real VSYNC-flip time as TimingLog.OnsetNS on that
		// clock, so subtracting triggerNS yields the displayed onset in ms from the
		// trigger. Responses use the same clock (UserEvent.TimestampNS - triggerNS),
		// so onsets, offsets, and responses are all one clock apart from nothing.
		clk := clock.NewClock()
		triggerNS := sdl.TicksNS()

		// Frame duration of the presentation display, used to trim the leading
		// fixation gap by one frame (see the loop).
		frameMs := 1000.0 / 60.0
		if mode, merr := sdl.GetDisplayForWindow(exp.Screen.Window).CurrentDisplayMode(); merr == nil && mode != nil && mode.RefreshRate > 0 {
			frameMs = 1000.0 / float64(mode.RefreshRate)
		}

		for i := range rows {
			r := &rows[i]

			// Fill the gap until this row's scheduled onset with the fixation
			// cross, as the first element of the row's own stream. Recomputing
			// it from the clock re-anchors the schedule on every row, so frame
			// rounding cannot accumulate into drift; presenting it inside the
			// stream means button presses during the gap are captured too.
			//
			// The gap is trimmed by one frame: the stream shows the fixation for
			// round(gap/frame) frames, after which the stimulus flips on the NEXT
			// VSYNC — one frame late. Presenting the fixation one frame shorter
			// lands that flip on the intended boundary instead. A gap under one
			// frame skips the fixation entirely; the stimulus then flips at the
			// next VSYNC, which is already ~the intended onset.
			elems := streams[i]
			lead := 0
			if gap := float64(r.onsetMs) - float64(clk.NowMillis()); gap > frameMs {
				elems = append([]stimuli.StreamElement{{
					Stimulus:   fix,
					DurationOn: time.Duration((gap - frameMs) * float64(time.Millisecond)),
				}}, elems...)
				lead = 1
			}

			events, timing, perr := stimuli.PresentStreamOfStimuli(exp.Screen, elems, 0, 0)

			// Log what was actually presented before handling an abort, so an
			// ESC halfway through a run still yields the rows that ran.
			entries = append(entries, rowEntries(r, timing, lead, triggerNS)...)
			entries = append(entries, responseEntries(rows, i, events, triggerNS)...)

			if perr != nil {
				return perr // includes EndLoop on ESC, handled by exp.Run
			}
		}

		// Hold the fixation cross briefly so the last stimulus is not cut off.
		if serr := exp.Show(fix); serr != nil {
			return serr
		}
		if remaining := lastMs + gracePeriodMs - int(clk.NowMillis()); remaining > 0 {
			if werr := exp.Wait(remaining); werr != nil {
				return werr
			}
		}
		return control.EndLoop
	})

	// Chronological order, like the gostim2 event log: a press during the gap
	// before a row is recorded while that row's stream runs, but belongs before
	// its onset.
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].actualMs < entries[j].actualMs })
	for _, e := range entries {
		intended := "n/a"
		if e.intendedMs >= 0 {
			intended = strconv.Itoa(e.intendedMs)
		}
		exp.Data.Add(intended, e.actualMs, e.event, e.cond, e.stim)
	}

	printSummary(entries)
}

// rowEntries turns one row's TimingLog into its onset (and, for a visual row,
// offset) data entries. lead is the number of elements prepended to the row's
// own stream (the fixation gap). Onsets and offsets are read from the SDL-clock
// TimingLog.OnsetNS/OffsetNS and expressed as ms from triggerNS.
//
// timing is short when the stream was aborted partway, so a row that did not
// reach the screen contributes nothing.
func rowEntries(r *row, timing []stimuli.TimingLog, lead int, triggerNS uint64) []logEntry {
	if len(timing) <= lead {
		return nil
	}
	// The onset is the real VSYNC flip (TimingLog.OnsetNS, SDL clock) rather than
	// the pre-flip loop-entry time (ActualOnset): (OnsetNS - triggerNS) is the
	// displayed onset in ms from the trigger, ~one frame past `before`.
	out := []logEntry{{
		intendedMs: r.onsetMs,
		actualMs:   int(int64(timing[lead].OnsetNS-triggerNS) / 1_000_000),
		event:      r.stype + "_ONSET",
		cond:       r.cond,
		stim:       r.stimuli,
	}}
	// A sound is not held on screen and so has no offset, matching gostim2,
	// which logs an offset only for the visual it takes down. The offset is
	// anchored to the SDL VSYNC of the takedown flip (OffsetNS), symmetric with
	// the OnsetNS-based onset above: both are on the SDL clock and on the same
	// side of a frame boundary. (Logging the Go-clock ActualOffset here read
	// every visual offset ~1 frame early, because it is sampled one frame before
	// the clearing flip.) OffsetNS is always set for a stimulus that reached the
	// screen — real for an element with an ISI, synthesised from the onset for a
	// stream's final element; the != 0 guard skips only the degenerate
	// zero-on-frame case.
	if last := len(timing) - 1; r.isVisual() && last == lead+len(r.elements)-1 && timing[last].OffsetNS != 0 {
		out = append(out, logEntry{
			intendedMs: r.onsetMs + r.totalMs(),
			actualMs:   int(int64(timing[last].OffsetNS-triggerNS) / 1_000_000),
			event:      r.stype + "_OFFSET",
			cond:       r.cond,
			stim:       r.stimuli,
		})
	}
	return out
}

// responseEntries turns the key presses collected during row i's stream into
// data entries. A press is labelled with the condition in force when it
// happened — the last row whose onset it follows, which for a press during the
// gap is the *previous* row (the one that asked for the button). This is what
// makes the left/right button task scorable.
//
// The press time is the SDL hardware event timestamp (UserEvent.TimestampNS)
// expressed as ms from triggerNS — the same clock as the onsets and offsets, so
// a response and the stimulus it follows are directly comparable.
func responseEntries(rows []row, i int, events []stimuli.UserEvent, triggerNS uint64) []logEntry {
	var out []logEntry
	for _, ev := range events {
		if ev.Event.Type != sdl.EVENT_KEY_DOWN {
			continue
		}
		key := ev.Event.KeyboardEvent()
		if key.Repeat { // auto-repeat from a held button is not a new press
			continue
		}
		at := int(int64(ev.TimestampNS-triggerNS) / 1_000_000)
		cond := ""
		if at >= rows[i].onsetMs {
			cond = rows[i].cond
		} else if i > 0 {
			cond = rows[i-1].cond
		}
		out = append(out, logEntry{
			intendedMs: -1,
			actualMs:   at,
			event:      "RESPONSE",
			cond:       cond,
			stim:       key.Key.KeyName(),
		})
	}
	return out
}

// printSummary reports, on the console, how many responses were collected and
// how far the achieved onsets fell from the schedule. The error should stay
// within about half a refresh period (~8 ms at 60 Hz); a larger one means the
// machine could not keep up and the run's timing is suspect.
func printSummary(entries []logEntry) {
	var worst, sum, n, responses int
	for _, e := range entries {
		switch {
		case e.event == "RESPONSE":
			responses++
		case strings.HasSuffix(e.event, "_ONSET"):
			d := e.actualMs - e.intendedMs
			if d < 0 {
				d = -d
			}
			if d > worst {
				worst = d
			}
			sum += d
			n++
		}
	}
	if n == 0 {
		log.Printf("no stimulus was presented")
		return
	}
	log.Printf("presented %d stimuli; onset error: mean %.1f ms, worst %d ms; %d responses",
		n, float64(sum)/float64(n), worst, responses)
}

// resources builds the stimulus objects for the tables' rows, sharing one
// object — and so one texture or audio stream — per distinct item.
type resources struct {
	exp      *control.Experiment
	pictures map[string]*stimuli.Picture
	sounds   map[string]*stimuli.Sound
	texts    map[string]*stimuli.TextLine
	boxes    map[string]*stimuli.TextBox
}

func newResources(exp *control.Experiment) *resources {
	return &resources{
		exp:      exp,
		pictures: map[string]*stimuli.Picture{},
		sounds:   map[string]*stimuli.Sound{},
		texts:    map[string]*stimuli.TextLine{},
		boxes:    map[string]*stimuli.TextBox{},
	}
}

// build returns the stream elements presenting one row, loading and preloading
// every stimulus it needs.
func (res *resources) build(r *row) ([]stimuli.StreamElement, error) {
	out := make([]stimuli.StreamElement, 0, len(r.elements))
	for _, e := range r.elements {
		s, err := res.stimulus(r.stype, e.spec)
		if err != nil {
			return nil, err
		}
		out = append(out, stimuli.StreamElement{
			Stimulus:    s,
			DurationOn:  time.Duration(e.durMs) * time.Millisecond,
			DurationOff: time.Duration(e.gapMs) * time.Millisecond,
		})
	}
	return out, nil
}

// stimulus returns the (cached) stimulus for one item of a row.
func (res *resources) stimulus(stype, spec string) (stimuli.Stimulus, error) {
	switch stype {
	case "IMAGE", "IMAGE_STREAM":
		if p, ok := res.pictures[spec]; ok {
			return p, nil
		}
		data, err := assetFS.ReadFile("stimuli/" + spec)
		if err != nil {
			return nil, fmt.Errorf("missing embedded image %q", spec)
		}
		p := stimuli.NewPictureFromMemory(data, 0, 0)
		if err := stimuli.PreloadVisualOnScreen(res.exp.Screen, p); err != nil {
			return nil, fmt.Errorf("cannot preload image %q: %w", spec, err)
		}
		res.pictures[spec] = p
		return p, nil

	case "SOUND", "SOUND_STREAM":
		if s, ok := res.sounds[spec]; ok {
			return s, nil
		}
		data, err := assetFS.ReadFile("stimuli/" + spec)
		if err != nil {
			return nil, fmt.Errorf("missing embedded sound %q", spec)
		}
		s := stimuli.NewSoundFromMemory(data)
		// PresentStreamOfStimuli requires audio elements to be bound to the
		// device before the call.
		if err := s.PreloadDevice(res.exp.AudioDevice); err != nil {
			return nil, fmt.Errorf("cannot preload sound %q: %w", spec, err)
		}
		res.sounds[spec] = s
		return s, nil

	case "TEXT", "TEXT_STREAM":
		if t, ok := res.texts[spec]; ok {
			return t, nil
		}
		t := stimuli.NewTextLine(spec, 0, 0, res.exp.ForegroundColor)
		if err := stimuli.PreloadVisualOnScreen(res.exp.Screen, t); err != nil {
			return nil, fmt.Errorf("cannot preload text %q: %w", spec, err)
		}
		res.texts[spec] = t
		return t, nil

	case "BOX":
		if b, ok := res.boxes[spec]; ok {
			return b, nil
		}
		// Width 0 means no wrapping: the text breaks only at its own newlines.
		b := stimuli.NewTextBox(spec, 0, control.Origin(), res.exp.ForegroundColor)
		if err := stimuli.PreloadVisualOnScreen(res.exp.Screen, b); err != nil {
			return nil, fmt.Errorf("cannot preload text box: %w", err)
		}
		res.boxes[spec] = b
		return b, nil
	}
	return nil, fmt.Errorf("unknown stimulus type %q", stype)
}

// loadProtocol parses a stimulus table (raw is its contents; name is used only
// in error messages) and returns its rows in presentation order.
func loadProtocol(name string, raw []byte) ([]row, error) {
	rd := csv.NewReader(bytes.NewReader(raw))
	rd.Comma = '\t'
	rd.TrimLeadingSpace = true
	rd.LazyQuotes = true
	// The stimuli column holds free text; only the header fixes the width.
	rd.FieldsPerRecord = -1

	records, err := rd.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("table is empty")
	}

	col := map[string]int{}
	for i, h := range records[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, want := range []string{"onset_time", "duration", "type", "stimuli"} {
		if _, ok := col[want]; !ok {
			return nil, fmt.Errorf("missing required column %q", want)
		}
	}

	var rows []row
	for i, rec := range records[1:] {
		line := i + 2
		field := func(k string) string {
			j, ok := col[k]
			if !ok || j >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[j])
		}

		onset, err := strconv.Atoi(field("onset_time"))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid onset_time %q", line, field("onset_time"))
		}
		dur, err := strconv.Atoi(field("duration"))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid duration %q", line, field("duration"))
		}
		if dur <= 0 {
			return nil, fmt.Errorf("line %d: duration must be positive, got %d", line, dur)
		}

		stype := strings.ToUpper(field("type"))
		if stype == "STREAM" { // gostim2 accepted STREAM as an alias
			stype = "IMAGE_STREAM"
		}
		stim := field("stimuli")

		r := row{line: line, onsetMs: onset, durMs: dur, stype: stype,
			cond: field("cond"), stimuli: stim}

		switch stype {
		case "TEXT", "IMAGE", "SOUND":
			r.elements = []element{{spec: stim, durMs: dur}}
		case "BOX":
			// A literal "\n" in the table starts a new line.
			r.elements = []element{{spec: strings.ReplaceAll(stim, `\n`, "\n"), durMs: dur}}
		case "TEXT_STREAM", "IMAGE_STREAM", "SOUND_STREAM":
			for _, item := range strings.Split(stim, "~") {
				e, err := parseStreamItem(item, dur)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", line, err)
				}
				r.elements = append(r.elements, e)
			}
		default:
			return nil, fmt.Errorf("line %d: unknown stimulus type %q", line, field("type"))
		}
		if r.elements[0].spec == "" {
			return nil, fmt.Errorf("line %d: empty stimuli field", line)
		}
		rows = append(rows, r)
	}

	// Rows are presented one after another, so an overlap would silently push
	// the rest of the run late. The schedule must be checked, not repaired.
	for i := 1; i < len(rows); i++ {
		if end := rows[i-1].onsetMs + rows[i-1].totalMs(); rows[i].onsetMs < end {
			return nil, fmt.Errorf("line %d: onset %d ms overlaps the previous row, which ends at %d ms",
				rows[i].line, rows[i].onsetMs, end)
		}
	}
	return rows, nil
}

// parseStreamItem parses one "~"-separated item of a *_STREAM row: a bare
// name, or name:duration, or name:duration:gap, with durations in ms.
func parseStreamItem(item string, defaultDur int) (element, error) {
	parts := strings.Split(item, ":")
	e := element{spec: strings.TrimSpace(parts[0]), durMs: defaultDur}
	if len(parts) >= 2 {
		d, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || d <= 0 {
			return e, fmt.Errorf("invalid duration in stream item %q", item)
		}
		e.durMs = d
	}
	if len(parts) >= 3 {
		g, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil || g < 0 {
			return e, fmt.Errorf("invalid gap in stream item %q", item)
		}
		e.gapMs = g
	}
	if len(parts) > 3 {
		return e, fmt.Errorf("stream item %q has too many :-separated fields", item)
	}
	return e, nil
}
