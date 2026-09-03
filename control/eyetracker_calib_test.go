// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"errors"
	"strings"
	"testing"

	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/eyetracker"
)

// plainTracker runs its own calibration, the way an EyeLink does through
// pylink. It deliberately does NOT implement eyetracker.StepwiseCalibrator.
type plainTracker struct {
	eyetracker.NullTracker
	calls int
	opts  eyetracker.CalibrationOptions
	err   error
}

func (p *plainTracker) Calibrate(opts eyetracker.CalibrationOptions) error {
	p.calls++
	p.opts = opts
	return p.err
}

// TestCalibrateTrackerDelegatesToTheTracker covers the branch that keeps the
// EyeLink working: a tracker that draws its own targets must simply be asked
// to, and must NOT be driven target by target.
//
// It runs without a display because both paths it exercises return before any
// SDL call. The stepwise path cannot be tested here — it draws — and is
// covered end to end by eyetracker.TestAgainstTobiiBridge.
func TestCalibrateTrackerDelegatesToTheTracker(t *testing.T) {
	exp := &Experiment{}
	tr := &plainTracker{}

	opts := eyetracker.CalibrationOptions{Points: 13, DwellMs: 500}
	if err := exp.CalibrateTracker(tr, opts); err != nil {
		t.Fatalf("CalibrateTracker: %v", err)
	}
	if tr.calls != 1 {
		t.Errorf("Calibrate called %d times, want 1", tr.calls)
	}
	if tr.opts.Points != 13 {
		t.Errorf("Points = %d, want 13 passed through unchanged", tr.opts.Points)
	}

	// A failure from the tracker's own routine must reach the caller: a
	// session recorded against a calibration that never happened looks
	// entirely normal until the gaze is analysed.
	want := errors.New("the operator left setup without calibrating")
	tr.err = want
	if err := exp.CalibrateTracker(tr, opts); !errors.Is(err, want) {
		t.Errorf("CalibrateTracker returned %v, want the tracker's own error", err)
	}
}

// TestCalibrateTrackerSkip checks that Skip does nothing and says so. It
// exists so a script can run end-to-end without a participant, and the run
// must be identifiable afterwards as uncalibrated.
func TestCalibrateTrackerSkip(t *testing.T) {
	exp := &Experiment{Design: design.NewExperiment("test")}
	tr := &plainTracker{}

	if err := exp.CalibrateTracker(tr, eyetracker.CalibrationOptions{Skip: true}); err != nil {
		t.Fatalf("CalibrateTracker with Skip: %v", err)
	}
	if tr.calls != 0 {
		t.Errorf("Calibrate was called %d times despite Skip", tr.calls)
	}

	// Skip must be recorded, not silent.
	var found bool
	for _, line := range exp.Design.ExperimentInfo {
		if strings.Contains(line, "SKIPPED") {
			found = true
		}
	}
	if !found {
		t.Errorf("a skipped calibration left no note in the experiment info: %v",
			exp.Design.ExperimentInfo)
	}
}
