// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package eyetracker

import "fmt"

// Eye identifies which eye a sample or event came from.
type Eye uint8

const (
	EyeUnknown Eye = iota
	EyeLeft
	EyeRight
	EyeBinocular
)

func (e Eye) String() string {
	switch e {
	case EyeLeft:
		return "left"
	case EyeRight:
		return "right"
	case EyeBinocular:
		return "binocular"
	default:
		return "unknown"
	}
}

// ParseEye maps the wire representation of an eye back to an [Eye].
func ParseEye(s string) Eye {
	switch s {
	case "left", "L":
		return EyeLeft
	case "right", "R":
		return EyeRight
	case "binocular", "both", "LR":
		return EyeBinocular
	default:
		return EyeUnknown
	}
}

// Sample is one gaze sample.
//
// # Coordinates
//
// X and Y are in SCREEN PIXELS with the origin at the top-left corner and +Y
// pointing DOWN — the convention the tracker itself reports, not goxpyriment's
// centre-relative +Y-up convention. Converting is the caller's job, so that a
// sample written to a data file always means the same thing regardless of which
// screen it was collected on. Use [Geometry.ToCentre] for the conversion.
//
// # Timestamps
//
// TrackerMs is the tracker's OWN clock, in milliseconds. On an EyeLink this is
// the Host PC's clock, which is the clock the EDF file is written in, and it has
// no fixed relationship to any clock on the Display PC.
//
// LocalNs is the experiment machine's clock at the moment the sample was
// decoded, in nanoseconds, read from whatever function the transport was
// configured with (see [WithClock]). Two samples' LocalNs values are comparable
// with each other and with stimulus onsets ONLY if that clock is the same one
// the onsets were stamped with — which for goxpyriment means
// control.TicksNS(), not clock.GetTimeNS(). Mixing the two silently produces
// differences that look like latencies.
//
// LocalNs includes the transport delay: the tracker's link, the bridge process,
// the socket, and the decode. It is a receipt time, not an acquisition time.
// To relate the two clocks properly, use [Sync].
//
// # Pupil size
//
// PupilArea's unit is whatever the tracker reports, and the two makes do not
// agree: an EyeLink reports pupil AREA in its own arbitrary units, in the
// thousands; a Tobii reports pupil DIAMETER in millimetres, around 2 to 8.
// Nothing in the type can catch a confusion between them, so each bridge states
// its unit at open — the Tobii bridge returns pupil_units, and writes it into
// the header of its own gaze file — and it belongs in the run's -info.txt.
// Comparing pupil sizes across makes without converting is meaningless.
type Sample struct {
	TrackerMs float64 // tracker's own clock, milliseconds
	LocalNs   int64   // experiment machine's clock at decode, nanoseconds
	Eye       Eye
	X, Y      float64 // gaze position, screen pixels, origin top-left, +Y down
	PupilArea float64 // pupil size — UNITS DEPEND ON THE TRACKER, see below
	Valid     bool    // false when the tracker reported missing data (blink, lost track)
}

func (s Sample) String() string {
	if !s.Valid {
		return fmt.Sprintf("Sample{%.0f ms %s MISSING}", s.TrackerMs, s.Eye)
	}
	return fmt.Sprintf("Sample{%.0f ms %s (%.1f, %.1f) pa=%.0f}",
		s.TrackerMs, s.Eye, s.X, s.Y, s.PupilArea)
}

// EventKind names a parsed ocular event reported by the tracker.
type EventKind string

const (
	FixationStart EventKind = "fix_start"
	FixationEnd   EventKind = "fix_end"
	SaccadeStart  EventKind = "sacc_start"
	SaccadeEnd    EventKind = "sacc_end"
	BlinkStart    EventKind = "blink_start"
	BlinkEnd      EventKind = "blink_end"
)

// Event is a parsed ocular event — a fixation, saccade or blink boundary — as
// detected by the TRACKER's own online parser, not by goxpyriment.
//
// The parser and its thresholds belong to the tracker, so the same eye movement
// classified by two makes of tracker will not produce identical events. Record
// the samples if the classification has to be reproducible.
//
// Coordinates and timestamps follow the same conventions as [Sample]. Fields
// that do not apply to a given kind are zero: a FixationStart carries no
// EndMs, and a blink carries no position at all.
type Event struct {
	Kind    EventKind
	Eye     Eye
	LocalNs int64 // experiment machine's clock at decode, nanoseconds

	StartMs, EndMs float64 // tracker clock, milliseconds
	StartX, StartY float64 // screen pixels, origin top-left, +Y down
	EndX, EndY     float64
	AvgX, AvgY     float64 // fixation average position
	PupilArea      float64
	Amplitude      float64 // saccade amplitude, degrees (0 if not reported)
	PeakVelocity   float64 // saccade peak velocity, degrees/second
}

func (e Event) String() string {
	return fmt.Sprintf("Event{%s %s start=%.0f end=%.0f (%.1f,%.1f)->(%.1f,%.1f)}",
		e.Kind, e.Eye, e.StartMs, e.EndMs, e.StartX, e.StartY, e.EndX, e.EndY)
}

// Geometry describes the stimulus screen the tracker was calibrated against, so
// that tracker pixel coordinates can be converted to goxpyriment's
// centre-relative, +Y-up convention.
//
// It must match the resolution the tracker was told about at calibration time
// (on an EyeLink, the screen_pixel_coords command that [Bridge.Open] issues).
// A mismatch does not fail — it silently shifts and scales every gaze position.
type Geometry struct {
	WidthPx  int
	HeightPx int
}

// ToCentre converts tracker pixel coordinates (origin top-left, +Y down) to
// goxpyriment screen coordinates (origin centre, +Y UP).
//
// The Y flip is the whole point: passing a raw tracker Y to a stimulus position
// mirrors the display vertically, which is a bug that looks like a calibration
// problem.
func (g Geometry) ToCentre(x, y float64) (float64, float64) {
	return x - float64(g.WidthPx)/2, float64(g.HeightPx)/2 - y
}

// FromCentre converts goxpyriment screen coordinates (origin centre, +Y up) to
// tracker pixel coordinates (origin top-left, +Y down). It is the inverse of
// [Geometry.ToCentre], and is what you need to tell a tracker where a
// calibration target was drawn.
func (g Geometry) FromCentre(x, y float64) (float64, float64) {
	return x + float64(g.WidthPx)/2, float64(g.HeightPx)/2 - y
}

// Contains reports whether a tracker pixel coordinate falls on the screen.
// Gaze outside the screen is not an error — participants look away — but it is
// usually not a valid response either.
func (g Geometry) Contains(x, y float64) bool {
	return x >= 0 && y >= 0 && x < float64(g.WidthPx) && y < float64(g.HeightPx)
}
