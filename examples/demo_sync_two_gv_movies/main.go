// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// sync_two_gv_movies plays two .gv movies side by side under a single
// MasterClock to demonstrate the media package's Stage 1-4 features:
// multi-movie sync, burst-pause command groups, look-ahead Movie[At],
// post-vsync Movie[AtDisplay], and Display[Onset/Offset] events with
// optional TTL-trigger wiring.
//
// See examples/sync_two_gv_movies/README.md.
package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/funatsufumiya/go-gv-video/gvvideo"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/media"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/triggers"
)

func main() {
	// Flags must be registered BEFORE NewExperimentFromFlags (which
	// calls flag.Parse). The "-fL" / "-fR" form takes precedence over
	// the autodetected default paths.
	leftPath := flag.String("fL", findFixture("PhysicalViolation.gv"), "Path to left .gv movie")
	rightPath := flag.String("fR", findFixture("PhysicalViolation2.gv"), "Path to right .gv movie")
	atFrameLook := flag.Int("at", 30, "Movie[At] (look-ahead) target frame on the LEFT movie")
	atFrameDisp := flag.Int("atd", 60, "Movie[AtDisplay] (post-vsync) target frame on the LEFT movie")

	exp := control.NewExperimentFromFlags("Sync Two GV Movies", control.Black, control.White, 32)
	defer exp.End()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	terminate := false

	gvL, err := gvvideo.LoadGVVideo(*leftPath)
	if err != nil {
		exp.Fatal("load left movie %q: %v", *leftPath, err)
	}
	defer closeGV(gvL)
	gvR, err := gvvideo.LoadGVVideo(*rightPath)
	if err != nil {
		exp.Fatal("load right movie %q: %v", *rightPath, err)
	}
	defer closeGV(gvR)
	log.Printf("loaded LEFT  %s: %dx%d, %d frames @ %v fps",
		*leftPath, gvL.Header.Width, gvL.Header.Height, gvL.Header.FrameCount, gvL.Header.FPS)
	log.Printf("loaded RIGHT %s: %dx%d, %d frames @ %v fps",
		*rightPath, gvR.Header.Width, gvR.Header.Height, gvR.Header.FrameCount, gvR.Header.FPS)

	// TTL trigger — falls back to a silent NullOutputTTLDevice when no
	// DLP-IO8 is plugged in, so the example runs identically without
	// hardware. See triggers/CLAUDE.md.
	ttl, ttlPort, _ := triggers.AutoDetectDLPIO8()
	defer ttl.Close()
	if ttlPort != "" {
		log.Printf("TTL device on %s", ttlPort)
	} else {
		log.Print("no TTL device detected; trigger calls are silent no-ops")
	}

	mgr := media.NewMovieManager(exp.Screen)
	defer mgr.Close() // stops Stage 5 backend (CVDisplayLink / DRM fd)

	// Layout: split the screen into two columns separated by a small
	// inner gap, with margins from the screen edges. Each movie fits
	// inside its column while preserving aspect ratio.
	//
	// Use mgr.LogicalSize (the renderer's coordinate-space size, which
	// is what CenterToSDL and RenderTexture's destination FRect honor)
	// rather than exp.Screen.Size (physical pixels — 2x too big on
	// HiDPI/Retina when a logical presentation is in effect).
	fsw, fsh := mgr.LogicalSize()
	edgeMargin := fsw * 0.02 // 2% from left and right screen edges
	innerGap := fsw * 0.02   // 2% gap between the two movies
	topBotMargin := fsh * 0.05
	colW := (fsw - 2*edgeMargin - innerGap) / 2
	colH := fsh - 2*topBotMargin

	leftW, leftH := fitInside(float32(gvL.Header.Width), float32(gvL.Header.Height),
		colW, colH)
	rightW, rightH := fitInside(float32(gvR.Header.Width), float32(gvR.Header.Height),
		colW, colH)
	leftPos := sdl.FPoint{X: -fsw/2 + edgeMargin + colW/2, Y: 0}
	rightPos := sdl.FPoint{X: fsw/2 - edgeMargin - colW/2, Y: 0}

	log.Printf("layout: logical %.0fx%.0f, columns %.0fx%.0f (edge %.0f, gap %.0f)",
		fsw, fsh, colW, colH, edgeMargin, innerGap)
	log.Printf("  LEFT  scaled to %.0fx%.0f, center (%.0f, %.0f)",
		leftW, leftH, leftPos.X, leftPos.Y)
	log.Printf("  RIGHT scaled to %.0fx%.0f, center (%.0f, %.0f)",
		rightW, rightH, rightPos.X, rightPos.Y)

	leftMov, err := media.NewMovie(mgr, gvL,
		media.WithTag("left"),
		media.WithPosition(leftPos),
		media.WithSize(leftW, leftH),
		media.WithRepeat(-1), // loop forever; ESC quits
	)
	if err != nil {
		exp.Fatal("new left movie: %v", err)
	}
	defer leftMov.Close()

	rightMov, err := media.NewMovie(mgr, gvR,
		media.WithTag("right"),
		media.WithPosition(rightPos),
		media.WithSize(rightW, rightH),
		media.WithRepeat(-1),
	)
	if err != nil {
		exp.Fatal("new right movie: %v", err)
	}
	defer rightMov.Close()

	// Display[Onset/Offset] logging — fires once when the named tag
	// transitions visible/not-visible across consecutive flips.
	mgr.OnDisplayOnset("left", func(o media.Onset) {
		log.Printf("[Display.Onset]  left  ts=%d source=%s", o.TimestampNS, o.Source)
	})
	mgr.OnDisplayOnset("right", func(o media.Onset) {
		log.Printf("[Display.Onset]  right ts=%d source=%s", o.TimestampNS, o.Source)
	})
	mgr.OnDisplayOffset("left", func(o media.Onset) {
		log.Printf("[Display.Offset] left  ts=%d", o.TimestampNS)
	})
	mgr.OnDisplayOffset("right", func(o media.Onset) {
		log.Printf("[Display.Offset] right ts=%d", o.TimestampNS)
	})

	// Movie[At] look-ahead: fires inside DrawWithoutFlip while the
	// target frame is still being decoded, before it appears on screen.
	// Useful for adding overlays that should land on the same vsync.
	leftMov.OnAt(media.Frame(*atFrameLook), func(o media.Onset) {
		log.Printf("[Movie.At]        left frame %d decoded (look-ahead, ts=%d source=%s)",
			o.Frame, o.TimestampNS, o.Source)
	})

	// Movie[AtDisplay]: fires from NotifyFlipped after the target frame
	// has appeared on screen. Pair with a TTL pulse for hardware-aligned
	// EEG/MEG markers.
	leftMov.OnAtDisplay(media.Frame(*atFrameDisp), func(o media.Onset) {
		log.Printf("[Movie.AtDisplay] left frame %d displayed (ts=%d source=%s)",
			o.Frame, o.TimestampNS, o.Source)
		_ = ttl.Pulse(0, 5*time.Millisecond)
	})

	fix := stimuli.NewFixCross(40, 4, control.White)

	log.Print("Controls: [SPACE] burst pause/resume both, [R] burst-rewind both, [ESC] quit")

	// Atomic start: BeginBurst pins MasterClock.Now so both Play calls
	// anchor to the same media-time origin (PsyScope's frozen-clock
	// command-burst pattern; MultiMovieSyncStrategy.md §3.3).
	mgr.BeginBurst()
	leftMov.Play()
	rightMov.Play()
	mgr.EndBurst()

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
		switch key {
		case control.K_SPACE:
			mgr.BeginBurst()
			if leftMov.IsPaused() || !leftMov.IsActive() {
				leftMov.Play()
				rightMov.Play()
			} else {
				leftMov.Pause()
				rightMov.Pause()
			}
			mgr.EndBurst()
		case control.K_R:
			mgr.BeginBurst()
			_ = leftMov.SeekFrame(1)
			_ = rightMov.SeekFrame(1)
			mgr.EndBurst()
		}

		// Composite frame: clear → movies → fixation cross → flip → notify.
		// The fixation cross lands on the same vsync as the movies because
		// it is drawn between DrawWithoutFlip and FlipTS.
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := mgr.DrawWithoutFlip(); err != nil {
			return err
		}
		if err := fix.Present(exp.Screen, false, false); err != nil {
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

// findFixture returns the first existing path among the standard
// locations of bundled .gv test fixtures, falling back to the bare
// filename if nothing is found (so the eventual error message points
// at the most likely intended location).
func findFixture(name string) string {
	candidates := []string{
		name,
		"../demo_play_gv_videos/" + name,
		"tests/test_playgv/" + name,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

// fitInside returns the largest (w, h) preserving the source aspect
// ratio that fits inside (maxW, maxH).
func fitInside(srcW, srcH, maxW, maxH float32) (float32, float32) {
	sx := maxW / srcW
	sy := maxH / srcH
	s := sx
	if sy < s {
		s = sy
	}
	return srcW * s, srcH * s
}

// closeGV closes a GVVideo's underlying reader if it implements io.Closer.
// Mirrors the cleanup in stimuli/gvvideo.go (Unload).
func closeGV(v *gvvideo.GVVideo) {
	if v == nil || v.Reader == nil {
		return
	}
	if c, ok := v.Reader.(io.Closer); ok {
		_ = c.Close()
	}
}
