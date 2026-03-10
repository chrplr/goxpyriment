package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"time"

	"github.com/funatsufumiya/ebiten_gvvideo/gvplayer"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/pierrec/lz4/v4"
)

// GVHeader matches the binary format produced by images2gv
type GVHeader struct {
	Width      uint32
	Height     uint32
	FrameCount uint32
	FPS        float32
	Format     uint32 // 0 = Raw RGBA
	FrameBytes uint32 // Total uncompressed bytes per frame
}

type FrameIndex struct {
	Address uint64
	Size    uint64
}

type Player struct {
	header    GVHeader
	file      *os.File
	indices   []FrameIndex
	startTime time.Time
	frameImg  *ebiten.Image
	rgbaBuf   *image.RGBA
	lastFrame int
}

func NewCustomPlayer(path string) (*Player, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var header GVHeader
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		f.Close()
		return nil, err
	}

	if header.Format != 0 {
		f.Close()
		return nil, fmt.Errorf("unsupported format %d (only 0 is supported by custom player)", header.Format)
	}

	// Read index table at the end of the file
	indices := make([]FrameIndex, header.FrameCount)
	footerSize := int64(header.FrameCount) * 16
	if _, err := f.Seek(-footerSize, io.SeekEnd); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to seek to index table: %v", err)
	}

	for i := 0; i < int(header.FrameCount); i++ {
		if err := binary.Read(f, binary.LittleEndian, &indices[i].Address); err != nil {
			f.Close()
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &indices[i].Size); err != nil {
			f.Close()
			return nil, err
		}
	}

	rgbaBuf := image.NewRGBA(image.Rect(0, 0, int(header.Width), int(header.Height)))

	return &Player{
		header:    header,
		file:      f,
		indices:   indices,
		rgbaBuf:   rgbaBuf,
		frameImg:  ebiten.NewImage(int(header.Width), int(header.Height)),
		lastFrame: -1,
	}, nil
}

func (p *Player) Update() error {
	if p.startTime.IsZero() {
		p.startTime = time.Now()
	}

	elapsed := time.Since(p.startTime)
	frameIdx := int(elapsed.Seconds() * float64(p.header.FPS))
	frameIdx = frameIdx % int(p.header.FrameCount)

	if frameIdx != p.lastFrame {
		if err := p.loadFrame(frameIdx); err != nil {
			return err
		}
		p.lastFrame = frameIdx
	}

	return nil
}

func (p *Player) loadFrame(idx int) error {
	index := p.indices[idx]
	compressed := make([]byte, index.Size)
	if _, err := p.file.ReadAt(compressed, int64(index.Address)); err != nil {
		return err
	}

	n, err := lz4.UncompressBlock(compressed, p.rgbaBuf.Pix)
	if err != nil {
		return fmt.Errorf("lz4 decompression failed: %v", err)
	}
	if uint32(n) != p.header.FrameBytes {
		return fmt.Errorf("decompressed size mismatch: got %d, expected %d", n, p.header.FrameBytes)
	}

	p.frameImg.WritePixels(p.rgbaBuf.Pix)
	return nil
}

func (p *Player) Draw(screen *ebiten.Image) {
	screen.DrawImage(p.frameImg, nil)
}

type Game struct {
	player       *gvplayer.GVPlayer
	customPlayer *Player
}

func (g *Game) Update() error {
	if g.player != nil {
		return g.player.Update()
	}
	if g.customPlayer != nil {
		return g.customPlayer.Update()
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.player != nil {
		g.player.Draw(screen, nil)
	}
	if g.customPlayer != nil {
		g.customPlayer.Draw(screen)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if g.player != nil {
		return g.player.Width(), g.player.Height()
	}
	if g.customPlayer != nil {
		return int(g.customPlayer.header.Width), int(g.customPlayer.header.Height)
	}
	return 640, 480
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/player/main.go <file.gv>")
	}
	path := os.Args[1]

	game := &Game{}

	// Try standard gvplayer first
	p, err := gvplayer.NewGVPlayer(path)
	if err == nil {
		game.player = p
		p.Play()
		ebiten.SetWindowSize(p.Width(), p.Height())
	} else {
		// If it fails, try custom player for Format 0
		cp, err2 := NewCustomPlayer(path)
		if err2 != nil {
			log.Fatalf("failed to load video: %v (gvplayer error: %v)", err2, err)
		}
		game.customPlayer = cp
		ebiten.SetWindowSize(int(cp.header.Width), int(cp.header.Height))
	}

	ebiten.SetWindowTitle("images2gv player - " + path)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
