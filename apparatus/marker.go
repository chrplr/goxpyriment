// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import (
	"fmt"
	"log"
	"math/bits"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Onset markers for recordings (MEG, EEG, fMRI, display-timing checks).
//
// A Marker marks an event two ways at once: a square in the top-left corner of
// the drawing area, white for a few frames from the event and black on every
// other frame (for a photodiode taped to the screen), and a code written to a
// TTL device (for the recording system's trigger channel). Attached to a
// Screen with SetMarker, it is driven from inside Update — the one function
// every flip in the library goes through — so it marks onsets presented by any
// path: Show, streams, video, a sound started on a flip.
//
//	m.Arm(code)      // the next flip is an onset: square white, code written after it
//	onset, _ := exp.ShowTS(stim)
//
//	m.Fire(code)     // an event with no flip (a response, a sound played with Play):
//	                 // code written now, no square
//
// Both outputs are optional; a Marker with neither does nothing, and every
// method is safe on a nil *Marker, so experiment code can call them
// unconditionally.

// markerClockNS is the clock Fire reports on: SDL's, like every event and flip
// timestamp. A variable so tests can run without the SDL library loaded.
var markerClockNS = sdl.TicksNS

// MarkerOutput is the TTL device a Marker writes codes to.
// triggers.OutputTTLDevice satisfies it; it is declared here so that apparatus
// does not import triggers, which does not build for the browser.
type MarkerOutput interface {
	Send(mask byte) error
	AllLow() error
	Close() error
}

// markerLineOutput is implemented by devices that can drive one line on its
// own (every triggers.OutputTTLDevice can). The marker uses it for a code
// with a single bit set: on the DLP-IO8 and DLP-IO20, Send writes the eight
// lines one command at a time (about 0.6 ms and 3.5 ms), and a recording
// sampling during that window can latch a code that was never sent, while
// SetHigh is one command and has no such window.
type markerLineOutput interface {
	SetHigh(line int) error
	SetLow(line int) error
}

// MarkerOptions configures NewMarker.
type MarkerOptions struct {
	// Photodiode draws the corner square. Size is its side in drawing-space
	// units: logical units when Screen.SetLogicalSize is in effect, pixels
	// otherwise.
	Photodiode bool
	Size       float32

	// Frames is how many frames the square stays white from an onset
	// (default 3). More than one, so a frame lost in the display stack makes
	// the marker late rather than missing.
	Frames int

	// Output receives the codes; nil means no TTL. OutputDesc names it in
	// logs and data files.
	Output     MarkerOutput
	OutputDesc string

	// TTLDuration is how long a code stays on the lines before they are
	// cleared to 0. Zero means Frames frame periods, filled in by
	// Screen.SetMarker from the display's nominal refresh rate.
	TTLDuration time.Duration
}

// MarkerStats counts what a Marker did, for the session's metadata.
type MarkerStats struct {
	Onsets      uint64 // armed flips presented
	Codes       uint64 // codes written at onsets (Arm with a non-zero code)
	Fires       uint64 // codes written by Fire
	Clears      uint64 // lines cleared to 0 after a code
	WriteErrors uint64 // failed writes to the device
	Overlaps    uint64 // codes written while the previous one was still on the lines
	Merged      uint64 // onsets armed while the square was still white (the flashes merge)
}

// Marker is a photodiode square and a TTL output driven by Screen.Update.
// Create it with NewMarker and attach it with Screen.SetMarker.
type Marker struct {
	photodiode bool
	rect       sdl.FRect // the square, at the top-left corner of the drawing area
	frames     int
	out        MarkerOutput
	outDesc    string
	ttlDur     time.Duration

	// Flip-thread state: touched only by Arm, Busy and the Update hooks, all
	// of which run on the thread that presents.
	armed     bool
	code      byte
	left      int  // white frames still to present, this one included
	white     bool // the square in the frame being presented is white
	lastWhite bool // the last presented frame had a white square
	lastOnset uint64

	// TTL state, shared with the clearing goroutine and with Fire, which may
	// run on any goroutine. Device implementations are not internally
	// synchronised, so every write goes through mu.
	mu        sync.Mutex
	high      bool      // a code is on the lines
	cur       byte      // the code on the lines (valid while high)
	clearAt   time.Time // when to clear them
	closed    bool
	poke      chan struct{} // capacity 1: wakes the worker after a write
	done      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeErr  error

	onsets, codes, fires, clears, writeErrs, overlaps, merged atomic.Uint64
}

