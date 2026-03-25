package dxt

import (
	. "github.com/robroyd/dds/decoder/dxt/internal"
	"image/color"
)

type dxt3 struct {
	ColorDecoder
	alphaValues []byte
}

func (*dxt3) BlockSize() byte {
	return 16
}

func (d *dxt3) DecodeBlock(buffer []byte) {
	d.alphaValues = buffer[0:8:8]
	d.BlockColor(buffer[8:16:16])
}

func (d *dxt3) Pixel(index byte) color.Color {
	alpha := ExtractIndex(d.alphaValues, index, 4) * 17
	return d.PixelAlpha(index, alpha)
}
