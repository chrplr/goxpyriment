// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package eyetracker

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/chrplr/goxpyriment/clock"
)

// DefaultBridgeAddr is where eyelink_bridge.py listens unless told otherwise.
// Loopback, not a wildcard: the bridge exposes control of the tracker and the
// participant's gaze, and there is no authentication in the protocol.
const DefaultBridgeAddr = "127.0.0.1:5010"

// defaultBufferSize is how many samples are held between drains. At an
// EyeLink 1000's top rate of 1000 Hz binocular this is eight seconds of data,
// and about four at an EyeLink 1000 Plus / CL running 2000 Hz monocular,
// which is a comfortable margin for a per-trial drain and small enough that the
// buffer is never the reason a machine swaps.
const defaultBufferSize = 8192

// Bridge is a [Tracker] reached through a bridge process speaking the protocol
// in protocol.go. See the package documentation for why the process exists.
//
// A Bridge is safe for concurrent use. The experiment loop can call Latest
// while another goroutine drains samples to disk.
type Bridge struct {
	addr string

	// configuration, immutable after construction
	geom        Geometry
	host        string
	edf         string
	bufferSize  int
	dialTimeout time.Duration
	reqTimeout  time.Duration
	now         func() int64
	logf        func(string, ...any)

	mu        sync.RWMutex
	conn      net.Conn
	writeMu   sync.Mutex // serialises writes; a request may race a Close
	pending   map[int]chan *response
	nextID    int
	connected bool
	recording bool
	simulated bool
	bridgeID  string

	samples []Sample
	events  []Event
	last    Sample
	haveOne bool
	dropped int

	calMessage  string // the tracker's summary of the last calibration
	calVerified bool   // whether the tracker confirmed it stored one

	readerDone chan struct{}
	helloCh    chan *message
	closeOnce  sync.Once
}

// Option configures a [Bridge].
type Option func(*Bridge)

// WithGeometry declares the stimulus screen the tracker is calibrated against.
// It is sent to the tracker at Open, and is what [Bridge.Geometry] returns for
// converting gaze coordinates. Defaults to 1920x1080, which is almost certainly
// wrong for your rig — set it.
func WithGeometry(g Geometry) Option { return func(b *Bridge) { b.geom = g } }

// WithHost sets the tracker's own address, passed through to the bridge. For an
// EyeLink this is the Host PC on the dedicated link, conventionally 100.1.1.1.
// The empty string lets the bridge use its own default.
func WithHost(host string) Option { return func(b *Bridge) { b.host = host } }

// WithEDF names the data file the tracker opens on its own machine. EyeLink
// Host PCs impose an 8.3-style limit: at most 8 characters before the
// extension, letters, digits and underscore. A name that breaks the rule is
// rejected by the Host at Open, not silently truncated.
func WithEDF(name string) Option { return func(b *Bridge) { b.edf = name } }

// WithBufferSize sets how many samples are held between drains. When the buffer
// is full the OLDEST samples are discarded and [Bridge.Dropped] counts them:
// losing the oldest keeps the most recent gaze available for a gaze-contingent
// loop, which is the reading that has a deadline.
func WithBufferSize(n int) Option {
	return func(b *Bridge) {
		if n > 0 {
			b.bufferSize = n
		}
	}
}

// WithTimeouts sets the connection and per-request timeouts. Calibration is
// exempt from the request timeout — it waits on a human.
func WithTimeouts(dial, request time.Duration) Option {
	return func(b *Bridge) {
		if dial > 0 {
			b.dialTimeout = dial
		}
		if request > 0 {
			b.reqTimeout = request
		}
	}
}

