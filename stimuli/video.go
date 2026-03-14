package stimuli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gen2brain/mpeg"
	"github.com/Zyko0/go-sdl3/sdl"
)

type Video struct {
	mpg          *mpeg.MPEG
	file         *os.File
	texture      *sdl.Texture
	startTime    time.Time
	pauseTime    time.Time 
	totalPaused  time.Duration 
	
	Width        int32
	Height       int32
	fps          float64
	
	playing      bool
	paused       bool
	currentFrame int
}

func NewVideo(renderer *sdl.Renderer, path string) (*Video, error) {
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
	tex, err := renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_STREAMING, int(w), int(h))
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

func (v *Video) Update(renderer *sdl.Renderer) error {
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

// Rewind resets the video state so it can be played again
func (v *Video) Rewind() {
	v.mpg.Rewind()
	v.currentFrame = 0
	v.totalPaused = 0
	v.startTime = time.Now()
	v.playing = true
	v.paused = false
}

func (v *Video) Draw(renderer *sdl.Renderer, x, y int32) error {
	dest := sdl.FRect{X: float32(x), Y: float32(y), W: float32(v.Width), H: float32(v.Height)}
	return v.DrawAt(renderer, &dest)
}

func (v *Video) DrawAt(renderer *sdl.Renderer, dest *sdl.FRect) error {
	return renderer.RenderTexture(v.texture, nil, dest)
}

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

func (v *Video) Pause() {
	if v.playing && !v.paused {
		v.paused = true
		v.pauseTime = time.Now()
	}
}

func (v *Video) IsPlaying() bool { return v.playing }
func (v *Video) IsPaused() bool  { return v.paused }

func (v *Video) Close() {
	if v.file != nil { v.file.Close() }
	if v.texture != nil { v.texture.Destroy() }
}
