// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// Number Change Detection — replication of the paradigm from:
// Decarli G., Piazza M. & Izard V. (2023). Are infants' preferences in the
// number change detection paradigm driven by sequence patterns? Infancy.
// https://doi.org/10.1111/infa.12505
//
// Two streams of dot arrays (5 or 20 dots) are presented simultaneously on
// the left and right of a large projection screen. An experimenter codes the
// infant's looking direction in real time by holding the LEFT or RIGHT arrow
// key. Looking times are accumulated per trial and the log ratio (stream A vs
// stream B) is computed at the end of the session.
//
// Usage:
//
//	go run main.go [-exp preliminary|exp1|exp2]
//
// A setup dialog collects subject ID, experiment type, and the physical width
// of the projection screen (used to compute the pixel/cm scale factor).
//
// Experimenter controls during stream presentation:
//
//	LEFT arrow  — infant looking at left stream
//	RIGHT arrow — infant looking at right stream
//	(neither)   — infant looking away / not attending
//	ESC         — abort
//
// Counterbalancing (automatic, based on subject ID):
//   - Side of stream A (primary stream) alternates across the 4 trials.
//   - Parameter condition (intensive / extensive) used in pairs (trials 1–2 and 3–4).
//   - Constant-stream numerosity (5 or 20) alternates across even/odd subject IDs.

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Timing ──────────────────────────────────────────────────────────────────

const (
	stimulusDurationMs = 500 // each dot array shown for 500 ms (paper §2.2)
	isiDurationMs      = 300 // blank inter-stimulus interval (paper §2.2)
	numImagesPerBlock  = 24  // unique images per sequence block
	numBlocksPerTrial  = 2   // block repeated twice → 48 images, ~40 s per trial
	nTrials            = 4   // total trials per participant
)

// ── Numerosity ────────────────────────────────────────────────────────────────

const (
	numerosityLow  = 5
	numerosityHigh = 20
)

// ── Screen layout ─────────────────────────────────────────────────────────────
// Physical measurements (paper §2.2): 68×51 cm per stream, 43 cm gap.
// pxPerCm is computed at runtime from the screen width entered in the dialog.

var (
	pxPerCm  float32
	leftCtr  control.FPoint
	rightCtr control.FPoint
)

func initLayout(px float32) {
	pxPerCm = px
	leftCtr = control.FPoint{X: -(43*px/2 + 68*px/2), Y: 0}
	rightCtr = control.FPoint{X: 43*px/2 + 68*px/2, Y: 0}
}

// ── Dot parameters ────────────────────────────────────────────────────────────
// Derived from paper §2.2 physical measurements; converted via pxPerCm.
//
// Extensive equated: total area and convex hull matched across numerosities.
//   5 dots:  dot diameter 2.7–4.6 cm, array diameter 20–45 cm
//  20 dots:  dot diameter 1.3–2.3 cm, same array diameter
//
// Intensive equated: dot size and density matched across numerosities.
//   5 dots:  dot diameter 2.3–4.6 cm, array diameter 15–25 cm
//  20 dots:  dot diameter 2.3–4.6 cm, array diameter 25–50 cm

type dotParams struct {
	nDots                    int
	minDotRad, maxDotRad     float32 // dot radius (px)
	minCloudRad, maxCloudRad float32 // cloud (array) radius (px)
}

func physParams(px float32, nDots int, minDotCm, maxDotCm, minCloudCm, maxCloudCm float64) dotParams {
	return dotParams{
		nDots:       nDots,
		minDotRad:   float32(minDotCm) * px,
		maxDotRad:   float32(maxDotCm) * px,
		minCloudRad: float32(minCloudCm) * px,
		maxCloudRad: float32(maxCloudCm) * px,
	}
}

var extParams5, extParams20, intParams5, intParams20 dotParams

func initDotParams(px float32) {
	extParams5 = physParams(px, numerosityLow, 1.35, 2.30, 10.0, 22.5)
	extParams20 = physParams(px, numerosityHigh, 0.65, 1.15, 10.0, 22.5)
	intParams5 = physParams(px, numerosityLow, 1.15, 2.30, 7.5, 12.5)
	intParams20 = physParams(px, numerosityHigh, 1.15, 2.30, 12.5, 25.0)
}