// WithClock sets the function used to stamp arriving samples, in nanoseconds.
//
// The default is clock.GetTimeNS, the Go monotonic clock. THAT IS THE WRONG
// CHOICE if you intend to compare gaze timestamps against stimulus onsets:
// Screen.FlipTS and the keyboard timestamps are on SDL's clock, which has a
// different origin, and subtracting one from the other yields a difference that
// looks like a latency and is not. In an experiment, pass:
//
//	eyetracker.WithClock(func() int64 { return int64(control.TicksNS()) })
//
// It is not the default only because the eyetracker package does not import
// SDL, and should not have to for a bridge that also has to build for the
// browser.
func WithClock(now func() int64) Option {
	return func(b *Bridge) {
		if now != nil {
			b.now = now
		}
	}
}

// WithLogger sets where bridge-side log events and protocol warnings go.
// Defaults to the standard logger. Pass a no-op to silence it.
func WithLogger(logf func(string, ...any)) Option {
	return func(b *Bridge) {
		if logf != nil {
			b.logf = logf
		}
	}
}

// NewBridge returns a Bridge that will connect to a bridge process at addr.
// Pass the empty string for [DefaultBridgeAddr]. Nothing happens on the network
// until [Bridge.Open].
func NewBridge(addr string, opts ...Option) *Bridge {
	if addr == "" {
		addr = DefaultBridgeAddr
	}
	b := &Bridge{
		addr:        addr,
		geom:        Geometry{WidthPx: 1920, HeightPx: 1080},
		bufferSize:  defaultBufferSize,
		dialTimeout: 5 * time.Second,
		reqTimeout:  10 * time.Second,
		now:         clock.GetTimeNS,
		logf:        log.Printf,
		pending:     make(map[int]chan *response),
		nextID:      1,
		helloCh:     make(chan *message, 1),
		readerDone:  make(chan struct{}),
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

// Geometry returns the screen geometry the Bridge was configured with, for
// converting tracker pixels to goxpyriment coordinates.
func (b *Bridge) Geometry() Geometry { return b.geom }

// Simulated reports whether the bridge answered that it is faking the tracker.
// Write this into the data file. A simulated run mistaken for a real one is
// worse than no run at all.
func (b *Bridge) Simulated() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.simulated
}

// BridgeID returns the bridge's self-reported name, e.g. "eyelink".
func (b *Bridge) BridgeID() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.bridgeID
}

// Open connects to the bridge process, checks the protocol version, and asks it
// to connect to the tracker.
//
// It does NOT calibrate and does not start recording.
func (b *Bridge) Open() error {
	b.mu.Lock()
	if b.connected {
		b.mu.Unlock()
		return errors.New("eyetracker: bridge already open")
	}
	b.mu.Unlock()

	conn, err := net.DialTimeout("tcp", b.addr, b.dialTimeout)
	if err != nil {
		return fmt.Errorf("eyetracker: dialling bridge at %s: %w "+
			"(is eyelink_bridge.py running?)", b.addr, err)
	}

	b.mu.Lock()
	b.conn = conn
	b.connected = true
	b.samples = b.samples[:0]
	b.events = b.events[:0]
	b.dropped = 0
	b.haveOne = false
	b.readerDone = make(chan struct{})
	b.helloCh = make(chan *message, 1)
	b.closeOnce = sync.Once{}
	b.mu.Unlock()

	go b.readLoop(conn)

	// The bridge greets us before anything else. Waiting for it here means a
	// version mismatch fails at Open, where it is obvious, rather than as a
	// puzzling error several commands later.
	select {
	case hello := <-b.helloCh:
		if hello.Proto != protoVersion {
			b.hardClose()
			return fmt.Errorf("eyetracker: bridge speaks protocol %d, client speaks %d",
				hello.Proto, protoVersion)
		}
		b.mu.Lock()
		b.simulated = hello.Simulated
		b.bridgeID = hello.Bridge
		b.mu.Unlock()
	case <-time.After(b.dialTimeout):
		b.hardClose()
		return fmt.Errorf("eyetracker: no hello from bridge at %s within %v", b.addr, b.dialTimeout)
	case <-b.readerDone:
		b.hardClose()
		return fmt.Errorf("eyetracker: bridge at %s closed the connection before greeting", b.addr)
	}

	args := map[string]any{
		"width":  b.geom.WidthPx,
		"height": b.geom.HeightPx,
	}
	if b.host != "" {
		args["host"] = b.host
	}
	if b.edf != "" {
		args["edf"] = b.edf
	}
	if _, err := b.do("open", args, b.reqTimeout); err != nil {
		b.hardClose()
		return err
	}

	if b.Simulated() {
		b.logf("eyetracker: WARNING — bridge %q is SIMULATING the tracker; "+
			"gaze data from this run is not real", b.bridgeID)
	}
	return nil
}

