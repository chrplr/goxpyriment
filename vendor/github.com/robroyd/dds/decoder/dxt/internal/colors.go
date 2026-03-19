package internal

import (
	"image/color"
	"math"
)

// InterpolateColors interpolates two 565 color values to 4 color.NRGBA values.
// Each color needs to be two bytes wide and formatted as 565 color. It will compare both values and decide
// how the interpolation is handled.
func InterpolateColors(v0, v1 []byte) (cv [4]color.NRGBA) {
	cv[0] = c565toRGBA(v0)
	cv[1] = c565toRGBA(v1)

	if (uint16(v0[0]) | uint16(v0[1])<<8) <= (uint16(v1[0]) | uint16(v1[1])<<8) {
		cv[2] = interpolateColor(cv[0], cv[1], 1, 1)
		cv[3] = color.NRGBA{} // A: 0; important and implicit
	} else {
		cv[2] = interpolateColor(cv[0], cv[1], 2, 1)
		cv[3] = interpolateColor(cv[0], cv[1], 1, 2)
	}
	return cv
}

func c565toRGBA(b0 []byte) color.NRGBA {
	return color.NRGBA{
		R: byte(math.Round(float64(ExtractVector(b0, 11, 5)) * (255 / 31))),
		G: byte(math.Round(float64(ExtractVector(b0, 5, 6)) * (255 / 63))),
		B: byte(math.Round(float64(ExtractVector(b0, 0, 5)) * (255 / 31))),
		A: 255,
	}
}

func interpolateColor(c0, c1 color.NRGBA, w0, w1 float64) color.NRGBA {
	return color.NRGBA{
		R: Weighted(w0, c0.R, w1, c1.R),
		G: Weighted(w0, c0.G, w1, c1.G),
		B: Weighted(w0, c0.B, w1, c1.B),
		A: 255,
	}
}
