// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// imageExts are the still-image formats the Go standard library decodes.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
}

// imageDirSource turns a directory of numbered still images into a frame
// sequence. This is the right input when frames are generated programmatically
// (drifting gratings, custom animations) rather than filmed.
//
// Unlike the video sources it has no inherent frame rate; the caller supplies
// one with -fps.
type imageDirSource struct {
	files     []string
	w, h      int
	fps       float64
	forceSize bool
	next      int
}

// openImageDir collects and orders the images in dir, and takes the frame size
// from the first one.
func openImageDir(dir string, fps float64, forceSize, quiet bool) (*imageDirSource, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if imageExts[strings.ToLower(filepath.Ext(e.Name()))] {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .png/.jpg/.jpeg/.gif files in %s", dir)
	}

	// Order numerically, not lexicographically: with unpadded names, a plain
	// string sort puts frame_10 before frame_9 and the clip plays out of order.
	lexical := slices.Clone(files)
	slices.Sort(lexical)
	slices.SortFunc(files, func(a, b string) int { return naturalCompare(a, b) })

	if !quiet && !slices.Equal(files, lexical) {
		fmt.Fprintf(os.Stderr,
			"note: using natural numeric order; a plain string sort would order these differently\n"+
				"      (first frame here: %s)\n", filepath.Base(files[0]))
	}

	w, h, err := imageSize(files[0])
	if err != nil {
		return nil, fmt.Errorf("reading first frame %s: %w", files[0], err)
	}

	return &imageDirSource{
		files:     files,
		w:         w,
		h:         h,
		fps:       fps,
		forceSize: forceSize,
	}, nil
}

// imageSize decodes just the header of an image file to get its dimensions.
func imageSize(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func (s *imageDirSource) Size() (int, int) { return s.w, s.h }
func (s *imageDirSource) FPS() float64     { return s.fps }
func (s *imageDirSource) Close() error     { return nil }

func (s *imageDirSource) Describe() string {
	return fmt.Sprintf("image sequence (%d files)", len(s.files))
}

func (s *imageDirSource) NextFrame() ([]byte, error) {
	if s.next >= len(s.files) {
		return nil, io.EOF
	}
	path := s.files[s.next]
	s.next++

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}

	b := img.Bounds()
	if b.Dx() != s.w || b.Dy() != s.h {
		// Silently rescaling would corrupt a stimulus set in a way nobody
		// notices until the data are analysed, so refuse by default.
		if !s.forceSize {
			return nil, fmt.Errorf(
				"%s is %dx%d but the first frame is %dx%d\n"+
					"       (all frames must match; pass -force-size to crop/pad instead)",
				filepath.Base(path), b.Dx(), b.Dy(), s.w, s.h)
		}
	}

	// Fast path: already RGBA, origin-anchored and tightly packed, so Pix is
	// exactly the buffer the encoder wants.
	if rgba, ok := img.(*image.RGBA); ok &&
		b.Min == (image.Point{}) && rgba.Stride == s.w*4 && b.Dx() == s.w && b.Dy() == s.h {
		return rgba.Pix, nil
	}

	out := image.NewRGBA(image.Rect(0, 0, s.w, s.h))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out.Pix, nil
}

// naturalCompare orders strings so embedded numbers compare by value:
// "frame_9" sorts before "frame_10". Returns -1, 0 or 1.
func naturalCompare(a, b string) int {
	for len(a) > 0 && len(b) > 0 {
		aDigit, bDigit := isDigit(a[0]), isDigit(b[0])

		if aDigit && bDigit {
			aNum, aRest := splitDigits(a)
			bNum, bRest := splitDigits(b)
			// Compare numerically without parsing: strip leading zeros, then
			// longer runs are larger, equal lengths compare lexicographically.
			// This avoids overflow on absurdly long digit runs.
			aTrim := strings.TrimLeft(aNum, "0")
			bTrim := strings.TrimLeft(bNum, "0")
			if len(aTrim) != len(bTrim) {
				return cmpInt(len(aTrim), len(bTrim))
			}
			if c := strings.Compare(aTrim, bTrim); c != 0 {
				return c
			}
			// Equal in value; fall through so "01" and "1" stay deterministic.
			if c := cmpInt(len(aNum), len(bNum)); c != 0 {
				return c
			}
			a, b = aRest, bRest
			continue
		}

		if aDigit != bDigit {
			// Digits sort before letters, so frame_2 precedes frame_a.
			if aDigit {
				return -1
			}
			return 1
		}

		if a[0] != b[0] {
			if a[0] < b[0] {
				return -1
			}
			return 1
		}
		a, b = a[1:], b[1:]
	}
	return cmpInt(len(a), len(b))
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// splitDigits returns the leading digit run and the remainder.
func splitDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
