// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"goxpyriment/io"
	"image"
	"sync"
	"time"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/zergon321/reisen"
)

// Video represents a video stimulus that can be played with audio.
// It uses FFmpeg via the reisen library for decoding and SDL3 for rendering and audio playback.
type Video struct {
	FilePath string
	media    *reisen.Media
	vStream  *reisen.VideoStream
	aStream  *reisen.AudioStream

	// SDL resources
	texture     *sdl.Texture
	audioStream *sdl.AudioStream

	// Internal decoding state
	frameBuffer  chan *image.RGBA
	sampleBuffer chan [2]float32
	errChan      chan error
	stopChan     chan struct{}
	wg           sync.WaitGroup
	stopMutex    sync.Mutex
	
	// Status
	isPlaying bool
	isPaused  bool
	isDecoding bool
	
	// Playback management
	lastFrameTime time.Duration
	startTime     time.Time
	pauseTime     time.Duration
	
	// Visual data
	currentFrame *image.RGBA
	frameMutex   sync.Mutex
	
	// Video properties
	Width  int
	Height int
	FPS    float64
}

// NewVideo creates a new video stimulus.
func NewVideo(filePath string) *Video {
	return &Video{
		FilePath: filePath,
	}
}

// Preload opens the video file and prepares streams.
func (v *Video) Preload() error {
	media, err := reisen.NewMedia(v.FilePath)
	if err != nil {
		return err
	}
	v.media = media

	if len(v.media.VideoStreams()) == 0 {
		return fmt.Errorf("no video streams found in %s", v.FilePath)
	}

	v.vStream = v.media.VideoStreams()[0]
	if err := v.vStream.Open(); err != nil {
		return err
	}

	v.Width = v.vStream.Width()
	v.Height = v.vStream.Height()
	fps, _ := v.vStream.FrameRate()
	v.FPS = float64(fps)

	if len(v.media.AudioStreams()) > 0 {
		v.aStream = v.media.AudioStreams()[0]
		if err := v.aStream.Open(); err != nil {
			return err
		}
	}

	return nil
}

// PreloadDevice prepares SDL-specific resources for the video.
func (v *Video) PreloadDevice(screen *io.Screen, audioDevice sdl.AudioDeviceID) error {
	if v.media == nil {
		if err := v.Preload(); err != nil {
			return err
		}
	}

	// Create texture for video frames
	tex, err := screen.Renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_STREAMING, v.Width, v.Height)
	if err != nil {
		return err
	}
	v.texture = tex

	// Create audio stream if video has audio
	if v.aStream != nil {
		spec := sdl.AudioSpec{
			Format:   sdl.AUDIO_F32,
			Channels: 2,
			Freq:     int32(v.aStream.SampleRate()),
		}
		as, err := sdl.CreateAudioStream(&spec, &spec)
		if err != nil {
			return err
		}
		v.audioStream = as
		if err := audioDevice.BindAudioStream(v.audioStream); err != nil {
			return err
		}
	}

	return nil
}

// Play starts video decoding and playback.
func (v *Video) Play() error {
	if v.isPlaying && !v.isPaused {
		return nil
	}

	if v.isPaused {
		v.isPaused = false
		// Adjust start time to account for pause duration
		v.startTime = time.Now().Add(-v.pauseTime)
		return nil
	}

	v.frameBuffer = make(chan *image.RGBA, 5)
	v.sampleBuffer = make(chan [2]float32, 4096)
	v.errChan = make(chan error, 1)
	v.stopChan = make(chan struct{})
	v.isPlaying = true
	v.startTime = time.Now()
	v.lastFrameTime = 0

	// Start decoding goroutine
	v.isDecoding = true
	v.wg.Add(1)
	go v.decodeLoop()

	return nil
}

func (v *Video) decodeLoop() {
	defer func() {
		v.stopMutex.Lock()
		v.isDecoding = false
		v.isPlaying = false
		v.stopMutex.Unlock()
		v.wg.Done()
	}()

	if err := v.media.OpenDecode(); err != nil {
		v.errChan <- err
		return
	}
	defer v.media.CloseDecode()

	for {
		select {
		case <-v.stopChan:
			return
		default:
			packet, gotPacket, err := v.media.ReadPacket()
			if err != nil {
				v.errChan <- err
				return
			}
			if !gotPacket {
				return
			}

			switch packet.Type() {
			case reisen.StreamVideo:
				s := v.media.Streams()[packet.StreamIndex()].(*reisen.VideoStream)
				videoFrame, gotFrame, err := s.ReadVideoFrame()
				if err != nil {
					select {
					case v.errChan <- err:
					case <-v.stopChan:
					}
					return
				}
				if gotFrame && videoFrame != nil {
					select {
					case v.frameBuffer <- videoFrame.Image():
					case <-v.stopChan:
						return
					}
				}
			case reisen.StreamAudio:
				s := v.media.Streams()[packet.StreamIndex()].(*reisen.AudioStream)
				audioFrame, gotFrame, err := s.ReadAudioFrame()
				if err != nil {
					select {
					case v.errChan <- err:
					case <-v.stopChan:
					}
					return
				}
				if gotFrame && audioFrame != nil {
					// Convert float64 samples (reisen default) to float32 (SDL3)
					reader := bytes.NewReader(audioFrame.Data())
					for reader.Len() > 0 {
						var left64, right64 float64
						if err := binary.Read(reader, binary.LittleEndian, &left64); err != nil {
							break
						}
						if err := binary.Read(reader, binary.LittleEndian, &right64); err != nil {
							break
						}
						select {
						case v.sampleBuffer <- [2]float32{float32(left64), float32(right64)}:
						case <-v.stopChan:
							return
						}
					}
				}
			}
		}
	}
}

