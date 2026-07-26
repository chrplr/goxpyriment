// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Image helpers shared by the RSVP presentation program: block-average
// pixelation (used to synthesise oddball stimuli at runtime) and an
// area-average downscale (used to cap GPU texture memory before upload).
//
// Both operate on decoded image.Image values and return *image.RGBA; decoding
// and encoding are the caller's responsibility.

package main

import "image"

// toRGBA returns src as an *image.RGBA, copying pixel data if it is not already
// one. This gives the resize/pixelate loops fast, format-stable pixel access.
func toRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// pixelate returns a block-averaged ("pixelated") copy of src: the image is
// tiled into blockSize×blockSize cells and every pixel in a cell is replaced by
// the color sampled at the cell centre. Larger blockSize ⇒ coarser, more
// obviously artificial oddball. blockSize <= 1 returns src unchanged (as RGBA).
func pixelate(src image.Image, blockSize int) *image.RGBA {
	rgba := toRGBA(src)
	if blockSize <= 1 {
		return rgba
	}
	w := rgba.Rect.Dx()
	h := rgba.Rect.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y += blockSize {
		for x := 0; x < w; x += blockSize {
			endX := min(x+blockSize, w)
			endY := min(y+blockSize, h)
			// Sample the colour from the centre of the block.
			c := rgba.At(x+(endX-x)/2, y+(endY-y)/2)
			for by := y; by < endY; by++ {
				for bx := x; bx < endX; bx++ {
					dst.Set(bx, by, c)
				}
			}
		}
	}
	return dst
}

// resizeFit returns a downscaled copy of src whose largest dimension is at most
// maxDim, preserving aspect ratio. Each destination pixel is the average of the
// source block that maps onto it (area averaging — good quality for downscaling
// and dependency-free). If src already fits (or maxDim <= 0) it is returned
// unchanged (as RGBA). Never upscales.
func resizeFit(src image.Image, maxDim int) *image.RGBA {
	rgba := toRGBA(src)
	sw := rgba.Rect.Dx()
	sh := rgba.Rect.Dy()
	longest := max(sw, sh)
	if maxDim <= 0 || longest <= maxDim {
		return rgba
	}

	scale := float64(maxDim) / float64(longest)
	dw := int(float64(sw)*scale + 0.5)
	dh := int(float64(sh)*scale + 0.5)
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))

	for dy := 0; dy < dh; dy++ {
		// Source row span [sy0, sy1) covered by this destination row.
		sy0 := dy * sh / dh
		sy1 := (dy + 1) * sh / dh
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for dx := 0; dx < dw; dx++ {
			sx0 := dx * sw / dw
			sx1 := (dx + 1) * sw / dw
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var rSum, gSum, bSum, aSum, n uint32
			for sy := sy0; sy < sy1; sy++ {
				row := rgba.PixOffset(sx0, sy)
				for sx := sx0; sx < sx1; sx++ {
					rSum += uint32(rgba.Pix[row])
					gSum += uint32(rgba.Pix[row+1])
					bSum += uint32(rgba.Pix[row+2])
					aSum += uint32(rgba.Pix[row+3])
					row += 4
					n++
				}
			}
			o := dst.PixOffset(dx, dy)
			dst.Pix[o] = uint8(rSum / n)
			dst.Pix[o+1] = uint8(gSum / n)
			dst.Pix[o+2] = uint8(bSum / n)
			dst.Pix[o+3] = uint8(aSum / n)
		}
	}
	return dst
}
