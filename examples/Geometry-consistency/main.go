// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
//
// Geometry consistency — replication of the infant change-detection paradigm of:
// Dillon M. R., Izard V. & Spelke E. S. (2020). Infants' sensitivity to shape
// changes in 2D visual forms. Infancy, 25(5), 618–639.
// https://doi.org/10.1111/infa.12343
//
// Two streams of geometric figures alternate side by side inside equal bounding
// panels for 60 s. One stream changes in SHAPE AND AREA, the other in AREA
// ALONE; the infant's relative looking time to the shape-changing stream indexes
// shape detection. Eight conditions (1A, 1B, 2A, 2B, 3A–3D) vary the figures
// (closed triangles vs. open two-line figures), the property that changes
// (relative length vs. angle) and how much orientation and sense vary.
//
// An experimenter codes the infant's looking direction in real time by holding
// the LEFT or RIGHT arrow key; looking times are accumulated per trial from the
// hardware key timestamps.
//
// Usage (from the repo root — go.work resolves the workspace):
//
//	go run ./examples/Geometry-consistency -exp 1A -w -s 1
//
// Note "go run ." / "go run ./examples/Geometry-consistency", never
// "go run main.go": this package has two files.
//
// Experimenter controls:
//
//	SPACE       — infant is looking at the centre; start the trial (attractor)
//	LEFT  hold  — infant looking at the left panel
//	RIGHT hold  — infant looking at the right panel
//	(neither)   — infant looking away / not attending
//	ESC         — abort the session

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/results"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Timing (paper §2, p. 621) ─────────────────────────────────────────────────

const (
	stimulusDurationMs = 500 // each figure shown for 0.5 s
	isiDurationMs      = 300 // blank inter-stimulus interval, 0.3 s
	slotDurationMs     = stimulusDurationMs + isiDurationMs
)

// ── Per-presentation random variation (paper §2, p. 621) ──────────────────────

const (
	minScaleJitter = 0.85 // figures scaled randomly by ±0–15 %
	maxScaleJitter = 1.15
)

// ── Screen layout ─────────────────────────────────────────────────────────────
// A fixed 1920×1080 logical canvas keeps every pixel constant below (panel size,
// stroke width, position jitter) independent of the real display resolution.

const (
	logicalWidth  = 1920
	logicalHeight = 1080
	panelMarginPx = 10 // slack between the largest figure and the panel edge
)

// ── Session parameters, from the command line ─────────────────────────────────

type params struct {
	cond        condition
	nTrials     int
	nSlots      int     // presentations per trial
	panelPx     float64 // side of each square bounding panel
	unitPx      float64 // pixels per normalized figure unit
	strokePx    float64
	jitterPx    float64
	panelCentre float32 // |x| of each panel centre

	// panelStrokePx is the outline of the bounding rectangle each stream is
	// presented in (paper §2); 0 hides it.
	panelStrokePx float32
}

// panelOutlines returns the square bounding rectangle drawn around each stream.
// The paper presented "two simultaneous streams of figures each within a
// bounding rectangle" (§2), and it is that rectangle's centre the per-
// presentation position jitter is measured from.
func panelOutlines(p params) []*stimuli.PolyLine {
	if p.panelStrokePx <= 0 {
		return nil
	}
	h := float32(p.panelPx / 2)
	corners := []control.FPoint{{X: -h, Y: -h}, {X: h, Y: -h}, {X: h, Y: h}, {X: -h, Y: h}}
	var out []*stimuli.PolyLine
	for _, cx := range []float32{-p.panelCentre, p.panelCentre} {
		box := stimuli.NewPolyLine(corners, true, p.panelStrokePx, panelOutlineColour)
		box.Position = control.FPoint{X: cx, Y: 0}
		out = append(out, box)
	}
	return out
}

// panelOutlineColour is deliberately dimmer than the figures so the frame reads
// as context rather than as another stimulus.
var panelOutlineColour = control.RGB(110, 110, 110)

// ── Placement: one figure presentation in one panel ───────────────────────────

