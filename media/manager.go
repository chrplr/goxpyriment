// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package media

import (
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/media/present"
)

// MovieManager owns the master clock and the set of registered Movies.
// It mirrors the role of PSMovieManager in PsyScope Tahoe
// (PsyScopeEngine/Media/PSMovieManager.swift).
//
// Per-frame call sites:
//
// Movie-only frame:
//
//	ts, _ := mgr.Draw()       // clear → decode → render → flip → notify
//
// Mixed frame (movies composited with other stimuli):
//
//	exp.Screen.Clear()
//	mgr.DrawWithoutFlip()
//	otherStim.Draw(exp.Screen)
//	ts, _ := exp.Screen.FlipTS()
//	mgr.NotifyFlipped(ts)
type MovieManager struct {
	screen       *apparatus.Screen
	clock        *MasterClock
	presentTimer present.Timer

	mu     sync.Mutex
	movies []*Movie

	// Display[Onset/Offset] tracking.
	extStimuli      map[string]func() bool
	onsetCallbacks  map[string][]onsetCB
	offsetCallbacks map[string][]onsetCB
	lastVisible     map[string]bool
	pendingVisible  map[string]bool
	pendingFlipTS   uint64

	// Stage 5: stash of (deferred + onset + offset) callbacks awaiting a
	// HardwareVerified vsync timestamp from the present timer. Drained
	// on each NotifyFlipped; entries that go stale (older than
	// staleFlipMultiplier × frame period) fire with VsyncEstimated as a
	// fallback and a one-time runtime warning.
	pendingFlips []pendingFlip

	precisionWarned bool
	timeoutWarned   bool
	closed          bool
}

// staleFlipMultiplier is how many frame periods a pending flip may
// remain in the queue before we give up waiting for its hardware-
// verified vsync and fall back to VsyncEstimated. 3 × frame period
// ≈ 50 ms at 60 Hz — generous enough to absorb GC pauses and one
// missed vsync, tight enough to keep the queue bounded.
const staleFlipMultiplier = 3

// pendingFlip captures all callbacks for one flip that are waiting on a
// hardware-verified vsync timestamp.
type pendingFlip struct {
	flipTS        uint64
	deferredFires []stagedCallback // OnAtDisplay
	onsetFires    []stagedCallback // Display[Onset]
	offsetFires   []stagedCallback // Display[Offset]
}

// ManagerOption mutates a MovieManager at construction.
type ManagerOption func(*MovieManager)

// WithPresentTimer overrides the default auto-detected presentation
// timer. Pass present.NewFallback() to force vsync-estimated mode for
// testing, or pass a custom Timer for non-default backends.
func WithPresentTimer(t present.Timer) ManagerOption {
	return func(mgr *MovieManager) { mgr.presentTimer = t }
}

type onsetCB struct {
	id uint64
	fn func(Onset)
}

// nextOnsetCBID is global so callback IDs are unique even across
// multiple MovieManagers in the same process.
var nextOnsetCBID atomic.Uint64

// stagedCallback defers a fire until after locks are released, keeping
// callbacks free to register/unregister or call back into the manager.
type stagedCallback struct {
	fn  func(Onset)
	arg Onset
}

// NewMovieManager constructs a manager wired to the given Screen.
//
// By default the manager auto-detects the best presentation timer for
// the platform: macOS CVDisplayLink, Linux DRM_IOCTL_WAIT_VBLANK, or
// vsync-estimated fallback elsewhere. Override with WithPresentTimer.
//
// Call Close before the Screen is destroyed (in the example pattern,
// before exp.End): on macOS this stops the CVDisplayLink background
// thread; on Linux it closes the /dev/dri/cardN file descriptor.
func NewMovieManager(screen *apparatus.Screen, opts ...ManagerOption) *MovieManager {
	mgr := &MovieManager{
		screen:          screen,
		clock:           NewMasterClock(),
		extStimuli:      make(map[string]func() bool),
		onsetCallbacks:  make(map[string][]onsetCB),
		offsetCallbacks: make(map[string][]onsetCB),
		lastVisible:     make(map[string]bool),
		pendingVisible:  make(map[string]bool),
	}
	for _, opt := range opts {
		opt(mgr)
	}
	if mgr.presentTimer == nil {
		mgr.presentTimer = present.AutoDetect(screen)
	}
	log.Printf("media: present backend: %s (precision=%s)",
		mgr.presentTimer.Description(), mgr.presentTimer.Precision())
	return mgr
}

