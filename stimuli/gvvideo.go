// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.
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

// PlayGv plays a .gv video file once, frame by frame, synchronised to VSYNC.
// x and y are center-based screen coordinates. Returns all user input events
// collected during playback and per-frame timing logs. Playback can be
// interrupted early with Escape or a window-close event.
//
// Each VideoFrameLog entry records the SDL flip timestamp and the number of
// display frames skipped (SkippedFrames > 0 means the decode+draw loop was
// too slow and missed one or more VSYNC edges).
func PlayGv(screen *xio.Screen, path string, x, y float32) ([]UserEvent, []VideoFrameLog, error) {
	v, err := NewGvVideo(path)
	if err != nil {
		return nil, nil, fmt.Errorf("PlayGv: %w", err)
	}
	defer v.Unload()

	if err := v.preload(screen); err != nil {
		return nil, nil, fmt.Errorf("PlayGv: %w", err)
	}

	defer disableGC()()

	v.Position = sdl.FPoint{X: x, Y: y}

	frameDurNS := uint64(screen.FrameDuration().Nanoseconds())

	var userEvents []UserEvent
	var logs []VideoFrameLog
	var prevFlipNS uint64
	streamStart := time.Now()

	for i := 0; i < v.FrameCount; i++ {
		if err := v.updateFrame(i); err != nil {
			return userEvents, logs, fmt.Errorf("PlayGv frame %d: %w", i, err)
		}
		if err := screen.Clear(); err != nil {
			return userEvents, logs, fmt.Errorf("PlayGv: clearing screen: %w", err)
		}
		if err := v.Draw(screen); err != nil {
			return userEvents, logs, fmt.Errorf("PlayGv: drawing frame %d: %w", i, err)
		}
		flipNS, err := screen.FlipTS()
		if err != nil {
			return userEvents, logs, fmt.Errorf("PlayGv: flipping display: %w", err)
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

	return userEvents, logs, nil
}
