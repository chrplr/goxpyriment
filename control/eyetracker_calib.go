// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"fmt"
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/eyetracker"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Calibration target geometry, in pixels. An outer disc with a small dot at its
// centre is fixated more precisely than a plain dot of either size: the disc is
// easy to find in peripheral vision, and the dot gives the eye something small
// to settle on once found.
const (
	calTargetOuterRadius = 16
	calTargetInnerRadius = 3
	calDefaultDwellMs    = 700
)

// calTarget is a calibration target: a filled disc with a contrasting dot at
// its centre. It is a single VisualStimulus so that it reaches the screen in
// one flip, which is the point of drawing calibration here — a target split
// across two flips has no single onset to report.
type calTarget struct {
	stimuli.BaseVisual
	Outer, Inner sdl.Color
}

func (t *calTarget) Draw(screen *apparatus.Screen) error {
	outer := stimuli.NewCircle(calTargetOuterRadius, t.Outer)
	outer.SetPosition(t.Position)
	if err := outer.Draw(screen); err != nil {
		return err
	}
	inner := stimuli.NewCircle(calTargetInnerRadius, t.Inner)
	inner.SetPosition(t.Position)
	return inner.Draw(screen)
}

func (t *calTarget) Present(screen *apparatus.Screen, clear, update bool) error {
	return stimuli.PresentDrawable(t, screen, clear, update)
}

// CalibrateTracker calibrates a gaze tracker, drawing the targets itself when
// the tracker needs that.
//
// It handles both shapes of calibration API, so an experiment does not need to
// know which make of tracker it is talking to:
//
//   - A tracker that runs its own setup routine — an EyeLink through pylink —
//     is simply asked to do it. The Host PC's calibration screen is better
//     tested than anything we can draw.
//   - A tracker that implements [eyetracker.StepwiseCalibrator] — a Tobii,
//     whose SDK draws nothing at all — is driven target by target from here.
//     Each target is put on screen with [Experiment.ShowTS], held for
//     opts.DwellMs, and only then does the tracker sample it. That ordering is
//     the whole reason to do the drawing here: the target onset is on
//     goxpyriment's own flip clock, in goxpyriment's own window, on one
//     machine.
//
// It BLOCKS, for as long as the calibration takes — a target the tracker is
// sampling can hold for several seconds. Never call it inside a frame loop.
//
// Pressing Esc during a calibration ends the RUN, as it does everywhere else in
// goxpyriment (see [Experiment.Wait]); the tracker is taken out of calibration
// mode on the way out regardless, because one left in it will not record.
//
// The result goes into the run's -info.txt, including any target that yielded
// no usable data — that names the corner the participant never looked at, which
// is not otherwise recoverable after the session.
func (e *Experiment) CalibrateTracker(t eyetracker.Tracker,
	opts eyetracker.CalibrationOptions) error {

	if opts.Skip {
		log.Printf("control: calibration skipped by request; any gaze data " +
			"from this run is uncalibrated")
		e.AddExperimentInfo("eyetracker calibration: SKIPPED")
		return nil
	}

	sc, ok := t.(eyetracker.StepwiseCalibrator)
	if !ok {
		// The tracker draws its own targets. Nothing for us to do but ask.
		return t.Calibrate(opts)
	}
	return e.calibrateStepwise(t, sc, opts)
}

