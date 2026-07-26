// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// WhiteSquareSync (gv) — goxpyriment translation of the PsyScope script
// `WhiteSquareSync_oscill.psyscript`. Plays sequences of three .gv movies
// per trial, flashing a white square (photodiode pickup) and pulsing
// DLP-IO8 line 0 at the same frame markers PsyScope uses, so a
// photodiode + oscilloscope pair can verify display-onset synchrony with
// the TTL trigger.
//
// PsyScope semantics preserved 1:1 where possible:
//
//   - For every movie: at frame 1, run the white-square stimulus
//     (look-ahead via Movie[At] — fires from DrawWithoutFlip BEFORE the
//     frame is presented, so the square lands on the same vsync as
//     frame 1) AND raise the TTL line (hardware-verified via
//     Movie[AtDisplay] — fires from NotifyFlipped with the
//     CVDisplayLink/DRM-measured first-pixel-visible timestamp).
//     At frame 10, lower the TTL line.
//
//   - For M1 only: also at frame 186 (square + TTL HIGH) and frame 189
//     (TTL LOW).
//
//   - Movie filename overlay (yellow) in the top center for the first
//     500 ms after each movie starts.
//
//   - "Press a key when ready" start screen brackets a TTL HIGH/LOW
//     pulse for session-start synchronization.
//
//   - 1000 ms blank inter-trial interval after each (M1, M2, M3) triplet.
//
// Trials list (default behaviour):
//
//  1. If `-trials FILE` is given, read whitespace-separated 3-column
//     file, one trial per line.
//  2. Else, look for `3movies.txt` in `-dir` (default = current dir).
//  3. Else, auto-pair: glob M1-*.gv, M2-*.gv, M3-*.gv, sort, and
//     cycle-pair into trials.
//
// `-n N` caps the trial count.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/funatsufumiya/go-gv-video/gvvideo"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/media"
	"github.com/chrplr/goxpyriment/stimuli"
	"github.com/chrplr/goxpyriment/triggers"
)

// PsyScope-equivalent timing constants. The frame numbers come straight
// from the .psyscript; the durations come from the original event
// definitions (squarepic Duration: 400, M1Name Duration: 500, NewEvent
// Duration: 1000).
const (
	ttlLine        = 0
	squareDuration = 400 * time.Millisecond
	nameDuration   = 500 * time.Millisecond
	itiDuration    = 1000 * time.Millisecond
)

// trial is one row of the trials list — three .gv movies played
// sequentially as M1, M2, M3.
type trial struct {
	m1, m2, m3 string
}

