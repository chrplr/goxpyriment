// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package eyetracker

import (
	"errors"
	"sync"
	"time"

	"github.com/chrplr/goxpyriment/clock"
)

// Simulated is a [Tracker] that invents gaze from any position function —
// normally the mouse. It exists so that a gaze-contingent experiment can be
// written, run and debugged with no hardware in the room, which is where most
// of the work happens.
//
// It is deliberately honest about being fake: [Simulated.Marks] keeps the
// markers in memory instead of pretending to write a data file, and the samples
// carry a tracker clock that starts at zero when the tracker is opened.
type Simulated struct {
	// Rate is the sampling rate in Hz. Zero means 1000, an EyeLink 1000's top
	// monocular rate.
	Rate int

	// Eye is reported on every sample. Zero means EyeRight.
	Eye Eye

	// Pos returns the simulated gaze in TRACKER pixel coordinates: origin
	// top-left, +Y down, the same convention [Sample] uses. Returning ok=false
	// produces an invalid sample, which is how you simulate a blink or a lost
	// track — the case experiments usually forget to handle.
	//
	// It is called from the sampling goroutine, so it must be safe to call
	// concurrently with the experiment loop. Reading SDL's mouse state from a
	// non-main thread is not; sample the position in your frame loop, store it
	// in an atomic, and have Pos read that.
	Pos func() (x, y float64, ok bool)

	// BufferSize bounds the sample buffer. Zero means defaultBufferSize.
	BufferSize int

	mu        sync.RWMutex
	open      bool
	recording bool
	start     time.Time
	samples   []Sample
	last      Sample
	haveOne   bool
	dropped   int
	marks     []string
	stop      chan struct{}
	wg        sync.WaitGroup
}

// NewSimulated returns a Simulated tracker reading gaze from pos, which must
// return TRACKER pixel coordinates (origin top-left, +Y down).
//
// A nil pos parks the gaze at the origin, which is valid but rarely what you
// want; passing the mouse is the point of this type.
func NewSimulated(pos func() (x, y float64, ok bool)) *Simulated {
	return &Simulated{Rate: 1000, Eye: EyeRight, Pos: pos}
}

// Open starts the simulated tracker's clock.
func (s *Simulated) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.open {
		return errors.New("eyetracker: simulated tracker already open")
	}
	s.open = true
	s.start = time.Now()
	s.samples = s.samples[:0]
	s.marks = s.marks[:0]
	s.dropped = 0
	s.haveOne = false
	return nil
}

// Close stops sampling and releases the tracker.
func (s *Simulated) Close() error {
	if s.Recording() {
		if err := s.StopRecording(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.open = false
	s.mu.Unlock()
	return nil
}

// Connected reports whether the tracker is open.
func (s *Simulated) Connected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.open
}

// Calibrate returns immediately: there is nothing to calibrate.
func (s *Simulated) Calibrate(_ CalibrationOptions) error { return nil }

// StartRecording begins generating samples at the configured rate.
func (s *Simulated) StartRecording() error {
	s.mu.Lock()
	if !s.open {
		s.mu.Unlock()
		return errors.New("eyetracker: simulated tracker not open")
	}
	if s.recording {
		s.mu.Unlock()
		return nil
	}
	rate := s.Rate
	if rate <= 0 {
		rate = 1000
	}
	s.recording = true
	s.stop = make(chan struct{})
	stop := s.stop
	s.mu.Unlock()

	s.wg.Add(1)
	go s.sampleLoop(stop, time.Second/time.Duration(rate))
	return nil
}

// StopRecording stops generating samples and waits for the generator to exit,
// so that no sample arrives after the call returns.
func (s *Simulated) StopRecording() error {
	s.mu.Lock()
	if !s.recording {
		s.mu.Unlock()
		return nil
	}
	s.recording = false
	close(s.stop)
	s.mu.Unlock()
	s.wg.Wait()
	return nil
}

// Recording reports whether samples are being generated.
func (s *Simulated) Recording() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recording
}

// Mark records a marker in memory. Retrieve them with [Simulated.Marks].
func (s *Simulated) Mark(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.open {
		return errors.New("eyetracker: simulated tracker not open")
	}
	s.marks = append(s.marks, text)
	return nil
}

// Marks returns the markers passed to Mark since the tracker was opened.
func (s *Simulated) Marks() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.marks))
	copy(out, s.marks)
	return out
}

// Latest returns the most recent simulated sample.
func (s *Simulated) Latest() (Sample, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last, s.haveOne
}

// DrainSamples removes and returns every buffered sample, oldest first.
func (s *Simulated) DrainSamples() []Sample {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.samples) == 0 {
		return nil
	}
	out := make([]Sample, len(s.samples))
	copy(out, s.samples)
	s.samples = s.samples[:0]
	return out
}

// DrainEvents always returns nil: nothing here parses fixations or saccades.
// Simulating a parser would produce events that look real and mean nothing.
func (s *Simulated) DrainEvents() []Event { return nil }

// Dropped returns how many samples were discarded because the buffer filled.
func (s *Simulated) Dropped() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dropped
}

// TrackerTime returns milliseconds since Open.
func (s *Simulated) TrackerTime() (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.open {
		return 0, errors.New("eyetracker: simulated tracker not open")
	}
	return float64(time.Since(s.start).Nanoseconds()) / 1e6, nil
}

// sampleLoop generates samples until stopped.
func (s *Simulated) sampleLoop(stop <-chan struct{}, period time.Duration) {
	defer s.wg.Done()
	tick := time.NewTicker(period)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			s.emit()
		}
	}
}

// emit produces one sample from the position function.
func (s *Simulated) emit() {
	var x, y float64
	ok := true
	if s.Pos != nil {
		x, y, ok = s.Pos()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	eye := s.Eye
	if eye == EyeUnknown {
		eye = EyeRight
	}
	sam := Sample{
		TrackerMs: float64(time.Since(s.start).Nanoseconds()) / 1e6,
		LocalNs:   clock.GetTimeNS(),
		Eye:       eye,
		X:         x,
		Y:         y,
		PupilArea: 1000,
		Valid:     ok,
	}
	s.last = sam
	s.haveOne = true

	size := s.BufferSize
	if size <= 0 {
		size = defaultBufferSize
	}
	if len(s.samples) >= size {
		copy(s.samples, s.samples[1:])
		s.samples = s.samples[:len(s.samples)-1]
		s.dropped++
	}
	s.samples = append(s.samples, sam)
}
