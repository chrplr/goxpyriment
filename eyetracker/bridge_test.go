// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package eyetracker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeBridge is an in-process stand-in for eyelink_bridge.py. It speaks the
// protocol well enough to drive the client through every path, so the client
// can be tested without Python, without hardware, and without a network.
type fakeBridge struct {
	t    *testing.T
	ln   net.Listener
	mu   sync.Mutex
	conn net.Conn
	cmds []string

	proto     int
	simulated bool
	// failCmd, when set, is answered with ok:false.
	failCmd string
	// trackerMs is returned by tracker_time, and advances by tickMs per call
	// so that Sync sees a moving clock.
	trackerMs float64
	tickMs    float64
	// noHello suppresses the greeting, to test the Open timeout path.
	noHello bool
	// caps, when non-nil, is announced in hello as the optional commands this
	// back end implements. Left nil, hello omits the field, which is how a
	// bridge older than that field greets a current client.
	caps []string
}

func newFakeBridge(t *testing.T) *fakeBridge {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeBridge{t: t, ln: ln, proto: protoVersion, trackerMs: 1000}
	t.Cleanup(func() { ln.Close() })
	go f.accept()
	return f
}

func (f *fakeBridge) addr() string { return f.ln.Addr().String() }

func (f *fakeBridge) accept() {
	conn, err := f.ln.Accept()
	if err != nil {
		return
	}
	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()

	if !f.noHello {
		hello := map[string]any{
			"ev": "hello", "bridge": "fake", "proto": f.proto, "simulated": f.simulated,
		}
		if f.caps != nil {
			hello["caps"] = f.caps
		}
		f.send(hello)
	}

	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		var req request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			continue
		}
		f.mu.Lock()
		f.cmds = append(f.cmds, req.Cmd)
		f.mu.Unlock()

		if req.Cmd == f.failCmd {
			f.send(map[string]any{"id": req.ID, "ok": false, "error": "refused on purpose"})
			continue
		}
		result := map[string]any{}
		if req.Cmd == "tracker_time" {
			f.mu.Lock()
			f.trackerMs += f.tickMs
			result["time"] = f.trackerMs
			f.mu.Unlock()
		}
		f.send(map[string]any{"id": req.ID, "ok": true, "result": result})
	}
}

func (f *fakeBridge) send(obj map[string]any) {
	f.mu.Lock()
	conn := f.conn
	f.mu.Unlock()
	if conn == nil {
		return
	}
	line, _ := json.Marshal(obj)
	conn.Write(append(line, '\n'))
}

// sendSample pushes one unsolicited sample event.
func (f *fakeBridge) sendSample(t float64, x, y float64) {
	f.send(map[string]any{"ev": "sample", "t": t, "eye": "right", "x": x, "y": y, "pa": 1200.0})
}

func (f *fakeBridge) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.cmds))
	copy(out, f.cmds)
	return out
}

// waitFor polls cond until it holds or the deadline passes. Sample delivery is
// asynchronous, so a bare assertion after a send is a flake waiting to happen.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func openTestBridge(t *testing.T, f *fakeBridge, opts ...Option) *Bridge {
	t.Helper()
	all := append([]Option{
		WithGeometry(Geometry{WidthPx: 1920, HeightPx: 1080}),
		WithLogger(func(string, ...any) {}),
		WithTimeouts(2*time.Second, 2*time.Second),
	}, opts...)
	b := NewBridge(f.addr(), all...)
	if err := b.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { b.hardClose() })
	return b
}

func TestBridgeOpenSendsGeometry(t *testing.T) {
	f := newFakeBridge(t)
	b := openTestBridge(t, f)

	if !b.Connected() {
		t.Error("Connected() = false after Open")
	}
	if got := b.commandsSeen(f); len(got) == 0 || got[0] != "open" {
		t.Errorf("first command = %v, want open", got)
	}
	if b.Simulated() {
		t.Error("Simulated() = true, fake bridge reported false")
	}
	if b.BridgeID() != "fake" {
		t.Errorf("BridgeID() = %q, want fake", b.BridgeID())
	}
}

// commandsSeen is a small helper so the test reads in one direction.
func (b *Bridge) commandsSeen(f *fakeBridge) []string { return f.commands() }

