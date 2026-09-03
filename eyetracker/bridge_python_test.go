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

// TestAgainstPythonBridge runs the real eyelink_bridge.py in --simulate mode
// and drives it through a whole session.
//
// The Go-only tests use a fake server, which proves the client is
// self-consistent and nothing more: the two sides of a protocol written by the
// same hand agree with each other by construction. This one is the only check
// that the Python actually speaks what the Go actually expects, and it is worth
// having because the failure it catches — a renamed field, a number sent as a
// string — shows up otherwise at the tracker, with a participant waiting.
//
// It skips when python3 is absent. It never needs pylink or hardware.
func TestAgainstPythonBridge(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	script, err := filepath.Abs(filepath.Join("bridge", "eyelink_bridge.py"))
	if err != nil || !fileExists(script) {
		t.Skipf("bridge script not found at %s", script)
	}

	port := freePort(t)
	addr := net.JoinHostPort("127.0.0.1", port)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, script, "--simulate", "--port", port)
	// The bridge logs to stderr; surface it only on failure, through the
	// test's own output, so a green run stays quiet.
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

	// Retry Open rather than probing the port with a throwaway connection:
	// the bridge serves one client at a time, so a probe that dials and hangs
	// up is a whole session spent proving the socket exists.
	b := openWithRetry(t, addr, &stderr,
		WithGeometry(Geometry{WidthPx: 1280, HeightPx: 800}),
		WithEDF("gotest.edf"),
	)

	// The bridge must admit it is faking. A simulated run that passes for a
	// real one is the worst outcome available here.
	if !b.Simulated() {
		t.Error("Simulated() = false against a bridge started with --simulate")
	}
	if b.BridgeID() != "eyelink-sim" {
		t.Errorf("BridgeID() = %q, want eyelink-sim", b.BridgeID())
	}

	if err := b.Calibrate(CalibrationOptions{Points: 9}); err != nil {
		t.Errorf("Calibrate: %v", err)
	}

	off, err := b.Sync(5)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if off.Samples != 5 || off.BestRTT <= 0 {
		t.Errorf("Sync gave %d samples, best RTT %v", off.Samples, off.BestRTT)
	}

	if err := b.StartRecording(); err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	if !b.Recording() {
		t.Error("Recording() = false after StartRecording")
	}

	// The simulator runs at 1000 Hz; a tenth of a second is a hundred samples,
	// so requiring ten is generous enough to survive a loaded CI machine.
	waitFor(t, "samples from the Python bridge", func() bool {
		return len(b.samplesLen()) >= 10
	})

	last, ok := b.Latest()
	if !ok {
		t.Fatal("Latest() = !ok although samples arrived")
	}
	if !last.Valid {
		t.Error("simulated sample marked invalid")
	}
	if last.Eye != EyeRight {
		t.Errorf("Eye = %v, want right", last.Eye)
	}
	// The simulator sweeps a Lissajous path over the screen it was told about,
	// so a sample outside that screen means the geometry never arrived.
	g := b.Geometry()
	if !g.Contains(last.X, last.Y) {
		t.Errorf("sample at (%.1f, %.1f) is outside the %dx%d screen the bridge was given",
			last.X, last.Y, g.WidthPx, g.HeightPx)
	}
	if last.TrackerMs <= 0 {
		t.Errorf("TrackerMs = %v, want a positive tracker clock", last.TrackerMs)
	}
	if last.LocalNs == 0 {
		t.Error("LocalNs not stamped")
	}

	if err := b.Mark("TRIALID 1"); err != nil {
		t.Errorf("Mark: %v", err)
	}
	if err := b.StopRecording(); err != nil {
		t.Errorf("StopRecording: %v", err)
	}

	// The simulator writes a placeholder file that says in its first line that
	// it contains no gaze data. Fetching it exercises the whole path.
	edf := filepath.Join(t.TempDir(), "out.edf")
	if err := b.ReceiveDataFile(edf); err != nil {
		t.Fatalf("ReceiveDataFile: %v", err)
	}
	body, err := os.ReadFile(edf)
	if err != nil {
		t.Fatalf("reading the fetched file: %v", err)
	}
	if !strings.Contains(string(body), "SIMULATED") {
		t.Errorf("fetched file does not declare itself simulated: %q", string(body))
	}
	if !strings.Contains(string(body), "TRIALID 1") {
		t.Errorf("the marker did not reach the bridge; file was:\n%s", string(body))
	}

	if err := b.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if b.Connected() {
		t.Error("Connected() = true after Close")
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// freePort asks the kernel for an unused port and releases it. There is a race
// between releasing it here and the bridge binding it, which is why the caller
// retries rather than assuming the port is served.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	defer ln.Close()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("splitting %s: %v", ln.Addr(), err)
	}
	return port
}

// openWithRetry dials the bridge until the interpreter has started and bound
// its port, or the deadline passes. opts are the client options; the timeouts
// and the silent logger are added for every caller.
func openWithRetry(t *testing.T, addr string, stderr *strings.Builder, opts ...Option) *Bridge {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		b := NewBridge(addr, append(opts,
			WithTimeouts(10*time.Second, 10*time.Second),
			WithLogger(func(string, ...any) {}),
		)...)
		if err := b.Open(); err == nil {
			return b
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("could not open the bridge at %s: %v\nbridge stderr:\n%s", addr, lastErr, stderr.String())
	return nil
}