// Close releases the present timer's OS resources. After Close the
// manager continues to function but with vsync-estimated precision
// only. Idempotent; safe to defer next to the manager's other resources.
func (mgr *MovieManager) Close() error {
	mgr.mu.Lock()
	if mgr.closed {
		mgr.mu.Unlock()
		return nil
	}
	mgr.closed = true
	t := mgr.presentTimer
	mgr.presentTimer = present.NewFallback()
	mgr.mu.Unlock()
	if t != nil {
		return t.Close()
	}
	return nil
}

// Clock returns the manager's master clock. Callers may Reset it at
// trial boundaries.
func (mgr *MovieManager) Clock() *MasterClock { return mgr.clock }

// BeginBurst forwards to the master clock.
func (mgr *MovieManager) BeginBurst() { mgr.clock.BeginBurst() }

// EndBurst forwards to the master clock.
func (mgr *MovieManager) EndBurst() { mgr.clock.EndBurst() }

// add registers a Movie. Called by NewMovie.
func (mgr *MovieManager) add(m *Movie) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.movies = append(mgr.movies, m)
}

// Remove deregisters a Movie. The movie's pending visibility is
// dropped, so an Offset for its tag will fire on the next NotifyFlipped
// if it was visible at the previous flip.
func (mgr *MovieManager) Remove(m *Movie) {
	mgr.mu.Lock()
	for i, mm := range mgr.movies {
		if mm == m {
			mgr.movies = append(mgr.movies[:i], mgr.movies[i+1:]...)
			break
		}
	}
	mgr.mu.Unlock()
}

// Movies returns a snapshot of currently registered Movies in
// registration order.
func (mgr *MovieManager) Movies() []*Movie {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	out := make([]*Movie, len(mgr.movies))
	copy(out, mgr.movies)
	return out
}

// LastFlipTS returns the most recent timestamp passed to NotifyFlipped,
// or zero if no flip has occurred yet.
func (mgr *MovieManager) LastFlipTS() uint64 {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.pendingFlipTS
}

// RegisterStimulusName declares a non-movie stimulus by name. The
// visibility predicate is queried inside Draw / DrawWithoutFlip; when
// its truth value changes between flips, OnDisplayOnset and
// OnDisplayOffset callbacks for the name fire from NotifyFlipped.
//
// Movies are auto-registered with their tag and need not be passed here.
//
// Returns an unregister function.
func (mgr *MovieManager) RegisterStimulusName(name string, visible func() bool) func() {
	if name == "" || visible == nil {
		return func() {}
	}
	mgr.mu.Lock()
	mgr.extStimuli[name] = visible
	mgr.mu.Unlock()
	return func() {
		mgr.mu.Lock()
		defer mgr.mu.Unlock()
		delete(mgr.extStimuli, name)
	}
}

// OnDisplayOnset fires when the named stimulus transitions from
// not-visible at the previous flip to visible at the current flip.
// Onset.TimestampNS is the post-vsync FlipTS value
// (Source: VsyncEstimated until Stage 5 ships).
func (mgr *MovieManager) OnDisplayOnset(name string, fn func(Onset)) func() {
	return mgr.addOnsetCallback(name, fn, false)
}

// OnDisplayOffset is the symmetric counterpart of OnDisplayOnset.
func (mgr *MovieManager) OnDisplayOffset(name string, fn func(Onset)) func() {
	return mgr.addOnsetCallback(name, fn, true)
}