func TestBridgeRejectsProtocolMismatch(t *testing.T) {
	f := newFakeBridge(t)
	f.proto = protoVersion + 1
	b := NewBridge(f.addr(), WithLogger(func(string, ...any) {}),
		WithTimeouts(2*time.Second, 2*time.Second))
	err := b.Open()
	if err == nil {
		t.Fatal("Open succeeded against a bridge speaking a different protocol")
	}
	if b.Connected() {
		t.Error("Connected() = true after a failed Open")
	}
}

func TestBridgeOpenTimesOutWithoutHello(t *testing.T) {
	f := newFakeBridge(t)
	f.noHello = true
	b := NewBridge(f.addr(), WithLogger(func(string, ...any) {}),
		WithTimeouts(200*time.Millisecond, 200*time.Millisecond))
	start := time.Now()
	if err := b.Open(); err == nil {
		t.Fatal("Open succeeded although the bridge never greeted")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Open blocked for %v, want the dial timeout", elapsed)
	}
}

func TestBridgeSurfacesCommandFailure(t *testing.T) {
	f := newFakeBridge(t)
	f.failCmd = "start_recording"
	b := openTestBridge(t, f)

	err := b.StartRecording()
	if err == nil {
		t.Fatal("StartRecording returned nil although the bridge refused")
	}
	if b.Recording() {
		t.Error("Recording() = true after a refused StartRecording")
	}
}

func TestBridgeStreamsSamples(t *testing.T) {
	f := newFakeBridge(t)
	b := openTestBridge(t, f)

	if _, ok := b.Latest(); ok {
		t.Error("Latest() returned a sample before any arrived")
	}

	f.sendSample(1000, 960, 540)
	f.sendSample(1001, 970, 545)
	waitFor(t, "two samples", func() bool { return len(b.samplesLen()) == 2 })

	last, ok := b.Latest()
	if !ok {
		t.Fatal("Latest() = !ok after two samples")
	}
	if last.X != 970 || last.Y != 545 {
		t.Errorf("Latest() = (%v, %v), want (970, 545)", last.X, last.Y)
	}
	if !last.Valid {
		t.Error("sample with coordinates marked invalid")
	}
	if last.Eye != EyeRight {
		t.Errorf("Eye = %v, want right", last.Eye)
	}
	if last.LocalNs == 0 {
		t.Error("LocalNs not stamped")
	}

	drained := b.DrainSamples()
	if len(drained) != 2 {
		t.Fatalf("DrainSamples() returned %d samples, want 2", len(drained))
	}
	if drained[0].TrackerMs != 1000 || drained[1].TrackerMs != 1001 {
		t.Errorf("samples out of order: %v", drained)
	}
	if got := b.DrainSamples(); got != nil {
		t.Errorf("second DrainSamples() = %v, want nil", got)
	}
}

// samplesLen exposes the buffer length for the tests without draining it.
func (b *Bridge) samplesLen() []Sample {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Sample, len(b.samples))
	copy(out, b.samples)
	return out
}

func TestBridgeMarksSampleWithoutCoordinatesInvalid(t *testing.T) {
	f := newFakeBridge(t)
	b := openTestBridge(t, f)

	// A blink, as the Python bridge reports it: no x, no y, valid false. The
	// point of the test is that nothing downstream can read a position here.
	f.send(map[string]any{"ev": "sample", "t": 2000.0, "eye": "right", "valid": false, "pa": 0.0})
	waitFor(t, "the blink sample", func() bool { _, ok := b.Latest(); return ok })

	s, _ := b.Latest()
	if s.Valid {
		t.Error("sample without coordinates marked valid")
	}
	if s.X != 0 || s.Y != 0 {
		t.Errorf("invalid sample carries position (%v, %v)", s.X, s.Y)
	}
}

func TestBridgeDropsOldestWhenFull(t *testing.T) {
	f := newFakeBridge(t)
	b := openTestBridge(t, f, WithBufferSize(4))

	for i := 0; i < 6; i++ {
		f.sendSample(float64(1000+i), float64(i), 0)
	}
	waitFor(t, "the buffer to fill and drop", func() bool { return b.Dropped() == 2 })

	got := b.DrainSamples()
	if len(got) != 4 {
		t.Fatalf("buffer holds %d samples, want 4", len(got))
	}
	// The OLDEST two must be the ones gone: a gaze-contingent loop needs the
	// newest sample, so that is the end the buffer must never sacrifice.
	if got[0].TrackerMs != 1002 {
		t.Errorf("oldest retained sample is t=%v, want 1002", got[0].TrackerMs)
	}
	if got[3].TrackerMs != 1005 {
		t.Errorf("newest retained sample is t=%v, want 1005", got[3].TrackerMs)
	}
}

