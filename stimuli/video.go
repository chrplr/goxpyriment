// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.
package stimuli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	xio "github.com/chrplr/goxpyriment/apparatus"
	"github.com/gen2brain/mpeg"
)

// Video represents a playable video stimulus using MPEG decoding.
type Video struct {
	mpg         *mpeg.MPEG
	file        *os.File
	texture     *sdl.Texture
	startTime   time.Time
	pauseTime   time.Time
	totalPaused time.Duration

	Width  int32
	Height int32
	fps    float64

	playing      bool
	paused       bool
	currentFrame int
}

// NewVideo creates a new Video stimulus from the given file path.
// It initializes the MPEG decoder and creates an SDL texture for rendering.
func NewVideo(screen *xio.Screen, path string) (*Video, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open video: %w", err)
	}

	m, err := mpeg.New(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to initialize MPEG decoder: %w", err)
	}

	w, h := int32(m.Width()), int32(m.Height())
	// Create texture with streaming access for frame updates
	tex, err := screen.Renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_STREAMING, int(w), int(h))
	if err != nil {
		f.Close()
		return nil, err
	}

	return &Video{
		mpg:     m,
		file:    f,
		texture: tex,
		Width:   w,
		Height:  h,
		fps:     m.Framerate(),
	}, nil
}

// Update decodes and updates the video frame based on elapsed time.
// It should be called regularly (e.g., in each frame of the main loop) to ensure smooth playback.
func (v *Video) Update() error {
	if !v.playing || v.paused {
		return nil
	}

	elapsed := time.Since(v.startTime) - v.totalPaused
	targetFrame := int(elapsed.Seconds() * v.fps)

	for v.currentFrame < targetFrame {
		frame := v.mpg.Video().Decode()
		if frame == nil {
			v.playing = false
			return io.EOF
		}

		v.currentFrame++

		if v.currentFrame == targetFrame {
			rgba := frame.RGBA()
			// Update the GPU texture with the new frame data
			v.texture.Update(nil, rgba.Pix, int32(rgba.Stride))
		}
	}
	return nil
}

// Rewind resets the video state to the beginning so it can be played again.
func (v *Video) Rewind() {
	v.mpg.Rewind()
	v.currentFrame = 0
	v.totalPaused = 0
	v.startTime = time.Now()
	v.playing = true
	v.paused = false
}

// Draw renders the current video frame at the specified (x, y) coordinates.
func (v *Video) Draw(screen *xio.Screen, x, y int32) error {
	dest := sdl.FRect{X: float32(x), Y: float32(y), W: float32(v.Width), H: float32(v.Height)}
	return v.DrawAt(screen, &dest)
}

// DrawAt renders the current video frame into the specified destination rectangle.
func (v *Video) DrawAt(screen *xio.Screen, dest *sdl.FRect) error {
	return screen.Renderer.RenderTexture(v.texture, nil, dest)
}

// Play starts or resumes video playback.
func (v *Video) Play() {
	if !v.playing {
		v.startTime = time.Now()
		v.playing = true
		v.currentFrame = 0
		v.totalPaused = 0
	}
	if v.paused {
		v.totalPaused += time.Since(v.pauseTime)
		v.paused = false
	}
}

// Pause pauses video playback.
func (v *Video) Pause() {
	if v.playing && !v.paused {
		v.paused = true
		v.pauseTime = time.Now()
	}
}

// IsPlaying returns true if the video is currently playing (and not paused).
func (v *Video) IsPlaying() bool { return v.playing }

// IsPaused returns true if the video is currently paused.
func (v *Video) IsPaused() bool { return v.paused }

// Close releases resources associated with the video, including the file handle and SDL texture.
func (v *Video) Close() {
	if v.file != nil {
		v.file.Close()
	}
	if v.texture != nil {
		v.texture.Destroy()
	}
}
