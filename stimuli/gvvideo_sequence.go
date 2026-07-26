// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
package stimuli

import (
	"fmt"
	"io"
	"math"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/funatsufumiya/go-gv-video/gvvideo"

	xio "github.com/chrplr/goxpyriment/apparatus"
)

// GvVideoSequence plays multiple .gv clips back-to-back without dropping a
// frame at the boundary. All clips must share the same dimensions and FPS;
// a single GPU streaming texture is reused across all clips.
type GvVideoSequence struct {
	BaseVisual
	gvs                []*gvvideo.GVVideo
	offsets            []int  // offsets[i] = first global frame index of gvs[i]
	frameBytes         uint32 // max decompressed frame size across all clips
	maxCompressedBytes uint64 // max LZ4-compressed frame size across all clips
	texture            *sdl.Texture
	rgba               []byte // decompressed RGBA buffer, one frame
	compressedBuf      []byte // LZ4-compressed scratch buffer, max frame size
	Width              float32
	Height             float32
	FPS                float64
	FrameCount         int

	currentFrame int
	playing      bool
	paused       bool
	done         bool
}

// NewGvVideoSequence opens each .gv file in paths and returns a
// GvVideoSequence ready for playback. All clips must have the same Width,
// Height, and FPS; an error is returned if any file is incompatible.
// No GPU resources are allocated until the first Draw or Update call.
func NewGvVideoSequence(paths []string) (*GvVideoSequence, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("NewGvVideoSequence: no paths provided")
	}

	gvs := make([]*gvvideo.GVVideo, 0, len(paths))
	offsets := make([]int, 0, len(paths))
	var frameBytes uint32
	total := 0

	closeAll := func() {
		for _, g := range gvs {
			if c, ok := g.Reader.(io.Closer); ok {
				c.Close()
			}
		}
	}

	for i, path := range paths {
		gv, err := gvvideo.LoadGVVideo(path)
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("NewGvVideoSequence %s: %w", path, err)
		}
		if i == 0 {
			frameBytes = gv.Header.FrameBytes
		} else {
			h0, h := gvs[0].Header, gv.Header
			if h.Width != h0.Width || h.Height != h0.Height {
				if c, ok := gv.Reader.(io.Closer); ok {
					c.Close()
				}
				closeAll()
				return nil, fmt.Errorf("NewGvVideoSequence: %s: dimensions %dx%d differ from first clip %dx%d",
					path, h.Width, h.Height, h0.Width, h0.Height)
			}
			if h.FPS != h0.FPS {
				if c, ok := gv.Reader.(io.Closer); ok {
					c.Close()
				}
				closeAll()
				return nil, fmt.Errorf("NewGvVideoSequence: %s: FPS %.2f differs from first clip %.2f",
					path, h.FPS, h0.FPS)
			}
			if h.FrameBytes > frameBytes {
				frameBytes = h.FrameBytes
			}
		}
		offsets = append(offsets, total)
		total += int(gv.Header.FrameCount)
		gvs = append(gvs, gv)
	}

	var maxCompressedBytes uint64
	for _, gv := range gvs {
		if c := gvMaxCompressedSize(gv); c > maxCompressedBytes {
			maxCompressedBytes = c
		}
	}

	h := gvs[0].Header
	return &GvVideoSequence{
		gvs:                gvs,
		offsets:            offsets,
		frameBytes:         frameBytes,
		maxCompressedBytes: maxCompressedBytes,
		Width:              float32(h.Width),
		Height:             float32(h.Height),
		FPS:                float64(h.FPS),
		FrameCount:         total,
	}, nil
}

// segmentFor maps a global frame index to the owning GVVideo and local frame index.
func (s *GvVideoSequence) segmentFor(globalFrame int) (*gvvideo.GVVideo, uint32) {
	idx := len(s.offsets) - 1
	for i := 1; i < len(s.offsets); i++ {
		if globalFrame < s.offsets[i] {
			idx = i - 1
			break
		}
	}
	return s.gvs[idx], uint32(globalFrame - s.offsets[idx])
}

func (s *GvVideoSequence) preload(screen *xio.Screen) error {
	tex, err := screen.Renderer.CreateTexture(
		sdl.PIXELFORMAT_RGBA32,
		sdl.TEXTUREACCESS_STREAMING,
		int(s.gvs[0].Header.Width),
		int(s.gvs[0].Header.Height),
	)
	if err != nil {
		return fmt.Errorf("GvVideoSequence preload: %w", err)
	}
	s.texture = tex
	s.rgba = make([]byte, s.frameBytes)
	s.compressedBuf = make([]byte, s.maxCompressedBytes)
	return nil
}

func (s *GvVideoSequence) updateFrame(globalFrame int) error {
	gv, localFrame := s.segmentFor(globalFrame)
	if err := gvReadFrameInto(gv, localFrame, s.compressedBuf, s.rgba); err != nil {
		return fmt.Errorf("GvVideoSequence updateFrame %d: %w", globalFrame, err)
	}
	return s.texture.Update(nil, s.rgba, int32(s.gvs[0].Header.Width*4))
}

// Unload destroys the GPU texture and closes all file handles.
func (s *GvVideoSequence) Unload() error {
	_ = destroyTexture(&s.texture)
	for _, gv := range s.gvs {
		if gv != nil {
			if c, ok := gv.Reader.(io.Closer); ok {
				c.Close()
			}
		}
	}
	s.gvs = nil
	s.rgba = nil
	s.compressedBuf = nil
	return nil
}