func (mgr *MovieManager) addOnsetCallback(name string, fn func(Onset), offset bool) func() {
	if name == "" || fn == nil {
		return func() {}
	}
	mgr.maybeWarnPrecision()
	id := nextOnsetCBID.Add(1)
	cb := onsetCB{id: id, fn: fn}
	mgr.mu.Lock()
	if offset {
		mgr.offsetCallbacks[name] = append(mgr.offsetCallbacks[name], cb)
	} else {
		mgr.onsetCallbacks[name] = append(mgr.onsetCallbacks[name], cb)
	}
	mgr.mu.Unlock()
	return func() {
		mgr.mu.Lock()
		defer mgr.mu.Unlock()
		bucket := mgr.onsetCallbacks
		if offset {
			bucket = mgr.offsetCallbacks
		}
		list := bucket[name]
		for i, c := range list {
			if c.id == id {
				bucket[name] = append(list[:i], list[i+1:]...)
				return
			}
		}
	}
}

// maybeWarnPrecision emits a one-time runtime warning when the user
// registers a display-deferred callback against a presentation timer
// that only delivers vsync-period precision. Suppressed when the timer
// reports HardwareVerified. Required by media/Plan.md §10 (item 5) and
// the project rule against silent timing changes (root CLAUDE.md).
func (mgr *MovieManager) maybeWarnPrecision() {
	mgr.mu.Lock()
	if mgr.precisionWarned {
		mgr.mu.Unlock()
		return
	}
	mgr.precisionWarned = true
	timer := mgr.presentTimer
	mgr.mu.Unlock()
	if timer.Precision() == HardwareVerified {
		return
	}
	period := mgr.screen.FrameDuration()
	log.Printf("media: display-onset precision is vsync-period (~%v at this refresh rate). Install a sub-ms presentation backend (Stage 5: Vulkan VK_EXT_present_timing or Metal addPresentedHandler) for hardware-verified timing.", period)
}

// warnTimeoutOnce logs a single runtime warning when a flip's
// hardware-verified vsync timestamp doesn't arrive within
// staleFlipMultiplier × frame period and we fall back to VsyncEstimated.
func (mgr *MovieManager) warnTimeoutOnce(framePeriod time.Duration) {
	mgr.mu.Lock()
	if mgr.timeoutWarned {
		mgr.mu.Unlock()
		return
	}
	mgr.timeoutWarned = true
	mgr.mu.Unlock()
	log.Printf("media: hardware-verified vsync timestamp not delivered within %v; affected frames fall back to vsync-estimated. This typically indicates compositor latency or a backend-specific issue.",
		time.Duration(staleFlipMultiplier)*framePeriod)
}

// firePendingWithOnset fires each staged callback after overwriting its
// onset's TimestampNS and Source. Safe to call without holding any
// MovieManager lock.
func firePendingWithOnset(fires []stagedCallback, ts uint64, source OnsetSource) {
	for _, sc := range fires {
		sc.arg.TimestampNS = ts
		sc.arg.Source = source
		sc.fn(sc.arg)
	}
}

// Draw is the all-in-one per-frame entry point: clear, decode, render
// movies, flip, fire post-vsync callbacks. Use this for movie-only
// frames; for compositing with other stimuli use the
// DrawWithoutFlip → caller draws → FlipTS → NotifyFlipped pattern.
//
// Returns the post-flip SDL ticks (same reference as sdl.TicksNS).
func (mgr *MovieManager) Draw() (uint64, error) {
	if err := mgr.screen.Clear(); err != nil {
		return 0, fmt.Errorf("media.MovieManager.Draw: clearing screen: %w", err)
	}
	if err := mgr.DrawWithoutFlip(); err != nil {
		return 0, fmt.Errorf("media.MovieManager.Draw: %w", err)
	}
	ts, err := mgr.screen.FlipTS()
	if err != nil {
		return 0, fmt.Errorf("media.MovieManager.Draw: flipping display: %w", err)
	}
	mgr.NotifyFlipped(ts)
	return ts, nil
}

