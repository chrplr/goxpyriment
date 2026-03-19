// Package dxt holds decoder for all kind DXT based dds encodings
// Specification: https://www.khronos.org/registry/OpenGL/extensions/EXT/EXT_texture_compression_s3tc.txt.
package dxt

import (
	"fmt"
	. "github.com/robroyd/dds/decoder/dxt/internal"
	"image"
	"image/color"
	"image/draw"
	"io"
)

type (
	Decoder struct {
		strategy
		bounds image.Point
	}

	strategy interface {
		New(bounds image.Rectangle) draw.Image
		BlockSize() byte
		DecodeBlock(buffer []byte)
		Pixel(index byte) color.Color
	}
)

func New(fourCC string, width, height int) (*Decoder, error) {
	decoder := &Decoder{bounds: image.Pt(width, height)}

	switch fourCC {
	case "DXT1":
		decoder.strategy = new(dxt1)
	case "DXT3":
		decoder.strategy = new(dxt3)
	case "DXT5":
		decoder.strategy = new(dxt5)
	default:
		return nil, fmt.Errorf("DXT type '%s' not supported", fourCC)
	}

	return decoder, nil
}

func (d *Decoder) Decode(r io.Reader) (image.Image, error) {
	bounds := image.Rectangle{Max: d.bounds}
	rgba := d.New(bounds)
	if bounds.Empty() {
		return rgba, nil
	}

	rd := NewReader(r, d.BlockSize())
	for h := 0; h < d.bounds.Y; h += 4 {
		for w := 0; w < d.bounds.X; w += 4 {
			buffer, err := rd.Read()
			if err != nil {
				return nil, err
			}

			d.DecodeBlock(buffer)
			for y := 3; y >= 0; y-- {
				for x := 0; x < 4; x++ {
					pxIndex := byte(x + y*4)
					pxColor := d.Pixel(pxIndex)
					rgba.Set(w+x, h+y, pxColor)
				}
			}
		}
	}
	return rgba, nil
}
