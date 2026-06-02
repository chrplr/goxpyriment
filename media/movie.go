// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package media

import (
	"fmt"
	"sync"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/funatsufumiya/go-gv-video/gvvideo"
)

// ScaleMode controls how a Movie's native frame size maps onto the screen.
type ScaleMode int

const (
	// ScaleNone draws the frame at its native pixel size.
	ScaleNone ScaleMode = iota
	// ScaleFit fits the frame inside the screen while preserving aspect
	// ratio (PsyScope's Scale_X Scale_Y flags equivalent).
	ScaleFit
	// ScaleFill fills the screen with the frame, cropping to maintain
	// aspect ratio.
	ScaleFill
)

// Movie is a single playable .gv movie under a MovieManager's master
// clock. It mirrors the role of PSMovieDecoder in PsyScope Tahoe
// (PsyScopeEngine/Media/PSMovieDecoder.swift in that repository).
//
// A Movie is silent — audio is out of scope for this iteration (see
// media/Plan.md §1).
type Movie struct {
	// Tag is the movie identifier used by Display[Onset] events. It is
	// also reported on Onset.Name. Empty Tag disables Display events
	// for this movie.
	Tag string

	mgr    *MovieManager
	gv     *gvvideo.GVVideo
	width  float32
	height float32
	fps    float64
	fcount int

	// GPU + CPU buffers — texture allocated lazily on first Draw via
	// MovieManager.advanceMovie; rgba and compressedBuf allocated in NewMovie
	// so per-frame decode never allocates.
	texture       *sdl.Texture
	rgba          []byte // decompressed RGBA buffer, one frame
	compressedBuf []byte // LZ4-compressed scratch buffer, max frame size

	// Configuration (set at NewMovie time, may be mutated by setters).
	rate         float64
	repeatCount  int // -1 = infinite
	fromTime     time.Duration
	persistent   bool
	fadeIn       time.Duration
	position     sdl.FPoint
	scale        ScaleMode
	explicitW    float32 // 0 = use scale-mode-derived size
	explicitH    float32
	blendMode    sdl.BlendMode
	blendModeSet bool // false = leave SDL default; true = call SetBlendMode after texture allocation

	// Synchronised mutable state.
	mu                  sync.Mutex
	isActive            bool // Play called and not Stopped
	isPaused            bool
	mediaTimeOffset     time.Duration
	pauseStartMediaTime time.Duration
	loopRegionWindow    time.Duration

	currentFrameIndex  int // cumulative 1-based; advanced by NotifyFlipped
	totalFramesDecoded int // cumulative 1-based; advanced by DrawWithoutFlip
	loopCounter        int
	done               bool

	conditions []*timeCondition
	doneCh     chan Onset // lazily created by DoneCh
}

// Option mutates a Movie at construction. See WithTag, WithRate, etc.
type Option func(*Movie)

// WithTag sets the Movie's identifier (used for Display[Onset] events).
func WithTag(tag string) Option { return func(m *Movie) { m.Tag = tag } }

// WithRate sets the initial playback rate multiplier. 1.0 = normal,
// 0.5 = half speed, 2.0 = double speed. Must be > 0.
func WithRate(r float64) Option { return func(m *Movie) { m.rate = r } }

// WithRepeat sets the loop count. -1 = infinite, 1 = play once (default).
func WithRepeat(n int) Option { return func(m *Movie) { m.repeatCount = n } }

// WithFromTime sets the starting position within the movie. Defaults to 0.
func WithFromTime(d time.Duration) Option { return func(m *Movie) { m.fromTime = d } }

// WithScale sets the scale mode. Defaults to ScaleNone. Ignored when
// WithSize is also set.
func WithScale(s ScaleMode) Option { return func(m *Movie) { m.scale = s } }

// WithSize forces an explicit destination size in screen pixels,
// overriding ScaleMode. Both width and height must be > 0 to take effect.
// Use for fitting movies into a sub-region of the window (e.g., side by
// side); use WithScale otherwise.
func WithSize(w, h float32) Option {
	return func(m *Movie) {
		m.explicitW = w
		m.explicitH = h
	}
}