// Update handles frame timing and audio streaming. 
// Should be called frequently in the main loop.
func (v *Video) Update() error {
	if !v.isPlaying || v.isPaused {
		return nil
	}

	// Check for errors from decoder
	select {
	case err := <-v.errChan:
		return err
	default:
	}

	// Push audio samples to SDL stream
	if v.audioStream != nil {
		select {
		case sample := <-v.sampleBuffer:
			data := []float32{sample[0], sample[1]}
			byteData := unsafe.Slice((*byte)(unsafe.Pointer(&data[0])), len(data)*4)
			v.audioStream.PutData(byteData)
			
			// Drain more samples if available to avoid blocking
		loop:
			for {
				select {
				case s := <-v.sampleBuffer:
					d := []float32{s[0], s[1]}
					bd := unsafe.Slice((*byte)(unsafe.Pointer(&d[0])), len(d)*4)
					v.audioStream.PutData(bd)
				default:
					break loop
				}
			}
		default:
		}
	}

	// Frame synchronization
	now := time.Since(v.startTime)
	frameDuration := time.Second / time.Duration(v.FPS)
	
	if now >= v.lastFrameTime+frameDuration {
		select {
		case frame := <-v.frameBuffer:
			v.frameMutex.Lock()
			v.currentFrame = frame
			v.frameMutex.Unlock()
			v.lastFrameTime += frameDuration
		default:
			// No frame ready yet
		}
	}

	return nil
}

// Draw renders the current video frame to the screen.
func (v *Video) Draw(screen *io.Screen) error {
	v.frameMutex.Lock()
	frame := v.currentFrame
	v.frameMutex.Unlock()

	if frame == nil {
		return nil
	}

	// Update texture with frame pixels
	err := v.texture.Update(nil, frame.Pix, int32(frame.Stride))
	if err != nil {
		return err
	}

	// Draw the texture
	// Calculate destination rectangle (centered by default)
	w, h, _ := screen.Renderer.RenderOutputSize()
	dest := sdl.FRect{
		X: float32(w-int32(v.Width)) / 2,
		Y: float32(h-int32(v.Height)) / 2,
		W: float32(v.Width),
		H: float32(v.Height),
	}
	
	return screen.Renderer.RenderTexture(v.texture, nil, &dest)
}

// Present implements the Stimulus interface.
func (v *Video) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := v.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

// Pause pauses the video playback.
func (v *Video) Pause() {
	if !v.isPlaying || v.isPaused {
		return
	}
	v.isPaused = true
	v.pauseTime = time.Since(v.startTime)
}

// IsPaused returns true if the video is currently paused.
func (v *Video) IsPaused() bool {
	return v.isPaused
}

// IsPlaying returns true if the video is currently active (playing or paused).
func (v *Video) IsPlaying() bool {
	return v.isPlaying
}

// IsFinished returns true if the video has finished playback.
func (v *Video) IsFinished() bool {
	return !v.isPlaying && !v.isPaused
}

// Stop stops the video playback and cleans up decoding resources.
func (v *Video) Stop() {
	v.stopMutex.Lock()
	if v.isDecoding {
		select {
		case <-v.stopChan:
			// Already closed
		default:
			close(v.stopChan)
		}
	}
	v.isPlaying = false
	v.isPaused = false
	v.currentFrame = nil
	v.stopMutex.Unlock()

	v.wg.Wait()
}

// Seek moves the playback position to the specified time.
// Note: This stops the current playback and should be called before Play() or while the video is stopped.
func (v *Video) Seek(t time.Duration) error {
	if v.isPlaying {
		v.Stop()
	}
	if v.vStream == nil {
		return fmt.Errorf("video stream not preloaded")
	}
	return v.vStream.Rewind(t)
}

// Unload cleans up all resources associated with the video stimulus.
func (v *Video) Unload() error {
	v.Stop()
	if v.texture != nil {
		v.texture.Destroy()
		v.texture = nil
	}
	if v.audioStream != nil {
		v.audioStream.Destroy()
		v.audioStream = nil
	}
	if v.vStream != nil {
		v.vStream.Close()
		v.vStream = nil
	}
	if v.aStream != nil {
		v.aStream.Close()
		v.aStream = nil
	}
	if v.media != nil {
		v.media.Close()
		v.media = nil
	}
	return nil
}
