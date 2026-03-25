// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// generate_irregular produces 9 dot-pattern PNG files with irregular (random
// but reproducible) layouts, one per numerosity 1–9. A fixed seed per
// numerosity ensures the same pattern is generated every time.
// Run once before building the experiment:
//
//	go run ./cmd/generate_irregular/   (from examples/Number-Comparison/)

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
	"math/rand"
	"os"
	"path/filepath"
)

const (
	imgSize = 300
	dotR    = 18
	minGap  = 6  // minimum gap between dot edges (px)
	margin  = 28 // minimum distance from dot centre to image edge
)

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(cx+dx, cy+dy, c)
			}
		}
	}
}

// generateDotImage places n dots via rejection sampling with a minimum
// centre-to-centre distance of 2*dotR+minGap.
func generateDotImage(n int, rng *rand.Rand) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{128, 128, 128, 255}), image.Point{}, draw.Src)

	type pt struct{ x, y int }
	var placed []pt
	minDist := float64(2*dotR + minGap)
	lo := margin + dotR
	hi := imgSize - margin - dotR

	for len(placed) < n {
		for attempt := 0; attempt < 20000; attempt++ {
			x := lo + rng.Intn(hi-lo+1)
			y := lo + rng.Intn(hi-lo+1)
			ok := true
			for _, p := range placed {
				dx, dy := float64(x-p.x), float64(y-p.y)
				if math.Sqrt(dx*dx+dy*dy) < minDist {
					ok = false
					break
				}
			}
			if ok {
				placed = append(placed, pt{x, y})
				fillCircle(img, x, y, dotR, color.RGBA{0, 0, 0, 255})
				break
			}
		}
	}
	return img
}

func main() {
	outDir := flag.String("output", "assets/irregular", "output directory for PNG files")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	for n := 1; n <= 9; n++ {
		// Seed is fixed per numerosity — same layout every build.
		rng := rand.New(rand.NewSource(int64(n) * 98765))
		img := generateDotImage(n, rng)
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
		fmt.Printf("wrote %s  (%d dots)\n", path, n)
	}
}