// calibrateStepwise drives a calibration whose targets we have to draw.
func (e *Experiment) calibrateStepwise(t eyetracker.Tracker,
	sc eyetracker.StepwiseCalibrator, opts eyetracker.CalibrationOptions) error {

	w, h, err := e.Screen.Size()
	if err != nil {
		return fmt.Errorf("control.CalibrateTracker: reading the screen size: %w", err)
	}
	// The tracker reports gaze against the screen it was TOLD about at open,
	// so the same geometry has to place the targets. Taking it from the Screen
	// instead would silently shift and scale every subsequent gaze position if
	// the two ever disagreed.
	geom := eyetracker.Geometry{WidthPx: int(w), HeightPx: int(h)}
	if b, ok := t.(*eyetracker.Bridge); ok {
		if g := b.Geometry(); g.WidthPx > 0 && g.HeightPx > 0 {
			geom = g
		}
	}

	dwell := opts.DwellMs
	if dwell <= 0 {
		dwell = calDefaultDwellMs
	}
	points := eyetracker.StandardPoints(opts.Points)

	if err := sc.CalibrationEnter(); err != nil {
		return fmt.Errorf("control.CalibrateTracker: entering calibration mode: %w", err)
	}
	// Unconditional: a tracker left in calibration mode will not record, and
	// the paths out of here include an Esc that unwinds through a panic.
	defer func() {
		if err := sc.CalibrationLeave(); err != nil {
			log.Printf("control: leaving calibration mode: %v", err)
		}
	}()

	target := &calTarget{Outer: White, Inner: Black}
	onsets := make([]uint64, 0, len(points))

	for i, p := range points {
		// Normalized display area (origin TOP-LEFT, +Y down) -> tracker pixels
		// -> goxpyriment's centre-origin, +Y-UP coordinates. The Y flip in
		// ToCentre is the whole reason to go through Geometry: placing the
		// target at the raw pixel Y mirrors the calibration vertically, and
		// every gaze position afterwards is then wrong in a way that reads as
		// a bad calibration rather than a units bug.
		cx, cy := geom.ToCentre(p[0]*float64(geom.WidthPx), p[1]*float64(geom.HeightPx))
		target.SetPosition(sdl.FPoint{X: float32(cx), Y: float32(cy)})

		onset, err := e.ShowTS(target)
		if err != nil {
			return fmt.Errorf("control.CalibrateTracker: showing target %d: %w", i+1, err)
		}
		onsets = append(onsets, onset)

		// Time for a saccade to the target and a stable fixation on it. Wait
		// also pumps SDL and honours Esc, so the operator keeps control of a
		// procedure that is otherwise several seconds per point.
		if err := e.Wait(dwell); err != nil {
			return err
		}

		// The tracker blocks here while it samples.
		if err := sc.CalibrationCollect(p[0], p[1]); err != nil {
			// A refused point is the tracker saying it saw nothing usable,
			// which usually means the participant was not looking at it.
			// Discard whatever it did keep and offer the point once more
			// rather than either abandoning the session or looping forever.
			log.Printf("control: target %d/%d at (%.2f, %.2f) was refused (%v); retrying once",
				i+1, len(points), p[0], p[1], err)
			if err := sc.CalibrationDiscard(p[0], p[1]); err != nil {
				log.Printf("control: discarding target %d: %v", i+1, err)
			}
			if _, err := e.ShowTS(target); err != nil {
				return fmt.Errorf("control.CalibrateTracker: reshowing target %d: %w", i+1, err)
			}
			if err := e.Wait(dwell); err != nil {
				return err
			}
			if err := sc.CalibrationCollect(p[0], p[1]); err != nil {
				// Still nothing. Carry on: the tracker can compute a
				// calibration from the points it did get, and CalibrationCompute
				// reports which ones contributed nothing.
				log.Printf("control: target %d/%d gave no usable data", i+1, len(points))
			}
		}
	}

	if err := e.Blank(200); err != nil {
		return err
	}

	res, err := sc.CalibrationCompute()
	// Record the result before acting on the error: a failed calibration is
	// exactly the one whose per-target counts are worth having in the file.
	e.AddExperimentInfo("eyetracker calibration: " + res.Summary())
	e.AddExperimentInfo(fmt.Sprintf(
		"eyetracker calibration targets: %d, dwell %d ms, first onset %d ns (SDL clock)",
		len(points), dwell, firstOnset(onsets)))
	if err != nil {
		return fmt.Errorf("control.CalibrateTracker: %w", err)
	}
	if eye, mono := res.Monocular(); mono {
		log.Printf("control: WARNING — only the %s eye was calibrated; "+
			"this run must not be analysed as binocular", eye)
	}
	log.Printf("control: calibration %s", res.Summary())
	return nil
}

func firstOnset(onsets []uint64) uint64 {
	if len(onsets) == 0 {
		return 0
	}
	return onsets[0]
}
