// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
package stimuli

import (
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/funatsufumiya/go-gv-video/gvvideo"

	xio "github.com/chrplr/goxpyriment/apparatus"
)

// VideoFrameLog records the actual display timing for one video frame.
type VideoFrameLog struct {
	Frame         int    // video frame index (0-based)
	FlipNS        uint64 // SDL nanosecond timestamp after the flip
	SkippedFrames int    // display frames late (0 = on time, ≥1 = frame(s) dropped)
}

// GvVideo is a video stimulus decoded from a .gv file (LZ4-compressed RGBA frames).
//
// Embeds BaseVisual for position management. Overrides Unload to destroy
// the GPU texture and close the underlying file handle.
type GvVideo struct {
	BaseVisual    // Position, GetPosition, SetPosition, Preload, Unload (Unload overridden below)
	gv            *gvvideo.GVVideo
	texture       *sdl.Texture
	rgba          []byte // decompressed RGBA buffer, one frame
	compressedBuf []byte // LZ4-compressed scratch buffer, max frame size
	Width         float32
	Height        float32
	FrameCount    int
	FPS           float64
	filePath      string

	currentFrame int
	playing      bool
	paused       bool
	done         bool // true once all frames shown; ensures Update() keeps returning io.EOF
}

// NewGvVideo opens a .gv file, reads its header and frame index, and returns
// a GvVideo ready for playback. No GPU resources are allocated yet.
func NewGvVideo(path string) (*GvVideo, error) {
	gv, err := gvvideo.LoadGVVideo(path)
	if err != nil {
		return nil, fmt.Errorf("NewGvVideo: %w", err)
	}
	return &GvVideo{
		gv:         gv,
		Width:      float32(gv.Header.Width),
		Height:     float32(gv.Header.Height),
		FrameCount: int(gv.Header.FrameCount),
		FPS:        float64(gv.Header.FPS),
		filePath:   path,
	}, nil
}

// preload creates the streaming GPU texture and allocates the decompression buffer.
func (v *GvVideo) preload(screen *xio.Screen) error {
	tex, err := screen.Renderer.CreateTexture(
		sdl.PIXELFORMAT_RGBA32,
		sdl.TEXTUREACCESS_STREAMING,
		int(v.gv.Header.Width),
		int(v.gv.Header.Height),
	)
	if err != nil {
		return fmt.Errorf("GvVideo preload: create texture: %w", err)
	}
	v.texture = tex
	v.rgba = make([]byte, v.gv.Header.FrameBytes)
	v.compressedBuf = make([]byte, gvMaxCompressedSize(v.gv))
	return nil
}

// Preload is provided by BaseVisual (no-op; GPU setup requires a Screen,
// so it is deferred to the first Draw call via preload).

// Unload overrides BaseVisual.Unload to destroy the GPU texture and close the file.
func (v *GvVideo) Unload() error {
	if v.texture != nil {
		v.texture.Destroy()
		v.texture = nil
	}
	if v.gv != nil {
		if closer, ok := v.gv.Reader.(io.Closer); ok {
			closer.Close()
		}
		v.gv = nil
	}
	v.rgba = nil
	v.compressedBuf = nil
	return nil
}

// updateFrame decompresses frame frameID into the GPU texture.
func (v *GvVideo) updateFrame(frameID int) error {
	if err := gvReadFrameInto(v.gv, uint32(frameID), v.compressedBuf, v.rgba); err != nil {
		return fmt.Errorf("GvVideo updateFrame %d: %w", frameID, err)
	}
	return v.texture.Update(nil, v.rgba, int32(v.gv.Header.Width*4))
}

// Draw renders the current texture centered at v.Position.
// If not yet preloaded, it lazy-initialises GPU resources.
func (v *GvVideo) Draw(screen *xio.Screen) error {
	if v.texture == nil {
		if err := v.preload(screen); err != nil {
			return fmt.Errorf("GvVideo.Draw: %w", err)
		}
	}
	destRect := screen.CenteredRect(v.Position, v.Width, v.Height)
	return screen.Renderer.RenderTexture(v.texture, nil, destRect)
}

