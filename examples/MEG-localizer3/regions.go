// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package main

import (
	"fmt"
	"math"
)

// The 24-region dartboard of Kurki et al. (2022), Figure 1B: three annuli
// crossed with eight 45-degree sectors whose boundaries lie on the vertical and
// horizontal meridians. Each region carries a 4x4 checkerboard.
//
// The checks are laid out from a *global* radial and angular index rather than
// per region, so the 24 regions together form one continuous dartboard instead
// of 24 independently-phased patches — which is what the figure shows.
//
// Each region is rasterised once into its own bounding box, with transparent
// pixels outside the sector. At run time a region's contrast is set by
// modulating its texture's alpha: drawn over a mid-grey background with
// BLENDMODE_BLEND the result is a*checker + (1-a)*grey, so alpha *is* Michelson
// contrast, and the paper's linear contrast fade is a linear alpha ramp with no
// per-frame pixel work at all.

const (
	// NRings is the number of annuli, NSectors the number of angular sectors.
	NRings   = 3
	NSectors = 8
	// NRegions is the number of independently stimulated regions.
	NRegions = NRings * NSectors

	// Checks per region, radially and angularly.
	ChecksRadial  = 4
	ChecksAngular = 4

	// DefaultPrime is the sequence period; see sequence.go for why 467.
	DefaultPrime = 467

	// supersample is the linear subsampling factor used to antialias the arc
	// and meridian edges. 3 means 9 subsamples per pixel.
	supersample = 3
)

// RingRadiiDeg are the annulus boundaries in degrees of visual angle.
// The innermost radius is the grey disc that carries the fixation cross.
var RingRadiiDeg = [NRings + 1]float64{0.5, 2.3, 4.7, 8.4}

// MaxEccentricityDeg is the outer radius of the stimulus.
const MaxEccentricityDeg = 8.4

// Region describes one stimulated patch of the visual field.
type Region struct {
	Index  int
	Ring   int // 0 = innermost
	Sector int // 0 = [0,45) degrees, counter-clockwise from the right horizontal meridian

	RInDeg, ROutDeg            float64
	ThetaStartDeg, ThetaEndDeg float64
}

// Regions returns the region table in index order, index = ring*NSectors + sector.
func Regions() []Region {
	regs := make([]Region, 0, NRegions)
	for ring := 0; ring < NRings; ring++ {
		for sector := 0; sector < NSectors; sector++ {
			regs = append(regs, Region{
				Index:         ring*NSectors + sector,
				Ring:          ring,
				Sector:        sector,
				RInDeg:        RingRadiiDeg[ring],
				ROutDeg:       RingRadiiDeg[ring+1],
				ThetaStartDeg: float64(sector) * 360.0 / NSectors,
				ThetaEndDeg:   float64(sector+1) * 360.0 / NSectors,
			})
		}
	}
	return regs
}

// RegionImage is one region rasterised into its own bounding box.
//
// OffsetX/OffsetY are the top-left corner of that box inside the square of
// side SizePx, in SDL pixel coordinates (origin top-left, +Y down).
type RegionImage struct {
	Region
	OffsetX, OffsetY int
	W, H             int
	Pix              []byte // RGBA, len = W*H*4
}

