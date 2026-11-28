// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Shape-Change-Detection: an adult version of the two-stream change-detection
// paradigm used with infants (Libertus & Brannon, 2010; Decarli, Piazza &
// Izard, 2023 — see ../Number-Change-Detection for the dot-cloud original).
//
// Two streams of shapes flash in synchrony, one on each side of a fixation
// cross. Both sides start with the same shape. On most trials one side starts
// alternating with a new shape at a random image; the participant clicks (any
// mouse button) or presses SPACE as soon as they see it. The trial ends at the
// first response. RT is measured from the onset of the first changed image,
// with SDL hardware timestamps on both sides of the subtraction.
//
// Each image is one stimuli.DrawFunc that draws the cross and both shapes, so
// the left and right streams are drawn in the same frame and cannot drift
// apart.
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Parameters ───────────────────────────────────────────────────────────────

const (
	imageOn  = 500 * time.Millisecond // as in the infant studies
	imageOff = 300 * time.Millisecond // blank between images

	maxImages      = 20 // stream length; a trial with no response ends here
	firstChangeImg = 4  // the change happens at image firstChangeImg..lastChangeImg (1-based)
	lastChangeImg  = 14 // leaves at least maxImages-lastChangeImg+1 images to respond

	changeTrialsPerSide = 10
	catchTrials         = 5 // no change at all: measures false alarms

	baseSize   = 120.0 // pixels: diameter of a circle; other shapes get the same area
	sizeJitter = 0.20  // ±20 % per image and per side, so a size change is not a cue

	itiMs = 1000
)

// ── Shapes ───────────────────────────────────────────────────────────────────

// Every shape is convex (stimuli.Shape fills convex polygons) and, at a given
// size, has the area of a circle of diameter size, so area is not a cue
// either.
type shapeKind int

const (
	circle shapeKind = iota
	triangle
	square
	pentagon
	hexagon
	nShapes
)

var shapeNames = [...]string{"circle", "triangle", "square", "pentagon", "hexagon"}

func (k shapeKind) String() string { return shapeNames[k] }

// vertices[k] is the number of vertices of each polygon (0 for the circle).
var vertices = [...]int{0, 3, 4, 5, 6}

// makeShape returns shape k of the given size, centred at (x, 0).
func makeShape(k shapeKind, size, x float32) stimuli.VisualStimulus {
	pos := control.FPoint{X: x, Y: 0}
	if k == circle {
		c := stimuli.NewCircle(size/2, control.White)
		c.SetPosition(pos)
		return c
	}
	n := vertices[k]
	area := math.Pi * float64(size) * float64(size) / 4
	// A regular n-gon of circumradius R has area n/2 · R² · sin(2π/n).
	r := math.Sqrt(2 * area / (float64(n) * math.Sin(2*math.Pi/float64(n))))
	// Start at the top (+Y is up) so the triangle and pentagon point up; the
	// square is rotated by half a step so that its sides are horizontal.
	start := math.Pi / 2
	if n == 4 {
		start = math.Pi / 4
	}
	pts := make([]control.FPoint, n)
	for i := range pts {
		a := start + 2*math.Pi*float64(i)/float64(n)
		pts[i] = control.FPoint{X: float32(r * math.Cos(a)), Y: float32(r * math.Sin(a))}
	}
	s := stimuli.NewShape(pts, control.White)
	s.SetPosition(pos)
	return s
}

func jitteredSize() float32 {
	return float32(baseSize * (1 + sizeJitter*(2*rand.Float64()-1)))
}

// ── Trials ───────────────────────────────────────────────────────────────────

type trial struct {
	side        string // "left", "right" or "none" (catch trial)
	base, other shapeKind
	changeImg   int // 1-based image at which the change starts; 0 for a catch trial
}

func buildTrials() []trial {
	var trials []trial
	add := func(side string) {
		base := shapeKind(design.RandInt(0, int(nShapes)-1))
		other := base
		for other == base {
			other = shapeKind(design.RandInt(0, int(nShapes)-1))
		}
		t := trial{side: side, base: base, other: other}
		if side != "none" {
			t.changeImg = design.RandInt(firstChangeImg, lastChangeImg)
		}
		trials = append(trials, t)
	}
	for i := 0; i < changeTrialsPerSide; i++ {
		add("left")
		add("right")
	}
	for i := 0; i < catchTrials; i++ {
		add("none")
	}
	design.ShuffleList(trials)
	return trials
}

// scenes builds one composite image per stream position. Up to the change both
// sides show the base shape; from the change on, the changing side alternates
// other, base, other, … like the infants' "changing stream".
func scenes(t trial, xLeft, xRight float32) []stimuli.Stimulus {
	fix := stimuli.NewFixCross(20, 2, control.White)
	out := make([]stimuli.Stimulus, maxImages)
	for i := 0; i < maxImages; i++ {
		img := i + 1
		kindAt := func(side string) shapeKind {
			if t.side != side || img < t.changeImg {
				return t.base
			}
			if (img-t.changeImg)%2 == 0 {
				return t.other
			}
			return t.base
		}
		left := makeShape(kindAt("left"), jitteredSize(), xLeft)
		right := makeShape(kindAt("right"), jitteredSize(), xRight)
		out[i] = stimuli.DrawFunc(func(s *apparatus.Screen) error {
			if err := fix.Draw(s); err != nil {
				return err
			}
			if err := left.Draw(s); err != nil {
				return err
			}
			return right.Draw(s)
		})
	}
	return out
}

