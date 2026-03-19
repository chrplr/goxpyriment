package internal

import (
	"image"
	"image/color"
	"image/draw"
)

type ColorDecoder struct {
	colors  [4]color.NRGBA
	indices []byte
}

func (*ColorDecoder) New(bounds image.Rectangle) draw.Image {
	return image.NewNRGBA(bounds)
}

func (cd *ColorDecoder) BlockColor(colorsBlock []byte) {
	cd.colors = InterpolateColors(colorsBlock[0:2:2], colorsBlock[2:4:4])
	cd.indices = colorsBlock[4:8:8]
}

func (cd *ColorDecoder) PixelColor(pixelIndex byte) color.NRGBA {
	colorIndex := ExtractIndex(cd.indices, pixelIndex, 2)
	return cd.colors[colorIndex]
}

func (cd *ColorDecoder) PixelAlpha(pixelIndex, alpha byte) color.NRGBA {
	clr := cd.PixelColor(pixelIndex)
	clr.A = alpha
	return clr
}