// DrawWithoutFlip decodes any frames newly due for each active movie,
// uploads the latest frame's pixels to the movie's texture, renders
// each movie's texture, and fires look-ahead OnAt and OnDone callbacks
// for any frames newly decoded this call. It does NOT clear the
// backbuffer or call Present — the caller owns those.
func (mgr *MovieManager) DrawWithoutFlip() error {
	mgr.mu.Lock()
	movies := append([]*Movie(nil), mgr.movies...)
	mgr.mu.Unlock()

	mgr.clock.BeginBurst()
	defer mgr.clock.EndBurst()

	now := mgr.clock.Now()

	var staged []stagedCallback
	pending := make(map[string]bool)

	for _, m := range movies {
		s, vis, err := mgr.advanceMovie(m, now)
		if err != nil {
			return fmt.Errorf("media.MovieManager.DrawWithoutFlip: advancing movie %q: %w", m.Tag, err)
		}
		staged = append(staged, s...)
		if vis && m.Tag != "" {
			pending[m.Tag] = true
		}
	}

	mgr.mu.Lock()
	for name, vis := range mgr.extStimuli {
		if vis() {
			pending[name] = true
		}
	}
	mgr.pendingVisible = pending
	mgr.mu.Unlock()

	// Fire look-ahead callbacks AFTER releasing all locks so callbacks
	// can re-enter the manager (e.g., to register a new condition or
	// add a stimulus).
	for _, sc := range staged {
		sc.fn(sc.arg)
	}
	return nil
}