// NewMarker returns a Marker. When opts.Output is set it starts the goroutine
// that clears the lines after each code; Close stops it.
func NewMarker(opts MarkerOptions) *Marker {
	frames := opts.Frames
	if frames < 1 {
		frames = 3
	}
	m := &Marker{
		photodiode: opts.Photodiode,
		rect:       sdl.FRect{X: 0, Y: 0, W: opts.Size, H: opts.Size},
		frames:     frames,
		out:        opts.Output,
		outDesc:    opts.OutputDesc,
		ttlDur:     opts.TTLDuration,
	}
	if m.out != nil {
		m.poke = make(chan struct{}, 1)
		m.done = make(chan struct{})
		m.wg.Add(1)
		go m.clearWorker()
	}
	return m
}

// SetMarker attaches m to the screen, so that every Update draws its square
// and writes its armed code; nil detaches it. A zero TTLDuration is replaced
// by Frames periods of the display's nominal refresh rate.
func (s *Screen) SetMarker(m *Marker) {
	if m != nil && m.ttlDur == 0 {
		m.ttlDur = time.Duration(m.frames) * s.FrameDuration()
	}
	s.marker = m
}

// Marker returns the attached Marker, or nil.
func (s *Screen) Marker() *Marker { return s.marker }

// SetPrePresentHook installs f to run inside Update after everything,
// the marker square included, has been drawn and just before the frame is
// presented; nil removes it. It is the only place the finished frame can be
// read back (Renderer.ReadPixels): SDL invalidates the backbuffer on present.
// Meant for checks and screenshots, not for drawing stimuli.
func (s *Screen) SetPrePresentHook(f func() error) { s.prePresent = f }

// Arm makes the next flip an onset: the square is white on it and for
// Frames-1 flips after, and code (when non-zero, and a TTL output is set) is
// written right after it, then cleared TTLDuration later. Code 0 marks the
// onset with the square only. Arming again before the flip replaces the code.
//
// Arm must be called on the thread that presents (inside exp.Run).
func (m *Marker) Arm(code byte) {
	if m == nil || (!m.photodiode && m.out == nil) {
		return
	}
	m.armed = true
	m.code = code
}

// Fire writes code to the TTL output now and returns the SDL clock time
// (sdl.TicksNS) taken just before the write. It draws no square — use it for
// events that are not a flip: a response once it is detected, a sound started
// with Play. A zero code or a Marker with no TTL output writes nothing and
// returns 0. Safe on any goroutine.
func (m *Marker) Fire(code byte) uint64 {
	if m == nil || m.out == nil || code == 0 {
		return 0
	}
	ts := markerClockNS()
	if m.sendCode(code) {
		m.fires.Add(1)
	}
	return ts
}

// Busy reports whether the square still needs flips: the onset is armed, the
// square is still white, or the last presented frame showed it white and no
// black frame has replaced it yet. A presentation that is about to stop
// flipping (to wait for a key, say) keeps presenting while Busy, so the square
// does not stay white on the panel. Always false without a photodiode square.
func (m *Marker) Busy() bool {
	if m == nil || !m.photodiode {
		return false
	}
	return m.armed || m.left > 0 || m.lastWhite
}

// PhotodiodeEnabled reports whether the corner square is drawn.
func (m *Marker) PhotodiodeEnabled() bool { return m != nil && m.photodiode }

// TTLEnabled reports whether codes go to a TTL output.
func (m *Marker) TTLEnabled() bool { return m != nil && m.out != nil }

