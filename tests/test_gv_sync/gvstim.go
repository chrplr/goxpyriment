// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

// Synthesises the .gv stimulus used by the `gvsync` test: a square-wave flash
// train of `cycles` repetitions of (framesOn bright frames, framesOff dark
// frames).
//
// Generating it here rather than shipping a binary fixture keeps the file in
// step with the flags — change -frames-on and the stimulus changes with it —
// and means the test has no external asset to lose. The writer is deliberately
// minimal; cmd/gv-convert is the general-purpose converter.

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/pierrec/lz4/v4"
)

// gvStimSpec describes the flash train to synthesise.
type gvStimSpec struct {
	Width, Height int
	FPS           float64
	FramesOn      int // bright frames per cycle
	FramesOff     int // dark frames per cycle
	Cycles        int
	SquarePx      int  // side of the centred bright square, 0 = full screen
	Bright, Dark  byte // luminance levels
}

// FrameCount is the total number of video frames in the generated file.
func (s gvStimSpec) FrameCount() int { return s.Cycles * (s.FramesOn + s.FramesOff) }

// IsBright reports whether video frame i falls in a bright phase. The test uses
// this to decide when to fire a trigger, so the stimulus and the trigger schedule
// are derived from one definition rather than two that can drift apart.
func (s gvStimSpec) IsBright(i int) bool {
	period := s.FramesOn + s.FramesOff
	return period > 0 && i%period < s.FramesOn
}

// IsOnset reports whether video frame i is the first bright frame of a cycle.
func (s gvStimSpec) IsOnset(i int) bool {
	period := s.FramesOn + s.FramesOff
	return period > 0 && i%period == 0 && s.FramesOn > 0
}

// writeGVStim renders the flash train to a .gv file: a 24-byte header, then
// LZ4-compressed raw-RGBA frames, then an (address, size) index table.
//
// Only two distinct frames exist, so each is compressed once and the resulting
// block is written repeatedly — a 300-frame 1920x1080 train would otherwise
// cost 2.4 GB of compression work for two images.
func writeGVStim(path string, s gvStimSpec) error {
	if s.FramesOn <= 0 || s.Cycles <= 0 {
		return fmt.Errorf("gvstim: frames-on (%d) and cycles (%d) must be positive", s.FramesOn, s.Cycles)
	}
	if s.Width <= 0 || s.Height <= 0 {
		return fmt.Errorf("gvstim: invalid size %dx%d", s.Width, s.Height)
	}

	brightBlock, err := compressFrame(renderFlashFrame(s, true))
	if err != nil {
		return fmt.Errorf("gvstim: compressing bright frame: %w", err)
	}
	darkBlock, err := compressFrame(renderFlashFrame(s, false))
	if err != nil {
		return fmt.Errorf("gvstim: compressing dark frame: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	n := s.FrameCount()
	header := struct {
		Width, Height, FrameCount uint32
		FPS                       float32
		Format, FrameBytes        uint32
	}{
		Width: uint32(s.Width), Height: uint32(s.Height), FrameCount: uint32(n),
		FPS: float32(s.FPS), Format: 0, // 0 = raw RGBA
		FrameBytes: uint32(s.Width * s.Height * 4),
	}
	if err := binary.Write(f, binary.LittleEndian, header); err != nil {
		return err
	}

	type entry struct{ addr, size uint64 }
	index := make([]entry, 0, n)
	for i := range n {
		block := darkBlock
		if s.IsBright(i) {
			block = brightBlock
		}
		pos, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		if _, err := f.Write(block); err != nil {
			return err
		}
		index = append(index, entry{uint64(pos), uint64(len(block))})
	}
	for _, e := range index {
		if err := binary.Write(f, binary.LittleEndian, e.addr); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, e.size); err != nil {
			return err
		}
	}
	return nil
}

// renderFlashFrame builds one RGBA frame: a centred bright square on a dark
// field, or a uniformly dark field.
func renderFlashFrame(s gvStimSpec, bright bool) []byte {
	buf := make([]byte, s.Width*s.Height*4)

	// Start uniformly dark, opaque.
	for i := 0; i < len(buf); i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = s.Dark, s.Dark, s.Dark, 255
	}
	if !bright {
		return buf
	}

	x0, y0, x1, y1 := 0, 0, s.Width, s.Height
	if s.SquarePx > 0 {
		// Centre a square of SquarePx sides, clipped to the frame.
		x0 = max((s.Width-s.SquarePx)/2, 0)
		y0 = max((s.Height-s.SquarePx)/2, 0)
		x1 = min(x0+s.SquarePx, s.Width)
		y1 = min(y0+s.SquarePx, s.Height)
	}
	for y := y0; y < y1; y++ {
		row := y * s.Width * 4
		for x := x0; x < x1; x++ {
			i := row + x*4
			buf[i], buf[i+1], buf[i+2] = s.Bright, s.Bright, s.Bright
		}
	}
	return buf
}

// compressFrame LZ4-compresses one raw frame into a .gv frame block.
func compressFrame(raw []byte) ([]byte, error) {
	out := make([]byte, lz4.CompressBlockBound(len(raw)))
	n, err := lz4.CompressBlock(raw, out, nil)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		// Incompressible: .gv has no way to express a stored block, and a
		// zero-length one would decode as a corrupt frame.
		return nil, fmt.Errorf("frame is incompressible")
	}
	return out[:n], nil
}