// advanceMovie runs the per-frame decode + render for one movie and
// returns any look-ahead Onsets that should fire after locks release.
func (mgr *MovieManager) advanceMovie(m *Movie, now time.Duration) ([]stagedCallback, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isActive {
		return nil, false, nil
	}

	// Lazy GPU allocation. The renderer is on the SDL_Renderer thread
	// (the goxpyriment main loop); CreateTexture must be called from there.
	if m.texture == nil {
		tex, err := mgr.screen.Renderer.CreateTexture(
			sdl.PIXELFORMAT_RGBA32,
			sdl.TEXTUREACCESS_STREAMING,
			int(m.width), int(m.height),
		)
		if err != nil {
			return nil, false, fmt.Errorf("media: create texture for movie %q: %w", m.Tag, err)
		}
		m.texture = tex
		if m.blendModeSet {
			if err := m.texture.SetBlendMode(m.blendMode); err != nil {
				return nil, false, fmt.Errorf("media: set blend mode for movie %q: %w", m.Tag, err)
			}
		}
	}

	eff := m.effectiveLocked(now)

	// Cumulative target frame from effective time.
	totalTarget := int(math.Floor(eff.Seconds()*m.fps)) + 1
	if totalTarget < 1 {
		totalTarget = 1
	}
	maxTotal := math.MaxInt
	if m.repeatCount > 0 {
		maxTotal = m.fcount * m.repeatCount
	}
	endOfFile := false
	if totalTarget >= maxTotal {
		totalTarget = maxTotal
		endOfFile = true
	}

	var staged []stagedCallback

	// PauseWithLoop sawtooth wrap: when the effective time rolls
	// backward (the bounce point cycles past the pause origin), rewind
	// totalFramesDecoded so the decode loop can replay the cycle.
	//
	// IMPORTANT: condition fired-flags are NOT cleared here. A trigger
	// registered as one-shot (the default) stays one-shot across
	// sawtooth wraps — otherwise an auto trigger at, say, frame 50
	// would beep every cycle of a paused-with-loop region that crosses
	// frame 50, which is surprising. Explicit user actions that mean
	// "re-run the timeline" (SeekTime, SeekFrame, Stop) reset
	// fired-flags via their own code paths. To re-fire on every loop
	// iteration, re-register the condition inside the callback (see
	// the scheduleAtPlusN helper in examples/demo_bouncing_gv_movies/main.go
	// for the pattern).
	if totalTarget < m.totalFramesDecoded {
		m.totalFramesDecoded = totalTarget - 1
		if m.totalFramesDecoded < 0 {
			m.totalFramesDecoded = 0
		}
	}

	// Look-ahead pass: enumerate every newly-decoded cumulative frame
	// (no actual decode work yet — only condition firing) so that
	// Movie[At "f:N"] fires exactly when totalFramesDecoded reaches N.
	oldDecoded := m.totalFramesDecoded
	for step := oldDecoded + 1; step <= totalTarget; step++ {
		m.totalFramesDecoded = step
		m.loopCounter = (step - 1) / m.fcount
		stepTime := time.Duration(float64(step-1) / m.fps * float64(time.Second))
		for _, c := range m.conditions {
			if c.deferToDisplay {
				continue
			}
			if c.matches(step, m.currentFrameIndex, stepTime, false) {
				c.fired = true
				staged = append(staged, stagedCallback{
					fn: c.fn,
					arg: Onset{
						TimestampNS: sdl.TicksNS(),
						Frame:       step,
						Source:      LookAhead,
						Movie:       m,
						Name:        m.Tag,
					},
				})
			}
		}
	}

	if endOfFile {
		m.done = true
		// Look-ahead Done firing.
		for _, c := range m.conditions {
			if c.deferToDisplay {
				continue
			}
			if c.matches(m.totalFramesDecoded, m.currentFrameIndex, eff, true) {
				c.fired = true
				staged = append(staged, stagedCallback{
					fn: c.fn,
					arg: Onset{
						TimestampNS: sdl.TicksNS(),
						Frame:       m.totalFramesDecoded,
						Source:      LookAhead,
						Movie:       m,
						Name:        m.Tag,
					},
				})
			}
		}
	}

	// Decompress + upload only the latest frame (LZ4 + RGBA copy is the
	// expensive path; intermediate steps were enumerated above for
	// condition firing only).
	if m.totalFramesDecoded != oldDecoded && m.totalFramesDecoded >= 1 {
		withinLoop := uint32((m.totalFramesDecoded - 1) % m.fcount)
		if err := gvReadFrameInto(m.gv, withinLoop, m.compressedBuf, m.rgba); err != nil {
			return staged, false, fmt.Errorf("media: decode movie %q frame %d (loop %d): %w",
				m.Tag, withinLoop+1, m.loopCounter, err)
		}
		if err := m.texture.Update(nil, m.rgba, int32(m.gv.Header.Width*4)); err != nil {
			return staged, false, fmt.Errorf("media: upload movie %q frame: %w", m.Tag, err)
		}
	}

	visible := m.isVisibleLocked()

	// Render this movie's texture (unless ended without persistent).
	if visible {
		dest := mgr.computeDest(m)
		// FadeIn ramp: linear opacity 0→1 over m.fadeIn of effective time.
		if m.fadeIn > 0 && eff < m.fadeIn {
			alpha := uint8(255.0 * float64(eff) / float64(m.fadeIn))
			_ = m.texture.SetAlphaMod(alpha)
			defer func() { _ = m.texture.SetAlphaMod(255) }()
		}
		if err := mgr.screen.Renderer.RenderTexture(m.texture, nil, &dest); err != nil {
			return staged, visible, fmt.Errorf("media: render movie %q: %w", m.Tag, err)
		}
	}

	return staged, visible, nil
}

// computeDest builds the destination FRect for a Movie based on its
// scale mode (or explicit WithSize) and position.
//
// Sizes and positions are in the renderer's logical coordinate space —
// the same space CenterToSDL uses (LogicalSize if SetLogicalSize was
// called, otherwise Renderer.CurrentOutputSize). NOT Screen.Size, which
// returns physical render-output pixels and would be 2x larger than
// expected on a HiDPI/Retina display when a logical presentation is in
// effect.
func (mgr *MovieManager) computeDest(m *Movie) sdl.FRect {
	var w, h float32
	switch {
	case m.explicitW > 0 && m.explicitH > 0:
		w, h = m.explicitW, m.explicitH
	case m.scale == ScaleFit || m.scale == ScaleFill:
		lw, lh := mgr.logicalSize()
		sx := lw / m.width
		sy := lh / m.height
		s := sx
		if (m.scale == ScaleFit && sy < s) || (m.scale == ScaleFill && sy > s) {
			s = sy
		}
		w, h = m.width*s, m.height*s
	default: // ScaleNone
		w, h = m.width, m.height
	}
	cx, cy := mgr.screen.CenterToSDL(m.position.X, m.position.Y)
	return sdl.FRect{X: cx - w/2, Y: cy - h/2, W: w, H: h}
}

