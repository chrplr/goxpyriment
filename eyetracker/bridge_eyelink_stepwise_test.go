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

// startEyeLinkBridge runs eyelink_bridge.py with --simulate and the extra
// arguments, and opens a client on it. It skips without python3.
func startEyeLinkBridge(t *testing.T, extra ...string) *Bridge {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	script, err := filepath.Abs(filepath.Join("bridge", "eyelink_bridge.py"))
	if err != nil || !fileExists(script) {
		t.Skipf("bridge script not found at %s", script)
	}
	port := freePort(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	args := append([]string{script, "--simulate", "--port", port}, extra...)
	cmd := exec.CommandContext(ctx, python, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("starting the bridge: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
		if t.Failed() && stderr.Len() > 0 {
			t.Logf("bridge stderr:\n%s", stderr.String())
		}
	})
	b := openWithRetry(t, net.JoinHostPort("127.0.0.1", port), &stderr,
		WithGeometry(Geometry{WidthPx: 1280, HeightPx: 800}))
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// TestEyeLinkBridgeStepwise drives the simulated EyeLink through a stepwise
// calibration the way control.CalibrateTracker does. The simulator sequences
// the targets as the Host does, so it also checks that the point list reaches
// the bridge and that the Host's order is enforced.
func TestEyeLinkBridgeStepwise(t *testing.T) {
	b := startEyeLinkBridge(t)
	if !b.SupportsStepwiseCalibration() {
		t.Fatalf("the EyeLink bridge does not advertise stepwise calibration; caps %v",
			b.Capabilities())
	}

	// Without the target list the Host cannot know where the targets are.
	if err := b.CalibrationEnter(); err == nil {
		t.Error("CalibrationEnter without points succeeded")
	}

	// HV9 and HV13 only.
	b.SetCalibrationPoints(StandardPoints(5))
	if err := b.CalibrationEnter(); err == nil {
		t.Error("CalibrationEnter with 5 points succeeded; an EyeLink has no HV slot layout for them")
	}

	pts := StandardPoints(9)
	b.SetCalibrationPoints(pts)
	if err := b.CalibrationEnter(); err != nil {
		t.Fatalf("CalibrationEnter: %v", err)
	}
	if err := b.CalibrationCollect(pts[1][0], pts[1][1]); err == nil {
		t.Error("collecting the second target first succeeded; the Host is waiting for the first")
	}
	for i, p := range pts {
		if err := b.CalibrationCollect(p[0], p[1]); err != nil {
			t.Fatalf("CalibrationCollect target %d: %v", i+1, err)
		}
	}
	res, err := b.CalibrationCompute()
	if err != nil {
		t.Fatalf("CalibrationCompute: %v", err)
	}
	if !res.OK() || len(res.Points) != len(pts) {
		t.Errorf("result %+v, want success with %d points", res, len(pts))
	}
	if !strings.Contains(res.Message, "SIMULATED") {
		t.Errorf("Message = %q, want the simulator's own summary", res.Message)
	}
	if !strings.Contains(res.Summary(), "SIMULATED") {
		t.Errorf("Summary() = %q does not carry the tracker's message", res.Summary())
	}
	if err := b.CalibrationLeave(); err != nil {
		t.Errorf("CalibrationLeave: %v", err)
	}
}

// TestEyeLinkBridgeNoStepwise checks the escape hatch: with --no-stepwise the
// bridge advertises nothing, so control.CalibrateTracker falls back to the
// tracker's own setup routine.
func TestEyeLinkBridgeNoStepwise(t *testing.T) {
	b := startEyeLinkBridge(t, "--no-stepwise")
	if b.SupportsStepwiseCalibration() {
		t.Errorf("SupportsStepwiseCalibration() = true with --no-stepwise; caps %v",
			b.Capabilities())
	}
	b.SetCalibrationPoints(StandardPoints(9))
	if err := b.CalibrationEnter(); err == nil {
		t.Error("CalibrationEnter succeeded with --no-stepwise")
	}
	if err := b.Calibrate(CalibrationOptions{Points: 9}); err != nil {
		t.Errorf("Calibrate: %v", err)
	}
}

// TestEyeLinkRelayAgainstFakePylink runs the REAL EyeLinkTracker's stepwise
// code — the relay display, the setup thread, the key queue — against a fake
// pylink that plays the Host's setup routine (testdata/fakepylink). The fake
// is a model of the Host; `eyelink_bridge.py --check-calibration` on the rig
// is what checks the model.
func TestEyeLinkRelayAgainstFakePylink(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	bridgeDir, _ := filepath.Abs("bridge")
	fakeDir, _ := filepath.Abs(filepath.Join("testdata", "fakepylink"))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, filepath.Join(fakeDir, "check_stepwise.py"))
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+fakeDir+string(os.PathListSeparator)+bridgeDir,
		"PYTHONDONTWRITEBYTECODE=1")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "OK") {
		t.Fatalf("check_stepwise.py: %v\n%s", err, out)
	}
}