// WithBlendMode sets the SDL blend mode applied to the movie's texture.
// Defaults to SDL's built-in mode for streaming textures (typically
// BLENDMODE_NONE — opaque overwrite). Use sdl.BLENDMODE_BLEND for
// alpha-aware compositing, sdl.BLENDMODE_ADD for additive ("brighter
// where overlapping") blending, or sdl.BLENDMODE_MOD for modulated
// ("darker where overlapping").
//
// The blend mode is applied to the texture immediately after lazy
// allocation in MovieManager.DrawWithoutFlip; subsequent SetBlendMode
// calls take effect on the next frame's render.
func WithBlendMode(mode sdl.BlendMode) Option {
	return func(m *Movie) {
		m.blendMode = mode
		m.blendModeSet = true
	}
}

// WithPosition sets the center-relative draw position. Defaults to (0, 0).
func WithPosition(p sdl.FPoint) Option { return func(m *Movie) { m.position = p } }

// WithFadeIn sets a linear opacity ramp from 0 to 1 over the given
// duration of effective media time. Currently advisory: applied to the
// renderer's draw color alpha when non-zero.
func WithFadeIn(d time.Duration) Option { return func(m *Movie) { m.fadeIn = d } }

// WithPersistent makes the Movie's last frame stay on screen after the
// movie reaches Done (PsyScope's ClearType NO_CLEAR equivalent).
func WithPersistent(b bool) Option { return func(m *Movie) { m.persistent = b } }

// NewMovie wraps an already-loaded gvvideo.GVVideo in a Movie and
// registers it with the manager. The caller retains responsibility for
// closing the underlying gvvideo.GVVideo when the Movie is no longer
// needed; Movie.Close releases the GPU texture but does not touch the
// underlying file.
func NewMovie(mgr *MovieManager, gv *gvvideo.GVVideo, opts ...Option) (*Movie, error) {
	if mgr == nil {
		return nil, fmt.Errorf("media.NewMovie: nil MovieManager")
	}
	if gv == nil {
		return nil, fmt.Errorf("media.NewMovie: nil GVVideo")
	}
	m := &Movie{
		mgr:         mgr,
		gv:          gv,
		width:       float32(gv.Header.Width),
		height:      float32(gv.Header.Height),
		fps:         float64(gv.Header.FPS),
		fcount:      int(gv.Header.FrameCount),
		rate:        1.0,
		repeatCount: 1,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.rate <= 0 {
		return nil, fmt.Errorf("media.NewMovie: rate must be > 0, got %v", m.rate)
	}
	if m.fps <= 0 {
		return nil, fmt.Errorf("media.NewMovie: GV file reports fps=%v", m.fps)
	}
	if m.fcount <= 0 {
		return nil, fmt.Errorf("media.NewMovie: GV file reports frame count=%d", m.fcount)
	}
	m.rgba = make([]byte, gv.Header.FrameBytes)
	m.compressedBuf = make([]byte, gvMaxCompressedSize(gv))
	mgr.add(m)
	return m, nil
}

// FPS returns the movie's frames-per-second from the GV header.
func (m *Movie) FPS() float64 { return m.fps }

// FrameCount returns the number of frames per loop iteration.
func (m *Movie) FrameCount() int { return m.fcount }

// Repeat returns the configured loop count (-1 = infinite).
func (m *Movie) Repeat() int { return m.repeatCount }

// Width and Height return the movie's native pixel dimensions.
func (m *Movie) Width() float32  { return m.width }
func (m *Movie) Height() float32 { return m.height }

// Frame returns the cumulative 1-based index of the frame currently
// displayed (advanced by NotifyFlipped). Returns 0 before the first
// frame appears on screen.
func (m *Movie) Frame() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentFrameIndex
}

// LoopCounter returns the 0-based loop iteration currently playing.
func (m *Movie) LoopCounter() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loopCounter
}

// Time returns the current effective media time within the movie
// (rate-scaled, cumulative across loops).
func (m *Movie) Time() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.effectiveLocked(m.mgr.clock.Now())
}

// IsPaused reports whether the movie is currently paused.
func (m *Movie) IsPaused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isPaused
}

// IsActive reports whether the movie has been Play-ed and not Stopped.
func (m *Movie) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isActive
}

// IsDone reports whether the movie has reached end-of-file.
func (m *Movie) IsDone() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.done
}

// Play starts the movie if it has not started, or resumes it if paused.
// On a fresh start the playhead is positioned at the configured
// WithFromTime offset (default 0). Calling Play on an already-playing,
// non-paused Movie is a no-op.
func (m *Movie) Play() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.mgr.clock.Now()
	if !m.isActive {
		// Sequential start (MultiMovieSyncStrategy.md §3.4): anchor the
		// offset so effective media time begins at fromTime regardless
		// of how long the manager's master clock has been running.
		m.mediaTimeOffset = now - time.Duration(float64(m.fromTime)/m.rate)
		m.isActive = true
		m.isPaused = false
		m.done = false
		m.loopCounter = 0
		m.currentFrameIndex = 0
		m.totalFramesDecoded = 0
		for _, c := range m.conditions {
			c.fired = false
		}
		return
	}
	if m.isPaused {
		m.mediaTimeOffset += now - m.pauseStartMediaTime
		m.isPaused = false
		m.loopRegionWindow = 0
	}
}

