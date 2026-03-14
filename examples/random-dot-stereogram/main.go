// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"flag"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/io"
	"log"
)

// RDS represents a Random Dot Stereogram stimulus.
type RDS struct {
	ImgSize   [2]int
	InnerSize [2]int
	Shift     int
	Gap       int
	Scale     int // scale factor for dots
	Position  sdl.FPoint
	Texture   *sdl.Texture
}

// NewRDS creates a new Random Dot Stereogram stimulus.
func NewRDS(imgSize, innerSize [2]int, shift, gap, scale int) *RDS {
	return &RDS{
		ImgSize:   imgSize,
		InnerSize: innerSize,
		Shift:     shift,
		Gap:       gap,
		Scale:     scale,
		Position:  sdl.FPoint{X: 0, Y: 0},
	}
}

// generateMatrix creates a random binary matrix.
func generateMatrix(rows, cols int) [][]int {
	matrix := make([][]int, rows)
	for i := range matrix {
		matrix[i] = make([]int, cols)
		for j := range matrix[i] {
			if design.CoinFlip(0.5) {
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

// Preload generates the stereogram texture.
func (rds *RDS) Preload(screen *io.Screen) error {
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

// Draw renders the RDS texture.
func (rds *RDS) Draw(screen *io.Screen) error {
	if rds.Texture == nil {
		if err := rds.Preload(screen); err != nil {
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
		H: float32(h)	}

	return screen.Renderer.RenderTexture(rds.Texture, nil, destRect)
}

func main() {
	develop := flag.Bool("d", false, "Developer mode (windowed 800x600)")
	flag.Parse()

	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 800, 600, false
	}

	exp := control.NewExperiment("Random Dot Stereogram", width, height, fullscreen, control.Gray, control.White, 32)

	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// Create RDS stimulus
	// Python defaults: imgsize=(80, 80), inner_size=(30, 30), shift=6, gap=10
	// We'll scale it up for better visibility
	rds := NewRDS([2]int{80, 80}, [2]int{30, 30}, 6, 10, 4)

	err := exp.Run(func() error {
		if _, _, err := exp.HandleEvents(); err != nil {
			return err
		}

		if err := exp.Screen.Clear(); err != nil {
			return err
		}

		if err := rds.Draw(exp.Screen); err != nil {
			return err
		}

		return exp.Screen.Update()
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatalf("experiment error: %v", err)
	}
}
