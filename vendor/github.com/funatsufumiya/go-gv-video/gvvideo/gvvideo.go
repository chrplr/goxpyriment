package gvvideo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/draw"
	"io"
	"os"

	"github.com/pierrec/lz4/v4"
	"github.com/robroyd/dds/decoder/dxt"
)

// GVFormat represents supported formats
const (
	GVFormatDXT1 = 1
	GVFormatDXT3 = 3
	GVFormatDXT5 = 5
)

type GVHeader struct {
	Width      uint32
	Height     uint32
	FrameCount uint32
	FPS        float32
	Format     uint32
	FrameBytes uint32
}

type GVAddressSizeBlock struct {
	Address uint64
	Size    uint64
}

type GVVideo struct {
	Header            GVHeader
	AddressSizeBlocks []GVAddressSizeBlock
	Reader            io.ReadSeeker
}

// returns the LZ4-decompressed byte slice for the specified frame (no DXT decode)
func (v *GVVideo) ReadFrameCompressed(frameID uint32) ([]byte, error) {
	if frameID >= v.Header.FrameCount {
		return nil, errors.New("end of video")
	}
	block := v.AddressSizeBlocks[frameID]
	if _, err := v.Reader.Seek(int64(block.Address), io.SeekStart); err != nil {
		return nil, err
	}
	compressed := make([]byte, block.Size)
	if _, err := io.ReadFull(v.Reader, compressed); err != nil {
		return nil, err
	}
	width := int(v.Header.Width)
	height := int(v.Header.Height)
	uncompressedSize := width * height * 4
	decompressed := make([]byte, uncompressedSize)
	if _, err := lz4.UncompressBlock(compressed, decompressed); err != nil {
		return nil, err
	}
	return decompressed, nil
}

// Decompresses the specified frame into the provided buffer (no DXT decode)
func (v *GVVideo) ReadFrameCompressedTo(frameID uint32, buf []byte) error {
	if frameID >= v.Header.FrameCount {
		return errors.New("end of video")
	}
	block := v.AddressSizeBlocks[frameID]
	if _, err := v.Reader.Seek(int64(block.Address), io.SeekStart); err != nil {
		return err
	}
	compressed := make([]byte, block.Size)
	if _, err := io.ReadFull(v.Reader, compressed); err != nil {
		return err
	}
	width := int(v.Header.Width)
	height := int(v.Header.Height)
	uncompressedSize := width * height * 4
	if len(buf) < uncompressedSize {
		return errors.New("buffer too small")
	}
	if _, err := lz4.UncompressBlock(compressed, buf[:uncompressedSize]); err != nil {
		return err
	}
	return nil
}

// ReadFrameTo decodes the specified frame into the provided RGBA buffer
func (v *GVVideo) ReadFrameTo(frameID uint32, buf *image.RGBA) error {
	if frameID >= v.Header.FrameCount {
		return errors.New("end of video")
	}
	block := v.AddressSizeBlocks[frameID]
	if _, err := v.Reader.Seek(int64(block.Address), io.SeekStart); err != nil {
		return err
	}
	compressed := make([]byte, block.Size)
	if _, err := io.ReadFull(v.Reader, compressed); err != nil {
		return err
	}
	width := int(v.Header.Width)
	height := int(v.Header.Height)
	uncompressedSize := width * height * 4
	decompressed := make([]byte, uncompressedSize)
	if _, err := lz4.UncompressBlock(compressed, decompressed); err != nil {
		return err
	}
	var fourCC string
	switch v.Header.Format {
	case GVFormatDXT1:
		fourCC = "DXT1"
	case GVFormatDXT3:
		fourCC = "DXT3"
	case GVFormatDXT5:
		fourCC = "DXT5"
	default:
		return errors.New("unsupported format")
	}
	decoder, err := dxt.New(fourCC, width, height)
	if err != nil {
		return err
	}
	img, err := decoder.Decode(bytes.NewReader(decompressed))
	if err != nil {
		return err
	}
	switch src := img.(type) {
	case *image.RGBA:
		copy(buf.Pix, src.Pix)
	case *image.NRGBA:
		rgba := image.NewRGBA(src.Bounds())
		draw.Draw(rgba, src.Bounds(), src, image.Point{}, draw.Src)
		copy(buf.Pix, rgba.Pix)
	default:
		return errors.New("not RGBA image")
	}
	return nil
}

func ReadHeader(r io.Reader) (GVHeader, error) {
	header := GVHeader{}
	if err := binary.Read(r, binary.LittleEndian, &header.Width); err != nil {
		return header, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.Height); err != nil {
		return header, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.FrameCount); err != nil {
		return header, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.FPS); err != nil {
		return header, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.Format); err != nil {
		return header, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.FrameBytes); err != nil {
		return header, err
	}
	return header, nil
}

func ReadAddressSizeBlocks(r io.ReadSeeker, frameCount uint32) ([]GVAddressSizeBlock, error) {
	blocks := make([]GVAddressSizeBlock, frameCount)
	// Seek to address blocks
	if _, err := r.Seek(-int64(frameCount*16), io.SeekEnd); err != nil {
		return nil, err
	}
	for i := range blocks {
		if err := binary.Read(r, binary.LittleEndian, &blocks[i].Address); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &blocks[i].Size); err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

func LoadGVVideo(path string) (*GVVideo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	header, err := ReadHeader(f)
	if err != nil {
		return nil, err
	}
	blocks, err := ReadAddressSizeBlocks(f, header.FrameCount)
	if err != nil {
		return nil, err
	}
	return &GVVideo{
		Header:            header,
		AddressSizeBlocks: blocks,
		Reader:            f,
	}, nil
}

// ReadFrame returns RGBA []uint8 for the specified frame
func (v *GVVideo) ReadFrame(frameID uint32) ([]uint8, error) {
	width := int(v.Header.Width)
	height := int(v.Header.Height)
	buf := image.NewRGBA(image.Rect(0, 0, width, height))
	err := v.ReadFrameTo(frameID, buf)
	if err != nil {
		return nil, err
	}
	return buf.Pix, nil
}

// LoadGVVideoFromReader loads GVVideo from any io.ReadSeeker (e.g. memory buffer)
func LoadGVVideoFromReader(r io.ReadSeeker) (*GVVideo, error) {
	header, err := ReadHeader(r)
	if err != nil {
		return nil, err
	}
	blocks, err := ReadAddressSizeBlocks(r, header.FrameCount)
	if err != nil {
		return nil, err
	}
	return &GVVideo{
		Header:            header,
		AddressSizeBlocks: blocks,
		Reader:            r,
	}, nil
}