// ── One trial ────────────────────────────────────────────────────────────────

type response struct {
	responded bool
	device    string // "mouse", "key" or "none"
	image     int    // 1-based image on screen (or in its blank) at the response; 0 if none
	ns        uint64 // SDL event timestamp
}

// runTrial presents one stream and stops it at the first response. It returns
// the response and the onset of every image that was shown, both on the SDL
// clock. An error satisfying control.IsEndLoop means ESC or window close.
func runTrial(exp *control.Experiment, t trial, xLeft, xRight float32) (response, []uint64, error) {
	elems := stimuli.MakeRegularStream(scenes(t, xLeft, xRight), imageOn, imageOff)

	// Onsets are taken from the onset hook, not from the returned TimingLog:
	// an image's TimingLog entry is appended only once the image has finished,
	// so a response during the changed image itself would find no entry for
	// it. Preallocated, because the hook runs with GC disabled.
	onsets := make([]uint64, maxImages)
	onOnset := func(i int, ns uint64) error {
		onsets[i] = ns
		return nil
	}

	resp := response{device: "none"}
	scanned := 0
	onFrame := func(ctx stimuli.FrameContext) error {
		for ; scanned < len(ctx.Events); scanned++ {
			ev := &ctx.Events[scanned]
			switch {
			case ev.Event.Type == control.EVENT_MOUSE_BUTTON_DOWN:
				resp.device = "mouse"
			case ev.Event.Type == control.EVENT_KEY_DOWN && ev.Event.KeyboardEvent().Key == control.K_SPACE:
				resp.device = "key"
			default:
				continue
			}
			resp.responded = true
			resp.ns = ev.TimestampNS
			return control.EndLoop // stop the stream: the trial is over
		}
		return nil
	}

	// The stream does not empty the event queue, so a click made during the
	// inter-trial interval would otherwise count as an instant response.
	exp.Keyboard.Clear()
	_, _, err := stimuli.PresentStreamOfStimuliHooks(exp.Screen, elems, 0, 0, onFrame, onOnset)
	if err != nil && !(control.IsEndLoop(err) && resp.responded) {
		return resp, onsets, err // ESC / quit, or a real error
	}
	// The image on screen (or in its blank) when the response was made: the
	// last one whose onset precedes it. Taken from the timestamps rather than
	// from ctx.Index, which is where the response was *noticed*, up to a frame
	// later.
	if resp.responded {
		for i, on := range onsets {
			if on != 0 && on <= resp.ns {
				resp.image = i + 1
			}
		}
	}
	return resp, onsets, nil
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	exp := control.NewExperimentFromFlags("Shape-Change-Detection", control.Black, control.White, 32)
	defer exp.End()

	exp.AddDataVariableNames([]string{
		"trial", "change_side", "base_shape", "new_shape", "change_image", "n_images",
		"responded", "device", "resp_image", "rt_ms", "resp_from_start_ms", "outcome",
	})

	trials := buildTrials()

	err := exp.Run(func() error {
		w, _ := exp.DrawArea()
		xLeft, xRight := -w/4, w/4

		// Translate the participant-facing text here.
		if err := exp.ShowInstructions(
			"Keep your eyes on the central cross.\n\n" +
				"Shapes will flash on the left and on the right.\n" +
				"At some point, the shapes on one side may start to change.\n\n" +
				"As soon as you see a change, click a mouse button (or press SPACE).\n" +
				"On some trials nothing changes: then do not respond.\n\n" +
				"Press SPACE to start."); err != nil {
			return err
		}

		var hitRTs []float64
		nChange, nFA := 0, 0
		for i, t := range trials {
			if err := exp.Blank(itiMs); err != nil {
				return err
			}
			resp, onsets, err := runTrial(exp, t, xLeft, xRight)
			if err != nil {
				return err // ESC: no row for the aborted trial
			}

			var rt, fromStart any = "NA", "NA"
			if resp.responded {
				fromStart = float64(int64(resp.ns-onsets[0])) / 1e6
			}
			var outcome string
			switch {
			case t.side == "none" && resp.responded:
				outcome = "false_alarm"
				nFA++
			case t.side == "none":
				outcome = "correct_rejection"
			case !resp.responded:
				outcome = "miss"
			case onsets[t.changeImg-1] == 0 || resp.ns < onsets[t.changeImg-1]:
				outcome = "anticipation" // responded before the change was on screen
			default:
				outcome = "hit"
				ms := float64(int64(resp.ns-onsets[t.changeImg-1])) / 1e6
				rt = ms
				hitRTs = append(hitRTs, ms)
			}
			if t.side != "none" {
				nChange++
			}

			newShape := "NA"
			if t.side != "none" {
				newShape = t.other.String()
			}
			exp.Data.Add(i+1, t.side, t.base.String(), newShape, t.changeImg, maxImages,
				resp.responded, resp.device, resp.image, rt, fromStart, outcome)
		}

		summary := fmt.Sprintf("Detected changes: %d / %d\nFalse alarms: %d / %d",
			len(hitRTs), nChange, nFA, catchTrials)
		if len(hitRTs) > 0 {
			sort.Float64s(hitRTs)
			summary += fmt.Sprintf("\nMedian response time: %.0f ms", hitRTs[len(hitRTs)/2])
		}
		if err := exp.ShowInstructions("Thank you!\n\n" + summary + "\n\nPress SPACE to exit."); err != nil {
			return err
		}
		return control.EndLoop
	})
	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