// Pause freezes the movie at its current frame. Idempotent.
func (m *Movie) Pause() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.isActive || m.isPaused {
		return
	}
	m.pauseStartMediaTime = m.mgr.clock.Now()
	m.isPaused = true
	m.loopRegionWindow = 0
}

// PauseWithLoop freezes the movie and sawtooths playback through
// |window| of media-time around the pause point. Positive window
// cycles forward (pause point → +|window|, snap back, repeat);
// negative window cycles backward. Window of zero falls back to Pause.
//
// Synchrony note: when called inside a MovieManager.BeginBurst /
// EndBurst pair, all participating movies snapshot the same
// pauseStartMediaTime, so they sawtooth in perfect lockstep.
//
// Trigger note: registered conditions (OnAt, OnAtDisplay, OnDone)
// fire ONCE per registration across the whole sawtooth — they are
// NOT re-armed on each forward sweep. To re-fire on every loop pass,
// re-register the condition inside the callback (see the
// scheduleAtPlusN helper in examples/bouncing_gv_movies/main.go for
// the unsub + re-register pattern). To rewind and re-arm everything,
// call SeekFrame(1) or Stop+Play.
func (m *Movie) PauseWithLoop(window time.Duration) {
	if window == 0 {
		m.Pause()
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.isActive {
		return
	}
	m.pauseStartMediaTime = m.mgr.clock.Now()
	m.isPaused = true
	m.loopRegionWindow = window
}

// Stop halts the movie, releases the GPU texture, and resets state.
// After Stop a Movie can be Play-ed again (it restarts from fromTime).
func (m *Movie) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isActive = false
	m.isPaused = false
	m.done = false
	m.currentFrameIndex = 0
	m.totalFramesDecoded = 0
	m.loopCounter = 0
	m.mediaTimeOffset = 0
	m.loopRegionWindow = 0
	if m.texture != nil {
		m.texture.Destroy()
		m.texture = nil
	}
	for _, c := range m.conditions {
		c.fired = false
	}
}

