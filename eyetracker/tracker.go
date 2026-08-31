// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package eyetracker provides a vendor-neutral interface to gaze trackers, and
// a socket client for hardware whose only API is a C SDK.
//
// # Why a bridge
//
// goxpyriment does not use CGo, and the SR Research EyeLink has no documented
// network protocol: the Display PC talks to the Host PC over a proprietary
// link, and the only supported way in is the C library (or pylink, which wraps
// it). Linking that library would cost the pure-Go build, cross-compilation,
// and the browser target, for one device.
//
// So the SDK runs in a separate process — the bridge — which speaks a small
// line-delimited JSON protocol over a local socket. [Bridge] is the Go client.
// The bridge script lives in eyetracker/bridge/eyelink_bridge.py; the protocol
// is specified in protocol.go and is deliberately generic, so a bridge for
// another tracker is a new script rather than a new Go package.
//
// This is the same shape PsychoPy's ioHub arrived at, for different reasons:
// there, the separate process exists to keep the GIL out of the sampling loop;
// here, it exists to keep the C compiler out of the build.
//
// # What NOT to put on the link
//
// The bridge is not in the timing path and must not be put there. An EyeLink
// Host PC records TTL input on its parallel port as INPUT events in the EDF,
// timestamped by the Host's own clock at the moment the edge arrives. For
// marking stimulus onsets that is strictly better than sending a message
// through the bridge: it costs no round trip, it cannot be delayed by the
// socket, and goxpyriment already drives that hardware (see the triggers
// package, and triggers.FireTriggerSync in particular).
//
// Use the bridge for setup, calibration, starting and stopping recording, and
// for reading gaze in gaze-contingent designs. Use a TTL for anything whose
// timestamp is the measurement.
//
// # Implementations
//
//   - [Bridge]      — a tracker behind a bridge process (EyeLink via pylink)
//   - [Simulated]   — a fake tracker driven by any position function, typically
//     the mouse; for developing without hardware
//   - [NullTracker] — silent no-op, so calling code never needs a nil check
package eyetracker

import "time"

// Tracker is the common interface to a gaze tracker.
//
// The lifecycle is Open → Calibrate → (StartRecording → … → StopRecording)* →
// Close. Recording may be started and stopped many times; most trackers want it
// per trial or per block rather than once for the session.
//
// Implementations must be safe to call from several goroutines. In practice the
// experiment loop calls Latest concurrently with whatever is draining samples
// to disk.
type Tracker interface {
	// Open connects to the tracker and prepares it for use.
	Open() error

	// Close stops any recording in progress and releases the tracker.
	// It is safe to call on an already-closed tracker.
	Close() error

	// Connected reports whether the tracker is currently usable.
	Connected() bool

	// Calibrate runs the tracker's own setup and calibration procedure.
	//
	// On an EyeLink this hands the screen to the Host PC's calibration
	// graphics and BLOCKS until the operator dismisses it, which may be
	// minutes. Never call it inside a frame loop.
	Calibrate(opts CalibrationOptions) error

	// StartRecording begins writing to the tracker's own data file and
	// streaming samples to the client.
	StartRecording() error

	// StopRecording ends the current recording.
	StopRecording() error

	// Recording reports whether a recording is in progress.
	Recording() bool

	// Mark writes a text marker into the tracker's own data file, timestamped
	// by the tracker.
	//
	// The marker's timestamp is when the tracker RECEIVED it, which for a
	// bridged tracker is one socket hop plus one link hop after the call. Do
	// not use Mark to timestamp a stimulus onset — use a TTL, and see the
	// package documentation.
	Mark(text string) error

	// Latest returns the most recent gaze sample, and false if none has
	// arrived yet. It never blocks: this is the call for a gaze-contingent
	// frame loop.
	Latest() (Sample, bool)

	// DrainSamples removes and returns every buffered sample, oldest first.
	// Call it at least once per trial; the buffer is bounded and discards the
	// OLDEST samples when full, reporting the loss through [Tracker.Dropped].
	DrainSamples() []Sample

	// DrainEvents removes and returns every buffered parsed ocular event.
	DrainEvents() []Event

	// Dropped returns the number of samples discarded because the buffer was
	// full since the tracker was opened. A nonzero value means the experiment
	// is not draining fast enough and the record has holes.
	Dropped() int

	// TrackerTime returns the tracker's own clock in milliseconds.
	TrackerTime() (float64, error)
}

// CalibrationOptions configures [Tracker.Calibrate]. The zero value asks for
// the tracker's own defaults, which is usually what you want: the Host PC's
// calibration screen is better tested than anything an experiment can draw.
type CalibrationOptions struct {
	// Points is the number of calibration targets (typically 3, 5, 9 or 13).
	// Zero leaves the tracker's configured default alone.
	Points int

	// Skip runs no calibration at all and returns nil. It exists so that a
	// script can be run end-to-end without a participant, without the caller
	// growing an if.
	Skip bool
}

// Offset is the measured relationship between the tracker's clock and the
// experiment machine's clock, as returned by [Sync].
//
// The two clocks free-run independently and will drift apart over a session, so
// a single Offset is a snapshot. Measure it at the start and the end of a run
// and compare: the difference over the elapsed time is the drift rate, and if
// it is large enough to matter for the analysis, the alignment has to be
// interpolated rather than applied as a constant.
type Offset struct {
	// LocalNs is the experiment clock at which the offset was measured.
	LocalNs int64

	// TrackerMs is the tracker clock reported at that moment, corrected for
	// half of the best round trip.
	TrackerMs float64

	// DeltaMs is TrackerMs minus the local clock in milliseconds, so
	//
	//	trackerMs ≈ localNs/1e6 + DeltaMs
	DeltaMs float64

	// BestRTT is the shortest round trip observed. The correction assumes the
	// trip is symmetric, so the residual error is bounded by BestRTT/2 — that
	// is the honest uncertainty on DeltaMs, and it should be reported with it.
	BestRTT time.Duration

	// WorstRTT is the longest round trip observed, and Samples the number of
	// round trips made. A large spread between best and worst means the link
	// or the bridge was busy, and the estimate is correspondingly soft.
	WorstRTT time.Duration
	Samples  int
}

// NullTracker is a no-op [Tracker]. Every method succeeds and no sample ever
// arrives, so an experiment written against the interface runs unchanged with
// no hardware present.
//
// It follows the same pattern as triggers.NullOutputTTLDevice: a constructor
// that finds no device returns this rather than nil, so callers never nil-check.
type NullTracker struct{}

func (NullTracker) Open() error                          { return nil }
func (NullTracker) Close() error                         { return nil }
func (NullTracker) Connected() bool                      { return false }
func (NullTracker) Calibrate(_ CalibrationOptions) error { return nil }
func (NullTracker) StartRecording() error                { return nil }
func (NullTracker) StopRecording() error                 { return nil }
func (NullTracker) Recording() bool                      { return false }
func (NullTracker) Mark(_ string) error                  { return nil }
func (NullTracker) Latest() (Sample, bool)               { return Sample{}, false }
func (NullTracker) DrainSamples() []Sample               { return nil }
func (NullTracker) DrainEvents() []Event                 { return nil }
func (NullTracker) Dropped() int                         { return 0 }
func (NullTracker) TrackerTime() (float64, error)        { return 0, nil }

// compile-time checks
var (
	_ Tracker = NullTracker{}
	_ Tracker = (*Bridge)(nil)
	_ Tracker = (*Simulated)(nil)
)