func TestBridgeReceivesOcularEvents(t *testing.T) {
	f := newFakeBridge(t)
	b := openTestBridge(t, f)

	f.send(map[string]any{
		"ev": "fix_end", "eye": "left", "start": 100.0, "end": 340.0,
		"sx": 10.0, "sy": 20.0, "ex": 12.0, "ey": 22.0, "ax": 11.0, "ay": 21.0,
	})
	waitFor(t, "the fixation event", func() bool { return len(b.eventsPeek()) == 1 })

	evs := b.DrainEvents()
	if len(evs) != 1 {
		t.Fatalf("DrainEvents() returned %d, want 1", len(evs))
	}
	e := evs[0]
	if e.Kind != FixationEnd {
		t.Errorf("Kind = %v, want %v", e.Kind, FixationEnd)
	}
	if e.Eye != EyeLeft {
		t.Errorf("Eye = %v, want left", e.Eye)
	}
	if e.StartMs != 100 || e.EndMs != 340 {
		t.Errorf("times = (%v, %v), want (100, 340)", e.StartMs, e.EndMs)
	}
	if e.AvgX != 11 || e.AvgY != 21 {
		t.Errorf("average gaze = (%v, %v), want (11, 21)", e.AvgX, e.AvgY)
	}
}

func (b *Bridge) eventsPeek() []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Event, len(b.events))
	copy(out, b.events)
	return out
}

func TestBridgeSyncMeasuresOffset(t *testing.T) {
	f := newFakeBridge(t)
	f.trackerMs = 500000
	// A fake local clock, so the offset is exact and the test cannot flake on
	// how fast the machine happens to be.
	var fakeNs int64
	b := openTestBridge(t, f, WithClock(func() int64 {
		fakeNs += 1_000_000 // every clock read advances 1 ms
		return fakeNs
	}))

	off, err := b.Sync(4)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if off.Samples != 4 {
		t.Errorf("Samples = %d, want 4", off.Samples)
	}
	if off.BestRTT <= 0 {
		t.Errorf("BestRTT = %v, want positive", off.BestRTT)
	}
	if off.WorstRTT < off.BestRTT {
		t.Errorf("WorstRTT %v < BestRTT %v", off.WorstRTT, off.BestRTT)
	}
	// The tracker clock is ~500 s ahead of a local clock that starts near zero.
	if off.DeltaMs < 499_000 || off.DeltaMs > 501_000 {
		t.Errorf("DeltaMs = %v, want ≈500000", off.DeltaMs)
	}
}

func TestBridgeRefusesUseBeforeOpen(t *testing.T) {
	b := NewBridge("127.0.0.1:1", WithLogger(func(string, ...any) {}))
	if err := b.StartRecording(); err == nil {
		t.Error("StartRecording succeeded on an unopened bridge")
	}
	if err := b.Mark("x"); err == nil {
		t.Error("Mark succeeded on an unopened bridge")
	}
	if err := b.Close(); err != nil {
		t.Errorf("Close on an unopened bridge = %v, want nil", err)
	}
}

func TestBridgeReportsLostConnection(t *testing.T) {
	f := newFakeBridge(t)
	b := openTestBridge(t, f)

	f.mu.Lock()
	f.conn.Close()
	f.mu.Unlock()

	// The command must fail rather than hang until the request timeout: a
	// dropped bridge is the common failure in a scanner room and the
	// experimenter needs to see it now, not in ten seconds.
	waitFor(t, "the client to notice the drop", func() bool {
		return b.Mark("after the drop") != nil
	})
}

func TestGeometryConversionRoundTrips(t *testing.T) {
	g := Geometry{WidthPx: 1920, HeightPx: 1080}

	// The centre of the screen in tracker pixels is the origin for us.
	x, y := g.ToCentre(960, 540)
	if x != 0 || y != 0 {
		t.Errorf("ToCentre(centre) = (%v, %v), want (0, 0)", x, y)
	}

	// The top of the screen must come out POSITIVE. Getting this backwards
	// mirrors every stimulus vertically, and it is the recurring bug this
	// conversion exists to prevent.
	_, top := g.ToCentre(960, 0)
	if top <= 0 {
		t.Errorf("ToCentre(top of screen) y = %v, want positive", top)
	}

	for _, p := range [][2]float64{{0, 0}, {1919, 1079}, {960, 540}, {100, 800}} {
		cx, cy := g.ToCentre(p[0], p[1])
		bx, by := g.FromCentre(cx, cy)
		if bx != p[0] || by != p[1] {
			t.Errorf("round trip of (%v, %v) gave (%v, %v)", p[0], p[1], bx, by)
		}
	}

	if !g.Contains(0, 0) || !g.Contains(1919, 1079) {
		t.Error("Contains rejected a corner that is on screen")
	}
	if g.Contains(1920, 540) || g.Contains(-1, 540) {
		t.Error("Contains accepted a point off screen")
	}
}

