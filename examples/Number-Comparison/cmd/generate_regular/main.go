// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// generate_regular produces 9 playing-card-style dot-pattern PNG files,
// one per numerosity 1–9. Run once before building the experiment:
//
//	go run ./cmd/generate_regular/   (from examples/Number-Comparison/)

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

const (
	imgSize = 300
	dotR    = 18
)

// cardPositions[n] lists the (fx, fy) fractional positions of each dot for
// numerosity n (1-indexed). Coordinates are fractions of the image dimension,
// modelled on standard playing-card layouts.
var cardPositions = [10][][2]float64{
	{}, // index 0 unused
	// 1: single centre dot
	{{0.5, 0.5}},
	// 2: top and bottom
	{{0.5, 0.25}, {0.5, 0.75}},
	// 3: column of three
	{{0.5, 0.2}, {0.5, 0.5}, {0.5, 0.8}},
	// 4: four corners
	{{0.25, 0.25}, {0.75, 0.25}, {0.25, 0.75}, {0.75, 0.75}},
	// 5: four corners + centre
	{{0.25, 0.25}, {0.75, 0.25}, {0.5, 0.5}, {0.25, 0.75}, {0.75, 0.75}},
	// 6: two columns × three rows
	{{0.25, 0.2}, {0.75, 0.2}, {0.25, 0.5}, {0.75, 0.5}, {0.25, 0.8}, {0.75, 0.8}},
	// 7: six + one centre-top pip
	{{0.25, 0.175}, {0.75, 0.175}, {0.5, 0.35}, {0.25, 0.525}, {0.75, 0.525}, {0.25, 0.825}, {0.75, 0.825}},
	// 8: six + top-centre + bottom-centre pips
	{{0.25, 0.15}, {0.75, 0.15}, {0.5, 0.3}, {0.25, 0.5}, {0.75, 0.5}, {0.5, 0.7}, {0.25, 0.85}, {0.75, 0.85}},
	// 9: two columns × four rows + centre
	{{0.25, 0.15}, {0.75, 0.15}, {0.25, 0.35}, {0.75, 0.35}, {0.5, 0.5}, {0.25, 0.65}, {0.75, 0.65}, {0.25, 0.85}, {0.75, 0.85}},
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(cx+dx, cy+dy, c)
			}
		}
	}
}

func generateRegular(n int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{128, 128, 128, 255}), image.Point{}, draw.Src)
	black := color.RGBA{0, 0, 0, 255}
	for _, pos := range cardPositions[n] {
		cx := int(math.Round(pos[0] * imgSize))
		cy := int(math.Round(pos[1] * imgSize))
		fillCircle(img, cx, cy, dotR, black)
	}
	return img
}

func main() {
	outDir := flag.String("output", "assets/regular", "output directory for PNG files")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	for n := 1; n <= 9; n++ {
		img := generateRegular(n)
		path := filepath.Join(*outDir, fmt.Sprintf("dot_%d.png", n))
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			log.Fatal(err)
		}
		f.Close()
		fmt.Printf("wrote %s\n", path)
	}
}