// RasterizeRegions renders every region into a bounding-box-cropped RGBA image.
//
// ringPx holds the annulus boundaries in pixels, and sizePx is the side of the
// square the dartboard is drawn in (nominally 2*ringPx[NRings]).
//
// The angle convention is the visual-field one used throughout goxpyriment:
// +Y is UP and angles run counter-clockwise from the right horizontal meridian.
// The pixel buffer is SDL-ordered (+Y down), so the vertical axis is flipped
// when converting a pixel to a field position. Getting this backwards mirrors
// the whole map about the horizontal meridian and looks like a subject problem
// rather than a units bug, so it is done once, here.
func RasterizeRegions(sizePx int, ringPx [NRings + 1]float64) []RegionImage {
	cx := float64(sizePx) / 2
	cy := float64(sizePx) / 2

	regs := Regions()
	out := make([]RegionImage, 0, NRegions)

	for _, reg := range regs {
		r0 := ringPx[reg.Ring]
		r1 := ringPx[reg.Ring+1]
		a0 := reg.ThetaStartDeg * math.Pi / 180
		a1 := reg.ThetaEndDeg * math.Pi / 180

		x0, y0, x1, y1 := sectorBounds(cx, cy, r0, r1, a0, a1, sizePx)
		w := x1 - x0
		h := y1 - y0
		if w <= 0 || h <= 0 {
			// Degenerate only if the geometry is misconfigured; emit an empty
			// image rather than silently dropping a region and renumbering.
			out = append(out, RegionImage{Region: reg, OffsetX: 0, OffsetY: 0, W: 0, H: 0})
			continue
		}

		pix := make([]byte, w*h*4)
		const nSub = supersample * supersample

		for py := y0; py < y1; py++ {
			for px := x0; px < x1; px++ {
				cover := 0
				lum := 0
				for sy := 0; sy < supersample; sy++ {
					for sx := 0; sx < supersample; sx++ {
						fx := float64(px) + (float64(sx)+0.5)/supersample
						fy := float64(py) + (float64(sy)+0.5)/supersample
						dx := fx - cx
						dy := cy - fy // flip: +Y up in the visual field
						r := math.Hypot(dx, dy)
						if r < r0 || r >= r1 {
							continue
						}
						th := math.Atan2(dy, dx)
						if th < 0 {
							th += 2 * math.Pi
						}
						if th < a0 || th >= a1 {
							continue
						}
						cover++
						ri := int((r - r0) / (r1 - r0) * ChecksRadial)
						if ri >= ChecksRadial {
							ri = ChecksRadial - 1
						}
						ai := int((th - a0) / (a1 - a0) * ChecksAngular)
						if ai >= ChecksAngular {
							ai = ChecksAngular - 1
						}
						// Global check indices, so neighbouring regions line up.
						gr := reg.Ring*ChecksRadial + ri
						ga := reg.Sector*ChecksAngular + ai
						if (gr+ga)%2 == 0 {
							lum += 255
						}
					}
				}
				o := ((py-y0)*w + (px - x0)) * 4
				if cover == 0 {
					continue // leaves RGBA = 0,0,0,0
				}
				v := byte(lum / cover)
				pix[o+0] = v
				pix[o+1] = v
				pix[o+2] = v
				pix[o+3] = byte(255 * cover / nSub)
			}
		}

		out = append(out, RegionImage{
			Region: reg, OffsetX: x0, OffsetY: y0, W: w, H: h, Pix: pix,
		})
	}
	return out
}

// sectorBounds returns the pixel bounding box of an annular sector, clamped to
// the square and widened by one pixel so antialiased edges are not clipped.
//
// The extreme points of an annular sector lie either at a corner (one of the
// two edge angles at one of the two radii) or where the sector crosses a
// coordinate axis. Evaluating both radii at the edge angles and at any axis
// direction strictly inside the sector is a superset of the extremes.
func sectorBounds(cx, cy, r0, r1, a0, a1 float64, sizePx int) (x0, y0, x1, y1 int) {
	angles := []float64{a0, a1}
	for _, ax := range []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2} {
		if ax > a0 && ax < a1 {
			angles = append(angles, ax)
		}
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, a := range angles {
		for _, r := range []float64{r0, r1} {
			ix := cx + r*math.Cos(a)
			iy := cy - r*math.Sin(a) // +Y up in the field, +Y down in pixels
			minX, maxX = math.Min(minX, ix), math.Max(maxX, ix)
			minY, maxY = math.Min(minY, iy), math.Max(maxY, iy)
		}
	}
	x0 = clampInt(int(math.Floor(minX))-1, 0, sizePx)
	y0 = clampInt(int(math.Floor(minY))-1, 0, sizePx)
	x1 = clampInt(int(math.Ceil(maxX))+1, 0, sizePx)
	y1 = clampInt(int(math.Ceil(maxY))+1, 0, sizePx)
	return
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// RegionTableLines renders the region table for the data file's header, so
// analysis code can map a region index to a patch of visual field without
// reading this source. ringDeg holds the achieved annulus boundaries in
// degrees, which differ from [RingRadiiDeg] when the design was rescaled to
// fit the screen.
func RegionTableLines(ringDeg [NRings + 1]float64) []string {
	lines := []string{
		"regions: index ring sector r_in_deg r_out_deg theta_start_deg theta_end_deg",
		"regions: angles are counter-clockwise from the right horizontal meridian, +Y up",
	}
	for _, r := range Regions() {
		lines = append(lines, fmt.Sprintf("regions: %2d %d %d %.3f %.3f %.1f %.1f",
			r.Index, r.Ring, r.Sector,
			ringDeg[r.Ring], ringDeg[r.Ring+1], r.ThetaStartDeg, r.ThetaEndDeg))
	}
	return lines
}
