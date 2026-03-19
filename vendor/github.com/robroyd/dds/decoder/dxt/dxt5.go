package dxt

import (
	. "github.com/robroyd/dds/decoder/dxt/internal"
	"image/color"
)

type dxt5 struct {
	ColorDecoder
	alphaValues  [8]byte
	alphaIndices []byte
}

func (*dxt5) BlockSize() byte {
	return 16
}

func (d *dxt5) DecodeBlock(buffer []byte) {
	d.alphaValues = d.interpolateAlphaValues(buffer[0:2:2])
	d.alphaIndices = buffer[2:8:8]
	d.BlockColor(buffer[8:16:16])
}

func (d *dxt5) Pixel(index byte) color.Color {
	alphaIndex := ExtractIndex(d.alphaIndices, index, 3)
	alpha := d.alphaValues[alphaIndex]
	return d.PixelAlpha(index, alpha)
}

func (d *dxt5) interpolateAlphaValues(a0 []byte) (av [8]byte) {
	av[0] = a0[0]
	av[1] = a0[1]

	if a0[0] <= a0[1] {
		av[2] = Weighted(4, a0[0], 1, a0[1])
		av[3] = Weighted(3, a0[0], 2, a0[1])
		av[4] = Weighted(2, a0[0], 3, a0[1])
		av[5] = Weighted(1, a0[0], 4, a0[1])
		av[6] = 0
		av[7] = 255
	} else {
		av[2] = Weighted(6, a0[0], 1, a0[1])
		av[3] = Weighted(5, a0[0], 2, a0[1])
		av[4] = Weighted(4, a0[0], 3, a0[1])
		av[5] = Weighted(3, a0[0], 4, a0[1])
		av[6] = Weighted(2, a0[0], 5, a0[1])
		av[7] = Weighted(1, a0[0], 6, a0[1])
	}
	return
}
