// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
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

// audioBufferAhead is how much decoded MP2 audio Update keeps queued on the SDL
// stream. Large enough to survive a slow frame, small enough that Pause stops
// the sound promptly and that a mid-clip Pause/Play cannot desync audio from
// video by more than this amount.
const audioBufferAhead = 100 * time.Millisecond

// Video represents a playable video stimulus using MPEG decoding.
//
// MPEG-1 program streams may carry an MP2 audio track. Audio is silent unless
// [Video.PreloadDevice] is called with an audio device before playback; when it
// is not, audio packets are discarded at the demuxer rather than buffered.
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

	// HasAudio reports whether the file carries an audio stream at all.
	HasAudio bool

	audioStream *sdl.AudioStream
	audioBytes  int32 // audioBufferAhead expressed in bytes of device format
	audioEnded  bool

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

	// Audio decoding stays off until PreloadDevice binds a device. Left on, the
	// demuxer would keep writing audio packets into a buffer nothing ever
	// drains, growing without bound for the length of the clip.
	hasAudio := m.NumAudioStreams() > 0 && m.Samplerate() > 0
	m.SetAudioEnabled(false)

	return &Video{
		mpg:      m,
		file:     f,
		texture:  tex,
		Width:    w,
		Height:   h,
		fps:      m.Framerate(),
		HasAudio: hasAudio,
	}, nil
}

// PreloadDevice enables the MP2 audio track and binds it to an SDL audio
// device. Call it before Play; without it the video plays silently.
//
// It is a no-op returning nil when the file has no audio stream, so callers can
// invoke it unconditionally.
func (v *Video) PreloadDevice(audioDevice sdl.AudioDeviceID) error {
	if !v.HasAudio || v.audioStream != nil {
		return nil
	}

	v.mpg.SetAudioEnabled(true)
	// SetAudioEnabled sets its flag before checking that a decoder exists, so
	// AudioEnabled() is not a reliable guard — test the decoder itself, or
	// SetAudioFormat below dereferences nil.
	if v.mpg.Audio() == nil {
		return fmt.Errorf("stimuli.Video.PreloadDevice: no decodable audio stream")
	}
	// Normalized interleaved float32 — matches sdl.AUDIO_F32 byte for byte,
	// so Samples.Bytes() can be handed to PutData without conversion.
	v.mpg.SetAudioFormat(mpeg.AudioF32N)

	channels := int32(v.mpg.Channels())
	freq := int32(v.mpg.Samplerate())
	spec := sdl.AudioSpec{Format: sdl.AUDIO_F32, Channels: channels, Freq: freq}

	stream, err := sdl.CreateAudioStream(&spec, &spec)
	if err != nil {
		return fmt.Errorf("stimuli.Video.PreloadDevice: creating audio stream: %w", err)
	}
	if err := audioDevice.BindAudioStream(stream); err != nil {
		stream.Destroy()
		return fmt.Errorf("stimuli.Video.PreloadDevice: binding audio stream: %w", err)
	}

	v.audioStream = stream
	// bytes = seconds × frames/s × channels × 4 bytes per float32
	v.audioBytes = int32(audioBufferAhead.Seconds() * float64(freq) * float64(channels) * 4)
	return nil
}

// SetVolume scales audio playback gain (1.0 = unchanged, 0.0 = silent).
// It is a no-op when audio is not wired to a device.
func (v *Video) SetVolume(gain float32) error {
	if v.audioStream == nil {
		return nil
	}
	return v.audioStream.SetGain(gain)
}

// updateAudio tops the SDL queue back up to audioBufferAhead. The audio track
// is free-running once queued: video is driven by the wall clock and drops
// frames under load, audio never does, so the two stay aligned to within the
// queue depth.
func (v *Video) updateAudio() error {
	if v.audioStream == nil || v.audioEnded || v.mpg.Audio() == nil {
		return nil
	}

	queued, err := v.audioStream.Queued()
	if err != nil {
		return fmt.Errorf("stimuli.Video: querying audio queue: %w", err)
	}

	for queued < v.audioBytes {
		samples := v.mpg.Audio().Decode()
		if samples == nil {
			// Audio ran out before the video did. Flush so the resampler emits
			// its lookahead frames instead of truncating the tail.
			v.audioEnded = true
			return v.audioStream.Flush()
		}
		buf := samples.Bytes()
		if err := v.audioStream.PutData(buf); err != nil {
			return fmt.Errorf("stimuli.Video: queueing audio: %w", err)
		}
		queued += int32(len(buf))
	}
	return nil
}

// Update decodes and updates the video frame based on elapsed time.
// It should be called regularly (e.g., in each frame of the main loop) to ensure smooth playback.
func (v *Video) Update() error {
	if !v.playing || v.paused {
		return nil
	}

	if err := v.updateAudio(); err != nil {
		return err
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
	if v.audioStream != nil {
		// Drop audio already queued from the previous pass, or it would play
		// over the restarted video.
		_ = v.audioStream.Clear()
		v.audioEnded = false
	}
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
		v.audioEnded = false
	}
	if v.paused {
		v.totalPaused += time.Since(v.pauseTime)
		v.paused = false
	}
}

// Pause pauses video playback.
//
// Queued audio is discarded so the sound stops with the image rather than
// running on for the length of the buffer. Because that audio was already
// decoded, playback resumes up to audioBufferAhead ahead of the video.
func (v *Video) Pause() {
	if v.playing && !v.paused {
		v.paused = true
		v.pauseTime = time.Now()
		if v.audioStream != nil {
			_ = v.audioStream.Clear()
		}
	}
}

// IsPlaying returns true if the video is currently playing (and not paused).
func (v *Video) IsPlaying() bool { return v.playing }

// IsPaused returns true if the video is currently paused.
func (v *Video) IsPaused() bool { return v.paused }

// Close releases resources associated with the video, including the file handle and SDL texture.
func (v *Video) Close() {
	if v.audioStream != nil {
		v.audioStream.Unbind()
		v.audioStream.Destroy()
		v.audioStream = nil
	}
	if v.file != nil {
		v.file.Close()
	}
	if v.texture != nil {
		v.texture.Destroy()
	}
}
