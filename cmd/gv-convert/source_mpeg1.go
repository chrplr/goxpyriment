// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/gen2brain/mpeg"
)

// frameSource yields consecutive RGBA frames from a video file.
type frameSource interface {
	// Size returns the frame dimensions in pixels.
	Size() (w, h int)
	// FPS returns the source frame rate.
	FPS() float64
	// NextFrame returns the next frame as RGBA bytes (width*height*4), or
	// io.EOF when the source is exhausted. The returned slice is owned by the
	// caller and must not be reused by the source.
	NextFrame() ([]byte, error)
	// Close releases the underlying file or process.
	Close() error
	// Describe names the decoder, for progress output.
	Describe() string
}

// mpeg1Source decodes MPEG-1 program streams in pure Go, with no external
// tools. This is the only zero-dependency path; everything else needs ffmpeg.
type mpeg1Source struct {
	f    *os.File
	mpg  *mpeg.MPEG
	w, h int
}

// openMPEG1 opens an MPEG-1 program stream (.mpg/.mpeg). It returns an error
// if the file is not a program stream, which is also how the caller decides to
// fall back to ffmpeg.
func openMPEG1(path string) (*mpeg1Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	m, err := mpeg.New(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	if m.NumVideoStreams() == 0 {
		f.Close()
		return nil, fmt.Errorf("no video stream")
	}

	// .gv carries no audio, so decoding it would only waste time — and leaving
	// it enabled makes the demuxer buffer audio packets nothing ever drains.
	m.SetAudioEnabled(false)

	return &mpeg1Source{f: f, mpg: m, w: m.Width(), h: m.Height()}, nil
}

func (s *mpeg1Source) Size() (int, int) { return s.w, s.h }
func (s *mpeg1Source) FPS() float64     { return s.mpg.Framerate() }
func (s *mpeg1Source) Describe() string { return "MPEG-1 (pure Go)" }
func (s *mpeg1Source) Close() error     { return s.f.Close() }

func (s *mpeg1Source) NextFrame() ([]byte, error) {
	frame := s.mpg.Video().Decode()
	if frame == nil {
		return nil, io.EOF
	}
	// frame.RGBA() reuses an internal buffer, and the writer holds frames until
	// its workers finish, so hand over a copy.
	rgba := frame.RGBA()
	out := make([]byte, len(rgba.Pix))
	copy(out, rgba.Pix)
	return out, nil
}
