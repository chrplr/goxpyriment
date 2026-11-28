// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// demo_canvas_stream: the Shape-Change-Detection task (../Shape-Change-Detection)
// built the other way round. Each image of the stream is pre-rendered into an
// off-screen stimuli.Canvas (both shapes and the fixation cross bound into one
// texture) before the trial starts, and the stream presents that sequence of
// canvases.
//
// Everything else (design, response handling, scoring, data columns) is
// copied unchanged, so
//
//	diff examples/Shape-Change-Detection/main.go examples/demo_canvas_stream/main.go
//
// shows exactly what the canvas approach changes. One extra data column,
// prep_ms, records the time to render each trial's canvases, and the -info.txt
// records their size and GPU memory. Both programs record max_dur_err_ms, the
// worst on-screen duration error per trial, for comparing their timing. See
// README.md for the comparison.
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

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
	r := circumradius(n, size)
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

// circumradius returns the circumradius of the regular n-gon whose area is
// that of a circle of diameter size.
func circumradius(n int, size float32) float64 {
	area := math.Pi * float64(size) * float64(size) / 4
	// A regular n-gon of circumradius R has area n/2 · R² · sin(2π/n).
	return math.Sqrt(2 * area / (float64(n) * math.Sin(2*math.Pi/float64(n))))
}

// canvasHeight is the height of every canvas: just enough for the tallest
// shape at the largest size (the triangle, which reaches its circumradius
// above the centre), plus a margin. A band rather than the whole screen,
// because every canvas is a full texture of width × height × 4 bytes.
func canvasHeight() float32 {
	return float32(2*math.Ceil(circumradius(3, baseSize*(1+sizeJitter)))) + 16
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

// scenes renders one canvas per stream position, before the trial. Up to the
// change both sides show the base shape; from the change on, the changing side
// alternates other, base, other, … like the infants' "changing stream".
//
// Inside a canvas, positions are relative to the canvas centre, with +Y up,
// just as on the screen, so the shapes are placed exactly as in
// Shape-Change-Detection. The stream centres each canvas on the screen.
//
// The caller must Unload the canvases: each one holds a GPU texture.
func scenes(exp *control.Experiment, t trial, width float32) ([]stimuli.Stimulus, []*stimuli.Canvas, error) {
	fix := stimuli.NewFixCross(20, 2, control.White)
	xLeft, xRight := -width/4, width/4
	height := canvasHeight()
	out := make([]stimuli.Stimulus, maxImages)
	canvases := make([]*stimuli.Canvas, 0, maxImages)
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
		cv := stimuli.NewCanvas(width, height, control.Black)
		canvases = append(canvases, cv)
		for _, v := range []stimuli.VisualStimulus{
			fix,
			makeShape(kindAt("left"), jitteredSize(), xLeft),
			makeShape(kindAt("right"), jitteredSize(), xRight),
		} {
			if err := cv.Blit(v, exp.Screen); err != nil {
				return nil, canvases, fmt.Errorf("rendering image %d: %w", img, err)
			}
		}
		out[i] = cv
	}
	return out, canvases, nil
}

// ── One trial ────────────────────────────────────────────────────────────────

type response struct {
	responded bool
	device    string // "mouse", "key" or "none"
	image     int    // 1-based image on screen (or in its blank) at the response; 0 if none
	ns        uint64 // SDL event timestamp
}

// cost is what the canvas approach measures about a trial.
type cost struct {
	prepMs      float64 // wall-clock time to render the trial's canvases
	maxDurErrMs float64 // worst on-screen duration error (see maxDurErrMs)
}

// runTrial renders the trial's canvases, presents them as one stream and stops
// it at the first response. It returns the response and the onset of every
// image that was shown, both on the SDL clock. An error satisfying
// control.IsEndLoop means ESC or window close.
func runTrial(exp *control.Experiment, t trial, width float32) (response, []uint64, cost, error) {
	var c cost
	start := time.Now()
	imgs, canvases, err := scenes(exp, t, width)
	defer func() {
		for _, cv := range canvases {
			_ = cv.Unload() // Canvas.Unload only destroys the texture; it cannot fail
		}
	}()
	if err != nil {
		return response{}, nil, c, err
	}
	// CPU side only: SDL hands the drawing to the GPU, which may still be
	// executing it when the stream starts.
	c.prepMs = float64(time.Since(start).Microseconds()) / 1000
	elems := stimuli.MakeRegularStream(imgs, imageOn, imageOff)

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
	_, logs, err := stimuli.PresentStreamOfStimuliHooks(exp.Screen, elems, 0, 0, onFrame, onOnset)
	if err != nil && !(control.IsEndLoop(err) && resp.responded) {
		return resp, onsets, c, err // ESC / quit, or a real error
	}
	c.maxDurErrMs = maxDurErrMs(exp, logs)
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
	return resp, onsets, c, nil
}

// maxDurErrMs returns the worst difference, in ms, between an image's on-screen
// duration and its nominal whole-frame duration, over the images in logs. Each
// image should stay on for a whole number of frames, so 0 (to a fraction of a
// millisecond) means no frame was dropped and about one frame means one was.
// logs covers only the images that finished, not the one a response
// interrupted.
func maxDurErrMs(exp *control.Experiment, logs []stimuli.TimingLog) float64 {
	fd := exp.Screen.FrameDuration()
	nominal := ((imageOn + fd/2) / fd) * fd
	worst := 0.0
	for _, l := range logs {
		if l.OnsetNS == 0 || l.OffsetNS == 0 {
			continue
		}
		e := math.Abs(float64(int64(l.OffsetNS-l.OnsetNS)-nominal.Nanoseconds())) / 1e6
		worst = math.Max(worst, e)
	}
	return worst
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	exp := control.NewExperimentFromFlags("demo_canvas_stream", control.Black, control.White, 32)
	defer exp.End()

	exp.AddDataVariableNames([]string{
		"trial", "change_side", "base_shape", "new_shape", "change_image", "n_images",
		"responded", "device", "resp_image", "rt_ms", "resp_from_start_ms", "outcome",
		"prep_ms", "max_dur_err_ms",
	})

	trials := buildTrials()

	err := exp.Run(func() error {
		w, _ := exp.DrawArea()
		h := canvasHeight()
		exp.AddExperimentInfo(fmt.Sprintf(
			"canvas %.0fx%.0f px, %d per trial, %.1f MB of textures per trial (RGBA, computed)",
			w, h, maxImages, float64(w)*float64(h)*4*maxImages/1e6))

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
			resp, onsets, c, err := runTrial(exp, t, w)
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
				resp.responded, resp.device, resp.image, rt, fromStart, outcome,
				c.prepMs, c.maxDurErrMs)
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
