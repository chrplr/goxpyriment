// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package control

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/chrplr/goxpyriment/apparatus"
)

// Onset markers: a photodiode square and TTL codes at the moments an
// experiment marks (see apparatus.Marker for what is drawn and written when).
// This file is the experiment-level setup: the opt-in command-line flags, the
// configuration, opening the device and recording what was used.
//
//	func main() {
//		control.MarkerFlags()                 // before NewExperimentFromFlags
//		exp := control.NewExperimentFromFlags(...)
//		...
//		exp.Marker.Arm(code)                  // the next flip is an onset
//		onset, _ := exp.ShowTS(stim)
//		...
//		exp.Marker.Fire(responseCode)         // an event with no flip
//	}
//
// exp.Marker is never nil after Initialize: without -photodiode or -ttl it is
// a marker that does nothing, so paradigms call it unconditionally.

// DefaultPhotodiodeSize is the side of the photodiode square, in screen
// pixels, when none is given.
const DefaultPhotodiodeSize = 100

// DefaultMarkerFrames is how many frames the square stays white from an onset
// when none is given: 50 ms at 60 Hz.
const DefaultMarkerFrames = 3

// MarkerConfig chooses what Initialize sets up as exp.Marker. MarkerFlags fills
// it from the command line; a program built with NewExperiment can set
// exp.MarkerConfig itself before Initialize.
type MarkerConfig struct {
	// Photodiode draws the corner square; PhotodiodeSize is its side in
	// screen pixels, whatever the logical size (0 = DefaultPhotodiodeSize).
	Photodiode     bool
	PhotodiodeSize float32
	// Frames the square stays white from an onset (0 = DefaultMarkerFrames).
	Frames int
	// TTL is a device spec for triggers.Open ("dlpio8", "megttlbox:/dev/ttyACM0",
	// ...); "" means no TTL output. A device that cannot be opened makes
	// Initialize fail: a recording must not start without its triggers.
	TTL string
	// TTLDevice is an already-open output, used instead of TTL — for a device
	// a program configures itself, or a fake in a test.
	TTLDevice apparatus.MarkerOutput
	// TTLDuration is how long a code stays on the lines (0 = Frames frame
	// periods of the display's nominal refresh rate).
	TTLDuration time.Duration
}

// enabled reports whether the configuration asks for anything at all.
func (c MarkerConfig) enabled() bool {
	return c.Photodiode || c.TTL != "" || c.TTLDevice != nil
}

// markerFlagValues holds the flags MarkerFlags registered, read back by
// NewExperimentFromFlags after it parses the command line. nil when
// MarkerFlags was not called.
var markerFlagValues *struct {
	photodiode *bool
	size       *float64
	frames     *int
	ttl        *string
	ttlMs      *float64
}

// markerFatal is log.Fatalf, as a variable so tests can observe the refusal.
var markerFatal = log.Fatalf

// MarkerFlags registers the onset-marker flags: -photodiode, -photodiode-size,
// -marker-frames, -ttl and -ttl-ms. Call it before NewExperimentFromFlags,
// which parses the command line and hands the values to Initialize. Programs
// that do not call it have none of these flags.
//
// An optional MarkerConfig sets the defaults, e.g. a MEG paradigm that wants
// the square on unless told otherwise:
//
//	control.MarkerFlags(control.MarkerConfig{Photodiode: true})
func MarkerFlags(defaults ...MarkerConfig) {
	if flag.Parsed() {
		markerFatal("control.MarkerFlags: called after the command line was parsed; call it before NewExperimentFromFlags")
		return
	}
	if markerFlagValues != nil {
		markerFatal("control.MarkerFlags: called twice")
		return
	}
	var d MarkerConfig
	if len(defaults) > 0 {
		d = defaults[0]
	}
	if d.PhotodiodeSize == 0 {
		d.PhotodiodeSize = DefaultPhotodiodeSize
	}
	if d.Frames == 0 {
		d.Frames = DefaultMarkerFrames
	}
	v := &struct {
		photodiode *bool
		size       *float64
		frames     *int
		ttl        *string
		ttlMs      *float64
	}{}
	v.photodiode = flag.Bool("photodiode", d.Photodiode,
		"Show a square in the top-left corner, white for -marker-frames frames from every marked onset "+
			"and black on every other frame, for a photodiode")
	v.size = flag.Float64("photodiode-size", float64(d.PhotodiodeSize),
		"Side of the photodiode square, in screen pixels (the same whatever the program's logical size)")
	v.frames = flag.Int("marker-frames", d.Frames,
		"Frames the photodiode square stays white from an onset; also the default TTL pulse length")
	v.ttl = flag.String("ttl", d.TTL,
		"TTL device receiving a code at every marked onset, as KIND[:PORT]: dlpio8[:PORT|auto], dlpio20[:PORT|auto], "+
			"megttlbox:/dev/ttyACM0, mmbts:/dev/ttyACM0, parallel[:/dev/parport0], gpio, ft232h, labjackt4:HOST, null; "+
			"empty means no TTL")
	v.ttlMs = flag.Float64("ttl-ms", d.TTLDuration.Seconds()*1000,
		"How long each code stays on the TTL lines, in ms (0 = -marker-frames frame periods)")
	markerFlagValues = v
}

// markerConfigFromFlags returns the configuration MarkerFlags' flags parsed
// to, and false when MarkerFlags was not called.
func markerConfigFromFlags() (MarkerConfig, bool) {
	v := markerFlagValues
	if v == nil {
		return MarkerConfig{}, false
	}
	return MarkerConfig{
		Photodiode:     *v.photodiode,
		PhotodiodeSize: float32(*v.size),
		Frames:         *v.frames,
		TTL:            *v.ttl,
		TTLDuration:    time.Duration(*v.ttlMs * float64(time.Millisecond)),
	}, true
}

