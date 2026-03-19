package gvplayer

import (
	"bytes"
	"image"
	"io"
	"os"
	"time"

	"github.com/funatsufumiya/go-gv-video/gvvideo"
	"github.com/hajimehoshi/ebiten/v2"
)

type PlayerState int

const (
	Stopped PlayerState = iota
	Playing
	Paused
)

type GVPlayer struct {
	video         *gvvideo.GVVideo
	frameImage    *ebiten.Image
	frameBuf      *image.RGBA
	state         PlayerState
	startTime     time.Time
	pauseTime     time.Time
	seekTime      time.Duration
	Loop          bool
	async         bool
	frameCh       chan []byte
	stopCh        chan struct{}
	lastFrameID   uint32
	lastFrameTime time.Duration
}

func (p *GVPlayer) Width() int {
	return int(p.video.Header.Width)
}

func (p *GVPlayer) Height() int {
	return int(p.video.Header.Height)
}

func NewGVPlayer(path string) (*GVPlayer, error) {
	return NewGVPlayerWithOption(path, true)
}

func NewGVPlayerWithOption(path string, async bool) (*GVPlayer, error) {
	video, err := gvvideo.LoadGVVideo(path)
	if err != nil {
		return nil, err
	}
	w := int(video.Header.Width)
	h := int(video.Header.Height)
	img := ebiten.NewImage(w, h)
	buf := image.NewRGBA(image.Rect(0, 0, w, h))
	p := &GVPlayer{
		video:      video,
		frameImage: img,
		frameBuf:   buf,
		state:      Stopped,
		async:      async,
		frameCh:    make(chan []byte, 1),
		stopCh:     make(chan struct{}),
	}
	return p, nil
}

// Loads a GV file into memory and creates a GVPlayer (async default true)
func LoadGVPlayerOnMemory(path string) (*GVPlayer, error) {
	return LoadGVPlayerOnMemoryWithOption(path, true)
}

// Loads a GV file into memory and creates a GVPlayer with async option
func LoadGVPlayerOnMemoryWithOption(path string, async bool) (*GVPlayer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(data)
	return NewGVPlayerFromReaderWithOption(reader, async)
}

// Creates a GVPlayer from an io.Reader (on-memory loading, async default true)
func NewGVPlayerFromReader(reader io.ReadSeeker) (*GVPlayer, error) {
	return NewGVPlayerFromReaderWithOption(reader, true)
}

// Creates a GVPlayer from an io.Reader with async option
func NewGVPlayerFromReaderWithOption(reader io.ReadSeeker, async bool) (*GVPlayer, error) {
	video, err := gvvideo.LoadGVVideoFromReader(reader)
	if err != nil {
		return nil, err
	}
	w := int(video.Header.Width)
	h := int(video.Header.Height)
	img := ebiten.NewImage(w, h)
	buf := image.NewRGBA(image.Rect(0, 0, w, h))
	p := &GVPlayer{
		video:      video,
		frameImage: img,
		frameBuf:   buf,
		state:      Stopped,
		async:      async,
		frameCh:    make(chan []byte, 1),
		stopCh:     make(chan struct{}),
	}
	return p, nil
}

func (p *GVPlayer) Play() {
	if p.state == Playing {
		return
	}
	p.state = Playing
	p.startTime = time.Now()
	if p.async {
		go p.asyncUpdateLoop()
	}
}

func (p *GVPlayer) Pause() {
	if p.state != Playing {
		return
	}
	p.state = Paused
	p.pauseTime = time.Now()
}

func (p *GVPlayer) Stop() {
	p.state = Stopped
	p.seekTime = 0
	if p.async {
		close(p.stopCh)
		p.stopCh = make(chan struct{})
	}
}

func (p *GVPlayer) Seek(to time.Duration) {
	p.seekTime = to
}

func (p *GVPlayer) Update() error {
	if p.state != Playing {
		return nil
	}
	elapsed := time.Since(p.startTime) + p.seekTime
	frameID := uint32(elapsed.Seconds() * float64(p.video.Header.FPS))
	if frameID >= p.video.Header.FrameCount {
		if p.Loop {
			p.startTime = time.Now()
			p.seekTime = 0
			frameID = 0
		} else {
			p.state = Stopped
			return nil
		}
	}
	if p.async {
		if frameID != p.lastFrameID {
			select {
			case pix := <-p.frameCh:
				copy(p.frameBuf.Pix, pix)
				p.frameImage.WritePixels(p.frameBuf.Pix)
				p.lastFrameID = frameID
				p.lastFrameTime = time.Duration(float64(frameID) / float64(p.video.Header.FPS) * float64(time.Second))
			default:
			}
		}
		return nil
	}
	err := p.video.ReadFrameTo(frameID, p.frameBuf)
	if err != nil {
		return err
	}
	p.frameImage.WritePixels(p.frameBuf.Pix)
	p.lastFrameID = frameID
	p.lastFrameTime = time.Duration(float64(frameID) / float64(p.video.Header.FPS) * float64(time.Second))
	return nil
}

func (p *GVPlayer) CurrentTime() time.Duration {
	return p.lastFrameTime
}

func (p *GVPlayer) asyncUpdateLoop() {
	for {
		select {
		case <-p.stopCh:
			return
		default:
			elapsed := time.Since(p.startTime) + p.seekTime
			frameID := uint32(elapsed.Seconds() * float64(p.video.Header.FPS))
			if frameID != p.lastFrameID {
				buf := image.NewRGBA(image.Rect(0, 0, p.Width(), p.Height()))
				err := p.video.ReadFrameTo(frameID, buf)
				if err == nil {
					select {
					case p.frameCh <- buf.Pix:
					default:
					}
				}
			}
			time.Sleep(time.Millisecond * 5)
		}
	}
}

func (p *GVPlayer) Draw(screen *ebiten.Image, op *ebiten.DrawImageOptions) {
	if p.frameImage != nil {
		screen.DrawImage(p.frameImage, op)
	}
}