// DrawAt renders the current texture into dest, a custom screen rectangle.
// Lazy-initialises GPU resources if not yet done.
func (v *GvVideo) DrawAt(screen *xio.Screen, dest *sdl.FRect) error {
	if v.texture == nil {
		if err := v.preload(screen); err != nil {
			return fmt.Errorf("GvVideo.DrawAt: %w", err)
		}
	}
	return screen.Renderer.RenderTexture(v.texture, nil, dest)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (v *GvVideo) Present(screen *xio.Screen, clear, update bool) error {
	return PresentDrawable(v, screen, clear, update)
}

// Play starts playback from frame 0, or resumes after Pause.
func (v *GvVideo) Play() {
	if !v.playing {
		v.playing, v.paused, v.currentFrame, v.done = true, false, 0, false
	} else if v.paused {
		v.paused = false
	}
}

// Pause freezes playback at the current frame. Update becomes a no-op until Play is called.
func (v *GvVideo) Pause() {
	if v.playing && !v.paused {
		v.paused = true
	}
}

// Rewind jumps back to frame 0 and resumes playback.
// Random-access frame storage means no file seek or re-open is required.
func (v *GvVideo) Rewind() {
	v.currentFrame, v.playing, v.paused, v.done = 0, true, false, false
}

// IsPlaying returns true if Play has been called and the video has not yet ended.
func (v *GvVideo) IsPlaying() bool { return v.playing }

// IsPaused returns true if Pause has been called and not yet resumed.
func (v *GvVideo) IsPaused() bool { return v.paused }

// CurrentFrame returns the 0-based index of the frame currently loaded in the
// GPU texture — the one the next flip will put on screen — or -1 before the
// first Update.
//
// Use it to act on a specific frame in a hand-rolled playback loop, for example
// to raise a TTL line on the frame a stimulus appears.
func (v *GvVideo) CurrentFrame() int { return v.currentFrame - 1 }

// Update decompresses and uploads the next frame to the GPU texture.
// Returns io.EOF when all frames have been presented (and on every subsequent call).
// Returns nil without advancing if the video is paused or not yet started.
// Lazy-initialises GPU resources on the first call.
func (v *GvVideo) Update(screen *xio.Screen) error {
	if v.done {
		return io.EOF
	}
	if !v.playing || v.paused {
		return nil
	}
	if v.texture == nil {
		if err := v.preload(screen); err != nil {
			return fmt.Errorf("GvVideo.Update: %w", err)
		}
	}
	if v.currentFrame >= v.FrameCount {
		v.playing, v.done = false, true
		return io.EOF
	}
	if err := v.updateFrame(v.currentFrame); err != nil {
		return fmt.Errorf("GvVideo.Update: %w", err)
	}
	v.currentFrame++
	return nil
}

// Close releases GPU texture and file handle. Alias for Unload for API symmetry with Video.
func (v *GvVideo) Close() { v.Unload() }

// GetPosition, SetPosition are provided by BaseVisual.

// framesPerVideoFrame returns how many display refreshes each video frame must
// be held for so the clip plays at its authored rate.
//
// Both rates are rounded to the nearest integer first, so a display reporting
// 59.94 Hz and a file authored at 29.97 fps are treated as 60 and 30. Only an
// exact integer ratio is supported: at 60 Hz a 30 fps clip holds each frame for
// 2 refreshes, but a 24 fps clip would need 2.5 and is rejected rather than
// silently played at the wrong speed (the 3:2 pulldown that would be required
// makes frame onsets uneven, which defeats the point of using .gv).
func framesPerVideoFrame(refreshHz, fileFPS float64) (int, error) {
	if fileFPS <= 0 {
		return 0, fmt.Errorf("video reports fps=%g; the .gv header is missing a frame rate", fileFPS)
	}
	if refreshHz <= 0 {
		return 0, fmt.Errorf("could not determine the display refresh rate")
	}

	refresh := int(math.Round(refreshHz))
	fps := int(math.Round(fileFPS))
	if fps < 1 {
		return 0, fmt.Errorf("video fps %g rounds to %d", fileFPS, fps)
	}
	if refresh < fps {
		return 0, fmt.Errorf(
			"display refresh %d Hz is slower than the video's %d fps; the clip cannot be shown at its authored rate",
			refresh, fps)
	}
	if refresh%fps != 0 {
		return 0, fmt.Errorf(
			"display refresh %d Hz is not an integer multiple of the video's %d fps (%.3f frames per refresh); "+
				"re-encode the clip at a divisor of %d fps, e.g. gv-convert -fps %d",
			refresh, fps, float64(refresh)/float64(fps), refresh, largestDivisorAtMost(refresh, fps))
	}
	return refresh / fps, nil
}

// largestDivisorAtMost returns the largest divisor of refresh that is <= want,
// used only to suggest a workable frame rate in the error above.
func largestDivisorAtMost(refresh, want int) int {
	for d := want; d >= 1; d-- {
		if refresh%d == 0 {
			return d
		}
	}
	return 1
}

// GvFrameContext is passed to a [GvFrameCallback] once per video frame, right
// after the flip that puts that frame on screen.
type GvFrameContext struct {
	Screen *xio.Screen
	// Frame is the 0-based video frame index just presented.
	Frame int
	// OnsetNS is the SDL nanosecond timestamp captured immediately after the
	// frame's first flip. It shares its reference frame with SDL event
	// timestamps, so it subtracts directly from a keypress timestamp.
	OnsetNS uint64
	// Hold is how many display refreshes this frame is shown for.
	Hold int
}

// GvFrameCallback runs once per video frame, immediately after that frame's
// first flip. Return nil to continue, [sdl.EndLoop] to stop playback cleanly,
// or any other error to abort.
//
// It is called inside a GC-disabled, VSYNC-locked loop: do not allocate
// heavily, and never sleep or call a blocking trigger Pulse, or the next frame
// will be late. Raise a TTL line here and lower it from a later frame instead.
type GvFrameCallback func(ctx GvFrameContext) error

// GvMark asks [PlayGvFunc] to mark the onset of one video frame with the
// screen's onset marker (see apparatus.Marker, set up by control.MarkerFlags):
// the photodiode square turns white in the frame's first flip and Code is
// written to the TTL device right after it. Code 0 marks it with the square
// only. Without a marker attached to the screen a mark does nothing.
type GvMark struct {
	Frame int  // 0-based video frame index
	Code  byte // TTL code; 0 = photodiode square only
}

// PlayGvFunc is [PlayGv] with a per-frame callback, for acting on the onset of
// a specific frame — logging a timestamp, or aborting early on some condition.
// onFrame may be nil.
//
// marks lists the frames whose onset is marked for a recording:
//
//	stimuli.PlayGvFunc(exp.Screen, "movie.gv", 0, 0, nil,
//		stimuli.GvMark{Frame: 0, Code: 1}, stimuli.GvMark{Frame: 186, Code: 2})
//
// The marker is armed before the frame's first refresh, so the square and the
// code land on the flip reported as that frame's FlipNS, whatever the number
// of refreshes each video frame is held for. Arming from onFrame cannot do
// that: it runs after the frame's first flip, so it can only mark the flip
// after it. A frame outside the video, or listed twice, is an error before
// playback starts. When the last marked frame leaves the square still white
// at the end of the video, the last frame is held, redrawn, until the square
// is black again (at most the marker's frame count).
func PlayGvFunc(screen *xio.Screen, path string, x, y float32, onFrame GvFrameCallback, marks ...GvMark) ([]UserEvent, []VideoFrameLog, error) {
	return playGv(screen, path, x, y, onFrame, marks)
}

// gvMarkTable checks marks against the video's frame count and returns them
// as a lookup from frame index to code; nil when there are none.
func gvMarkTable(marks []GvMark, frameCount int) (map[int]byte, error) {
	if len(marks) == 0 {
		return nil, nil
	}
	table := make(map[int]byte, len(marks))
	for _, m := range marks {
		if m.Frame < 0 || m.Frame >= frameCount {
			return nil, fmt.Errorf("mark on frame %d: the video has frames 0-%d", m.Frame, frameCount-1)
		}
		if _, dup := table[m.Frame]; dup {
			return nil, fmt.Errorf("frame %d is marked twice", m.Frame)
		}
		table[m.Frame] = m.Code
	}
	return table, nil
}

// PlayGv plays a .gv video file once, frame by frame, synchronised to VSYNC.
// x and y are center-based screen coordinates. Returns all user input events
// collected during playback and per-frame timing logs. Playback can be
// interrupted early with Escape or a window-close event.
//
// Each VideoFrameLog entry records the SDL flip timestamp and the number of
// display frames skipped (SkippedFrames > 0 means the decode+draw loop was
// too slow and missed one or more VSYNC edges).
func PlayGv(screen *xio.Screen, path string, x, y float32) ([]UserEvent, []VideoFrameLog, error) {
	return playGv(screen, path, x, y, nil, nil)
}

// playGv is the shared implementation of PlayGv and PlayGvFunc. onFrame and
// marks may be nil.
func playGv(screen *xio.Screen, path string, x, y float32, onFrame GvFrameCallback, marks []GvMark) ([]UserEvent, []VideoFrameLog, error) {
	v, err := NewGvVideo(path)
	if err != nil {
		return nil, nil, fmt.Errorf("PlayGv: %w", err)
	}
	defer v.Unload()

	markAt, err := gvMarkTable(marks, v.FrameCount)
	if err != nil {
		return nil, nil, fmt.Errorf("PlayGv %q: %w", path, err)
	}

	if err := v.preload(screen); err != nil {
		return nil, nil, fmt.Errorf("PlayGv: %w", err)
	}

	defer disableGC()()

	v.Position = sdl.FPoint{X: x, Y: y}

	frameDur := screen.FrameDuration()
	frameDurNS := uint64(frameDur.Nanoseconds())

	// Hold each video frame for as many refreshes as its authored rate needs,
	// or the clip plays at the monitor's rate instead of its own — twice too
	// fast for a 30 fps file on a 60 Hz display.
	hold, err := framesPerVideoFrame(float64(time.Second)/float64(frameDur), v.FPS)
	if err != nil {
		return nil, nil, fmt.Errorf("PlayGv %q: %w", path, err)
	}

	var userEvents []UserEvent
	var logs []VideoFrameLog
	var prevFlipNS uint64
	streamStart := time.Now()

	for i := 0; i < v.FrameCount; i++ {
		if err := v.updateFrame(i); err != nil {
			return userEvents, logs, fmt.Errorf("PlayGv frame %d: %w", i, err)
		}

		// Re-draw before every flip: SDL leaves backbuffer contents undefined
		// after a present, so holding a frame by not drawing flickers on
		// double-buffered drivers.
		var flipNS uint64
		for h := 0; h < hold; h++ {
			if err := screen.Clear(); err != nil {
				return userEvents, logs, fmt.Errorf("PlayGv: clearing screen: %w", err)
			}
			if err := v.Draw(screen); err != nil {
				return userEvents, logs, fmt.Errorf("PlayGv: drawing frame %d: %w", i, err)
			}
			// Armed before the frame's first flip, so the marker lands on
			// the flip logged as this frame's onset.
			if h == 0 && markAt != nil {
				if code, ok := markAt[i]; ok {
					screen.Marker().Arm(code)
				}
			}
			ts, err := screen.FlipTS()
			if err != nil {
				return userEvents, logs, fmt.Errorf("PlayGv: flipping display: %w", err)
			}
			if h == 0 {
				flipNS = ts // the frame's onset is its first flip
				// Called before the remaining hold flips so a trigger raised
				// here lands as close to the onset as possible.
				if onFrame != nil {
					if err := onFrame(GvFrameContext{
						Screen:  screen,
						Frame:   i,
						OnsetNS: flipNS,
						Hold:    hold,
					}); err != nil {
						if errors.Is(err, sdl.EndLoop) {
							return userEvents, logs, nil
						}
						return userEvents, logs, fmt.Errorf("PlayGv: frame callback at frame %d: %w", i, err)
					}
				}
			}
		}

		skipped := 0
		if prevFlipNS > 0 && frameDurNS > 0 {
			gapNS := flipNS - prevFlipNS
			displayFrames := int(math.Round(float64(gapNS) / float64(frameDurNS)))
			if displayFrames > hold {
				skipped = displayFrames - hold
			}
		}
		logs = append(logs, VideoFrameLog{Frame: i, FlipNS: flipNS, SkippedFrames: skipped})
		prevFlipNS = flipNS

		userEvents = collectEvents(streamStart, userEvents)

		// Check for early exit
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

	// A mark near the end can leave the square white when the video stops,
	// and whatever follows may not flip for a while (a key wait): hold the
	// last frame until the square is black. Busy is false within the marker's
	// frame count plus one; the bound only guards against a bug.
	if markAt != nil {
		m := screen.Marker()
		for n := 0; m.Busy() && n <= m.Frames()+1; n++ {
			if err := screen.Clear(); err != nil {
				return userEvents, logs, fmt.Errorf("PlayGv: clearing screen: %w", err)
			}
			if err := v.Draw(screen); err != nil {
				return userEvents, logs, fmt.Errorf("PlayGv: drawing the last frame: %w", err)
			}
			if err := screen.Update(); err != nil {
				return userEvents, logs, fmt.Errorf("PlayGv: flipping display: %w", err)
			}
		}
	}

	return userEvents, logs, nil
}
