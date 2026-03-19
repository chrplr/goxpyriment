package dxt

import (
	. "github.com/robroyd/dds/decoder/dxt/internal"
	"image/color"
)

type dxt1 struct {
	ColorDecoder
}

func (*dxt1) BlockSize() byte {
	return 8
}

func (d *dxt1) DecodeBlock(buffer []byte) {
	d.BlockColor(buffer[0:8:8])
}

func (d *dxt1) Pixel(index byte) color.Color {
	return d.PixelColor(index)
}