// ── Stream types & parameter conditions ──────────────────────────────────────

type streamType int

const (
	streamConstant streamType = iota
	streamAlternating
	streamRandom
)

func (s streamType) String() string {
	return [...]string{"constant", "alternating", "random"}[s]
}

type paramCond int

const (
	condIntensive paramCond = iota
	condExtensive
)

func (p paramCond) String() string {
	if p == condIntensive {
		return "intensive"
	}
	return "extensive"
}

func paramsFor(n int, c paramCond) dotParams {
	if c == condExtensive {
		if n == numerosityLow {
			return extParams5
		}
		return extParams20
	}
	if n == numerosityLow {
		return intParams5
	}
	return intParams20
}

// ── Sequence generation ───────────────────────────────────────────────────────

// buildSequence returns 24 numerosity values for the given stream type.
// constantNum is used only by streamConstant; alternating and random always
// use both numerosityLow and numerosityHigh.
func buildSequence(st streamType, constantNum int) []int {
	s := make([]int, numImagesPerBlock)
	switch st {
	case streamConstant:
		for i := range s {
			s[i] = constantNum
		}
	case streamAlternating:
		lo, hi := numerosityLow, numerosityHigh
		if rand.Intn(2) == 1 { // randomise which numerosity comes first
			lo, hi = hi, lo
		}
		for i := range s {
			if i%2 == 0 {
				s[i] = lo
			} else {
				s[i] = hi
			}
		}
	case streamRandom:
		// Exactly 12 of each numerosity, randomly ordered (paper §3, "random stream")
		for i := 0; i < 12; i++ {
			s[i] = numerosityLow
			s[i+12] = numerosityHigh
		}
		rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
	}
	return s
}

// ── Dot-array generation ──────────────────────────────────────────────────────

func makeDotArray(n int, c paramCond, ctr control.FPoint) *stimuli.DotCloud {
	p := paramsFor(n, c)
	cloudRad := p.minCloudRad + rand.Float32()*(p.maxCloudRad-p.minCloudRad)
	dotRad := p.minDotRad + rand.Float32()*(p.maxDotRad-p.minDotRad)
	transparent := control.RGBA(0, 0, 0, 0)
	dc := stimuli.NewDotCloud(cloudRad, transparent, control.Black)
	dc.SetPosition(ctr) // position before Make so dots are placed at ctr + offset
	for tries := 0; tries < 5; tries++ {
		if dc.Make(n, dotRad, 2) {
			break
		}
		dotRad *= 0.85 // shrink dots slightly and retry on placement failure
	}
	return dc
}

func makeStreamArrays(st streamType, constantNum int, c paramCond, ctr control.FPoint) []*stimuli.DotCloud {
	seq := buildSequence(st, constantNum)
	arrays := make([]*stimuli.DotCloud, numImagesPerBlock)
	for i, n := range seq {
		arrays[i] = makeDotArray(n, c, ctr)
	}
	return arrays
}

// ── Attractor ─────────────────────────────────────────────────────────────────

// showAttractor displays an animated pulsing red circle at the centre.
// Returns when the experimenter presses SPACE (signalling infant is attending).
func showAttractor(exp *control.Experiment) error {
	bottomY := float32(-exp.Screen.Height/2 + 80)
	msg := stimuli.NewTextLine("[Experimenter] Press SPACE when infant is looking at the centre", 0, bottomY, control.Black)
	defer msg.Unload()
	circ := stimuli.NewCircle(60, control.Red)
	circ.SetPosition(control.FPoint{X: 0, Y: 0})

	start := time.Now()
	for {
		circ.Radius = float32(40 + 30*math.Sin(2*math.Pi*time.Since(start).Seconds()*1.5))
		_ = exp.Screen.Clear()
		_ = circ.Draw(exp.Screen)
		_ = msg.Draw(exp.Screen)
		_ = exp.Screen.PacedFlip()

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
		time.Sleep(16 * time.Millisecond) // ~60 fps
	}
}