// Close stops any recording, tells the bridge to release the tracker, and drops
// the connection. Safe to call more than once, and safe on a Bridge that was
// never opened.
func (b *Bridge) Close() error {
	b.mu.RLock()
	open := b.connected
	rec := b.recording
	b.mu.RUnlock()
	if !open {
		return nil
	}

	var firstErr error
	if rec {
		if err := b.StopRecording(); err != nil {
			firstErr = err
		}
	}
	// Best effort: if the bridge has already gone away there is nothing useful
	// to report, and failing here would mask the recording error above.
	if _, err := b.do("close", nil, b.reqTimeout); err != nil && firstErr == nil {
		firstErr = err
	}
	b.hardClose()
	return firstErr
}

// hardClose drops the connection without talking to the bridge.
func (b *Bridge) hardClose() {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		conn := b.conn
		b.conn = nil
		b.connected = false
		b.recording = false
		b.mu.Unlock()
		if conn != nil {
			conn.Close()
		}
	})
}

// Connected reports whether the bridge connection is up.
func (b *Bridge) Connected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// Recording reports whether a recording is in progress.
func (b *Bridge) Recording() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.recording
}

// Calibrate runs the tracker's own calibration procedure and BLOCKS until the
// operator finishes it. There is no request timeout on this call.
//
// It returns an error unless the tracker confirms it stored a usable
// calibration. The tracker's setup routine exits the same way whether or not
// anything was calibrated, so "the operator closed the window" is not evidence
// of success — and a session recorded against an absent or stale calibration
// looks entirely normal until the gaze is analysed. [Bridge.CalibrationMessage]
// carries the tracker's own summary, including the validation error when the
// operator ran one.
func (b *Bridge) Calibrate(opts CalibrationOptions) error {
	if opts.Skip {
		return nil
	}
	args := map[string]any{}
	if opts.Points > 0 {
		args["points"] = opts.Points
	}
	res, err := b.do("calibrate", args, 0)
	if err != nil {
		return err
	}
	msg, _ := res["message"].(string)
	verified, _ := res["verified"].(bool)
	b.mu.Lock()
	b.calMessage = msg
	b.calVerified = verified
	b.mu.Unlock()
	if !verified {
		// The bridge could not read the result back. That is not proof of
		// failure, so it is not an error -- but it must not read as success.
		log.Printf("eyetracker: the tracker did not confirm a stored calibration; " +
			"check it on the Host before trusting any gaze position")
	}
	return nil
}

// CalibrationMessage returns the tracker's own summary of the last calibration,
// such as its validation error. It is empty until [Bridge.Calibrate] succeeds.
func (b *Bridge) CalibrationMessage() (string, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.calMessage, b.calVerified
}

// StartRecording opens the tracker's data file and starts the sample stream.
func (b *Bridge) StartRecording() error {
	if _, err := b.do("start_recording", nil, b.reqTimeout); err != nil {
		return err
	}
	b.mu.Lock()
	b.recording = true
	b.mu.Unlock()
	return nil
}

// StopRecording ends the recording. The tracker's data file stays open on its
// own machine until Close; retrieve it with [Bridge.ReceiveDataFile].
func (b *Bridge) StopRecording() error {
	_, err := b.do("stop_recording", nil, b.reqTimeout)
	b.mu.Lock()
	b.recording = false
	b.mu.Unlock()
	return err
}