// logicalSize returns the renderer's effective output size in the same
// coordinate space CenterToSDL uses. Caller-side layout code should
// prefer this over apparatus.Screen.Size (which returns physical pixels).
func (mgr *MovieManager) logicalSize() (float32, float32) {
	if mgr.screen.LogicalSize != nil {
		return mgr.screen.LogicalSize.X, mgr.screen.LogicalSize.Y
	}
	w, h, err := mgr.screen.Renderer.CurrentOutputSize()
	if err != nil || w <= 0 || h <= 0 {
		// Last-resort fallback — physical pixels. Better than zero.
		w, h, _ = mgr.screen.Size()
	}
	return float32(w), float32(h)
}

// LogicalSize exposes the renderer's effective coordinate-space size
// for callers that need it for their own layout (e.g., positioning
// movies side by side via WithSize).
func (mgr *MovieManager) LogicalSize() (float32, float32) {
	return mgr.logicalSize()
}

// NotifyFlipped is called after Screen.FlipTS to publish post-vsync
// events: it advances each movie's currentFrameIndex to its
// totalFramesDecoded value, fires display-deferred OnAtDisplay
// callbacks, and emits Display[Onset]/[Offset] for any names that
// changed visibility since the previous flip.
//
// ts must be the value returned by Screen.FlipTS for the just-presented
// frame (same reference as sdl.TicksNS).
//
// When the manager has a HardwareVerified present timer, callbacks are
// fired with the OS-measured first-pixel-visible timestamp instead of
// the post-Present FlipTS. If the OS timestamp is not yet available at
// the time of NotifyFlipped (e.g., the CVDisplayLink callback hasn't
// fired yet for this vsync), the callbacks are deferred to a future
// NotifyFlipped — typically the next frame, ~16 ms later — and fired
// then with the correct vsync timestamp. This matches PsyScope Tahoe's
// pendingMovieAtDisplay deferral pattern.
func (mgr *MovieManager) NotifyFlipped(ts uint64) {
	// Hand the FlipTS to the present timer first. Linux DRM uses this
	// hook to query DRM_IOCTL_WAIT_VBLANK for the just-completed vsync;
	// macOS CVDisplayLink treats it as a no-op (vsync timestamps
	// publish asynchronously via the CV callback).
	mgr.presentTimer.RecordFlip(ts)

	mgr.mu.Lock()
	movies := append([]*Movie(nil), mgr.movies...)
	pending := mgr.pendingVisible
	last := mgr.lastVisible
	mgr.lastVisible = pending
	mgr.pendingFlipTS = ts
	// Snapshot callback maps so firing happens off-lock.
	onsetSnap := make(map[string][]onsetCB, len(mgr.onsetCallbacks))
	for k, v := range mgr.onsetCallbacks {
		onsetSnap[k] = append([]onsetCB(nil), v...)
	}
	offsetSnap := make(map[string][]onsetCB, len(mgr.offsetCallbacks))
	for k, v := range mgr.offsetCallbacks {
		offsetSnap[k] = append([]onsetCB(nil), v...)
	}
	movieByTag := make(map[string]*Movie, len(mgr.movies))
	for _, m := range mgr.movies {
		if m.Tag != "" {
			movieByTag[m.Tag] = m
		}
	}
	mgr.mu.Unlock()

	// Advance each movie's currentFrameIndex and stage its
	// display-deferred conditions.
	var deferredFires []stagedCallback
	for _, m := range movies {
		m.mu.Lock()
		m.currentFrameIndex = m.totalFramesDecoded
		eff := m.effectiveLocked(mgr.clock.Now())
		for _, c := range m.conditions {
			if !c.deferToDisplay {
				continue
			}
			if c.matches(m.totalFramesDecoded, m.currentFrameIndex, eff, m.done) {
				c.fired = true
				deferredFires = append(deferredFires, stagedCallback{
					fn: c.fn,
					arg: Onset{
						// TimestampNS and Source filled by firePendingWithOnset.
						Frame: m.currentFrameIndex,
						Movie: m,
						Name:  m.Tag,
					},
				})
			}
		}
		m.mu.Unlock()
	}

	// Display[Onset] / Display[Offset] diff.
	var onsetFires, offsetFires []stagedCallback
	for name := range pending {
		if !last[name] {
			for _, cb := range onsetSnap[name] {
				onsetFires = append(onsetFires, stagedCallback{
					fn: cb.fn,
					arg: Onset{
						Name:  name,
						Movie: movieByTag[name],
					},
				})
			}
		}
	}
	for name := range last {
		if !pending[name] {
			for _, cb := range offsetSnap[name] {
				offsetFires = append(offsetFires, stagedCallback{
					fn: cb.fn,
					arg: Onset{
						Name:  name,
						Movie: movieByTag[name],
					},
				})
			}
		}
	}

	// Try to get the hardware-verified onset for THIS flip.
	onset, source, ok := mgr.presentTimer.OnsetForFlip(ts)
	if ok {
		// Common case: OS timestamp already available, fire immediately.
		// Order: per-movie OnAtDisplay, then Display[Onset], then
		// Display[Offset]. The last two are independent across names but
		// ordered for reproducibility.
		firePendingWithOnset(deferredFires, onset, source)
		firePendingWithOnset(onsetFires, onset, source)
		firePendingWithOnset(offsetFires, onset, source)
	} else if len(deferredFires)+len(onsetFires)+len(offsetFires) > 0 {
		// Defer this flip's callbacks; they'll fire from a future
		// NotifyFlipped once the OS publishes the matching vsync
		// timestamp.
		mgr.mu.Lock()
		mgr.pendingFlips = append(mgr.pendingFlips, pendingFlip{
			flipTS:        ts,
			deferredFires: deferredFires,
			onsetFires:    onsetFires,
			offsetFires:   offsetFires,
		})
		mgr.mu.Unlock()
	}

	// Drain any previously-deferred flips whose vsync has now arrived
	// (or which have aged out of the wait window).
	mgr.processPendingFlips()
}