// Size is the side of the square in drawing-space units.
func (m *Marker) Size() float32 {
	if m == nil {
		return 0
	}
	return m.rect.W
}

// Frames is how many frames the square stays white from an onset.
func (m *Marker) Frames() int {
	if m == nil {
		return 0
	}
	return m.frames
}

// TTLDuration is how long a code stays on the lines.
func (m *Marker) TTLDuration() time.Duration {
	if m == nil {
		return 0
	}
	return m.ttlDur
}

// OutputDesc names the TTL output, "" when there is none.
func (m *Marker) OutputDesc() string {
	if m == nil {
		return ""
	}
	return m.outDesc
}

// LastOnsetNS is the flip timestamp of the last armed onset (0 before any).
func (m *Marker) LastOnsetNS() uint64 {
	if m == nil {
		return 0
	}
	return m.lastOnset
}

// Stats returns the counters. Safe on any goroutine.
func (m *Marker) Stats() MarkerStats {
	if m == nil {
		return MarkerStats{}
	}
	return MarkerStats{
		Onsets: m.onsets.Load(), Codes: m.codes.Load(), Fires: m.fires.Load(),
		Clears: m.clears.Load(), WriteErrors: m.writeErrs.Load(),
		Overlaps: m.overlaps.Load(), Merged: m.merged.Load(),
	}
}

// Close stops the clearing goroutine, drives every line low and closes the
// output. Later writes are dropped. Safe to call more than once and on any
// goroutine; only the first call does anything.
func (m *Marker) Close() error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		if m.done != nil {
			close(m.done)
			m.wg.Wait()
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.closed = true
		if m.out == nil {
			return
		}
		if err := m.out.AllLow(); err != nil {
			m.closeErr = fmt.Errorf("apparatus.Marker.Close: driving lines low: %w", err)
		}
		if err := m.out.Close(); err != nil && m.closeErr == nil {
			m.closeErr = fmt.Errorf("apparatus.Marker.Close: %w", err)
		}
	})
	return m.closeErr
}

// ── Update hooks ─────────────────────────────────────────────────────────────

// beginFrame advances the square's state for the frame about to be presented
// and reports whether it is white. An onset armed while the square is still
// white restarts the count: the two flashes merge.
func (m *Marker) beginFrame() bool {
	if m.armed {
		if m.left > 0 {
			m.merged.Add(1)
		}
		m.left = m.frames
	}
	m.white = m.left > 0
	return m.white
}

// afterFlip runs right after the frame is presented. The code is written
// first: it is the timing-critical part, and everything else can wait the few
// microseconds the write takes.
func (m *Marker) afterFlip(flipNS uint64) {
	if m.armed {
		if m.code != 0 && m.out != nil && m.sendCode(m.code) {
			m.codes.Add(1)
		}
		m.armed = false
		m.lastOnset = flipNS
		m.onsets.Add(1)
	}
	if m.left > 0 {
		m.left--
	}
	m.lastWhite = m.white
}

// drawSquare draws the square on top of the finished frame, white or black.
// Draw colour and blend mode are restored: the square must not leak state
// into the next frame's stimuli. Blend mode is forced to NONE, as in
// fillWholeTarget, so a stimulus that left a blending mode set cannot tint it.
func (m *Marker) drawSquare(r *sdl.Renderer) error {
	white := m.beginFrame()
	if !m.photodiode {
		return nil
	}
	prevColor, err := r.DrawColor()
	if err != nil {
		return fmt.Errorf("apparatus.Marker: reading draw color: %w", err)
	}
	prevBlend, err := r.DrawBlendMode()
	if err != nil {
		return fmt.Errorf("apparatus.Marker: reading blend mode: %w", err)
	}
	if prevBlend != sdl.BLENDMODE_NONE {
		if err := r.SetDrawBlendMode(sdl.BLENDMODE_NONE); err != nil {
			return fmt.Errorf("apparatus.Marker: setting blend mode: %w", err)
		}
		defer func() { _ = r.SetDrawBlendMode(prevBlend) }()
	}
	var v uint8
	if white {
		v = 255
	}
	if err := r.SetDrawColor(v, v, v, 255); err != nil {
		return fmt.Errorf("apparatus.Marker: setting draw color: %w", err)
	}
	if err := r.RenderFillRect(&m.rect); err != nil {
		return fmt.Errorf("apparatus.Marker: drawing the square: %w", err)
	}
	return r.SetDrawColor(prevColor.R, prevColor.G, prevColor.B, prevColor.A)
}

