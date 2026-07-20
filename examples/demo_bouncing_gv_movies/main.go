// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// bouncing_gv_movies plays two .gv movies that smoothly bounce around
// the screen while their sizes oscillate, additively blending wherever
// they overlap. Demonstrates every runtime mutator the media package
// exposes:
//
//   - mov.Play / Pause / PauseWithLoop / Stop
//   - mov.SetRate (accelerate / decelerate, with bounds)
//   - mov.SeekFrame (rewind to start)
//   - mov.SetPosition / SetSize (per-frame animation hooks)
//   - mov.SetBlendMode (toggle blend mode at runtime)
//
// Plus two automatic OnAtDisplay triggers (left frame 50 → low beep,
// right frame 80 → high beep) that record both movies' full state via
// mov.Snapshot to the experiment data file with the hardware-verified
// onset timestamp.
//
// Multi-movie synchrony invariant: every command that touches more
// than one movie's playback state (rate, pause, seek, pause-with-loop)
// is wrapped in mgr.BeginBurst / EndBurst, so both movies observe the
// SAME MasterClock.Now value when they apply the change. There is no
// drift across pause/resume, rate change, or PauseWithLoop sawtooth
// cycles.
//
// See examples/bouncing_gv_movies/README.md for the full key cheatsheet.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/funatsufumiya/go-gv-video/gvvideo"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/media"
	"github.com/chrplr/goxpyriment/stimuli"
)

// Tunable bounds on rate and bounce speed (defensively clamped).
const (
	minRate     = 0.1
	maxRate     = 8.0
	rateStep    = 1.25 // multiply by this on +; divide by this on -
	speedStepUp = 1.25
	speedStepDn = 0.8
)

// movieAnim holds the per-movie animation state for one bouncing,
// pulsing movie. Distances are in the renderer's logical coordinate
// space (the same space mgr.LogicalSize and Screen.CenterToSDL use).
type movieAnim struct {
	cx, cy     float32 // center position, screen-center-relative
	vx, vy     float32 // velocity in px/sec
	sizePhase  float32 // current phase of the size sin-wave (radians)
	sizeAngVel float32 // angular velocity for size phase (rad/sec)
	sizeMinW   float32
	sizeMaxW   float32
	aspect     float32
}

// advance integrates the animation by dt seconds and applies the bounce
// rule (reverse on velocity component when center reaches screen edge).
func (a *movieAnim) advance(dt, screenW, screenH float32) (cx, cy, w, h float32) {
	a.sizePhase += a.sizeAngVel * dt
	s := (float32(math.Sin(float64(a.sizePhase))) + 1) / 2
	w = a.sizeMinW + (a.sizeMaxW-a.sizeMinW)*s
	h = w / a.aspect

	a.cx += a.vx * dt
	a.cy += a.vy * dt

	halfSW := screenW / 2
	halfSH := screenH / 2
	if a.cx > halfSW {
		a.cx = halfSW
		a.vx = -absF(a.vx)
	} else if a.cx < -halfSW {
		a.cx = -halfSW
		a.vx = absF(a.vx)
	}
	if a.cy > halfSH {
		a.cy = halfSH
		a.vy = -absF(a.vy)
	} else if a.cy < -halfSH {
		a.cy = -halfSH
		a.vy = absF(a.vy)
	}
	return a.cx, a.cy, w, h
}