// Mark writes a text marker into the tracker's data file.
//
// Read the warning on [Tracker.Mark] before using this to timestamp anything.
// For an EyeLink, a TTL pulse into the Host PC's parallel port is the right way
// to mark a stimulus onset; this call crosses a socket first.
func (b *Bridge) Mark(text string) error {
	_, err := b.do("mark", map[string]any{"text": text}, b.reqTimeout)
	return err
}

// TrackerTime returns the tracker's own clock in milliseconds.
func (b *Bridge) TrackerTime() (float64, error) {
	res, err := b.do("tracker_time", nil, b.reqTimeout)
	if err != nil {
		return 0, err
	}
	v, ok := res["time"].(float64)
	if !ok {
		return 0, fmt.Errorf("eyetracker: bridge returned no usable time: %v", res)
	}
	return v, nil
}

// ReceiveDataFile asks the bridge to copy the tracker's data file to local. On
// an EyeLink this pulls the EDF off the Host PC, which takes seconds to minutes
// depending on the recording, so it is exempt from the request timeout.
//
// Call it after [Bridge.StopRecording] and before [Bridge.Close].
func (b *Bridge) ReceiveDataFile(local string) error {
	_, err := b.do("receive_file", map[string]any{"path": local}, 0)
	return err
}

// Sync estimates the offset between the tracker's clock and the local clock by
// making n round trips and keeping the fastest, the way NTP does.
//
// The estimate assumes the round trip is symmetric, so its residual error is
// bounded by half the best round trip — reported in [Offset.BestRTT], and meant
// to be written into the data file next to the offset. An offset quoted without
// it is not a measurement.
//
// Call it before and after a run: the change over the session is the drift
// between two free-running clocks, and it is the number that says whether a
// constant offset is good enough for the analysis.
func (b *Bridge) Sync(n int) (Offset, error) {
	if n < 1 {
		n = 5
	}
	best := Offset{BestRTT: time.Duration(1<<63 - 1)}
	var worst time.Duration
	var got int
	for i := 0; i < n; i++ {
		t0 := b.now()
		tracker, err := b.TrackerTime()
		if err != nil {
			if got == 0 {
				return Offset{}, err
			}
			break
		}
		t1 := b.now()
		rtt := time.Duration(t1 - t0)
		got++
		if rtt > worst {
			worst = rtt
		}
		if rtt < best.BestRTT {
			mid := float64(t0+t1) / 2 / 1e6 // local clock, ms, at the midpoint
			best = Offset{
				LocalNs:   (t0 + t1) / 2,
				TrackerMs: tracker,
				DeltaMs:   tracker - mid,
				BestRTT:   rtt,
			}
		}
	}
	best.WorstRTT = worst
	best.Samples = got
	return best, nil
}

// Latest returns the most recent sample without blocking.
func (b *Bridge) Latest() (Sample, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.last, b.haveOne
}

// DrainSamples removes and returns every buffered sample, oldest first.
func (b *Bridge) DrainSamples() []Sample {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.samples) == 0 {
		return nil
	}
	out := make([]Sample, len(b.samples))
	copy(out, b.samples)
	b.samples = b.samples[:0]
	return out
}

// DrainEvents removes and returns every buffered ocular event, oldest first.
func (b *Bridge) DrainEvents() []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events) == 0 {
		return nil
	}
	out := make([]Event, len(b.events))
	copy(out, b.events)
	b.events = b.events[:0]
	return out
}

// Dropped returns how many samples were discarded because the buffer filled.
// Anything other than zero means the record has holes; report it in the data
// file rather than discovering it during analysis.
func (b *Bridge) Dropped() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.dropped
}