func main() {
	moviesDir := flag.String("dir", ".", "Directory containing .gv movies (and optional 3movies.txt)")
	trialsFile := flag.String("trials", "", "Path to trials list (3 columns whitespace-separated). Empty = look for <dir>/3movies.txt, else auto-pair.")
	maxTrials := flag.Int("n", 0, "Limit number of trials run (0 = no limit)")
	sqW := flag.Float64("sqW", 200, "White square width in screen pixels")
	sqH := flag.Float64("sqH", 200, "White square height in screen pixels")
	sqX := flag.Float64("sqX", 0, "White square center X (screen-center-relative px; +X = right)")
	sqY := flag.Float64("sqY", 380, "White square center Y (screen-center-relative px; +Y = up)")
	nameY := flag.Float64("nameY", 320, "Movie name overlay center Y (+Y = up)")

	exp := control.NewExperimentFromFlags("WhiteSquareSync (gv)", control.Black, control.White, 24)
	defer exp.End()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	terminate := false
	checkTerminate := func() {
		select {
		case <-sigChan:
			terminate = true
		default:
		}
	}

	// Resolve trials list.
	trials, err := resolveTrials(*moviesDir, *trialsFile)
	if err != nil {
		exp.Fatal("load trials: %v", err)
	}
	if *maxTrials > 0 && len(trials) > *maxTrials {
		trials = trials[:*maxTrials]
	}
	if len(trials) == 0 {
		exp.Fatal("no trials")
	}
	log.Printf("loaded %d trials from %q", len(trials), *moviesDir)

	// DLP-IO8 — falls back to a silent NullOutputTTLDevice when no
	// hardware is plugged in (matches the PsyScope script's optional
	// SerialPorts behavior — the experiment still runs, the markers
	// just go to /dev/null).
	ttl, port, _ := triggers.AutoDetectDLPIO8()
	defer ttl.Close()
	if port != "" {
		log.Printf("DLP-IO8 detected on %s; using line %d (= PsyScope pin 1)", port, ttlLine)
	} else {
		log.Print("no DLP-IO8 detected; TTL calls are silent no-ops")
	}

	mgr := media.NewMovieManager(exp.Screen)
	defer mgr.Close()

	// Data file columns. One row per event (square shown, TTL toggled,
	// movie started, movie ended); columns are easy to filter or pivot.
	exp.AddDataVariableNames([]string{
		"trial", "movie_idx", "movie", "event", "frame", "ts_ns", "source",
	})

	// Pre-build the white square once — the rectangle is constant across
	// trials, only its visibility flips on/off via the squareDeadlineNS
	// timer set by the OnAt callbacks.
	square := stimuli.NewRectangle(float32(*sqX), float32(*sqY),
		float32(*sqW), float32(*sqH), control.White)

	// Start screen — press a key to begin, with a TTL HIGH/LOW bracket
	// matching the original Startmsg event.
	if err := showStartMsg(exp, ttl, &terminate, sigChan); err != nil && !control.IsEndLoop(err) {
		log.Printf("startmsg: %v", err)
	}

	// Run trials.
	for trialIdx, t := range trials {
		if terminate {
			break
		}
		movies := []string{t.m1, t.m2, t.m3}
		for movieIdx, basename := range movies {
			checkTerminate()
			if terminate {
				break
			}

			moviePath := basename
			if !filepath.IsAbs(moviePath) {
				moviePath = filepath.Join(*moviesDir, moviePath)
			}
			if !strings.HasSuffix(strings.ToLower(moviePath), ".gv") {
				moviePath += ".gv"
			}

			isM1 := movieIdx == 0
			log.Printf("[trial %d/%d, movie %d] %s", trialIdx+1, len(trials), movieIdx+1, filepath.Base(moviePath))

			err := playOneMovie(exp, mgr, ttl, moviePath, square,
				isM1, trialIdx+1, movieIdx+1, float32(*nameY), &terminate)
			if err != nil && !control.IsEndLoop(err) {
				log.Printf("play error (%s): %v", moviePath, err)
			}
		}

		// Inter-trial interval (PsyScope NewEvent NULL 1000 ms).
		if !terminate {
			_ = exp.Blank(int(itiDuration.Milliseconds()))
		}
	}

	log.Printf("done — completed %d trials", len(trials))
}

// resolveTrials decides which source to pull trials from, in order:
// explicit --trials, then default <dir>/3movies.txt, then auto-pair.
func resolveTrials(moviesDir, explicit string) ([]trial, error) {
	if explicit != "" {
		log.Printf("using --trials %s", explicit)
		return loadTrials(explicit)
	}
	defaultPath := filepath.Join(moviesDir, "3movies.txt")
	if _, err := os.Stat(defaultPath); err == nil {
		log.Printf("using default trials file %s", defaultPath)
		return loadTrials(defaultPath)
	}
	log.Printf("no trials file at %s; auto-pairing M1-*.gv / M2-*.gv / M3-*.gv from %s",
		defaultPath, moviesDir)
	return autoTrials(moviesDir)
}