func TestSimulatedTrackerProducesSamples(t *testing.T) {
	var mu sync.Mutex
	x, y := 100.0, 200.0
	s := NewSimulated(func() (float64, float64, bool) {
		mu.Lock()
		defer mu.Unlock()
		return x, y, true
	})
	s.Rate = 200

	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if !s.Connected() {
		t.Error("Connected() = false after Open")
	}
	if _, ok := s.Latest(); ok {
		t.Error("Latest() returned a sample before recording started")
	}

	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	waitFor(t, "simulated samples", func() bool { _, ok := s.Latest(); return ok })

	got, _ := s.Latest()
	if got.X != 100 || got.Y != 200 {
		t.Errorf("Latest() = (%v, %v), want the position function's (100, 200)", got.X, got.Y)
	}

	if err := s.Mark("TRIALID 1"); err != nil {
		t.Fatalf("Mark: %v", err)
	}
	if marks := s.Marks(); len(marks) != 1 || marks[0] != "TRIALID 1" {
		t.Errorf("Marks() = %v, want [TRIALID 1]", marks)
	}

	if err := s.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}
	// StopRecording waits for the generator, so nothing may arrive afterwards.
	s.DrainSamples()
	time.Sleep(20 * time.Millisecond)
	if extra := s.DrainSamples(); len(extra) != 0 {
		t.Errorf("%d samples arrived after StopRecording returned", len(extra))
	}
}

func TestSimulatedTrackerReportsInvalidGaze(t *testing.T) {
	s := NewSimulated(func() (float64, float64, bool) { return 0, 0, false })
	s.Rate = 500
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	waitFor(t, "a simulated blink", func() bool { s, ok := s.Latest(); return ok && !s.Valid })
}

func TestParseEye(t *testing.T) {
	for in, want := range map[string]Eye{
		"left": EyeLeft, "L": EyeLeft,
		"right": EyeRight, "R": EyeRight,
		"binocular": EyeBinocular, "both": EyeBinocular,
		"": EyeUnknown, "nonsense": EyeUnknown,
	} {
		if got := ParseEye(in); got != want {
			t.Errorf("ParseEye(%q) = %v, want %v", in, got, want)
		}
	}
}

func ExampleGeometry_ToCentre() {
	g := Geometry{WidthPx: 1920, HeightPx: 1080}
	// A gaze near the top-left of the screen, as the tracker reports it.
	x, y := g.ToCentre(480, 270)
	fmt.Printf("%.0f, %.0f\n", x, y)
	// Output: -480, 270
}

// TestSupportsStepwiseCalibration covers the question control.CalibrateTracker
// asks before it decides who draws the targets. *Bridge satisfies
// StepwiseCalibrator whatever answered the socket, so only the bridge's own
// answer distinguishes a Tobii from an EyeLink.
func TestSupportsStepwiseCalibration(t *testing.T) {
	tests := []struct {
		name string
		caps []string
		want bool
	}{
		{"tobii announces the commands", []string{
			"calibration_enter", "calibration_collect", "calibration_discard",
			"calibration_compute", "calibration_leave"}, true},
		// An EyeLink bridge announces an empty list: it has none of them, and
		// treating that as "unknown" would drive a calibration that fails at
		// the first target.
		{"eyelink announces none", []string{}, false},
		// A bridge older than the caps field says nothing at all. That is not
		// the same as saying no, and the old behaviour is the safe guess.
		{"an older bridge cannot say", nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeBridge(t)
			f.caps = tc.caps
			b := NewBridge(f.addr())
			if err := b.Open(); err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer b.Close()
			if got := b.SupportsStepwiseCalibration(); got != tc.want {
				t.Errorf("SupportsStepwiseCalibration() = %v, want %v (caps %v)",
					got, tc.want, tc.caps)
			}
		})
	}
}