// SetRate changes the playback rate multiplier. The current effective
// media time is preserved across the rate change.
func (m *Movie) SetRate(r float64) error {
	if r <= 0 {
		return fmt.Errorf("media.Movie.SetRate: rate must be > 0, got %v", r)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.mgr.clock.Now()
	eff := m.effectiveLocked(now)
	m.rate = r
	// Want effectiveLocked(now) == eff after the rate change.
	// effective = (now - offset) * rate  →  offset = now - eff/rate
	m.mediaTimeOffset = now - time.Duration(float64(eff)/r)
	if m.isPaused {
		m.pauseStartMediaTime = now
	}
	return nil
}

// SeekTime jumps to the given effective media time and forces a re-decode
// on the next Draw. Conditions reset their fired flags so a re-pass over
// the same target re-fires.
func (m *Movie) SeekTime(d time.Duration) error {
	if d < 0 {
		return fmt.Errorf("media.Movie.SeekTime: negative time %v", d)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.mgr.clock.Now()
	m.mediaTimeOffset = now - time.Duration(float64(d)/m.rate)
	if m.isPaused {
		m.pauseStartMediaTime = now
	}
	m.totalFramesDecoded = 0
	m.currentFrameIndex = 0
	m.loopCounter = 0
	m.done = false
	for _, c := range m.conditions {
		c.fired = false
	}
	return nil
}

// SeekFrame jumps to the given cumulative 1-based frame.
//
//	mov.SeekFrame(1)              // first frame of first loop
//	mov.SeekFrame(mov.FrameCount()+1)  // first frame of second loop
func (m *Movie) SeekFrame(n int) error {
	if n < 1 {
		return fmt.Errorf("media.Movie.SeekFrame: frame %d < 1", n)
	}
	maxFrame := m.fcount
	if m.repeatCount > 0 {
		maxFrame = m.fcount * m.repeatCount
	}
	if m.repeatCount > 0 && n > maxFrame {
		return fmt.Errorf("media.Movie.SeekFrame: frame %d > %d (frames per loop %d × loops %d)", n, maxFrame, m.fcount, m.repeatCount)
	}
	d := time.Duration(float64(n-1) / m.fps * float64(time.Second))
	return m.SeekTime(d)
}

// SetPosition updates the center-relative draw position. Safe to call
// every frame from an animation loop.
func (m *Movie) SetPosition(p sdl.FPoint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.position = p
}

// Position returns the current center-relative draw position.
func (m *Movie) Position() sdl.FPoint {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.position
}

// SetSize updates the explicit destination size. Both w and h must be
// > 0 to take effect; passing (0, 0) clears the explicit size and
// reverts to the scale-mode-derived geometry. Safe to call every frame.
func (m *Movie) SetSize(w, h float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.explicitW = w
	m.explicitH = h
}

// SetBlendMode updates the SDL blend mode applied to the movie's texture.
// If the texture is already allocated, the change takes effect immediately;
// otherwise it is stored and applied at lazy-allocation time.
func (m *Movie) SetBlendMode(mode sdl.BlendMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blendMode = mode
	m.blendModeSet = true
	if m.texture != nil {
		return m.texture.SetBlendMode(mode)
	}
	return nil
}

// Rate returns the current playback rate multiplier.
func (m *Movie) Rate() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rate
}

// Size returns the current explicit destination size set via WithSize /
// SetSize. Returns (0, 0) when no explicit size is set (the renderer
// then derives size from the ScaleMode).
func (m *Movie) Size() (float32, float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.explicitW, m.explicitH
}

// BlendMode returns the current blend mode and whether it was
// explicitly set (via WithBlendMode or SetBlendMode). When isSet is
// false the texture uses SDL's default blend mode.
func (m *Movie) BlendMode() (mode sdl.BlendMode, isSet bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.blendMode, m.blendModeSet
}

// LoopRegionWindow returns the current PauseWithLoop window, or 0 when
// the movie is not in pause-with-loop mode. Sign indicates direction:
// positive = forward sawtooth, negative = backward sawtooth.
func (m *Movie) LoopRegionWindow() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loopRegionWindow
}

// Snapshot is an atomic read of every observable property of a Movie.
// Use Snapshot rather than calling individual getters in sequence when
// you need a coherent view (e.g., for data logging) — the Movie's mutex
// is held only once.
type Snapshot struct {
	// Identity
	Tag string

	// Playback state
	Frame              int           // cumulative 1-based displayed-frame index
	TotalFramesDecoded int           // cumulative 1-based decoded-frame counter (look-ahead)
	Time               time.Duration // effective media time (rate-scaled, cumulative)
	Rate               float64
	LoopCounter        int // 0-based loop iteration
	Repeat             int // configured loop count (-1 = infinite)
	IsActive           bool
	IsPaused           bool
	IsDone             bool
	LoopRegionWindow   time.Duration // PauseWithLoop window (0 if not in loop)

	// Geometry
	Position sdl.FPoint
	SizeW    float32 // explicit destination width; 0 if scale-derived
	SizeH    float32
	NativeW  float32
	NativeH  float32
	Scale    ScaleMode

	// Rendering
	BlendMode    sdl.BlendMode
	BlendModeSet bool
	FadeIn       time.Duration
	Persistent   bool

	// Configuration
	FromTime time.Duration
}

// Snapshot returns an atomic read of every observable property.
func (m *Movie) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{
		Tag:                m.Tag,
		Frame:              m.currentFrameIndex,
		TotalFramesDecoded: m.totalFramesDecoded,
		Time:               m.effectiveLocked(m.mgr.clock.Now()),
		Rate:               m.rate,
		LoopCounter:        m.loopCounter,
		Repeat:             m.repeatCount,
		IsActive:           m.isActive,
		IsPaused:           m.isPaused,
		IsDone:             m.done,
		LoopRegionWindow:   m.loopRegionWindow,
		Position:           m.position,
		SizeW:              m.explicitW,
		SizeH:              m.explicitH,
		NativeW:            m.width,
		NativeH:            m.height,
		Scale:              m.scale,
		BlendMode:          m.blendMode,
		BlendModeSet:       m.blendModeSet,
		FadeIn:             m.fadeIn,
		Persistent:         m.persistent,
		FromTime:           m.fromTime,
	}
}

// Close releases GPU resources. Does NOT close the underlying GVVideo.
func (m *Movie) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.texture != nil {
		m.texture.Destroy()
		m.texture = nil
	}
}