// do sends one request and waits for its response. A timeout of 0 waits
// indefinitely, for commands that block on a human or on a file transfer.
func (b *Bridge) do(cmd string, args map[string]any, timeout time.Duration) (map[string]any, error) {
	b.mu.Lock()
	if !b.connected || b.conn == nil {
		b.mu.Unlock()
		return nil, fmt.Errorf("eyetracker: %s: bridge not open", cmd)
	}
	id := b.nextID
	b.nextID++
	ch := make(chan *response, 1)
	b.pending[id] = ch
	conn := b.conn
	done := b.readerDone
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
	}()

	line, err := json.Marshal(request{ID: id, Cmd: cmd, Args: args})
	if err != nil {
		return nil, fmt.Errorf("eyetracker: encoding %s: %w", cmd, err)
	}
	b.writeMu.Lock()
	_, err = conn.Write(append(line, '\n'))
	b.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("eyetracker: sending %s: %w", cmd, err)
	}

	var timeoutCh <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timeoutCh = t.C
	}

	select {
	case resp := <-ch:
		if !resp.OK {
			return nil, fmt.Errorf("eyetracker: %s: %s", cmd, resp.Error)
		}
		return resp.Result, nil
	case <-done:
		return nil, fmt.Errorf("eyetracker: %s: bridge connection lost", cmd)
	case <-timeoutCh:
		return nil, fmt.Errorf("eyetracker: %s: no response within %v", cmd, timeout)
	}
}

// readLoop decodes the bridge's output until the connection ends. Closing
// readerDone releases every caller blocked in do.
func (b *Bridge) readLoop(conn net.Conn) {
	defer close(b.readerDone)

	sc := bufio.NewScanner(conn)
	// Samples are tiny; the generous cap is for a bridge that decides to log a
	// stack trace at us. The default 64 KB would end the session over it.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		var m message
		if err := json.Unmarshal(raw, &m); err != nil {
			b.logf("eyetracker: undecodable line from bridge: %v", err)
			continue
		}
		if m.isEvent() {
			b.handleEvent(&m)
			continue
		}
		b.mu.Lock()
		ch, ok := b.pending[m.ID]
		b.mu.Unlock()
		if !ok {
			b.logf("eyetracker: response to unknown request id %d", m.ID)
			continue
		}
		ch <- &response{ID: m.ID, OK: m.OK, Error: m.Error, Result: m.Result}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) && b.Connected() {
		b.logf("eyetracker: reading from bridge: %v", err)
	}
}

// handleEvent dispatches one unsolicited event.
func (b *Bridge) handleEvent(m *message) {
	now := b.now()
	switch {
	case m.Ev == "hello":
		select {
		case b.helloCh <- m:
		default:
		}
	case m.Ev == "sample":
		b.pushSample(m.toSample(now))
	case m.Ev == "log":
		b.logf("eyetracker: bridge [%s] %s", m.Level, m.Msg)
	case isOcularEvent(m.Ev):
		b.pushEvent(m.toEvent(now))
	default:
		b.logf("eyetracker: unknown event %q from bridge", m.Ev)
	}
}

// pushSample appends a sample, discarding the oldest if the buffer is full.
func (b *Bridge) pushSample(s Sample) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last = s
	b.haveOne = true
	if len(b.samples) >= b.bufferSize {
		// Drop the oldest. A gaze-contingent loop reads the newest sample and
		// has a deadline; the analysis reads the whole buffer and does not.
		// Whichever end we sacrifice, say so: b.dropped is the only evidence
		// afterwards that the record is not continuous.
		copy(b.samples, b.samples[1:])
		b.samples = b.samples[:len(b.samples)-1]
		b.dropped++
	}
	b.samples = append(b.samples, s)
}

// pushEvent appends an ocular event, discarding the oldest if the buffer is
// full. Events are rare next to samples, so the same bound is generous.
func (b *Bridge) pushEvent(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events) >= b.bufferSize {
		copy(b.events, b.events[1:])
		b.events = b.events[:len(b.events)-1]
	}
	b.events = append(b.events, e)
}