func absF(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// ---------------------------------------------------------------------------
// Synchronised multi-movie helpers — every one of these wraps the per-movie
// mutations in BeginBurst/EndBurst so both movies see the same
// MasterClock.Now value. This is what preserves perfect synchrony across
// every speed / pause / seek / pause-with-loop operation.
// ---------------------------------------------------------------------------

func togglePauseBoth(mgr *media.MovieManager, left, right *media.Movie) {
	mgr.BeginBurst()
	defer mgr.EndBurst()
	if left.IsPaused() || !left.IsActive() {
		left.Play()
		right.Play()
		return
	}
	left.Pause()
	right.Pause()
}

func pauseLoopBoth(mgr *media.MovieManager, left, right *media.Movie, window time.Duration) {
	mgr.BeginBurst()
	defer mgr.EndBurst()
	left.PauseWithLoop(window)
	right.PauseWithLoop(window)
}

func setRateBoth(mgr *media.MovieManager, left, right *media.Movie, r float64) float64 {
	if r < minRate {
		r = minRate
	}
	if r > maxRate {
		r = maxRate
	}
	mgr.BeginBurst()
	defer mgr.EndBurst()
	_ = left.SetRate(r)
	_ = right.SetRate(r)
	return r
}

func multiplyRateBoth(mgr *media.MovieManager, left, right *media.Movie, factor float64) float64 {
	cur := left.Rate()
	return setRateBoth(mgr, left, right, cur*factor)
}

func seekFrameBoth(mgr *media.MovieManager, left, right *media.Movie, n int) {
	mgr.BeginBurst()
	defer mgr.EndBurst()
	if err := left.SeekFrame(n); err != nil {
		log.Printf("seek left: %v", err)
	}
	if err := right.SeekFrame(n); err != nil {
		log.Printf("seek right: %v", err)
	}
}

func toggleBlendBoth(left, right *media.Movie) sdl.BlendMode {
	cur, _ := left.BlendMode()
	var next sdl.BlendMode
	if cur == sdl.BLENDMODE_ADD {
		next = sdl.BLENDMODE_BLEND
	} else {
		next = sdl.BLENDMODE_ADD
	}
	_ = left.SetBlendMode(next)
	_ = right.SetBlendMode(next)
	return next
}

func scaleSpeedBoth(la, ra *movieAnim, factor float32) {
	la.vx *= factor
	la.vy *= factor
	ra.vx *= factor
	ra.vy *= factor
}

func scaleSizeMaxBoth(la, ra *movieAnim, factor, screenW float32) {
	cap := screenW * 0.95
	la.sizeMaxW *= factor
	ra.sizeMaxW *= factor
	if la.sizeMaxW < la.sizeMinW {
		la.sizeMaxW = la.sizeMinW
	}
	if la.sizeMaxW > cap {
		la.sizeMaxW = cap
	}
	if ra.sizeMaxW < ra.sizeMinW {
		ra.sizeMaxW = ra.sizeMinW
	}
	if ra.sizeMaxW > cap {
		ra.sizeMaxW = cap
	}
}

// scheduleAtPlusN demonstrates dynamic registration of an OnAtDisplay
// condition — the goxpyriment equivalent of issuing a PsyScope
// `Movie[ AtDisplay THISMOVIE "f:N" ]` at runtime. Reads the target
// movie's current frame, computes target = current + plus, registers
// a one-shot OnAtDisplay callback that plays the configured tone and
// snapshots both movies into the data file when the target frame
// actually appears on screen, then unsubscribes itself so repeated
// key presses queue distinct one-shot triggers without leaking
// condition entries.
//
// `tag` is the tag of the target movie (used for the event label in
// the data file). `tone` may be nil if audio init failed; the snapshot
// + log still happen.
func scheduleAtPlusN(exp *control.Experiment, left, right, target *media.Movie, tag string, tone *stimuli.Tone, plus int) {
	snap := target.Snapshot()
	targetFrame := snap.Frame + plus

	// Log the scheduling event itself, so the data file records both
	// the click time and the eventual fire time. Source = wall-clock
	// because this is not a vsync-aligned event.
	scheduleEvent := fmt.Sprintf("schedule:%s@%d", tag, targetFrame)
	writeRow(exp, sdl.TicksNS(), "wall-clock", scheduleEvent, snap)

	var unsub func()
	unsub = target.OnAtDisplay(media.Frame(targetFrame), func(o media.Onset) {
		defer unsub() // one-shot: remove this condition once it fires
		if tone != nil {
			_ = tone.Play()
		}
		fireEvent := fmt.Sprintf("scheduled_fire:%s@%d", tag, targetFrame)
		writeBoth(exp, o.TimestampNS, o.Source.String(), fireEvent, left, right)
		log.Printf("[scheduled] %s frame %d displayed @ %d (%s) → tone",
			tag, targetFrame, o.TimestampNS, o.Source)
	})
	log.Printf("[%s] tone scheduled for %s frame %d (current=%d, +%d frames)",
		map[string]string{"left": "J", "right": "K"}[tag], tag, targetFrame, snap.Frame, plus)
}

// ---------------------------------------------------------------------------
// Data logging — writes one row per movie per event, so you can pivot in
// pandas/R by movie tag or by event.
// ---------------------------------------------------------------------------

// dataColumns matches AddDataVariableNames below; kept as a const so the
// Add-call ordering is auditable in one place.
var dataColumns = []string{
	"ts_ns",          // sdl.TicksNS at event time (Onset.TimestampNS for triggers)
	"ts_source",      // hardware-verified | vsync-estimated | look-ahead | wall-clock
	"event",          // human-readable event name
	"movie",          // tag of the movie this row describes
	"frame",          // current cumulative 1-based displayed frame
	"time_ms",        // effective media time in ms
	"rate",           // current playback rate
	"loop",           // current 0-based loop iteration
	"cx",             // center X (screen-center-relative px)
	"cy",             // center Y
	"w",              // explicit destination width (px); 0 if scale-derived
	"h",              // explicit destination height
	"blend",          // blend mode name
	"is_paused",      // true / false
	"loop_window_ms", // PauseWithLoop window in ms (0 if not in loop)
}

func writeRow(exp *control.Experiment, ts uint64, source, event string, snap media.Snapshot) {
	exp.Data.Add(
		ts, source, event, snap.Tag,
		snap.Frame,
		snap.Time.Milliseconds(),
		snap.Rate,
		snap.LoopCounter,
		snap.Position.X, snap.Position.Y,
		snap.SizeW, snap.SizeH,
		blendName(snap.BlendMode, snap.BlendModeSet),
		snap.IsPaused,
		snap.LoopRegionWindow.Milliseconds(),
	)
}

func writeBoth(exp *control.Experiment, ts uint64, source, event string, left, right *media.Movie) {
	writeRow(exp, ts, source, event, left.Snapshot())
	writeRow(exp, ts, source, event, right.Snapshot())
}

func blendName(m sdl.BlendMode, isSet bool) string {
	if !isSet {
		return "default"
	}
	switch m {
	case sdl.BLENDMODE_NONE:
		return "none"
	case sdl.BLENDMODE_BLEND:
		return "blend"
	case sdl.BLENDMODE_ADD:
		return "add"
	case sdl.BLENDMODE_MOD:
		return "mod"
	case sdl.BLENDMODE_MUL:
		return "mul"
	default:
		return fmt.Sprintf("0x%x", uint32(m))
	}
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	leftPath := flag.String("fL", findFixture("PhysicalViolation.gv"), "Path to left .gv movie")
	rightPath := flag.String("fR", findFixture("PhysicalViolation2.gv"), "Path to right .gv movie")
	maxSpeed := flag.Float64("speed", 500.0, "Initial peak bouncing speed in px/sec, per axis")
	sizeFracMin := flag.Float64("sizeMin", 0.18, "Minimum movie width as fraction of screen width")
	sizeFracMax := flag.Float64("sizeMax", 0.55, "Maximum movie width as fraction of screen width")
	leftTriggerFrame := flag.Int("leftAt", 50, "LEFT-movie frame that triggers the low beep + snapshot")
	rightTriggerFrame := flag.Int("rightAt", 80, "RIGHT-movie frame that triggers the high beep + snapshot")

	exp := control.NewExperimentFromFlags("Bouncing GV Movies", control.Black, control.White, 32)
	defer exp.End()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	terminate := false

	gvL, err := gvvideo.LoadGVVideo(*leftPath)
	if err != nil {
		exp.Fatal("load LEFT movie %q: %v", *leftPath, err)
	}
	defer closeGV(gvL)
	gvR, err := gvvideo.LoadGVVideo(*rightPath)
	if err != nil {
		exp.Fatal("load RIGHT movie %q: %v", *rightPath, err)
	}
	defer closeGV(gvR)
	log.Printf("loaded LEFT  %s: %dx%d, %d frames @ %v fps",
		*leftPath, gvL.Header.Width, gvL.Header.Height, gvL.Header.FrameCount, gvL.Header.FPS)
	log.Printf("loaded RIGHT %s: %dx%d, %d frames @ %v fps",
		*rightPath, gvR.Header.Width, gvR.Header.Height, gvR.Header.FrameCount, gvR.Header.FPS)

	// Two beeps at one octave apart so the LEFT and RIGHT triggers are
	// audibly distinct. Tones are async: Play() is non-blocking and
	// safe to call from inside an OnAtDisplay callback.
	tone1 := stimuli.NewTone(440, 150, 0.5) // A4 — left frame trigger
	if err := tone1.PreloadDevice(exp.AudioDevice); err != nil {
		log.Printf("warning: tone1 preload: %v (LEFT trigger will be silent)", err)
		tone1 = nil
	} else {
		defer tone1.Unload()
	}
	tone2 := stimuli.NewTone(880, 150, 0.5) // A5 — right frame trigger
	if err := tone2.PreloadDevice(exp.AudioDevice); err != nil {
		log.Printf("warning: tone2 preload: %v (RIGHT trigger will be silent)", err)
		tone2 = nil
	} else {
		defer tone2.Unload()
	}

	mgr := media.NewMovieManager(exp.Screen)
	defer mgr.Close()

	fsw, fsh := mgr.LogicalSize()
	minW := float32(*sizeFracMin) * fsw
	maxW := float32(*sizeFracMax) * fsw
	if maxW < minW {
		maxW = minW
	}
	aspectL := float32(gvL.Header.Width) / float32(gvL.Header.Height)
	aspectR := float32(gvR.Header.Width) / float32(gvR.Header.Height)
	log.Printf("layout: logical %.0fx%.0f; size oscillates %.0f..%.0f", fsw, fsh, minW, maxW)

	leftMov, err := media.NewMovie(mgr, gvL,
		media.WithTag("left"),
		media.WithRepeat(-1),
		media.WithSize(minW, minW/aspectL),
		media.WithBlendMode(sdl.BLENDMODE_ADD),
	)
	if err != nil {
		exp.Fatal("new LEFT movie: %v", err)
	}
	defer leftMov.Close()

	rightMov, err := media.NewMovie(mgr, gvR,
		media.WithTag("right"),
		media.WithRepeat(-1),
		media.WithSize(minW, minW/aspectR),
		media.WithBlendMode(sdl.BLENDMODE_ADD),
	)
	if err != nil {
		exp.Fatal("new RIGHT movie: %v", err)
	}
	defer rightMov.Close()

	leftAnim := &movieAnim{
		cx: -fsw * 0.25, cy: -fsh * 0.20,
		vx: float32(*maxSpeed) * 0.85, vy: float32(*maxSpeed) * 0.55,
		sizeAngVel: 2 * math.Pi / 4.0,
		sizeMinW:   minW, sizeMaxW: maxW,
		aspect: aspectL,
	}
	rightAnim := &movieAnim{
		cx: fsw * 0.25, cy: fsh * 0.20,
		vx: -float32(*maxSpeed) * 0.65, vy: -float32(*maxSpeed) * 0.75,
		sizePhase:  math.Pi,
		sizeAngVel: 2 * math.Pi / 5.7,
		sizeMinW:   minW, sizeMaxW: maxW,
		aspect: aspectR,
	}

	exp.AddDataVariableNames(dataColumns)

	// Frame triggers — fire when the target frame has actually appeared
	// on the display (Source: hardware-verified on macOS / Linux,
	// vsync-estimated on Windows). See docs/MediaMovies.md §7.
	leftMov.OnAtDisplay(media.Frame(*leftTriggerFrame), func(o media.Onset) {
		if tone1 != nil {
			_ = tone1.Play()
		}
		writeBoth(exp, o.TimestampNS, o.Source.String(),
			fmt.Sprintf("frame_trigger:left@%d", *leftTriggerFrame),
			leftMov, rightMov)
		log.Printf("[trigger] LEFT frame %d displayed @ %d (%s) → tone1 (440 Hz)",
			*leftTriggerFrame, o.TimestampNS, o.Source)
	})
	rightMov.OnAtDisplay(media.Frame(*rightTriggerFrame), func(o media.Onset) {
		if tone2 != nil {
			_ = tone2.Play()
		}
		writeBoth(exp, o.TimestampNS, o.Source.String(),
			fmt.Sprintf("frame_trigger:right@%d", *rightTriggerFrame),
			leftMov, rightMov)
		log.Printf("[trigger] RIGHT frame %d displayed @ %d (%s) → tone2 (880 Hz)",
			*rightTriggerFrame, o.TimestampNS, o.Source)
	})

	// Atomic start: both movies anchor to the same MasterClock.Now.
	mgr.BeginBurst()
	leftMov.Play()
	rightMov.Play()
	mgr.EndBurst()

	log.Print(`Controls:
  SPACE      pause / resume both (atomic)
  L          pause-with-loop (sawtooth over 10 frames forward)
  1 / 2 / 3  set rate to 1.0× / 2.0× / 0.5× (atomic)
  + / -      multiply / divide rate by 1.25 (atomic)
  Z          seek both to frame 1 (atomic; re-arms frame triggers)
  R          reset bounce positions
  F          toggle blend mode (ADD ↔ BLEND)
  Up / Down  scale bounce speed (×1.25 / ×0.8)
  W / N      scale size oscillation max (×1.15 / ÷1.15)
  A          snapshot LEFT movie  → data file
  B          snapshot RIGHT movie → data file
  J          schedule LEFT  tone1 + snapshot at frame current+20 (dynamic Movie[AtDisplay])
  K          schedule RIGHT tone2 + snapshot at frame current+20 (dynamic Movie[AtDisplay])
  ESC        quit`)

	lastTick := time.Now()
	runErr := exp.Run(func() error {
		if terminate {
			return control.EndLoop
		}
		select {
		case <-sigChan:
			terminate = true
			return control.EndLoop
		default:
		}

		key, _, evErr := exp.HandleEvents()
		if evErr == control.EndLoop {
			terminate = true
			return control.EndLoop
		}

		// Key handling — every multi-movie mutator goes through a
		// burst-wrapping helper to preserve perfect synchrony.
		switch key {
		case control.K_SPACE:
			togglePauseBoth(mgr, leftMov, rightMov)
		case control.K_L:
			window := time.Duration(10.0 / leftMov.FPS() * float64(time.Second))
			pauseLoopBoth(mgr, leftMov, rightMov, window)
			log.Printf("[L] PauseWithLoop window = %v (10 frames @ %v fps)", window, leftMov.FPS())
		case control.K_1:
			r := setRateBoth(mgr, leftMov, rightMov, 1.0)
			log.Printf("[1] rate set to %.3f×", r)
		case control.K_2:
			r := setRateBoth(mgr, leftMov, rightMov, 2.0)
			log.Printf("[2] rate set to %.3f×", r)
		case control.K_3:
			r := setRateBoth(mgr, leftMov, rightMov, 0.5)
			log.Printf("[3] rate set to %.3f×", r)
		case sdl.K_EQUALS, sdl.K_PLUS:
			r := multiplyRateBoth(mgr, leftMov, rightMov, rateStep)
			log.Printf("[+] rate × %.3f → %.3f×", rateStep, r)
		case sdl.K_MINUS:
			r := multiplyRateBoth(mgr, leftMov, rightMov, 1.0/rateStep)
			log.Printf("[-] rate ÷ %.3f → %.3f×", rateStep, r)
		case sdl.K_Z:
			seekFrameBoth(mgr, leftMov, rightMov, 1)
			log.Print("[Z] seek both to frame 1 (frame triggers re-armed)")
		case control.K_R:
			leftAnim.cx, leftAnim.cy = -fsw*0.4, -fsh*0.4
			rightAnim.cx, rightAnim.cy = fsw*0.4, fsh*0.4
			log.Print("[R] bounce positions reset")
		case control.K_F:
			next := toggleBlendBoth(leftMov, rightMov)
			log.Printf("[F] blend mode → %s", blendName(next, true))
		case control.K_UP:
			scaleSpeedBoth(leftAnim, rightAnim, speedStepUp)
			log.Printf("[Up] bounce speed × %.3f", speedStepUp)
		case control.K_DOWN:
			scaleSpeedBoth(leftAnim, rightAnim, speedStepDn)
			log.Printf("[Down] bounce speed × %.3f", speedStepDn)
		case sdl.K_W:
			scaleSizeMaxBoth(leftAnim, rightAnim, 1.15, fsw)
			log.Print("[W] size-oscillation max × 1.15")
		case control.K_N:
			scaleSizeMaxBoth(leftAnim, rightAnim, 1.0/1.15, fsw)
			log.Print("[N] size-oscillation max ÷ 1.15")
		case sdl.K_A:
			ts := sdl.TicksNS()
			writeRow(exp, ts, "wall-clock", "key_press:A", leftMov.Snapshot())
			log.Printf("[A] LEFT snapshot logged @ %d", ts)
		case control.K_B:
			ts := sdl.TicksNS()
			writeRow(exp, ts, "wall-clock", "key_press:B", rightMov.Snapshot())
			log.Printf("[B] RIGHT snapshot logged @ %d", ts)
		case control.K_J:
			scheduleAtPlusN(exp, leftMov, rightMov, leftMov, "left", tone1, 20)
		case control.K_K:
			scheduleAtPlusN(exp, leftMov, rightMov, rightMov, "right", tone2, 20)
		}

		// dt from wall clock; cap to dampen huge jumps after stalls.
		now := time.Now()
		dt := float32(now.Sub(lastTick).Seconds())
		lastTick = now
		if dt > 0.1 {
			dt = 0.1
		}

		// Animation freezes while playback is paused so the bounce
		// trajectory matches the frozen video frames. PauseWithLoop
		// counts as "paused" for IsPaused so animation also freezes
		// while the sawtooth plays — visually clearer (movie content
		// cycles, panel position holds).
		if !leftMov.IsPaused() && leftMov.IsActive() {
			lcx, lcy, lw, lh := leftAnim.advance(dt, fsw, fsh)
			leftMov.SetPosition(sdl.FPoint{X: lcx, Y: lcy})
			leftMov.SetSize(lw, lh)
			rcx, rcy, rw, rh := rightAnim.advance(dt, fsw, fsh)
			rightMov.SetPosition(sdl.FPoint{X: rcx, Y: rcy})
			rightMov.SetSize(rw, rh)
		}

		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := mgr.DrawWithoutFlip(); err != nil {
			return err
		}
		ts, err := exp.Screen.FlipTS()
		if err != nil {
			return err
		}
		mgr.NotifyFlipped(ts)
		return nil
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Printf("loop error: %v", runErr)
	}
	log.Print("done.")
}

func findFixture(name string) string {
	candidates := []string{
		name,
		"../demo_playgv/" + name,
		"tests/test_playgv/" + name,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

func closeGV(v *gvvideo.GVVideo) {
	if v == nil || v.Reader == nil {
		return
	}
	if c, ok := v.Reader.(io.Closer); ok {
		_ = c.Close()
	}
}