// effectiveLocked computes effective media time. Caller must hold m.mu.
//
// Three regimes:
//
//   - Active and not paused: standard linear playback,
//     raw = now - mediaTimeOffset.
//   - Paused with loopRegionWindow == 0: pinned at pause time.
//   - Paused with loopRegionWindow != 0 (PauseWithLoop): sawtooth around
//     the pause point. Positive window cycles forward through |window|
//     of media-time and snaps back; negative window cycles backward.
//     Both movies in the same MovieManager paused inside one
//     BeginBurst/EndBurst share the same pauseStartMediaTime, so they
//     stay perfectly in lockstep across the sawtooth.
func (m *Movie) effectiveLocked(now time.Duration) time.Duration {
	if !m.isActive {
		return 0
	}
	var raw time.Duration
	switch {
	case m.isPaused && m.loopRegionWindow != 0:
		window := m.loopRegionWindow
		if window < 0 {
			window = -window
		}
		elapsed := now - m.pauseStartMediaTime
		if elapsed < 0 {
			elapsed = 0
		}
		cycleOffset := time.Duration(int64(elapsed) % int64(window))
		base := m.pauseStartMediaTime - m.mediaTimeOffset
		if m.loopRegionWindow > 0 {
			raw = base + cycleOffset
		} else {
			raw = base - cycleOffset
			if raw < 0 {
				raw = 0
			}
		}
	case m.isPaused:
		raw = m.pauseStartMediaTime - m.mediaTimeOffset
	default:
		raw = now - m.mediaTimeOffset
	}
	return time.Duration(float64(raw) * m.rate)
}

// isVisibleLocked reports whether this movie's tag should appear in the
// MovieManager's visibility set for Display[Onset/Offset] tracking.
// Caller must hold m.mu.
func (m *Movie) isVisibleLocked() bool {
	if !m.isActive {
		return false
	}
	if m.totalFramesDecoded < 1 {
		return false
	}
	if m.done && !m.persistent {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Condition registration (Stages 3 and 4)
// ---------------------------------------------------------------------------

// OnDone registers a callback that fires once when the movie reaches
// end-of-file. Look-ahead: the callback fires from inside DrawWithoutFlip,
// before the final frame is presented. Returns an unsubscribe function.
func (m *Movie) OnDone(fn func(Onset)) func() {
	return m.addCondition(Done{}, false, fn)
}

// DoneCh returns a buffered channel that receives an Onset when the
// movie reaches end-of-file. The channel is created on first call and
// reused; only one consumer should read from it.
func (m *Movie) DoneCh() <-chan Onset {
	m.mu.Lock()
	if m.doneCh == nil {
		ch := make(chan Onset, 1)
		m.doneCh = ch
		m.mu.Unlock()
		m.OnDone(func(o Onset) {
			select {
			case ch <- o:
			default:
			}
		})
		return ch
	}
	ch := m.doneCh
	m.mu.Unlock()
	return ch
}

// OnAt registers a look-ahead callback that fires from the per-frame
// decode loop, before the target frame is presented this vsync.
//
// Use this for visual side effects that must land on the same vsync
// as the target frame: an overlay added in the callback will be drawn
// by the caller's compositor immediately after MovieManager.DrawWithoutFlip
// and presented in the same FlipTS as the movie's target frame.
//
// The Onset.Source is LookAhead; the TimestampNS is the SDL ticks at
// fire time, NOT a vsync timestamp.
func (m *Movie) OnAt(t Target, fn func(Onset)) func() {
	return m.addCondition(t, false, fn)
}

// OnAtDisplay registers a hardware-timed callback that fires from
// MovieManager.NotifyFlipped after the target frame has appeared on
// the display. The Onset.TimestampNS is the post-vsync FlipTS value
// (Source: VsyncEstimated until Stage 5 ships hardware-verified timing).
func (m *Movie) OnAtDisplay(t Target, fn func(Onset)) func() {
	m.mgr.maybeWarnPrecision()
	return m.addCondition(t, true, fn)
}

func (m *Movie) addCondition(t Target, displayDeferred bool, fn func(Onset)) func() {
	if fn == nil {
		return func() {}
	}
	m.mu.Lock()
	cond := &timeCondition{target: t, deferToDisplay: displayDeferred, fn: fn}
	m.conditions = append(m.conditions, cond)
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, c := range m.conditions {
			if c == cond {
				m.conditions = append(m.conditions[:i], m.conditions[i+1:]...)
				return
			}
		}
	}
}
