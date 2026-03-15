package engine

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/Zyko0/go-sdl3/sdl"
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

type VideoResource struct {
	Header    GVHeader
	File      *os.File
	Indices   []FrameIndex
	Texture   *sdl.Texture
	RGBA      []byte
	FPS       float64
	W, H      float32
	LastFrame int // Index of last decoded frame
}

func loadVideo(renderer *sdl.Renderer, fullPath string) (*VideoResource, error) {
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open video file: %v", err)
	}

	var header GVHeader
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to read video header: %v", err)
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
			return nil, fmt.Errorf("failed to read index table entry %d: %v", i, err)
		}
		if err := binary.Read(f, binary.LittleEndian, &indices[i].Size); err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to read index table size %d: %v", i, err)
		}
	}

	w, h := int32(header.Width), int32(header.Height)
	tex, err := renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_STREAMING, int(w), int(h))
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to create streaming texture: %v", err)
	}

	return &VideoResource{
		Header:    header,
		File:      f,
		Indices:   indices,
		Texture:   tex,
		RGBA:      make([]byte, header.FrameBytes),
		FPS:       float64(header.FPS),
		W:         float32(w),
		H:         float32(h),
		LastFrame: -1,
	}, nil
}

func (v *VideoResource) Destroy() {
	if v.Texture != nil {
		v.Texture.Destroy()
	}
	if v.File != nil {
		v.File.Close()
	}
}

func (v *VideoResource) UpdateFrame(targetFrame int) bool {
	if targetFrame < 0 {
		targetFrame = 0
	}
	// Loop the video
	targetFrame = targetFrame % int(v.Header.FrameCount)

	if targetFrame == v.LastFrame {
		return false
	}

	index := v.Indices[targetFrame]
	compressed := make([]byte, index.Size)
	if _, err := v.File.ReadAt(compressed, int64(index.Address)); err != nil {
		return false
	}

	n, err := lz4.UncompressBlock(compressed, v.RGBA)
	if err != nil {
		return false
	}
	if uint32(n) != v.Header.FrameBytes {
		return false
	}

	// Update the texture
	v.Texture.Update(nil, v.RGBA, int32(v.Header.Width*4))

	v.LastFrame = targetFrame
	return true
}