// ── TTL output ───────────────────────────────────────────────────────────────

// sendCode writes code and schedules the clear. It reports whether the write
// succeeded. A write failure is logged once and counted, not returned: a
// session should not die over a trigger, and the count goes into its metadata.
func (m *Marker) sendCode(code byte) bool {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return false
	}
	if m.high {
		m.overlaps.Add(1)
	}
	err := m.write(code)
	if err == nil {
		m.high = true
		m.cur = code
		m.clearAt = time.Now().Add(m.ttlDur)
	}
	m.mu.Unlock()
	if err != nil {
		m.writeFailed(err)
		return false
	}
	select {
	case m.poke <- struct{}{}:
	default: // a wake-up is already pending; it will read the new clearAt
	}
	return true
}

func (m *Marker) writeFailed(err error) {
	if m.writeErrs.Add(1) == 1 {
		log.Printf("WARNING: marker TTL write to %s failed: %v (further failures are counted, not logged)", m.outDesc, err)
	}
}

// write puts code on the lines; m.mu must be held. A single-bit code written
// to lines that are all low goes out as one SetHigh when the device can do
// that (see markerLineOutput); anything else is a Send.
func (m *Marker) write(code byte) error {
	if lo, ok := m.out.(markerLineOutput); ok && !m.high && singleBit(code) {
		return lo.SetHigh(bits.TrailingZeros8(code))
	}
	return m.out.Send(code)
}

// clearLines drives the lines back to 0; m.mu must be held. A single line is
// lowered on its own, for the reason write raises it on its own.
func (m *Marker) clearLines() error {
	if lo, ok := m.out.(markerLineOutput); ok && singleBit(m.cur) {
		return lo.SetLow(bits.TrailingZeros8(m.cur))
	}
	return m.out.Send(0)
}

func singleBit(b byte) bool { return b != 0 && b&(b-1) == 0 }

// maxClearRetries is how many times a failed clear is retried, 1 ms apart.
const maxClearRetries = 3

// clearWorker drives the lines back to 0 once a code has been on them for
// TTLDuration. One goroutine and one timer for the whole session. A code
// written before the previous one was cleared moves clearAt later, so the
// worker never clears early. The falling edge carries no information, so the
// timer's slack (around a millisecond) does not matter; spinning for a precise
// edge would compete with the real-time thread that presents.
func (m *Marker) clearWorker() {
	defer m.wg.Done()
	t := time.NewTimer(time.Hour)
	t.Stop()
	retries := 0
	for {
		select {
		case <-m.done:
			t.Stop()
			return
		case <-m.poke:
			m.mu.Lock()
			high, at := m.high, m.clearAt
			m.mu.Unlock()
			if high {
				t.Reset(time.Until(at))
			}
		case <-t.C:
			m.mu.Lock()
			if m.high && !m.closed {
				if d := time.Until(m.clearAt); d > 0 {
					t.Reset(d)
				} else if err := m.clearLines(); err != nil && retries < maxClearRetries {
					// Try again shortly rather than leave the lines high; a
					// device that keeps failing is given up on, not polled.
					retries++
					m.mu.Unlock()
					m.writeFailed(err)
					t.Reset(time.Millisecond)
					continue
				} else {
					if err != nil {
						m.writeFailed(err)
					} else {
						m.clears.Add(1)
					}
					m.high = false
					retries = 0
				}
			}
			m.mu.Unlock()
		}
	}
}
