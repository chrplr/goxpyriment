// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.
package stimuli

import (
	"fmt"
	"io"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/funatsufumiya/go-gv-video/gvvideo"

	xio "github.com/chrplr/goxpyriment/apparatus"
)

// GvVideo is a video stimulus decoded from a .gv file (LZ4-compressed RGBA frames).
//
// Embeds BaseVisual for position management. Overrides Unload to destroy
// the GPU texture and close the underlying file handle.
type GvVideo struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload (Unload overridden below)
	gv         *gvvideo.GVVideo
	texture    *sdl.Texture
	rgba       []byte
	Width      float32
	Height     float32
	FrameCount int
	FPS        float64
	filePath   string
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
	return nil
}

// updateFrame decompresses frame frameID into the GPU texture.
func (v *GvVideo) updateFrame(frameID int) error {
	if err := v.gv.ReadFrameCompressedTo(uint32(frameID), v.rgba); err != nil {
		return fmt.Errorf("GvVideo updateFrame %d: %w", frameID, err)
	}
	return v.texture.Update(nil, v.rgba, int32(v.gv.Header.Width*4))
}

// Draw renders the current texture centered at v.Position.
// If not yet preloaded, it lazy-initialises GPU resources.
func (v *GvVideo) Draw(screen *xio.Screen) error {
	if v.texture == nil {
		if err := v.preload(screen); err != nil {
			return err
		}
	}
	destX, destY := screen.CenterToSDL(v.Position.X, v.Position.Y)
	destRect := &sdl.FRect{
		X: destX - v.Width/2,
		Y: destY - v.Height/2,
		W: v.Width,
		H: v.Height,
	}
	return screen.Renderer.RenderTexture(v.texture, nil, destRect)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (v *GvVideo) Present(screen *xio.Screen, clear, update bool) error {
	return PresentDrawable(v, screen, clear, update)
}

// GetPosition, SetPosition are provided by BaseVisual.

// PlayGv plays a .gv video file once, frame by frame, synchronised to VSYNC.
// x and y are center-based screen coordinates. It returns all user input events
// collected during playback. Playback can be interrupted early with Escape or
// a window-close event.
func PlayGv(screen *xio.Screen, path string, x, y float32) ([]UserEvent, error) {
	v, err := NewGvVideo(path)
	if err != nil {
		return nil, err
	}
	defer v.Unload()

	if err := v.preload(screen); err != nil {
		return nil, err
	}

	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	v.Position = sdl.FPoint{X: x, Y: y}

	var userEvents []UserEvent
	streamStart := time.Now()

	for i := 0; i < v.FrameCount; i++ {
		if err := v.updateFrame(i); err != nil {
			return userEvents, fmt.Errorf("PlayGv frame %d: %w", i, err)
		}
		if err := screen.Clear(); err != nil {
			return userEvents, err
		}
		if err := v.Draw(screen); err != nil {
			return userEvents, err
		}
		if err := screen.Update(); err != nil {
			return userEvents, err
		}
		userEvents = collectEvents(streamStart, userEvents)

		// Check for early exit
		for _, ue := range userEvents {
			if ue.Event.Type == sdl.EVENT_QUIT {
				return userEvents, nil
			}
			if ue.Event.Type == sdl.EVENT_KEY_DOWN {
				if ue.Event.KeyboardEvent().Key == sdl.K_ESCAPE {
					return userEvents, nil
				}
			}
		}
	}

	return userEvents, nil
}