// ── Stream presentation with live looking-time coding ─────────────────────────

// formatDurations serialises a slice of millisecond durations as a
// semicolon-separated string suitable for a CSV cell, e.g. "1200;340;88".
// Returns an empty string when the slice is empty.
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

// presentStreams displays both streams simultaneously for the full trial duration
// (48 frames × 800 ms = ~38.4 s). The experimenter holds LEFT or RIGHT to code
// the infant's gaze.
//
// Returns cumulative looking times (lookLeft, lookRight) in ms, plus the list of
// individual press durations for each key. Durations are measured with
// hardware-precision SDL3 timestamps: KEY_DOWN and KEY_UP event timestamps are
// used for presses that complete during the trial; WaitKeyReleaseTS is called for
// any key that is still held when the display loop ends.
func presentStreams(exp *control.Experiment, left, right []*stimuli.DotCloud) (int64, int64, []int64, []int64, error) {
	var lookLeft, lookRight int64
	leftHeld, rightHeld := false, false
	prev := clock.GetTime()

	// Hardware timestamps of the most recent KEY_DOWN for each key.
	var leftDownTS, rightDownTS uint64
	// Per-press durations in ms.
	var leftDurations, rightDurations []int64

	accum := func() {
		now := clock.GetTime()
		dt := now - prev
		prev = now
		if leftHeld {
			lookLeft += dt
		}
		if rightHeld {
			lookRight += dt
		}
	}

	poll := func() error {
		state := exp.PollEvents(func(e sdl.Event) bool {
			switch e.Type {
			case sdl.EVENT_KEY_DOWN:
				ke := e.KeyboardEvent()
				switch ke.Key {
				case control.K_LEFT:
					if !leftHeld {
						leftHeld = true
						leftDownTS = ke.Timestamp
					}
				case control.K_RIGHT:
					if !rightHeld {
						rightHeld = true
						rightDownTS = ke.Timestamp
					}
				}
			case sdl.EVENT_KEY_UP:
				ke := e.KeyboardEvent()
				switch ke.Key {
				case control.K_LEFT:
					if leftHeld {
						leftHeld = false
						leftDurations = append(leftDurations, int64(ke.Timestamp-leftDownTS)/1_000_000)
					}
				case control.K_RIGHT:
					if rightHeld {
						rightHeld = false
						rightDurations = append(rightDurations, int64(ke.Timestamp-rightDownTS)/1_000_000)
					}
				}
			}
			return false
		})
		if state.QuitRequested {
			return control.EndLoop
		}
		return nil
	}

	waitRecording := func(durationMs int64) error {
		deadline := clock.GetTime() + durationMs
		for clock.GetTime() < deadline {
			accum()
			if err := poll(); err != nil {
				return err
			}
			time.Sleep(2 * time.Millisecond)
		}
		return nil
	}

	for blk := 0; blk < numBlocksPerTrial; blk++ {
		for i := 0; i < numImagesPerBlock; i++ {
			// Draw both streams on one frame.
			_ = exp.Screen.Clear()
			_ = left[i].Draw(exp.Screen)
			_ = right[i].Draw(exp.Screen)
			_ = exp.Screen.Update()
			if err := waitRecording(stimulusDurationMs); err != nil {
				return lookLeft, lookRight, leftDurations, rightDurations, err
			}
			// Blank ISI.
			_ = exp.Screen.Clear()
			_ = exp.Screen.Update()
			if err := waitRecording(isiDurationMs); err != nil {
				return lookLeft, lookRight, leftDurations, rightDurations, err
			}
		}
	}

	// If a key is still held at the end of the trial, use WaitKeyReleaseTS to
	// obtain the hardware-precision KEY_UP timestamp and close the last press.
	if leftHeld {
		if upTS, err := exp.Keyboard.WaitKeyReleaseTS(control.K_LEFT, 10_000); err == nil && upTS > 0 {
			leftDurations = append(leftDurations, int64(upTS-leftDownTS)/1_000_000)
		}
	}
	if rightHeld {
		if upTS, err := exp.Keyboard.WaitKeyReleaseTS(control.K_RIGHT, 10_000); err == nil && upTS > 0 {
			rightDurations = append(rightDurations, int64(upTS-rightDownTS)/1_000_000)
		}
	}

	return lookLeft, lookRight, leftDurations, rightDurations, nil
}

