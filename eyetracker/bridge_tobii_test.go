// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package eyetracker

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAgainstTobiiBridge runs the real tobii_bridge.py in --simulate mode and
// drives it through a whole session, calibration included.
//
// Like TestAgainstPythonBridge, this is the only check that the Python and the
// Go agree: the fake-server tests prove the client is self-consistent, which
// two sides written by the same hand are by construction. The failures this
// catches are a renamed field, a number sent as a string — and, specifically
// for this bridge, a nan reaching the wire, which is not a Go-side concern at
// all but kills the connection.
//
// It skips when python3 is absent. It never needs the Tobii SDK or hardware.
func TestAgainstTobiiBridge(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	script, err := filepath.Abs(filepath.Join("bridge", "tobii_bridge.py"))
	if err != nil || !fileExists(script) {
		t.Skipf("bridge script not found at %s", script)
	}

	dir := t.TempDir()
	port := freePort(t)
	addr := net.JoinHostPort("127.0.0.1", port)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// --edf-dir keeps the simulator's gaze file out of the source tree.
	cmd := exec.CommandContext(ctx, python, script,
		"--simulate", "--port", port, "--edf-dir", dir, "--rate", "500")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the bridge: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
		if t.Failed() && stderr.Len() > 0 {
			t.Logf("bridge stderr:\n%s", stderr.String())
		}
	})

	const w, h = 1920, 1080
	b := openWithRetry(t, addr, &stderr,
		WithGeometry(Geometry{WidthPx: w, HeightPx: h}),
		WithEDF("gotest_tobii.tsv"),
	)

	if !b.Simulated() {
		t.Error("Simulated() = false against a bridge started with --simulate")
	}
	if b.BridgeID() != "tobii-sim" {
		t.Errorf("BridgeID() = %q, want tobii-sim", b.BridgeID())
	}

	// The plain Calibrate must FAIL here, naming the alternative. A Tobii that
	// silently reported a calibration it never ran would be the worst outcome
	// available: the run looks normal until the gaze is analysed.
	if err := b.Calibrate(CalibrationOptions{Points: 9}); err == nil {
		t.Error("Calibrate() succeeded on a Tobii bridge; it draws no targets " +
			"and must refuse")
	} else if !strings.Contains(err.Error(), "CalibrateTracker") {
		t.Errorf("Calibrate() error does not name the alternative: %v", err)
	}

	// The stepwise path, which is what control.Experiment.CalibrateTracker
	// drives. Five points rather than nine keeps the test quick; the simulator
	// sleeps in collect_data on purpose.
	if err := b.CalibrationEnter(); err != nil {
		t.Fatalf("CalibrationEnter: %v", err)
	}
	points := StandardPoints(5)
	if len(points) != 5 {
		t.Fatalf("StandardPoints(5) returned %d points", len(points))
	}
	for _, p := range points {
		if err := b.CalibrationCollect(p[0], p[1]); err != nil {
			t.Fatalf("CalibrationCollect(%.2f, %.2f): %v", p[0], p[1], err)
		}
	}
	res, err := b.CalibrationCompute()
	if err != nil {
		t.Fatalf("CalibrationCompute: %v", err)
	}
	if !res.OK() {
		t.Errorf("CalibrationCompute status = %q, want a success", res.Status)
	}
	if len(res.Points) != len(points) {
		t.Errorf("CalibrationCompute returned %d points, want %d",
			len(res.Points), len(points))
	}
	// The counts must survive the JSON round trip as numbers. They decode into
	// float64, so an int type assertion on the Go side would silently yield
	// zero for every one of them — which is exactly what this asserts against.
	for i, p := range res.Points {
		if p.Samples == 0 || p.Used == 0 {
			t.Errorf("point %d: Samples=%d Used=%d, both should be nonzero",
				i, p.Samples, p.Used)
		}
	}
	if _, mono := res.Monocular(); mono {
		t.Errorf("Monocular() true for status %q", res.Status)
	}
	if msg, verified := b.CalibrationMessage(); !verified || msg == "" {
		t.Errorf("CalibrationMessage() = (%q, %v) after a successful compute", msg, verified)
	}
	if err := b.CalibrationLeave(); err != nil {
		t.Errorf("CalibrationLeave: %v", err)
	}

	if err := b.StartRecording(); err != nil {
		t.Fatalf("StartRecording: %v", err)
	}

	// The simulator runs at 500 Hz, emits BOTH eyes, and blinks for 40 ms in
	// every 400 ms. Waiting for 400 samples therefore covers a whole blink
	// cycle, which is the point: the invalid-sample path has to be exercised
	// over the real socket, not just asserted about.
	waitFor(t, "samples from the Tobii bridge", func() bool {
		return len(b.samplesLen()) >= 400
	})

	samples := b.DrainSamples()
	if len(samples) < 400 {
		t.Fatalf("DrainSamples returned %d samples", len(samples))
	}

	// Both eyes must appear. A one-eyed stream would mean the per-eye split
	// silently collapsed, which is the mistake this bridge is most likely to
	// make and is invisible in a single sample.
	var left, right, valid, invalid int
	g := b.Geometry()
	for _, s := range samples {
		switch s.Eye {
		case EyeLeft:
			left++
		case EyeRight:
			right++
		default:
			t.Fatalf("sample with eye %v; the Tobii bridge must name the eye", s.Eye)
		}
		if !s.Valid {
			invalid++
			continue
		}
		valid++
		if !g.Contains(s.X, s.Y) {
			t.Errorf("sample at (%.1f, %.1f) is outside the %dx%d screen",
				s.X, s.Y, g.WidthPx, g.HeightPx)
		}
		// pa is pupil DIAMETER in millimetres on this bridge, not the
		// EyeLink's arbitrary area units. A four-figure value here would mean
		// the two conventions had been confused somewhere on the way.
		if s.PupilArea < 1 || s.PupilArea > 9 {
			t.Errorf("PupilArea = %v; expected a pupil diameter in mm", s.PupilArea)
		}
		if s.TrackerMs <= 0 {
			t.Errorf("TrackerMs = %v, want a positive tracker clock", s.TrackerMs)
		}
		if s.LocalNs == 0 {
			t.Error("LocalNs not stamped")
		}
	}
	if left == 0 || right == 0 {
		t.Errorf("got %d left and %d right samples; both eyes must be reported",
			left, right)
	}
	if valid == 0 {
		t.Error("no valid samples at all")
	}
	// The blink. If this is zero the nan path never crossed the socket, and
	// the test proves nothing about the failure it was written for.
	if invalid == 0 {
		t.Error("no invalid samples: the simulated blink never reached the " +
			"client, so the nan-on-the-wire path is untested")
	}
	t.Logf("%d samples: %d left, %d right, %d valid, %d invalid",
		len(samples), left, right, valid, invalid)

	if err := b.Mark("TRIALID 1"); err != nil {
		t.Errorf("Mark: %v", err)
	}
	if err := b.StopRecording(); err != nil {
		t.Errorf("StopRecording: %v", err)
	}
	if b.Dropped() != 0 {
		t.Errorf("Dropped() = %d; the test drains fast enough that it should be 0",
			b.Dropped())
	}

	// Tobii has no tracker-side file: the bridge wrote this one itself, so
	// fetching it exercises the whole recording path rather than a transfer.
	out := filepath.Join(dir, "fetched.tsv")
	if err := b.ReceiveDataFile(out); err != nil {
		t.Fatalf("ReceiveDataFile: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading the fetched file: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"SIMULATED",   // it must never pass for a real recording
		"TRIALID 1",   // the marker reached the file, in order
		"pupil_units", // the unit is recorded, because nothing else states it
		"screen_width_px",
		"left_gaze_x_px",
		"right_gaze_x_px",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("fetched gaze file lacks %q", want)
		}
	}
	// Header lines plus a few hundred data rows.
	if n := strings.Count(text, "\n"); n < 100 {
		t.Errorf("fetched gaze file has only %d lines", n)
	}
	// Missing data must be an empty field, not the string "nan": an empty
	// field reads as missing in every analysis package, "nan" does not.
	if strings.Contains(text, "nan") || strings.Contains(text, "NaN") {
		t.Error("the gaze file writes nan for missing data; it must leave the field empty")
	}

	if err := b.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if b.Connected() {
		t.Error("Connected() = true after Close")
	}
}