// validate checks the parts of the configuration that need no screen.
func (c MarkerConfig) validate() error {
	if c.Photodiode && c.PhotodiodeSize < 0 {
		return fmt.Errorf("photodiode size %.0f must be positive", c.PhotodiodeSize)
	}
	if c.Frames < 0 {
		return fmt.Errorf("marker frames %d must be positive", c.Frames)
	}
	if c.TTLDuration < 0 {
		return fmt.Errorf("TTL duration %v must be positive", c.TTLDuration)
	}
	return nil
}

// withDefaults fills in the zero values.
func (c MarkerConfig) withDefaults() MarkerConfig {
	if c.PhotodiodeSize == 0 {
		c.PhotodiodeSize = DefaultPhotodiodeSize
	}
	if c.Frames == 0 {
		c.Frames = DefaultMarkerFrames
	}
	return c
}

// openMarkerOutput opens the TTL output a configuration names, before the
// window exists, so a bad -ttl fails without a fullscreen window flashing up.
// It returns nil when no TTL is configured.
func (e *Experiment) openMarkerOutput() (apparatus.MarkerOutput, string, error) {
	c := e.MarkerConfig
	if err := c.validate(); err != nil {
		return nil, "", fmt.Errorf("onset marker: %w", err)
	}
	if c.TTLDevice != nil {
		return c.TTLDevice, "TTL device supplied by the program", nil
	}
	if c.TTL == "" {
		return nil, "", nil
	}
	out, desc, err := openTTLSpec(c.TTL)
	if err != nil {
		return nil, "", fmt.Errorf("onset marker -ttl %q: %w", c.TTL, err)
	}
	if err := out.AllLow(); err != nil {
		_ = out.Close()
		return nil, "", fmt.Errorf("onset marker -ttl %q: driving the lines low: %w", c.TTL, err)
	}
	return out, desc, nil
}

// attachMarker builds exp.Marker from the configuration and the output opened
// earlier, attaches it to the screen and records it in the session metadata.
// Called by Initialize once the screen and the data file exist.
func (e *Experiment) attachMarker(out apparatus.MarkerOutput, desc string) error {
	c := e.MarkerConfig.withDefaults()
	if !c.Photodiode && out == nil {
		e.Marker = apparatus.NewMarker(apparatus.MarkerOptions{}) // does nothing
		return nil
	}
	if c.Photodiode {
		w, h := float32(e.Screen.Width), float32(e.Screen.Height)
		if ls := e.Screen.LogicalSize; ls != nil {
			w, h = ls.X, ls.Y
		}
		if c.PhotodiodeSize > w || c.PhotodiodeSize > h {
			return fmt.Errorf("onset marker: a %.0f px photodiode square does not fit on a %.0fx%.0f screen",
				c.PhotodiodeSize, w, h)
		}
	}
	e.Marker = apparatus.NewMarker(apparatus.MarkerOptions{
		Photodiode:  c.Photodiode,
		Size:        c.PhotodiodeSize,
		Frames:      c.Frames,
		Output:      out,
		OutputDesc:  desc,
		TTLDuration: c.TTLDuration,
	})
	e.Screen.SetMarker(e.Marker) // fills in a zero TTL duration from the refresh rate
	if c.Photodiode {
		plural := "s"
		if c.Frames == 1 {
			plural = ""
		}
		line := fmt.Sprintf("marker photodiode: %.0f px square at the top-left corner of the screen, "+
			"white %d frame%s from every marked onset, black on every other frame", c.PhotodiodeSize, c.Frames, plural)
		log.Print(line)
		e.AddExperimentInfo(line)
	}
	if out != nil {
		line := fmt.Sprintf("marker ttl: %s; code written right after the onset flip, lines cleared %.1f ms later",
			desc, float64(e.Marker.TTLDuration())/float64(time.Millisecond))
		log.Print(line)
		e.AddExperimentInfo(line)
	}
	return nil
}

// markerStatsLine is the line finalizeData writes at the end of a session that
// used a marker; "" when it used none.
func (e *Experiment) markerStatsLine() string {
	if !e.Marker.PhotodiodeEnabled() && !e.Marker.TTLEnabled() {
		return ""
	}
	s := e.Marker.Stats()
	return fmt.Sprintf("marker stats: onsets=%d codes=%d fires=%d clears=%d write_errors=%d overlaps=%d merged=%d",
		s.Onsets, s.Codes, s.Fires, s.Clears, s.WriteErrors, s.Overlaps, s.Merged)
}

// completeMarker keeps presenting while the marker's square still needs
// flips, so a presentation followed by a wait that never flips (a key wait,
// Wait) does not leave the square white on the panel. redraw draws the frame
// again, from a clear; the screen is presented after each call. Without a
// photodiode square, or once the square is black, it does nothing.
func (e *Experiment) completeMarker(redraw func() error) error {
	if e.Screen == nil || e.Screen.Marker() != e.Marker {
		return nil
	}
	for i := 0; e.Marker.Busy(); i++ {
		// Busy goes false after Frames+1 flips; anything longer is a bug, and
		// an endless loop here would freeze the session.
		if i > e.Marker.Frames()+1 {
			return fmt.Errorf("onset marker: square still busy after %d frames", i)
		}
		if err := redraw(); err != nil {
			return err
		}
		if err := e.Screen.Update(); err != nil {
			return err
		}
	}
	return nil
}