// processPendingFlips drains the pendingFlips queue: fires any flip
// whose hardware-verified vsync is now available, gives up on flips
// older than staleFlipMultiplier × frame period (firing them with
// VsyncEstimated as a fallback), and re-queues the rest.
func (mgr *MovieManager) processPendingFlips() {
	mgr.mu.Lock()
	pending := mgr.pendingFlips
	mgr.pendingFlips = nil
	mgr.mu.Unlock()
	if len(pending) == 0 {
		return
	}

	framePeriod := mgr.screen.FrameDuration()
	staleNS := uint64(time.Duration(staleFlipMultiplier) * framePeriod)
	nowSDL := sdl.TicksNS()

	type readyFlip struct {
		flip   pendingFlip
		ts     uint64
		source OnsetSource
	}
	var ready []readyFlip
	var stillPending []pendingFlip

	for _, p := range pending {
		onset, source, ok := mgr.presentTimer.OnsetForFlip(p.flipTS)
		switch {
		case ok:
			ready = append(ready, readyFlip{p, onset, source})
		case nowSDL > p.flipTS+staleNS:
			mgr.warnTimeoutOnce(framePeriod)
			ready = append(ready, readyFlip{p, p.flipTS, VsyncEstimated})
		default:
			stillPending = append(stillPending, p)
		}
	}

	if len(stillPending) > 0 {
		mgr.mu.Lock()
		mgr.pendingFlips = append(stillPending, mgr.pendingFlips...)
		mgr.mu.Unlock()
	}

	for _, r := range ready {
		firePendingWithOnset(r.flip.deferredFires, r.ts, r.source)
		firePendingWithOnset(r.flip.onsetFires, r.ts, r.source)
		firePendingWithOnset(r.flip.offsetFires, r.ts, r.source)
	}
}