type placement struct {
	stream   string // "shape_and_area" | "area_only"
	figureID string
	pts      []control.FPoint // already flipped, scaled and rotated, in pixels
	closed   bool
	pos      control.FPoint // panel centre + jitter
	rotDeg   float64
	scale    float64
	flip     bool
	dx, dy   float64
}

// place applies the per-presentation random transform to a figure and returns
// the pixel-space poly-line to draw. Order is mirror → scale → rotate, all about
// the figure's centroid, then translate by the position jitter.
func place(f figure, stream string, p params, rng *rand.Rand) placement {
	rot := (rng.Float64()*2 - 1) * p.cond.rotationDeg
	if p.cond.rotationDeg >= 360 {
		rot = rng.Float64() * 360
	}
	scale := minScaleJitter + rng.Float64()*(maxScaleJitter-minScaleJitter)
	flip := p.cond.allowFlip && rng.Intn(2) == 1

	// Uniform over the disc of radius jitterPx: sqrt() on the radius, otherwise
	// the samples pile up at the centre.
	r := p.jitterPx * math.Sqrt(rng.Float64())
	th := rng.Float64() * 2 * math.Pi
	dx, dy := r*math.Cos(th), r*math.Sin(th)

	k := scale * p.unitPx
	sinR, cosR := math.Sin(deg2rad(rot)), math.Cos(deg2rad(rot))
	pts := make([]control.FPoint, len(f.pts))
	for i, v := range f.pts {
		x := float64(v.X) * k
		y := float64(v.Y) * k
		if flip {
			x = -x
		}
		pts[i] = control.FPoint{
			X: float32(x*cosR - y*sinR),
			Y: float32(x*sinR + y*cosR),
		}
	}

	return placement{
		stream: stream, figureID: f.id, pts: pts, closed: f.closed,
		rotDeg: rot, scale: scale, flip: flip, dx: dx, dy: dy,
	}
}

// ── Trial structure ───────────────────────────────────────────────────────────

type trialSetup struct {
	shapeOnLeft bool
	context     figure // shown on even slots in BOTH streams
	changeFig   figure // shape-and-area partner (odd slots)
	areaFig     figure // area-only partner (odd slots)
	areaScale   float64
}

// buildTrials implements the paper's counterbalancing (§2, p. 621):
//   - the shape-and-area stream appears twice on each side across the 4 trials,
//     alternating, with its starting side counterbalanced across infants;
//   - half the infants see the smaller figure of the pair as the context, half
//     the larger one.
//
// cbGroup 0–3 encodes both binary factors; it is derived from the subject ID
// unless -cb overrides it.
func buildTrials(c condition, nTrials, cbGroup int) []trialSetup {
	contextIsSmall := cbGroup%2 == 0
	shapeStartsLeft := (cbGroup/2)%2 == 0

	var context, change figure
	var scale float64
	if contextIsSmall {
		context, change, scale = c.small, c.large, c.scaleFromSmall
	} else {
		context, change, scale = c.large, c.small, c.scaleFromLarge
	}
	area := context.scaled(scale, context.id+"_area-only")

	trials := make([]trialSetup, nTrials)
	for i := range trials {
		trials[i] = trialSetup{
			shapeOnLeft: shapeStartsLeft == (i%2 == 0),
			context:     context,
			changeFig:   change,
			areaFig:     area,
			areaScale:   scale,
		}
	}
	return trials
}

// buildStreams precomputes every presentation of one trial, for both panels, so
// that the VSYNC-locked loop only has to draw.
func buildStreams(t trialSetup, p params, rng *rand.Rand) (left, right []placement) {
	left = make([]placement, p.nSlots)
	right = make([]placement, p.nSlots)

	for i := 0; i < p.nSlots; i++ {
		// Both streams are in phase and both start on the context figure.
		shapeFig, areaFigure := t.context, t.context
		if i%2 == 1 {
			shapeFig, areaFigure = t.changeFig, t.areaFig
		}
		shape := place(shapeFig, "shape_and_area", p, rng)
		areaOnly := place(areaFigure, "area_only", p, rng)

		l, r := areaOnly, shape
		if t.shapeOnLeft {
			l, r = shape, areaOnly
		}
		l.pos = control.FPoint{X: -p.panelCentre + float32(l.dx), Y: float32(l.dy)}
		r.pos = control.FPoint{X: p.panelCentre + float32(r.dx), Y: float32(r.dy)}
		left[i], right[i] = l, r
	}
	return left, right
}