// TestStandardPoints checks the target layouts, which are easy to get subtly
// wrong and impossible to notice at a rig.
func TestStandardPoints(t *testing.T) {
	for _, n := range []int{3, 5, 9, 13} {
		pts := StandardPoints(n)
		if len(pts) != n {
			t.Errorf("StandardPoints(%d) returned %d points", n, len(pts))
		}
		if pts[0] != [2]float64{0.5, 0.5} {
			t.Errorf("StandardPoints(%d) does not start at the centre: %v", n, pts[0])
		}
		seen := map[[2]float64]bool{}
		for _, p := range pts {
			if p[0] < 0 || p[0] > 1 || p[1] < 0 || p[1] > 1 {
				t.Errorf("StandardPoints(%d): %v is not normalized", n, p)
			}
			if seen[p] {
				t.Errorf("StandardPoints(%d): %v appears twice", n, p)
			}
			seen[p] = true
		}
	}
	// An unknown count must give a usable layout rather than nothing.
	if len(StandardPoints(7)) != 9 {
		t.Error("StandardPoints(7) should fall back to 9")
	}
}

// TestCalibrationResultSummary checks that a partial calibration reports
// itself. A run that quietly proceeds on one eye and is later analysed as
// binocular is the failure these accessors exist to prevent.
func TestCalibrationResultSummary(t *testing.T) {
	r := CalibrationResult{
		Status: "calibration_status_success_left_eye",
		Points: []CalibrationPoint{
			{NX: 0.5, NY: 0.5, Samples: 30, Used: 60},
			{NX: 0.1, NY: 0.1, Samples: 30, Used: 0},
		},
	}
	if !r.OK() {
		t.Error("OK() = false for a monocular success")
	}
	eye, mono := r.Monocular()
	if !mono || eye != EyeLeft {
		t.Errorf("Monocular() = (%v, %v), want (left, true)", eye, mono)
	}
	s := r.Summary()
	for _, want := range []string{"LEFT EYE ONLY", "no usable data at", "(0.10,0.10)"} {
		if !strings.Contains(s, want) {
			t.Errorf("Summary() = %q, want it to contain %q", s, want)
		}
	}

	bad := CalibrationResult{Status: "calibration_status_failure"}
	if bad.OK() {
		t.Error("OK() = true for a failure")
	}
}
