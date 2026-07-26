// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/pierrec/lz4/v4"
)

// gvHeader is the 24-byte little-endian header of the Extreme GPU Friendly
// Video Format, matching what chrplr/images2gv writes and what
// stimuli.GvVideo reads.
type gvHeader struct {
	Width      uint32
	Height     uint32
	FrameCount uint32
	FPS        float32
	Format     uint32 // 0 = raw RGBA (the only format goxpyriment reads)
	FrameBytes uint32 // uncompressed bytes per frame
}

// formatRawRGBA is the Format value for LZ4-compressed raw RGBA frames.
// goxpyriment's reader (stimuli/gvvideo_buf.go) LZ4-decompresses straight into
// a texture and never DXT-decodes, so this is the only format it can play.
const formatRawRGBA = 0

// gvWriter streams frames to a .gv file: a header, then LZ4-compressed frames
// back to back, then an index table of (address, size) pairs at the end that
// makes seeking O(1).
//
// Frames are handed in sequentially but compressed on a worker pool, since LZ4
// is the bulk of the cost once decoding is warm. Results are reordered before
// writing so the index stays in presentation order.
type gvWriter struct {
	out    *os.File
	w, h   int
	fps    float64
	frames []gvIndexEntry

	jobs    chan gvJob
	results chan gvJob
	wg      sync.WaitGroup

	next     int // next frame index to hand out
	written  int // next frame index to write
	pending  map[int][]byte
	writeErr error
}

type gvIndexEntry struct {
	Address uint64
	Size    uint64
}

type gvJob struct {
	index int
	src   []byte // owned by the job; released after compression
	data  []byte
	err   error
}

// newGVWriter creates the output file and writes a placeholder header. The
// frame count is not known until the source is exhausted, so Close rewrites it.
func newGVWriter(path string, w, h int, fps float64) (*gvWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	g := &gvWriter{
		out:     f,
		w:       w,
		h:       h,
		fps:     fps,
		pending: make(map[int][]byte),
	}

	if err := g.writeHeader(0); err != nil {
		f.Close()
		return nil, err
	}

	workers := runtime.NumCPU()
	// Bound the queues so a fast decoder cannot buffer the whole clip in RAM:
	// at 1920x1080 a single frame is 8 MB.
	g.jobs = make(chan gvJob, workers)
	g.results = make(chan gvJob, workers)
	for range workers {
		g.wg.Add(1)
		go g.worker()
	}
	return g, nil
}

func (g *gvWriter) writeHeader(frameCount uint32) error {
	h := gvHeader{
		Width:      uint32(g.w),
		Height:     uint32(g.h),
		FrameCount: frameCount,
		FPS:        float32(g.fps),
		Format:     formatRawRGBA,
		FrameBytes: uint32(g.w * g.h * 4),
	}
	return binary.Write(g.out, binary.LittleEndian, h)
}

func (g *gvWriter) worker() {
	defer g.wg.Done()
	for job := range g.jobs {
		buf := make([]byte, lz4.CompressBlockBound(len(job.src)))
		n, err := lz4.CompressBlock(job.src, buf, nil)
		if err != nil {
			job.err = err
		} else {
			// n == 0 means the block was incompressible; lz4 leaves the caller
			// to store it raw, which the .gv reader cannot express. Emitting a
			// truncated block here would silently corrupt the frame.
			if n == 0 {
				job.err = fmt.Errorf("frame %d is incompressible", job.index)
			}
			job.data = buf[:n]
		}
		job.src = nil
		g.results <- job
	}
}

// AddFrame queues one RGBA frame (len must be width*height*4). The slice is
// retained until compression finishes, so callers must not reuse it; pass a
// fresh or copied buffer.
func (g *gvWriter) AddFrame(rgba []byte) error {
	if want := g.w * g.h * 4; len(rgba) != want {
		return fmt.Errorf("frame %d: got %d bytes, want %d", g.next, len(rgba), want)
	}

	// Drain whatever is ready before blocking, so writing overlaps compression.
	for {
		select {
		case job := <-g.results:
			if err := g.collect(job); err != nil {
				return err
			}
		default:
		}

		select {
		case g.jobs <- gvJob{index: g.next, src: rgba}:
			g.next++
			return nil
		case job := <-g.results:
			if err := g.collect(job); err != nil {
				return err
			}
		}
	}
}

// collect stores a finished job and flushes any now-contiguous run of frames.
func (g *gvWriter) collect(job gvJob) error {
	if job.err != nil {
		return fmt.Errorf("compressing frame %d: %w", job.index, job.err)
	}
	g.pending[job.index] = job.data

	for {
		data, ok := g.pending[g.written]
		if !ok {
			return nil
		}
		pos, err := g.out.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		if _, err := g.out.Write(data); err != nil {
			return err
		}
		g.frames = append(g.frames, gvIndexEntry{
			Address: uint64(pos),
			Size:    uint64(len(data)),
		})
		delete(g.pending, g.written)
		g.written++
	}
}

// FrameCount returns how many frames have been written so far.
func (g *gvWriter) FrameCount() int { return g.written }

// Close drains the workers, writes the index table, patches the frame count
// into the header, and closes the file.
func (g *gvWriter) Close() error {
	close(g.jobs)
	go func() {
		g.wg.Wait()
		close(g.results)
	}()

	for job := range g.results {
		if err := g.collect(job); err != nil {
			g.out.Close()
			return err
		}
	}
	if len(g.pending) != 0 {
		g.out.Close()
		return fmt.Errorf("internal error: %d frames never written", len(g.pending))
	}

	for _, e := range g.frames {
		if err := binary.Write(g.out, binary.LittleEndian, e.Address); err != nil {
			g.out.Close()
			return err
		}
		if err := binary.Write(g.out, binary.LittleEndian, e.Size); err != nil {
			g.out.Close()
			return err
		}
	}

	// The frame count was a placeholder; the reader needs the real one to find
	// the index table (it seeks frameCount*16 back from EOF).
	if _, err := g.out.Seek(0, io.SeekStart); err != nil {
		g.out.Close()
		return err
	}
	if err := g.writeHeader(uint32(len(g.frames))); err != nil {
		g.out.Close()
		return err
	}
	return g.out.Close()
}