// ── Gaze coding ───────────────────────────────────────────────────────────────

// gazeCoder accumulates looking time from the SDL hardware timestamps of the
// experimenter's LEFT/RIGHT key presses. Only that one clock is used — mixing it
// with clock.GetTime() deltas would compare two clocks with different origins
// (see clock/clock.go).
type gazeCoder struct {
	leftHeld, rightHeld     bool
	leftDownTS, rightDownTS uint64
	leftMs, rightMs         int64
	leftDurs, rightDurs     []int64
}

func (g *gazeCoder) down(key sdl.Keycode, ts uint64) {
	switch key {
	case control.K_LEFT:
		if !g.leftHeld {
			g.leftHeld, g.leftDownTS = true, ts
		}
	case control.K_RIGHT:
		if !g.rightHeld {
			g.rightHeld, g.rightDownTS = true, ts
		}
	}
}

func (g *gazeCoder) up(key sdl.Keycode, ts uint64) {
	switch key {
	case control.K_LEFT:
		if g.leftHeld {
			g.leftHeld = false
			g.close(ts, g.leftDownTS, &g.leftMs, &g.leftDurs)
		}
	case control.K_RIGHT:
		if g.rightHeld {
			g.rightHeld = false
			g.close(ts, g.rightDownTS, &g.rightMs, &g.rightDurs)
		}
	}
}

func (g *gazeCoder) close(upTS, downTS uint64, total *int64, durs *[]int64) {
	if upTS <= downTS {
		return
	}
	d := int64(upTS-downTS) / 1_000_000 // ns → ms
	*total += d
	*durs = append(*durs, d)
}

// finish closes any press still held when the trial ends, using the hardware
// KEY_UP timestamp.
func (g *gazeCoder) finish(exp *control.Experiment) {
	if g.leftHeld {
		if ts, err := exp.Keyboard.WaitKeyReleaseTS(control.K_LEFT, 10_000); err == nil && ts > 0 {
			g.close(ts, g.leftDownTS, &g.leftMs, &g.leftDurs)
		}
		g.leftHeld = false
	}
	if g.rightHeld {
		if ts, err := exp.Keyboard.WaitKeyReleaseTS(control.K_RIGHT, 10_000); err == nil && ts > 0 {
			g.close(ts, g.rightDownTS, &g.rightMs, &g.rightDurs)
		}
		g.rightHeld = false
	}
}

func formatDurations(ds []int64) string {
	if len(ds) == 0 {
		return ""
	}
	parts := make([]string, len(ds))
	for i, d := range ds {
		parts[i] = strconv.FormatInt(d, 10)
	}
	return strings.Join(parts, ";")
}

// ── Attractor ─────────────────────────────────────────────────────────────────

var pink = control.RGB(255, 105, 180)

// showAttractor displays a large pulsing pink dot at the centre of the screen,
// equidistant from the two panels, with a repeating sound. It returns when the
// experimenter presses SPACE, signalling that the infant is looking at centre.
func showAttractor(exp *control.Experiment, sound *stimuli.Sound) error {
	bottomY := float32(-logicalHeight/2 + 60)
	msg := stimuli.NewTextLine("[Experimenter] Press SPACE when the infant looks at the centre",
		0, bottomY, control.White)
	defer msg.Unload()
	dot := stimuli.NewCircle(60, pink)

	start := time.Now()
	lastSound := time.Time{}
	for {
		el := time.Since(start).Seconds()
		if sound != nil && time.Since(lastSound) > 2*time.Second {
			_ = sound.Play() // asynchronous: the pulse must keep animating
			lastSound = time.Now()
		}
		dot.Radius = float32(55 + 25*math.Sin(2*math.Pi*el*1.5))

		_ = exp.Screen.Clear()
		_ = dot.Draw(exp.Screen)
		_ = msg.Draw(exp.Screen)
		if err := exp.Screen.Flip(); err != nil {
			return err
		}

		done := false
		state := exp.PollEvents(func(e sdl.Event) bool {
			if e.Type == sdl.EVENT_KEY_DOWN && e.KeyboardEvent().Key == control.K_SPACE {
				done = true
			}
			return false
		})
		if state.QuitRequested {
			return control.EndLoop
		}
		if done {
			return nil
		}
	}
}

