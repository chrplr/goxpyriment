// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.
package stimuli

import (
	"math/rand"

	"github.com/Zyko0/go-sdl3/sdl"
	xio "github.com/chrplr/goxpyriment/apparatus"
)

// RDS is a Random Dot Stereogram stimulus — a side-by-side pair of random-dot
// images that, when viewed through a stereoscope or by cross-fusing, create a
// depth percept.
//
// Embeds BaseVisual for position management. Overrides Unload to destroy the
// GPU texture; Preload is a no-op (lazy-loaded on first Draw via preload).
type RDS struct {
	BaseVisual // Position, GetPosition, SetPosition, Preload, Unload (Unload overridden below)
	ImgSize    [2]int
	InnerSize  [2]int
	Shift      int
	Gap        int
	Scale      int // scale factor for dots
	Texture    *sdl.Texture
}

// NewRDS creates a new Random Dot Stereogram stimulus centered at (0,0).
func NewRDS(imgSize, innerSize [2]int, shift, gap, scale int) *RDS {
	return &RDS{
		ImgSize:   imgSize,
		InnerSize: innerSize,
		Shift:     shift,
		Gap:       gap,
		Scale:     scale,
		// BaseVisual.Position defaults to (0, 0)
	}
}

// generateMatrix creates a random binary matrix.
func generateMatrix(rows, cols int) [][]int {
	matrix := make([][]int, rows)
	for i := range matrix {
		matrix[i] = make([]int, cols)
		for j := range matrix[i] {
			if rand.Float64() <= 0.5 {
				matrix[i][j] = 1
			} else {
				matrix[i][j] = 0
			}
		}
	}
	return matrix
}

// copyMatrix copies a source matrix into a destination matrix at a specific offset.
func copyMatrix(dest [][]int, src [][]int, rowOffset, colOffset int) {
	for i := 0; i < len(src); i++ {
		for j := 0; j < len(src[i]); j++ {
			dest[rowOffset+i][colOffset+j] = src[i][j]
		}
	}
}

// cloneMatrix creates a deep copy of a matrix.
func cloneMatrix(src [][]int) [][]int {
	dest := make([][]int, len(src))
	for i := range src {
		dest[i] = make([]int, len(src[i]))
		copy(dest[i], src[i])
	}
	return dest
}

// preload generates the stereogram texture (lazy-loaded on first Draw).
// Previously this was a public Preload(screen) method with a non-standard
// signature; it has been made private to align with the lazy-init pattern
// used by all other visual stimuli (see base.go).
func (rds *RDS) preload(screen *xio.Screen) error {
	background := generateMatrix(rds.ImgSize[0], rds.ImgSize[1])
	foreground := generateMatrix(rds.InnerSize[0], rds.InnerSize[1])

	// top left position of the foreground before shifting
	x := (rds.ImgSize[0] - rds.InnerSize[0]) / 2
	y := (rds.ImgSize[1] - rds.InnerSize[1]) / 2

	rightImg := cloneMatrix(background)
	xRight := x - rds.Shift/2
	copyMatrix(rightImg, foreground, xRight, y)

	leftImg := cloneMatrix(background)
	xLeft := xRight + rds.Shift
	copyMatrix(leftImg, foreground, xLeft, y)

	// Combine images: [leftImg, gap, rightImg]
	totalRows := rds.ImgSize[0]*2 + rds.Gap
	totalCols := rds.ImgSize[1]

	// Create a surface to draw the dots
	// Each dot will be scale x scale pixels
	surfW := totalRows * rds.Scale
	surfH := totalCols * rds.Scale
	surface, err := sdl.CreateSurface(int(surfW), int(surfH), sdl.PIXELFORMAT_RGBA32)
	if err != nil {
		return err
	}
	defer surface.Destroy()

	// Fill with white (gap color)
	surface.FillRect(nil, surface.MapRGB(255, 255, 255))

	drawMatrix := func(matrix [][]int, rowOffset int) {
		for i := 0; i < len(matrix); i++ {
			for j := 0; j < len(matrix[i]); j++ {
				color := uint8(0)
				if matrix[i][j] == 1 {
					color = 255
				}
				rect := &sdl.Rect{
					X: int32((rowOffset + i) * rds.Scale),
					Y: int32(j * rds.Scale),
					W: int32(rds.Scale),
					H: int32(rds.Scale),
				}
				surface.FillRect(rect, surface.MapRGB(color, color, color))
			}
		}
	}

	drawMatrix(leftImg, 0)
	drawMatrix(rightImg, rds.ImgSize[0]+rds.Gap)

	texture, err := screen.Renderer.CreateTextureFromSurface(surface)
	if err != nil {
		return err
	}
	rds.Texture = texture
	return nil
}

// Preload is provided by BaseVisual (no-op; texture is lazy-loaded on first Draw).

// Draw renders the RDS texture.
func (rds *RDS) Draw(screen *xio.Screen) error {
	if rds.Texture == nil {
		if err := rds.preload(screen); err != nil {
			return err
		}
	}

	w, h, err := rds.Texture.Size()
	if err != nil {
		return err
	}

	destX, destY := screen.CenterToSDL(rds.Position.X, rds.Position.Y)
	destRect := &sdl.FRect{
		X: destX - float32(w)/2,
		Y: destY - float32(h)/2,
		W: float32(w),
		H: float32(h),
	}

	return screen.Renderer.RenderTexture(rds.Texture, nil, destRect)
}

// Present delegates to PresentDrawable — the standard clear → draw → update cycle.
func (rds *RDS) Present(screen *xio.Screen, clear, update bool) error {
	return PresentDrawable(rds, screen, clear, update)
}

// GetPosition, SetPosition are provided by BaseVisual.

// Unload overrides BaseVisual.Unload to destroy the GPU texture.
func (rds *RDS) Unload() error {
	if rds.Texture != nil {
		rds.Texture.Destroy()
		rds.Texture = nil
	}
	return nil
}
