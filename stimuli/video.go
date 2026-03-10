// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build cgo

package stimuli

import (
	"encoding/binary"
	"fmt"
	"github.com/chrplr/goxpyriment/io"
	"image"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/asticode/go-astiav"
)

// Video represents a video stimulus that can be played with optional audio.
// It uses FFmpeg via the go-astiav bindings for demuxing/decoding and SDL3
// for rendering frames and streaming audio to the experiment's audio device.
//
// Typical usage:
//
//	v := stimuli.NewVideo("assets/clip.mp4")
//	if err := v.Preload(); err != nil { log.Fatal(err) }
//	if err := v.PreloadDevice(exp.Screen, exp.AudioDevice); err != nil { log.Fatal(err) }
//	if err := v.Play(); err != nil { log.Fatal(err) }
//
//	err := exp.Run(func() error {
//	    if err := v.Update(); err != nil { return err }
//	    if err := v.Present(exp.Screen, true, true); err != nil { return err }
//	    if !v.IsPlaying() { return sdl.EndLoop }
//	    return nil
//	})
type Video struct {
	FilePath string
	
	// astiav resources
	fctx         *astiav.FormatContext
	vStream      *astiav.Stream
	vCodecCtx    *astiav.CodecContext
	vIndex       int
	aStream      *astiav.Stream
	aCodecCtx    *astiav.CodecContext
	aIndex       int
	swsCtx       *astiav.SoftwareScaleContext
	resampleCtx  *astiav.SoftwareResampleContext

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

// NewVideo creates a new video stimulus for the given file path.
// The underlying FFmpeg/SDL resources are allocated lazily in Preload and
// PreloadDevice.
func NewVideo(filePath string) *Video {
	return &Video{
		FilePath: filePath,
	}
}

// Preload opens the media file, discovers the best video and audio streams,
// allocates and opens the corresponding codec contexts, and prepares the
// software scaler/resampler. It does not allocate any SDL resources.
func (v *Video) Preload() error {
	v.fctx = astiav.AllocFormatContext()
	if err := v.fctx.OpenInput(v.FilePath, nil, nil); err != nil {
		return err
	}
	if err := v.fctx.FindStreamInfo(nil); err != nil {
		return err
	}

	v.vIndex = -1
	v.aIndex = -1

	for i, s := range v.fctx.Streams() {
		if s.CodecParameters().MediaType() == astiav.MediaTypeVideo && v.vIndex == -1 {
			v.vIndex = i
			v.vStream = s
		} else if s.CodecParameters().MediaType() == astiav.MediaTypeAudio && v.aIndex == -1 {
			v.aIndex = i
			v.aStream = s
		}
	}

	if v.vIndex == -1 {
		return fmt.Errorf("no video stream found in %s", v.FilePath)
	}

	// Setup Video Decoder
	vCodec := astiav.FindDecoder(v.vStream.CodecParameters().CodecID())
	if vCodec == nil {
		return fmt.Errorf("video decoder not found")
	}
	v.vCodecCtx = astiav.AllocCodecContext(vCodec)
	if err := v.vStream.CodecParameters().ToCodecContext(v.vCodecCtx); err != nil {
		return err
	}
	if err := v.vCodecCtx.Open(vCodec, nil); err != nil {
		return err
	}

	v.Width = v.vCodecCtx.Width()
	v.Height = v.vCodecCtx.Height()
	v.FPS = v.vStream.AvgFrameRate().Float64()
	if v.FPS == 0 {
		v.FPS = 25.0 // Fallback
	}

	// Setup Software Scaler for RGBA conversion
	swsCtx, err := astiav.CreateSoftwareScaleContext(
		v.Width, v.Height, v.vCodecCtx.PixelFormat(),
		v.Width, v.Height, astiav.PixelFormatRgba,
		astiav.NewSoftwareScaleContextFlags(astiav.SoftwareScaleContextFlagBilinear),
	)
	if err != nil {
		return err
	}
	v.swsCtx = swsCtx

	// Setup Audio Decoder if exists
	if v.aIndex != -1 {
		aCodec := astiav.FindDecoder(v.aStream.CodecParameters().CodecID())
		if aCodec != nil {
			v.aCodecCtx = astiav.AllocCodecContext(aCodec)
			if err := v.aStream.CodecParameters().ToCodecContext(v.aCodecCtx); err != nil {
				return err
			}
			if err := v.aCodecCtx.Open(aCodec, nil); err != nil {
				return err
			}

			// Setup Audio Resampler
			v.resampleCtx = astiav.AllocSoftwareResampleContext()
		}
	}

	return nil
}

// PreloadDevice prepares SDL‑specific resources for the video:
//   - creates a texture on the provided Screen's renderer for video frames,
//   - creates and binds an SDL AudioStream to the given audio device (if
//     the video has an audio stream).
//
// If Preload has not been called yet, PreloadDevice will call it first.
func (v *Video) PreloadDevice(screen *io.Screen, audioDevice sdl.AudioDeviceID) error {
	if v.fctx == nil {
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
	if v.aCodecCtx != nil {
		spec := sdl.AudioSpec{
			Format:   sdl.AUDIO_F32,
			Channels: 2,
			Freq:     int32(v.aCodecCtx.SampleRate()),
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

// Play starts (or resumes) video decoding and playback.
// It creates internal frame/audio buffers, launches the decoding goroutine,
// and resets the playback clock. Calling Play while paused resumes from the
// paused position.
func (v *Video) Play() error {
	v.stopMutex.Lock()
	defer v.stopMutex.Unlock()

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
	v.isDecoding = true
	v.startTime = time.Now()
	v.lastFrameTime = 0

	// Start decoding goroutine
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

	pkt := astiav.AllocPacket()
	defer pkt.Free()
	vFrame := astiav.AllocFrame()
	defer vFrame.Free()
	rgbaFrame := astiav.AllocFrame()
	defer rgbaFrame.Free()
	aFrame := astiav.AllocFrame()
	defer aFrame.Free()
	resampledFrame := astiav.AllocFrame()
	defer resampledFrame.Free()

	// Initialize rgbaFrame for scaling
	rgbaFrame.SetWidth(v.Width)
	rgbaFrame.SetHeight(v.Height)
	rgbaFrame.SetPixelFormat(astiav.PixelFormatRgba)
	if err := rgbaFrame.AllocBuffer(1); err != nil {
		v.errChan <- err
		return
	}

	// Initialize resampledFrame for audio
	if v.aCodecCtx != nil {
		resampledFrame.SetChannelLayout(astiav.ChannelLayoutStereo)
		resampledFrame.SetSampleFormat(astiav.SampleFormatFlt)
		resampledFrame.SetSampleRate(v.aCodecCtx.SampleRate())
		resampledFrame.SetNbSamples(1024) // Buffer for resampled samples
		if err := resampledFrame.AllocBuffer(0); err != nil {
			v.errChan <- err
			return
		}
	}

	for {
		select {
		case <-v.stopChan:
			return
		default:
			err := v.fctx.ReadFrame(pkt)
			if err != nil {
				if err == astiav.ErrEof {
					return
				}
				v.errChan <- err
				return
			}

			if pkt.StreamIndex() == v.vIndex {
				if err := v.vCodecCtx.SendPacket(pkt); err != nil {
					v.errChan <- err
					pkt.Unref()
					return
				}
				for {
					err := v.vCodecCtx.ReceiveFrame(vFrame)
					if err != nil {
						if err == astiav.ErrEagain || err == astiav.ErrEof {
							break
						}
						v.errChan <- err
						pkt.Unref()
						return
					}

					// Scale to RGBA
					if err := v.swsCtx.ScaleFrame(vFrame, rgbaFrame); err != nil {
						v.errChan <- err
						pkt.Unref()
						return
					}

					// Create image.RGBA and copy data
					rgba := image.NewRGBA(image.Rect(0, 0, v.Width, v.Height))
					data, err := rgbaFrame.Data().Bytes(1)
					if err != nil {
						v.errChan <- err
						pkt.Unref()
						return
					}
					copy(rgba.Pix, data)

					select {
					case v.frameBuffer <- rgba:
					case <-v.stopChan:
						pkt.Unref()
						return
					}
				}
			} else if pkt.StreamIndex() == v.aIndex && v.aCodecCtx != nil {
				if err := v.aCodecCtx.SendPacket(pkt); err != nil {
					v.errChan <- err
					pkt.Unref()
					return
				}
				for {
					err := v.aCodecCtx.ReceiveFrame(aFrame)
					if err != nil {
						if err == astiav.ErrEagain || err == astiav.ErrEof {
							break
						}
						v.errChan <- err
						pkt.Unref()
						return
					}

					// Resample
					if err := v.resampleCtx.ConvertFrame(aFrame, resampledFrame); err != nil {
						v.errChan <- err
						pkt.Unref()
						return
					}

					// Extract samples
					data, err := resampledFrame.Data().Bytes(0)
					if err != nil {
						v.errChan <- err
						pkt.Unref()
						return
					}
					
					// data is expected to be interleaved float32 stereo
					if nbSamples := resampledFrame.NbSamples(); nbSamples > 0 {
						const bytesPerSample = 4 // float32
						expectedLen := nbSamples * 2 * bytesPerSample
						if len(data) < expectedLen {
							v.errChan <- fmt.Errorf("audio buffer too small: got %d bytes, expected at least %d", len(data), expectedLen)
							pkt.Unref()
							return
						}
						for i := 0; i < nbSamples; i++ {
							base := i * 2 * bytesPerSample
							leftBits := binary.LittleEndian.Uint32(data[base : base+bytesPerSample])
							rightBits := binary.LittleEndian.Uint32(data[base+bytesPerSample : base+2*bytesPerSample])
							s := [2]float32{
								math.Float32frombits(leftBits),
								math.Float32frombits(rightBits),
							}
							select {
							case v.sampleBuffer <- s:
							case <-v.stopChan:
								pkt.Unref()
								return
							}
						}
					}
				}
			}
			pkt.Unref()
		}
	}
}

// Update advances decoding and playback state.
// It should be called frequently (typically once per frame) from the main
// experiment loop to:
//   - pull decoded frames/audio from internal buffers,
//   - push audio samples into the SDL AudioStream,
//   - update which frame should currently be displayed based on the FPS.
func (v *Video) Update() error {
	v.stopMutex.Lock()
	isPlaying := v.isPlaying
	isPaused := v.isPaused
	startTime := v.startTime
	lastFrameTime := v.lastFrameTime
	v.stopMutex.Unlock()

	if !isPlaying || isPaused {
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
	now := time.Since(startTime)
	frameDuration := time.Second / time.Duration(v.FPS)
	
	if now >= lastFrameTime+frameDuration {
		select {
		case frame := <-v.frameBuffer:
			v.frameMutex.Lock()
			v.currentFrame = frame
			v.frameMutex.Unlock()
			v.stopMutex.Lock()
			v.lastFrameTime += frameDuration
			v.stopMutex.Unlock()
		default:
			// No frame ready yet
		}
	}

	return nil
}

// Draw renders the current video frame centered on the screen.
func (v *Video) Draw(screen *io.Screen) error {
	return v.DrawAt(screen, nil)
}

// DrawAt renders the current video frame to the given destination rectangle.
// If dest is nil, the frame is centered at its native size using the
// renderer's current output size.
func (v *Video) DrawAt(screen *io.Screen, dest *sdl.FRect) error {
	v.frameMutex.Lock()
	frame := v.currentFrame
	v.frameMutex.Unlock()

	if frame == nil {
		return nil
	}

	// Update texture with frame pixels
	if err := v.texture.Update(nil, frame.Pix, int32(frame.Stride)); err != nil {
		return err
	}

	// Choose destination rectangle
	var dst sdl.FRect
	if dest != nil {
		dst = *dest
	} else {
		// Centered at native size
		w, h, _ := screen.Renderer.RenderOutputSize()
		dst = sdl.FRect{
			X: float32(w-int32(v.Width)) / 2,
			Y: float32(h-int32(v.Height)) / 2,
			W: float32(v.Width),
			H: float32(v.Height),
		}
	}

	return screen.Renderer.RenderTexture(v.texture, nil, &dst)
}

// Present implements the Stimulus interface for Video by drawing the current
// frame (optionally clearing the screen first) and optionally updating the
// screen. It does not itself advance decoding; call Update separately.
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

// Pause pauses video playback, preserving the current playback position.
// Use Play() to resume from this position.
func (v *Video) Pause() {
	v.stopMutex.Lock()
	defer v.stopMutex.Unlock()

	if !v.isPlaying || v.isPaused {
		return
	}
	v.isPaused = true
	v.pauseTime = time.Since(v.startTime)
}

// IsPaused returns true if the video is currently paused.
func (v *Video) IsPaused() bool {
	v.stopMutex.Lock()
	defer v.stopMutex.Unlock()

	return v.isPaused
}

// IsPlaying returns true if the video is currently active (playing or paused).
func (v *Video) IsPlaying() bool {
	v.stopMutex.Lock()
	defer v.stopMutex.Unlock()

	return v.isPlaying
}

// IsFinished returns true if the video has finished playback.
func (v *Video) IsFinished() bool {
	v.stopMutex.Lock()
	defer v.stopMutex.Unlock()

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
	v.stopMutex.Unlock()

	v.frameMutex.Lock()
	v.currentFrame = nil
	v.frameMutex.Unlock()

	v.wg.Wait()
}

// Seek moves the playback position to the specified time.
// Note: This stops the current playback and should be called before Play() or while the video is stopped.
func (v *Video) Seek(t time.Duration) error {
	v.stopMutex.Lock()
	isPlaying := v.isPlaying
	v.stopMutex.Unlock()

	if isPlaying {
		v.Stop()
	}
	if v.fctx == nil {
		return fmt.Errorf("video stream not preloaded")
	}

	// Seek using microseconds (AV_TIME_BASE)
	ts := int64(t.Seconds() * 1000000)
	if err := v.fctx.SeekFrame(-1, ts, astiav.NewSeekFlags().Add(astiav.SeekFlagBackward)); err != nil {
		return err
	}

	return nil
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
	
	if v.swsCtx != nil {
		v.swsCtx.Free()
		v.swsCtx = nil
	}
	if v.resampleCtx != nil {
		v.resampleCtx.Free()
		v.resampleCtx = nil
	}
	if v.vCodecCtx != nil {
		v.vCodecCtx.Free()
		v.vCodecCtx = nil
	}
	if v.aCodecCtx != nil {
		v.aCodecCtx.Free()
		v.aCodecCtx = nil
	}
	if v.fctx != nil {
		v.fctx.CloseInput()
		v.fctx.Free()
		v.fctx = nil
	}
	
	return nil
}