// ── Trial presentation ────────────────────────────────────────────────────────

// presentTrial runs one 60 s trial: nSlots presentations of 500 ms on / 300 ms
// off, frame-locked, while the experimenter codes the infant's gaze. It returns
// the SDL onset timestamp of each presentation (index-aligned with the streams).
func presentTrial(exp *control.Experiment, left, right []placement, boxes []*stimuli.PolyLine, strokePx float32, framesOn, framesOff int) ([]uint64, gazeCoder, error) {
	var g gazeCoder
	onsets := make([]uint64, len(left))

	// One PolyLine per panel, re-pointed each slot: the buffers inside are
	// allocated on the first Draw and reused for the rest of the trial.
	leftPL := stimuli.NewPolyLine(left[0].pts, left[0].closed, strokePx, control.White)
	rightPL := stimuli.NewPolyLine(right[0].pts, right[0].closed, strokePx, control.White)

	// The bounding rectangles stay up for the whole trial, including the blank
	// inter-stimulus interval: they are the spatial frame the figures appear in,
	// not part of the alternating stimulus.
	drawBoxes := func() error {
		for _, b := range boxes {
			if err := b.Draw(exp.Screen); err != nil {
				return err
			}
		}
		return nil
	}

	poll := func() error {
		st := exp.PollEvents(func(e sdl.Event) bool {
			switch e.Type {
			case sdl.EVENT_KEY_DOWN:
				ke := e.KeyboardEvent()
				g.down(ke.Key, ke.Timestamp)
			case sdl.EVENT_KEY_UP:
				ke := e.KeyboardEvent()
				g.up(ke.Key, ke.Timestamp)
			}
			return false
		})
		if st.QuitRequested {
			return control.EndLoop
		}
		return nil
	}

	// The presentation loop is VSYNC-locked; a GC pause here would cost frames.
	// Restored before g.finish below, which can block for as long as the
	// experimenter keeps a key held down.
	prevGC := debug.SetGCPercent(-1)
	restoreGC := func() { debug.SetGCPercent(prevGC) }
	defer restoreGC()

	exp.Keyboard.Clear()

	for slot := range left {
		leftPL.Points, leftPL.Closed, leftPL.Position = left[slot].pts, left[slot].closed, left[slot].pos
		rightPL.Points, rightPL.Closed, rightPL.Position = right[slot].pts, right[slot].closed, right[slot].pos

		for f := 0; f < framesOn; f++ {
			if err := exp.Screen.Clear(); err != nil {
				return onsets, g, err
			}
			if err := drawBoxes(); err != nil {
				return onsets, g, err
			}
			if err := leftPL.Draw(exp.Screen); err != nil {
				return onsets, g, err
			}
			if err := rightPL.Draw(exp.Screen); err != nil {
				return onsets, g, err
			}
			ts, err := exp.Screen.FlipTS() // presents, then holds to the frame boundary
			if err != nil {
				return onsets, g, err
			}
			if f == 0 {
				onsets[slot] = ts
			}
			if err := poll(); err != nil {
				return onsets, g, err
			}
		}

		for f := 0; f < framesOff; f++ {
			if err := exp.Screen.Clear(); err != nil {
				return onsets, g, err
			}
			if err := drawBoxes(); err != nil {
				return onsets, g, err
			}
			if _, err := exp.Screen.FlipTS(); err != nil {
				return onsets, g, err
			}
			if err := poll(); err != nil {
				return onsets, g, err
			}
		}
	}

	restoreGC()
	g.finish(exp)
	return onsets, g, nil
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	// Experiment-specific flags must be registered BEFORE
	// NewExperimentFromFlags, which is what calls flag.Parse; their values must
	// be read only AFTER that call.
	expFlag := flag.String("exp", "1A",
		"condition: 1A, 1B, 2A, 2B, 3A, 3B, 3C or 3D")
	cbFlag := flag.Int("cb", -1,
		"counterbalancing group 0-3 (-1 = derive from the subject ID)")
	trialsFlag := flag.Int("trials", 4, "number of trials")
	trialSecsFlag := flag.Float64("trial-secs", 60, "duration of each trial in seconds")
	panelFlag := flag.Float64("panel-px", 900,
		"side of each square bounding panel, in logical pixels")
	unitFlag := flag.Float64("unit-px", 0,
		"pixels per normalized figure unit (0 = fit the panel automatically)")
	strokeFlag := flag.Float64("stroke-px", 6, "figure stroke width in logical pixels")
	panelStrokeFlag := flag.Float64("panel-stroke-px", 2,
		"outline width of the bounding rectangle around each stream (0 = no outline)")
	jitterFlag := flag.Float64("jitter-px", 20,
		"radius of the random position jitter, in logical pixels")
	soundFlag := flag.String("attractor-sound", "",
		"WAV file played with the attractor (default: the embedded ping)")

	conds := buildConditions()
	// Just the codes: a select row divides the dialog's width evenly among all
	// its options, so eight buttons are ~66 px wide — room for "3D", not for a
	// sentence. The full description of each condition is in README.md, is
	// printed by -h, and appears on the instructions screen once the session
	// starts.
	condField := control.InfoField{
		Name:    "condition",
		Label:   "Condition (1 = triangles, 2 = length, 3 = angle)",
		Type:    control.FieldSelect,
		Options: conditionNames,
	}

	exp := control.NewExperimentFromFlags("Geometry-consistency",
		control.Black, control.White, 22, condField)
	defer exp.End()

	// Flags are parsed now. An explicit -exp always wins; otherwise take the
	// dialog's answer when there was one.
	expExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "exp" {
			expExplicit = true
		}
	})
	condName := strings.ToUpper(strings.TrimSpace(*expFlag))
	if !expExplicit {
		if v := strings.TrimSpace(exp.Info["condition"]); v != "" {
			condName = strings.ToUpper(strings.SplitN(v, " ", 2)[0])
		}
	}
	cond, ok := conds[condName]
	if !ok {
		exp.Fatal("unknown condition %q — use one of %s", condName, strings.Join(conditionNames, ", "))
	}

	// A typo in the condition table would silently break the design, so check
	// the invariant the paper relies on. The angle conditions are a little off
	// 2 because they fix the angles, not the areas.
	if r := cond.areaRatio(); r < 1.9 || r > 2.1 {
		exp.Fatal("condition %s: implied-area ratio is %.3f, expected ~2 — check the figure table", cond.name, r)
	}

	if *trialsFlag < 1 {
		exp.Fatal("-trials must be at least 1")
	}
	if *trialSecsFlag < float64(slotDurationMs)/1000 {
		exp.Fatal("-trial-secs must be at least %.1f (one presentation)", float64(slotDurationMs)/1000)
	}

	p := params{
		cond:        cond,
		nTrials:     *trialsFlag,
		nSlots:      int(math.Round(*trialSecsFlag * 1000 / slotDurationMs)),
		panelPx:     *panelFlag,
		strokePx:    *strokeFlag,
		jitterPx:    *jitterFlag,
		panelCentre: logicalWidth / 4,

		panelStrokePx: float32(*panelStrokeFlag),
	}

	// Pixels per normalized unit: fit the largest figure of this condition
	// inside its panel at maximum scale jitter and maximum position jitter.
	p.unitPx = *unitFlag
	if p.unitPx <= 0 {
		maxR := math.Max(cond.small.maxR, cond.large.maxR)
		maxR = math.Max(maxR, cond.small.maxR*cond.scaleFromSmall)
		maxR = math.Max(maxR, cond.large.maxR*cond.scaleFromLarge)
		room := p.panelPx/2 - p.jitterPx - p.strokePx/2 - panelMarginPx
		if room <= 0 {
			exp.Fatal("panel of %.0f px leaves no room for the figures — raise -panel-px", p.panelPx)
		}
		p.unitPx = room / (maxR * maxScaleJitter)
	}

	if err := exp.SetLogicalSize(logicalWidth, logicalHeight); err != nil {
		log.Printf("warning: SetLogicalSize: %v", err)
	}

	cbGroup := *cbFlag
	if cbGroup < 0 {
		cbGroup = exp.SubjectID % 4
	}
	cbGroup %= 4
	trials := buildTrials(cond, p.nTrials, cbGroup)

	// Session metadata, into the companion -info.txt.
	exp.Data.WriteComment("--PARADIGM")
	exp.Data.WriteComment("p reference: Dillon, Izard & Spelke (2020), Infancy 25(5), 618-639")
	exp.Data.WriteComment("p " + cond.describe())
	exp.Data.WriteComment(fmt.Sprintf("p counterbalancing group: %d (context=%s, shape-and-area starts %s)",
		cbGroup, trials[0].context.id, sideName(trials[0].shapeOnLeft)))
	exp.Data.WriteComment(fmt.Sprintf("p trials: %d x %d presentations of %d ms + %d ms blank",
		p.nTrials, p.nSlots, stimulusDurationMs, isiDurationMs))
	exp.Data.WriteComment(fmt.Sprintf("p geometry: unit=%.2f px, panel=%.0f px square at x=+/-%.0f, figure stroke=%.1f px, panel outline=%.1f px, jitter radius=%.1f px, canvas=%dx%d",
		p.unitPx, p.panelPx, p.panelCentre, p.strokePx, p.panelStrokePx, p.jitterPx, logicalWidth, logicalHeight))

	exp.AddDataVariableNames([]string{
		"trial", "condition", "cb_group", "context_figure", "change_figure",
		"area_only_scale", "shape_side",
		"look_left_ms", "look_right_ms",
		"look_shape_and_area_ms", "look_area_only_ms", "proportion_shape_and_area",
		"left_press_durations_ms", "right_press_durations_ms",
	})

	// A second CSV holds the per-presentation stimulus log.
	stem := strings.TrimSuffix(exp.Data.Filename, ".csv")
	frames, err := results.NewOutputFile(exp.Data.Directory, stem+"-frames.csv")
	if err != nil {
		exp.Fatal("creating the presentation log: %v", err)
	}
	frames.WriteLine("subject_id,trial,slot,side,stream,figure_id,onset_ns,rotation_deg,scale,flip,jitter_dx_px,jitter_dy_px")

	// Frame counts, so each presentation lands on a whole number of refreshes.
	frameDur := exp.Screen.FrameDuration()
	framesOn := int(math.Round(float64(stimulusDurationMs*time.Millisecond) / float64(frameDur)))
	framesOff := int(math.Round(float64(isiDurationMs*time.Millisecond) / float64(frameDur)))
	if framesOn < 1 || framesOff < 1 {
		exp.Fatal("refresh rate %.1f Hz is too low for %d/%d ms presentations",
			exp.Screen.RefreshRate(), stimulusDurationMs, isiDurationMs)
	}
	exp.Data.WriteComment(fmt.Sprintf("p frame plan: %d + %d refreshes per presentation at %.3f ms (%.1f ms per slot, nominal %d ms)",
		framesOn, framesOff, float64(frameDur)/1e6,
		float64((framesOn+framesOff))*float64(frameDur)/1e6, slotDurationMs))

	// Attractor sound: a caller-supplied rattle, else the embedded ping.
	var attractor *stimuli.Sound
	if *soundFlag != "" {
		attractor = stimuli.NewSound(*soundFlag)
	} else {
		attractor = stimuli.NewSoundFromMemory(assets_embed.CorrectWav)
	}
	if err := attractor.PreloadDevice(exp.AudioDevice); err != nil {
		log.Printf("warning: attractor sound unavailable (%v) — running silently", err)
		attractor = nil
	}
	// The audio sink suspends while idle and swallows the first sound after it;
	// a silent tone right after the start keypress wakes it up.
	warmup := stimuli.NewTone(440, 60, 0)
	_ = warmup.PreloadDevice(exp.AudioDevice)

	boxes := panelOutlines(p)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	instructions := fmt.Sprintf(
		"Geometry consistency — condition %s\n%s\n\n"+
			"%d trials of %.0f s. Between trials a pink dot appears at the centre:\n"+
			"press SPACE once the infant is looking at it.\n\n"+
			"During each trial, code the infant's gaze:\n"+
			"    hold LEFT   — looking at the left panel\n"+
			"    hold RIGHT  — looking at the right panel\n"+
			"    neither     — looking away\n"+
			"    ESC         — abort\n\n"+
			"Press SPACE to begin.",
		cond.name, cond.label, p.nTrials, float64(p.nSlots*slotDurationMs)/1000)

	runErr := exp.Run(func() error {
		if err := exp.ShowInstructions(instructions); err != nil {
			return err
		}
		_ = warmup.Play()

		var totalShape, totalArea int64

		for i, t := range trials {
			// Build the streams before the attractor, so generating them does
			// not eat into the infant's looking window.
			left, right := buildStreams(t, p, rng)

			if err := showAttractor(exp, attractor); err != nil {
				return err
			}

			onsets, g, err := presentTrial(exp, left, right, boxes, float32(p.strokePx), framesOn, framesOff)
			if err != nil {
				return err
			}

			lookShape, lookArea := g.rightMs, g.leftMs
			shapeDurs, areaDurs := g.rightDurs, g.leftDurs
			if t.shapeOnLeft {
				lookShape, lookArea = g.leftMs, g.rightMs
				shapeDurs, areaDurs = g.leftDurs, g.rightDurs
			}
			totalShape += lookShape
			totalArea += lookArea

			exp.Data.Add(
				i+1, cond.name, cbGroup, t.context.id, t.changeFig.id,
				t.areaScale, sideName(t.shapeOnLeft),
				g.leftMs, g.rightMs,
				lookShape, lookArea, proportion(lookShape, lookArea),
				formatDurations(shapeDurs), formatDurations(areaDurs),
			)
			_ = exp.Data.Save()

			for slot := range left {
				writeFrameRow(frames, exp.SubjectID, i+1, slot, "left", left[slot], onsets[slot])
				writeFrameRow(frames, exp.SubjectID, i+1, slot, "right", right[slot], onsets[slot])
			}
			_ = frames.Save()

			if i < len(trials)-1 {
				if err := exp.Blank(1500); err != nil {
					return err
				}
			}
		}

		summary := fmt.Sprintf(
			"Session complete — %d trials.\n\n"+
				"Looking time, shape-and-area change stream: %.1f s\n"+
				"Looking time, area-only change stream:      %.1f s\n\n"+
				"Proportion to the shape-and-area stream: %s\n"+
				"  (> 0.50 = the infant detected the shape change)\n\n"+
				"Press any key to exit.",
			p.nTrials, float64(totalShape)/1000, float64(totalArea)/1000,
			proportion(totalShape, totalArea))
		if err := exp.Show(stimuli.NewTextBox(summary, 1100, control.FPoint{}, control.White)); err != nil {
			return err
		}
		if _, err := exp.Keyboard.Wait(); err != nil && !control.IsEndLoop(err) {
			return err
		}
		return control.EndLoop
	})

	if err := frames.Finalize(); err != nil {
		log.Printf("warning: saving the presentation log: %v", err)
	}
	if runErr != nil && !control.IsEndLoop(runErr) {
		exp.Fatal("experiment error: %v", runErr)
	}
}

func writeFrameRow(f *results.OutputFile, subject, trial, slot int, side string, pl placement, onset uint64) {
	f.WriteLine(fmt.Sprintf("%d,%d,%d,%s,%s,%s,%d,%.3f,%.4f,%t,%.2f,%.2f",
		subject, trial, slot, side, pl.stream, pl.figureID, onset,
		pl.rotDeg, pl.scale, pl.flip, pl.dx, pl.dy))
}

func sideName(left bool) string {
	if left {
		return "left"
	}
	return "right"
}

// proportion is the paper's dependent measure: looking to the shape-and-area
// stream over looking to both. "NA" when the infant never looked at either.
func proportion(shape, area int64) string {
	if shape+area == 0 {
		return "NA"
	}
	return fmt.Sprintf("%.4f", float64(shape)/float64(shape+area))
}