// loadTrials reads a whitespace-separated 3-column file. Lines that are
// empty or start with '#' are skipped.
func loadTrials(path string) ([]trial, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var trials []trial
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("%s:%d: expected 3 columns, got %d (%q)", path, lineNum, len(fields), line)
		}
		trials = append(trials, trial{m1: fields[0], m2: fields[1], m3: fields[2]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return trials, nil
}

// autoTrials globs M1-*.gv, M2-*.gv, M3-*.gv in dir, sorts each set,
// and cycle-pairs them into trials. Number of trials = max group size.
func autoTrials(dir string) ([]trial, error) {
	m1Files, _ := filepath.Glob(filepath.Join(dir, "M1-*.gv"))
	m2Files, _ := filepath.Glob(filepath.Join(dir, "M2-*.gv"))
	m3Files, _ := filepath.Glob(filepath.Join(dir, "M3-*.gv"))
	if len(m1Files) == 0 || len(m2Files) == 0 || len(m3Files) == 0 {
		return nil, fmt.Errorf("auto-trials: need at least one M1-*.gv, M2-*.gv, M3-*.gv in %s (found %d/%d/%d)",
			dir, len(m1Files), len(m2Files), len(m3Files))
	}
	sort.Strings(m1Files)
	sort.Strings(m2Files)
	sort.Strings(m3Files)
	n := len(m1Files)
	if len(m2Files) > n {
		n = len(m2Files)
	}
	if len(m3Files) > n {
		n = len(m3Files)
	}
	trials := make([]trial, n)
	for i := 0; i < n; i++ {
		trials[i] = trial{
			m1: filepath.Base(m1Files[i%len(m1Files)]),
			m2: filepath.Base(m2Files[i%len(m2Files)]),
			m3: filepath.Base(m3Files[i%len(m3Files)]),
		}
	}
	return trials, nil
}

// showStartMsg displays "Press a key when ready" with TTL HIGH on
// appearance and TTL LOW after the keypress, matching the PsyScope
// Startmsg event's SerialOut "1" / "Q" actions.
func showStartMsg(exp *control.Experiment, ttl triggers.OutputTTLDevice,
	terminate *bool, sigChan <-chan os.Signal) error {
	msg := stimuli.NewTextLine("Press a key when ready  (Q or ESC quits)",
		0, 0, control.RGB(255, 38, 0))

	// First flip — message appears, raise TTL.
	if err := exp.Screen.Clear(); err != nil {
		return err
	}
	if err := msg.Draw(exp.Screen); err != nil {
		return err
	}
	if _, err := exp.Screen.FlipTS(); err != nil {
		return err
	}
	_ = ttl.SetHigh(ttlLine)

	// Wait for any keypress (signal-aware).
	for !*terminate {
		select {
		case <-sigChan:
			*terminate = true
			_ = ttl.SetLow(ttlLine)
			return nil
		default:
		}
		key, _, evErr := exp.HandleEvents()
		if evErr == control.EndLoop {
			*terminate = true
			_ = ttl.SetLow(ttlLine)
			return control.EndLoop
		}
		if key != 0 {
			break
		}
		// Re-render so the message stays visible (and the OS event
		// queue keeps flowing, especially on Wayland).
		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := msg.Draw(exp.Screen); err != nil {
			return err
		}
		if _, err := exp.Screen.FlipTS(); err != nil {
			return err
		}
	}
	_ = ttl.SetLow(ttlLine)
	_ = msg.Unload()
	return nil
}

// playOneMovie loads, plays to completion, and unloads a single .gv
// movie. Registers the PsyScope-equivalent triggers (look-ahead square
// at frame 1 [+186 for M1] via OnAt; hardware-verified TTL HIGH at
// frame 1 [+186 for M1] and TTL LOW at frame 10 [+189 for M1] via
// OnAtDisplay), draws the movie-name overlay for the first 500 ms,
// and writes one data row per event.
func playOneMovie(exp *control.Experiment, mgr *media.MovieManager, ttl triggers.OutputTTLDevice,
	moviePath string, square *stimuli.Rectangle,
	isM1 bool, trialNum, movieIdxNum int, nameY float32, terminate *bool) error {

	gv, err := gvvideo.LoadGVVideo(moviePath)
	if err != nil {
		return fmt.Errorf("load %s: %w", moviePath, err)
	}
	defer closeGV(gv)

	basename := filepath.Base(moviePath)
	tag := fmt.Sprintf("M%d", movieIdxNum)

	mov, err := media.NewMovie(mgr, gv,
		media.WithTag(tag),
		media.WithRepeat(1),
		media.WithScale(media.ScaleFit),
	)
	if err != nil {
		return fmt.Errorf("new movie %s: %w", basename, err)
	}
	defer func() {
		mov.Close()
		mgr.Remove(mov)
	}()

	// Visibility deadlines — set by triggers, decremented in the
	// per-frame draw block.
	var squareDeadlineNS uint64
	var nameDeadlineNS uint64

	logRow := func(event string, frame int, ts uint64, source string) {
		exp.Data.Add(trialNum, movieIdxNum, basename, event, frame, ts, source)
	}

	// Look-ahead: white square trigger at frame 1 (and 186 for M1).
	// `OnAt` fires from DrawWithoutFlip BEFORE the target frame is
	// presented, so when the square draw happens later in the same
	// per-frame body, both the movie's frame-N pixels AND the white
	// square land on the SAME vsync — exactly what RunEvent[squarepic]
	// achieves in PsyScope.
	mov.OnAt(media.Frame(1), func(o media.Onset) {
		squareDeadlineNS = o.TimestampNS + uint64(squareDuration.Nanoseconds())
		logRow("square_on", 1, o.TimestampNS, o.Source.String())
	})
	if isM1 {
		mov.OnAt(media.Frame(186), func(o media.Onset) {
			squareDeadlineNS = o.TimestampNS + uint64(squareDuration.Nanoseconds())
			logRow("square_on", 186, o.TimestampNS, o.Source.String())
		})
	}

	// Hardware-verified: TTL HIGH at frame 1, LOW at frame 10
	// (and HIGH/LOW at 186/189 for M1). `OnAtDisplay` fires from
	// NotifyFlipped with the CVDisplayLink/DRM-measured first-pixel-
	// visible timestamp — same precision contract as PsyScope's
	// Movie[AtDisplay] using Metal addPresentedHandler.
	mov.OnAtDisplay(media.Frame(1), func(o media.Onset) {
		_ = ttl.SetHigh(ttlLine)
		logRow("ttl_high", 1, o.TimestampNS, o.Source.String())
	})
	mov.OnAtDisplay(media.Frame(10), func(o media.Onset) {
		_ = ttl.SetLow(ttlLine)
		logRow("ttl_low", 10, o.TimestampNS, o.Source.String())
	})
	if isM1 {
		mov.OnAtDisplay(media.Frame(186), func(o media.Onset) {
			_ = ttl.SetHigh(ttlLine)
			logRow("ttl_high", 186, o.TimestampNS, o.Source.String())
		})
		mov.OnAtDisplay(media.Frame(189), func(o media.Onset) {
			_ = ttl.SetLow(ttlLine)
			logRow("ttl_low", 189, o.TimestampNS, o.Source.String())
		})
	}

	// Movie name overlay (yellow) at top-center, visible for 500 ms.
	nameStim := stimuli.NewTextLine(strings.TrimSuffix(basename, ".gv"),
		0, nameY, control.Yellow)
	defer func() { _ = nameStim.Unload() }()

	mgr.BeginBurst()
	mov.Play()
	mgr.EndBurst()

	movieStartTS := sdl.TicksNS()
	nameDeadlineNS = movieStartTS + uint64(nameDuration.Nanoseconds())
	logRow("movie_start", 0, movieStartTS, "wall-clock")

	runErr := exp.Run(func() error {
		if *terminate {
			return control.EndLoop
		}

		key, _, evErr := exp.HandleEvents()
		if evErr == control.EndLoop {
			*terminate = true
			return control.EndLoop
		}
		if key == control.K_Q {
			*terminate = true
			return control.EndLoop
		}

		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		if err := mgr.DrawWithoutFlip(); err != nil {
			return err
		}

		nowTS := sdl.TicksNS()
		if nowTS < nameDeadlineNS {
			if err := nameStim.Draw(exp.Screen); err != nil {
				return err
			}
		}
		if nowTS < squareDeadlineNS {
			if err := square.Draw(exp.Screen); err != nil {
				return err
			}
		}

		ts, err := exp.Screen.FlipTS()
		if err != nil {
			return err
		}
		mgr.NotifyFlipped(ts)

		if mov.IsDone() {
			return control.EndLoop
		}
		return nil
	})

	logRow("movie_done", mov.Frame(), sdl.TicksNS(), "wall-clock")
	// Make sure the line ends up LOW after the movie finishes; if the
	// experimenter aborts mid-pulse the next trial would otherwise see
	// the line still HIGH.
	_ = ttl.SetLow(ttlLine)
	return runErr
}

func closeGV(v *gvvideo.GVVideo) {
	if v == nil || v.Reader == nil {
		return
	}
	if c, ok := v.Reader.(io.Closer); ok {
		_ = c.Close()
	}
}
