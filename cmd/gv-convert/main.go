// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Command gv-convert converts a video file into the .gv format that
// goxpyriment's stimuli.GvVideo and media.Movie play.
//
// .gv trades file size for timing determinism: every frame is an independent
// LZ4-compressed raw-RGBA block with an index table at the end, so presenting a
// frame costs a seek, a decompress, and a texture upload — no inter-frame
// prediction, no variable-cost I/B frames, and therefore no decode jitter
// aligned to GOP boundaries. Expect large files; that is the point.
//
// MPEG-1 program streams are decoded in pure Go with no external tools. Any
// other format is piped through ffmpeg if it is installed. Passing a directory
// instead of a file converts a numbered image sequence.
//
// Usage:
//
//	gv-convert [options] <input-video|input-dir> <output.gv>
//
// Options:
//
//	-fps N        output frame rate (default: the source rate)
//	-max N        stop after N frames (0 = all)
//	-force-size   crop/pad mismatched images instead of failing
//	-q            suppress progress output
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("gv-convert: ")

	fps := flag.Float64("fps", 0, "output frame rate (0 = use the source rate)")
	maxFrames := flag.Int("max", 0, "stop after N frames (0 = all)")
	forceSize := flag.Bool("force-size", false,
		"crop/pad images that do not match the first frame instead of failing")
	quiet := flag.Bool("q", false, "suppress progress output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <input-video|input-dir> <output.gv>\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "Converts a video, or a directory of numbered images, to the .gv format\n")
		fmt.Fprintf(os.Stderr, "played by goxpyriment.\n\n")
		fmt.Fprintf(os.Stderr, "  MPEG-1 program streams  decoded in pure Go, no external tools\n")
		fmt.Fprintf(os.Stderr, "  other video formats     piped through ffmpeg (must be on PATH)\n")
		fmt.Fprintf(os.Stderr, "  a directory             .png/.jpg/.jpeg/.gif in natural numeric order\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	input, output := flag.Arg(0), flag.Arg(1)

	if _, err := os.Stat(input); err != nil {
		log.Fatalf("%v", err)
	}
	if !strings.EqualFold(filepath.Ext(output), ".gv") {
		log.Fatalf("output %q should have a .gv extension", output)
	}

	opt := options{
		fps:       *fps,
		maxFrames: *maxFrames,
		forceSize: *forceSize,
		quiet:     *quiet,
	}
	if err := convert(input, output, opt); err != nil {
		// A failed conversion leaves a file with a placeholder header that
		// readers would misparse; remove it rather than leave a trap.
		os.Remove(output)
		log.Fatalf("%v", err)
	}
}

// options carries the command-line settings through the conversion.
type options struct {
	fps       float64
	maxFrames int
	forceSize bool
	quiet     bool
}

// defaultImageFPS is used for image sequences when -fps is not given: a
// directory of stills carries no frame rate of its own.
const defaultImageFPS = 30

// openSource picks a decoder for the input. A directory is an image sequence;
// otherwise the pure-Go MPEG-1 decoder is tried first and ffmpeg is the
// fallback. MPEG-1 is preferred whenever it works: no external dependency and
// no process boundary to copy frames across.
func openSource(input string, opt options) (frameSource, error) {
	st, err := os.Stat(input)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		fps := opt.fps
		if fps <= 0 {
			fps = defaultImageFPS
			if !opt.quiet {
				fmt.Fprintf(os.Stderr,
					"image sequences have no frame rate; using %g fps (set -fps to change)\n", fps)
			}
		}
		return openImageDir(input, fps, opt.forceSize, opt.quiet)
	}

	src, mpegErr := openMPEG1(input)
	if mpegErr == nil {
		return src, nil
	}

	if !opt.quiet {
		fmt.Fprintf(os.Stderr, "not an MPEG-1 program stream (%v); trying ffmpeg\n", mpegErr)
	}
	fsrc, ffErr := openFFmpeg(input, opt.fps)
	if ffErr != nil {
		return nil, fmt.Errorf("cannot decode %s:\n  pure Go: %v\n  ffmpeg:  %v", input, mpegErr, ffErr)
	}
	return fsrc, nil
}

func convert(input, output string, opt options) error {
	fps, maxFrames, quiet := opt.fps, opt.maxFrames, opt.quiet

	src, err := openSource(input, opt)
	if err != nil {
		return err
	}
	defer src.Close()

	w, h := src.Size()
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid frame size %dx%d", w, h)
	}

	outFPS := fps
	if outFPS <= 0 {
		outFPS = src.FPS()
	}
	if outFPS <= 0 {
		outFPS = 30
		if !quiet {
			fmt.Fprintf(os.Stderr, "source frame rate unknown; defaulting to %g fps\n", outFPS)
		}
	}

	// ffmpeg already resampled to -fps; re-dropping frames here would double
	// the effect. Only the pure-Go path needs to do its own rate conversion.
	dropRate := 0.0
	if _, isMPEG := src.(*mpeg1Source); isMPEG && fps > 0 && src.FPS() > 0 {
		dropRate = src.FPS() / fps
	}

	if !quiet {
		fmt.Printf("input   : %s  [%s]\n", input, src.Describe())
		fmt.Printf("frames  : %dx%d @ %g fps\n", w, h, outFPS)
		fmt.Printf("output  : %s (%.1f MB per 100 frames uncompressed)\n",
			output, float64(w*h*4)*100/(1024*1024))
	}

	gw, err := newGVWriter(output, w, h, outFPS)
	if err != nil {
		return err
	}

	start := time.Now()
	srcIndex, nextWanted := 0, 0.0
	// Count frames handed to the writer, not frames it has flushed: workers
	// lag behind, so gw.FrameCount() would undercount here.
	submitted := 0
	for {
		if maxFrames > 0 && submitted >= maxFrames {
			break
		}
		frame, err := src.NextFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			gw.Close()
			return err
		}

		// Rate conversion for the pure-Go path: keep a frame only when the
		// source index reaches the next wanted output time.
		if dropRate > 0 {
			if float64(srcIndex) < nextWanted {
				srcIndex++
				continue
			}
			nextWanted += dropRate
		}
		srcIndex++

		if err := gw.AddFrame(frame); err != nil {
			gw.Close()
			return err
		}
		submitted++
		if !quiet && submitted%25 == 0 {
			fmt.Printf("\rconverted %d frames...", submitted)
		}
	}

	// Close drains the compression workers, so the final count is only exact
	// once it returns.
	if err := gw.Close(); err != nil {
		return err
	}
	n := gw.FrameCount()
	if n == 0 {
		return fmt.Errorf("no frames decoded from %s", input)
	}

	if !quiet {
		info, _ := os.Stat(output)
		fmt.Printf("\rconverted %d frames in %s", n, time.Since(start).Round(time.Millisecond))
		if info != nil {
			fmt.Printf(" -> %.1f MB", float64(info.Size())/(1024*1024))
		}
		fmt.Printf(" (%.1f s at %g fps)\n", float64(n)/outFPS, outFPS)
	}
	return nil
}
