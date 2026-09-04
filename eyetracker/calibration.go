// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package eyetracker

import (
	"fmt"
	"strings"
)

// StepwiseCalibrationReporter is implemented by a tracker that can only answer
// at RUNTIME whether stepwise calibration is available — a [Bridge], whose Go
// type satisfies [StepwiseCalibrator] whatever tracker is on the far side of
// the socket.
//
// A caller that has a [StepwiseCalibrator] must ask this before using it, when
// the value implements it: a type assertion alone reports what the Go type can
// do, and the question is what the hardware can do.
type StepwiseCalibrationReporter interface {
	SupportsStepwiseCalibration() bool
}

// StepwiseCalibrator is implemented by a tracker whose SDK draws no calibration
// graphics, so the CLIENT has to put each target on screen itself and tell the
// tracker when to sample it.
//
// A Tobii is like this: its SDK exposes only "collect data for the point the
// participant is looking at now" and draws nothing. An EyeLink is not — pylink
// runs the whole setup itself behind [Tracker.Calibrate], which is why that
// method still exists and why this interface is separate from it.
//
// The payoff of driving it here is that the target onset is on goxpyriment's
// own flip clock, in goxpyriment's own window, on one machine. Use
// control.Experiment.CalibrateTracker rather than these methods directly: this
// package cannot draw anything, because it also has to build for the browser.
//
// # Coordinates
//
// NX and NY are NORMALIZED on the tracker's active display area: (0,0) is the
// TOP-LEFT corner, (1,1) the bottom-right. They are neither pixels nor
// goxpyriment's centre-origin, +Y-up convention. Multiply by the screen size to
// get the tracker pixels the rest of this package speaks, then convert with
// [Geometry.ToCentre] to position a stimulus. Skipping the flip mirrors the
// calibration vertically, and every gaze position afterwards is wrong in a way
// that looks like a bad calibration rather than a units bug.
type StepwiseCalibrator interface {
	// CalibrationEnter puts the tracker into calibration mode. Nothing else
	// here may be called before it.
	CalibrationEnter() error

	// CalibrationCollect samples the participant's gaze for the target at
	// normalized (nx, ny), which must ALREADY be on screen and fixated.
	//
	// It BLOCKS while the tracker collects — up to about 10 seconds — so it
	// must never be called from a frame loop. An error means the tracker
	// refused the point, not that the link failed; the caller decides whether
	// to discard and retry it, because only the caller knows what is on screen.
	CalibrationCollect(nx, ny float64) error

	// CalibrationDiscard throws away the data collected for one point, so it
	// can be collected again.
	CalibrationDiscard(nx, ny float64) error

	// CalibrationCompute computes a calibration from the collected points and
	// applies it to the tracker.
	CalibrationCompute() (CalibrationResult, error)

	// CalibrationLeave leaves calibration mode. Call it even on the error
	// paths: a tracker left in calibration mode will not record.
	CalibrationLeave() error
}

// CalibrationResult is what the tracker computed from the collected points.
type CalibrationResult struct {
	// Status is the tracker's own verdict, verbatim — for a Tobii one of
	// "calibration_status_success", "..._success_left_eye",
	// "..._success_right_eye" or "calibration_status_failure".
	Status string

	// Points is one entry per target the tracker kept data for, in its order.
	Points []CalibrationPoint
}

// CalibrationPoint is one calibration target and how much usable data the
// tracker got from it.
type CalibrationPoint struct {
	NX, NY float64 // normalized display-area coordinates, origin top-left

	// Samples is how many samples the tracker collected for this point, and
	// Used how many of them it actually used in the fit, counted across both
	// eyes. A point with Samples > 0 and Used == 0 is one the participant was
	// not looking at.
	Samples int
	Used    int
}

// OK reports whether the tracker stored a usable calibration.
//
// It is true for a monocular success too — the tracker calibrated one eye and
// says so — because that is still a calibration, and rejecting it would stop a
// session that can legitimately continue with one eye. Use
// [CalibrationResult.Monocular] to notice and record it.
func (r CalibrationResult) OK() bool {
	return strings.HasPrefix(r.Status, "calibration_status_success")
}

// Monocular reports whether only one eye was calibrated, and which. A run that
// silently proceeds on one eye and is later analysed as binocular is the
// failure this exists to make visible.
func (r CalibrationResult) Monocular() (Eye, bool) {
	switch r.Status {
	case "calibration_status_success_left_eye":
		return EyeLeft, true
	case "calibration_status_success_right_eye":
		return EyeRight, true
	}
	return EyeUnknown, false
}

// Summary is a one-line description fit for a log line or an -info.txt entry,
// naming any target that yielded no usable data.
func (r CalibrationResult) Summary() string {
	var bad []string
	total, used := 0, 0
	for _, p := range r.Points {
		total += p.Samples
		used += p.Used
		if p.Used == 0 {
			bad = append(bad, fmt.Sprintf("(%.2f,%.2f)", p.NX, p.NY))
		}
	}
	s := fmt.Sprintf("%s: %d points, %d samples, %d used",
		r.Status, len(r.Points), total, used)
	if eye, mono := r.Monocular(); mono {
		s += fmt.Sprintf("; %s EYE ONLY", strings.ToUpper(eye.String()))
	}
	if len(bad) > 0 {
		s += "; no usable data at " + strings.Join(bad, " ")
	}
	return s
}

// StandardPoints returns a calibration target layout in normalized display-area
// coordinates, origin top-left.
//
// n may be 3, 5, 9 or 13; anything else falls back to 9, which is the usual
// choice. The centre comes first and the rest work outwards, because a tracker
// that has not yet found the eyes does better starting where they already are.
//
// The targets are inset to 0.1/0.9 rather than placed at 0/1: a target at the
// very edge of the display area is partly off-screen on most monitors, and a
// participant cannot fixate the middle of something they can only half see.
func StandardPoints(n int) [][2]float64 {
	const lo, mid, hi = 0.1, 0.5, 0.9
	switch n {
	case 3:
		return [][2]float64{{mid, mid}, {lo, mid}, {hi, mid}}
	case 5:
		return [][2]float64{
			{mid, mid}, {lo, lo}, {hi, lo}, {lo, hi}, {hi, hi},
		}
	case 13:
		const q1, q3 = 0.3, 0.7
		return [][2]float64{
			{mid, mid},
			{lo, lo}, {hi, lo}, {lo, hi}, {hi, hi},
			{mid, lo}, {lo, mid}, {hi, mid}, {mid, hi},
			{q1, q1}, {q3, q1}, {q1, q3}, {q3, q3},
		}
	default: // 9
		return [][2]float64{
			{mid, mid},
			{lo, lo}, {hi, lo}, {lo, hi}, {hi, hi},
			{mid, lo}, {lo, mid}, {hi, mid}, {mid, hi},
		}
	}
}