// Draw renders the current texture centered at s.Position.
// Lazy-initialises GPU resources on the first call.
func (s *GvVideoSequence) Draw(screen *xio.Screen) error {
	if s.texture == nil {
		if err := s.preload(screen); err != nil {
			return fmt.Errorf("GvVideoSequence.Draw: %w", err)
		}
	}
	destRect := screen.CenteredRect(s.Position, s.Width, s.Height)
	return screen.Renderer.RenderTexture(s.texture, nil, destRect)
}

// DrawAt renders the current texture into dest, a custom screen rectangle.
// Lazy-initialises GPU resources if not yet done.
func (s *GvVideoSequence) DrawAt(screen *xio.Screen, dest *sdl.FRect) error {
	if s.texture == nil {
		if err := s.preload(screen); err != nil {
			return fmt.Errorf("GvVideoSequence.DrawAt: %w", err)
		}
	}
	return screen.Renderer.RenderTexture(s.texture, nil, dest)
}

// Play starts playback from frame 0, or resumes after Pause.
func (s *GvVideoSequence) Play() {
	if !s.playing {
		s.playing, s.paused, s.currentFrame, s.done = true, false, 0, false
	} else if s.paused {
		s.paused = false
	}
}

// Pause freezes playback at the current frame.
func (s *GvVideoSequence) Pause() {
	if s.playing && !s.paused {
		s.paused = true
	}
}

// Rewind jumps back to frame 0 and resumes playback.
func (s *GvVideoSequence) Rewind() {
	s.currentFrame, s.playing, s.paused, s.done = 0, true, false, false
}

// IsPlaying returns true if Play has been called and the sequence has not ended.
func (s *GvVideoSequence) IsPlaying() bool { return s.playing }

// IsPaused returns true if Pause has been called and not yet resumed.
func (s *GvVideoSequence) IsPaused() bool { return s.paused }

// Update decompresses and uploads the next frame to the GPU texture.
// Returns io.EOF when all frames across all clips have been presented (and on
// every subsequent call). Returns nil without advancing if paused or not started.
// Lazy-initialises GPU resources on the first call.
func (s *GvVideoSequence) Update(screen *xio.Screen) error {
	if s.done {
		return io.EOF
	}
	if !s.playing || s.paused {
		return nil
	}
	if s.texture == nil {
		if err := s.preload(screen); err != nil {
			return fmt.Errorf("GvVideoSequence.Update: %w", err)
		}
	}
	if s.currentFrame >= s.FrameCount {
		s.playing, s.done = false, true
		return io.EOF
	}
	if err := s.updateFrame(s.currentFrame); err != nil {
		return fmt.Errorf("GvVideoSequence.Update: %w", err)
	}
	s.currentFrame++
	return nil
}

// Close releases GPU texture and file handles. Alias for Unload.
func (s *GvVideoSequence) Close() { s.Unload() }

// PlayGvSequence plays multiple .gv clips as a seamless VSYNC-locked sequence.
// x and y are center-based screen coordinates. Returns all user input events
// collected during playback and per-frame timing logs. Playback can be
// interrupted early with Escape or a window-close event.
func PlayGvSequence(screen *xio.Screen, paths []string, x, y float32) ([]UserEvent, []VideoFrameLog, error) {
	seq, err := NewGvVideoSequence(paths)
	if err != nil {
		return nil, nil, fmt.Errorf("PlayGvSequence: %w", err)
	}
	defer seq.Unload()

	if err := seq.preload(screen); err != nil {
		return nil, nil, fmt.Errorf("PlayGvSequence: %w", err)
	}

	defer disableGC()()

	seq.Position = sdl.FPoint{X: x, Y: y}

	frameDurNS := uint64(screen.FrameDuration().Nanoseconds())

	var userEvents []UserEvent
	var logs []VideoFrameLog
	var prevFlipNS uint64
	streamStart := time.Now()

	for i := 0; i < seq.FrameCount; i++ {
		if err := seq.updateFrame(i); err != nil {
			return userEvents, logs, fmt.Errorf("PlayGvSequence frame %d: %w", i, err)
		}
		if err := screen.Clear(); err != nil {
			return userEvents, logs, fmt.Errorf("PlayGvSequence: clearing screen: %w", err)
		}
		if err := seq.Draw(screen); err != nil {
			return userEvents, logs, fmt.Errorf("PlayGvSequence: drawing frame %d: %w", i, err)
		}
		flipNS, err := screen.FlipTS()
		if err != nil {
			return userEvents, logs, fmt.Errorf("PlayGvSequence: flipping display: %w", err)
		}

		skipped := 0
		if prevFlipNS > 0 && frameDurNS > 0 {
			gapNS := flipNS - prevFlipNS
			displayFrames := int(math.Round(float64(gapNS) / float64(frameDurNS)))
			if displayFrames > 1 {
				skipped = displayFrames - 1
			}
		}
		logs = append(logs, VideoFrameLog{Frame: i, FlipNS: flipNS, SkippedFrames: skipped})
		prevFlipNS = flipNS

		userEvents = collectEvents(streamStart, userEvents)

		for _, ue := range userEvents {
			if ue.Event.Type == sdl.EVENT_QUIT {
				return userEvents, logs, nil
			}
			if ue.Event.Type == sdl.EVENT_KEY_DOWN {
				if ue.Event.KeyboardEvent().Key == sdl.K_ESCAPE {
					return userEvents, logs, nil
				}
			}
		}
	}

	return userEvents, logs, nil
}