// ── Trial structure ───────────────────────────────────────────────────────────

type trialSetup struct {
	leftStream  streamType
	rightStream streamType
	cond        paramCond
	constantNum int  // numerosity shown by the constant stream (if any)
	streamALeft bool // true when the primary (A) stream is on the left
}

// buildTrials implements the counterbalancing from paper §2.3:
//   - Side of stream A alternates across the 4 trials.
//   - Param condition (intensive/extensive) used in pairs (trials 1–2 and 3–4).
//   - Constant-stream numerosity derived from subject ID.
func buildTrials(expType string, subjectID int) []trialSetup {
	var streamA, streamB streamType
	switch expType {
	case "preliminary":
		streamA, streamB = streamAlternating, streamConstant
	case "exp1":
		streamA, streamB = streamRandom, streamConstant
	case "exp2":
		streamA, streamB = streamAlternating, streamRandom
	default:
		log.Fatalf("unknown experiment type %q — use: preliminary, exp1, exp2", expType)
	}

	constNum := numerosityLow
	if subjectID%2 != 0 {
		constNum = numerosityHigh
	}

	condA, condB := condIntensive, condExtensive
	if (subjectID/2)%2 != 0 {
		condA, condB = condExtensive, condIntensive
	}

	aLeft := subjectID%2 == 0

	mkTrial := func(aOnLeft bool, c paramCond) trialSetup {
		if aOnLeft {
			return trialSetup{streamA, streamB, c, constNum, true}
		}
		return trialSetup{streamB, streamA, c, constNum, false}
	}

	return []trialSetup{
		mkTrial(aLeft, condA),
		mkTrial(!aLeft, condA),
		mkTrial(aLeft, condB),
		mkTrial(!aLeft, condB),
	}
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	// -exp sets the default experiment type shown in the dialog.
	cliExpType := flag.String("exp", "preliminary", "default experiment type shown in the dialog")
	flag.Parse()

	expTypes := []string{"preliminary", "exp1", "exp2"}
	defaultExpType := "preliminary"
	for _, e := range expTypes {
		if e == *cliExpType {
			defaultExpType = e
			break
		}
	}

	// ── Setup dialog ──────────────────────────────────────────────────────────
	fields := []control.InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		{
			Name:    "exp_type",
			Label:   "Experiment",
			Default: defaultExpType,
			Type:    control.FieldSelect,
			Options: expTypes,
		},
		{
			Name:    "screen_width_cm",
			Label:   "Projection screen width (cm)",
			Default: "180",
		},
		control.FullscreenField,
		control.DisplayField,
	}

	info, err := control.GetParticipantInfo("Number Change Detection", fields)
	if err != nil {
		log.Fatalf("setup dialog: %v", err)
	}

	subjectID, _ := strconv.Atoi(info["subject_id"])
	expType := info["exp_type"]

	screenWidthCm, err := strconv.ParseFloat(info["screen_width_cm"], 64)
	if err != nil || screenWidthCm < 10 || screenWidthCm > 1000 {
		log.Fatalf("invalid screen_width_cm %q — enter a value between 10 and 1000", info["screen_width_cm"])
	}
	// pxPerCm maps the 1920-px logical canvas to the physical screen width.
	initLayout(float32(1920.0 / screenWidthCm))
	initDotParams(pxPerCm)

	fullscreen := info["fullscreen"] == "true"
	width, height := 0, 0
	if !fullscreen {
		width, height = 1024, 768
	}

	exp := control.NewExperiment("Number Change Detection", width, height, fullscreen, control.White, control.Black, 24)
	exp.SubjectID = subjectID
	exp.ScreenNumber = control.DisplayIDFromInfo(info)
	exp.Info = info
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	if err := exp.SetLogicalSize(1920, 1080); err != nil {
		log.Printf("warning: SetLogicalSize: %v", err)
	}

	trials := buildTrials(expType, subjectID)

	exp.AddDataVariableNames([]string{
		"trial", "exp_type", "param_cond",
		"left_stream", "right_stream", "constant_num",
		"look_left_ms", "look_right_ms",
		"look_stream_a_ms", "look_stream_b_ms", "log_ratio_a_vs_b",
		"left_press_durations_ms", "right_press_durations_ms",
	})

	instr := fmt.Sprintf(
		"Number Change Detection — %s\n\n"+
			"Experimenter controls during each trial:\n"+
			"  LEFT arrow  : infant looking at LEFT stream\n"+
			"  RIGHT arrow : infant looking at RIGHT stream\n"+
			"  (neither)   : infant not looking / looking away\n"+
			"  ESC         : abort session\n\n"+
			"There will be %d trials (~40 s each).\n"+
			"A pulsing red circle appears between trials — press SPACE\n"+
			"once the infant is looking at the centre of the screen.\n\n"+
			"Press SPACE to begin.", expType, nTrials)

	runErr := exp.Run(func() error {
		if err := exp.ShowInstructions(instr); err != nil {
			return err
		}

		var totalA, totalB int64

		for trialIdx, t := range trials {
			// Generate dot arrays before showing the attractor so that
			// the generation time does not eat into the infant's looking window.
			leftArrays := makeStreamArrays(t.leftStream, t.constantNum, t.cond, leftCtr)
			rightArrays := makeStreamArrays(t.rightStream, t.constantNum, t.cond, rightCtr)

			// Show attractor; experimenter presses SPACE when infant attends.
			if err := showAttractor(exp); err != nil {
				return err
			}

			// Present both streams and record looking times.
			lookLeft, lookRight, leftDurs, rightDurs, err := presentStreams(exp, leftArrays, rightArrays)
			if err != nil {
				return err
			}

			// Resolve looking times relative to stream A (primary) vs stream B.
			lookA, lookB := lookLeft, lookRight
			lookDursA, lookDursB := leftDurs, rightDurs
			if !t.streamALeft {
				lookA, lookB = lookRight, lookLeft
				lookDursA, lookDursB = rightDurs, leftDurs
			}
			totalA += lookA
			totalB += lookB

			logRatio := "NA"
			if lookA > 0 && lookB > 0 {
				logRatio = fmt.Sprintf("%.4f", math.Log(float64(lookA)/float64(lookB)))
			}

			exp.Data.Add(
				trialIdx+1, expType, t.cond.String(),
				t.leftStream.String(), t.rightStream.String(), t.constantNum,
				lookLeft, lookRight,
				lookA, lookB, logRatio,
				formatDurations(lookDursA), formatDurations(lookDursB),
			)

			if trialIdx < nTrials-1 {
				if err := exp.Blank(1500); err != nil {
					return err
				}
			}
		}

		// Cumulative log ratio across all 4 trials (primary analysis, paper §2.4).
		cumLogRatio := "NA"
		if totalA > 0 && totalB > 0 {
			cumLogRatio = fmt.Sprintf("%.4f", math.Log(float64(totalA)/float64(totalB)))
		}

		summary := fmt.Sprintf(
			"Session complete — %d trials.\n\n"+
				"Cumulative log ratio  (stream A / stream B): %s\n"+
				"  positive → preference for stream A\n"+
				"  negative → preference for stream B\n\n"+
				"Press any key to exit.", nTrials, cumLogRatio)
		end := stimuli.NewTextBox(summary, 900, control.FPoint{}, control.Black)
		if err := exp.Show(end); err != nil {
			return err
		}
		if _, err := exp.Keyboard.Wait(); err != nil && !control.IsEndLoop(err) {
			return err
		}
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		exp.Fatal("experiment error: %v", runErr)
	}
}
