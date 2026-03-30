// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Tearing Test
//
// Displays a full-height vertical white bar sweeping horizontally across
// the screen.  Screen tearing appears as a horizontal discontinuity in the
// bar edge.
//
// On exit, frame-interval statistics (identical to the jitter sub-test of
// Timing-Tests) are printed to the console.
//
// Controls:
//
//	↑ / ↓   — speed   +/- 50 px/s  (range 50 – 3000)
//	← / →   — width   +/- 1 px     (range 1 – 200)
//	ESC / Q — quit
//
// Flags:
//
//	-w <px>    bar width in pixels  (default 4)
//	-v <px/s>  speed in pixels/sec  (default 800)
//	-d         windowed developer mode
//	-s <id>    subject ID
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"runtime/debug"
	"sort"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	speedStep   = float32(50)
	minSpeed    = float32(50)
	maxSpeed    = float32(3000)
	minBarWidth = float32(1)
	maxBarWidth = float32(200)
	hudFontSize = float32(18)
	warmup      = 10 // frames discarded from statistics at startup
)

// ── Statistics (mirrors tests/Timing-Tests/main.go) ──────────────────────────

type stats struct {
	mean, sd, minV, maxV, p5, p95 float64
	late05, late1                  int
	n                              int
	vals                           []float64
}

func computeStats(deltas []float64, targetMs float64) stats {
	n := len(deltas)
	if n == 0 {
		return stats{}
	}
	var sum float64
	mn, mx := deltas[0], deltas[0]
	for _, v := range deltas {
		sum += v
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	mean := sum / float64(n)
	var sqSum float64
	var late05, late1 int
	for _, v := range deltas {
		sqSum += (v - mean) * (v - mean)
		dev := math.Abs(v - targetMs)
		if dev > 0.5 {
			late05++
		}
		if dev > 1.0 {
			late1++
		}
	}
	sd := 0.0
	if n > 1 {
		sd = math.Sqrt(sqSum / float64(n-1))
	}
	sorted := make([]float64, n)
	copy(sorted, deltas)
	sort.Float64s(sorted)
	p5 := sorted[n*5/100]
	p95 := sorted[n*95/100]
	return stats{mean, sd, mn, mx, p5, p95, late05, late1, n, deltas}
}

func printStats(label string, s stats, targetMs float64) {
	fmt.Printf("\n── %s ───────────────────────────────\n", label)
	fmt.Printf("  n       : %d\n", s.n)
	fmt.Printf("  target  : %.3f ms\n", targetMs)
	fmt.Printf("  mean    : %.3f ms\n", s.mean)
	fmt.Printf("  SD      : %.3f ms\n", s.sd)
	fmt.Printf("  min/max : %.3f / %.3f ms\n", s.minV, s.maxV)
	fmt.Printf("  p5/p95  : %.3f / %.3f ms\n", s.p5, s.p95)
	fmt.Printf("  >0.5 ms : %d (%.1f %%)\n", s.late05, 100*float64(s.late05)/float64(s.n))
	fmt.Printf("  >1.0 ms : %d (%.1f %%)\n", s.late1, 100*float64(s.late1)/float64(s.n))
	printHistogram(s.vals)
}

func printHistogram(vals []float64) {
	const nBins = 10
	const barWidth = 40
	n := len(vals)
	if n == 0 {
		return
	}
	mn, mx := vals[0], vals[0]
	for _, v := range vals {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	binW := (mx - mn) / nBins
	if binW == 0 {
		binW = 1
	}
	counts := make([]int, nBins)
	for _, v := range vals {
		b := int((v - mn) / binW)
		if b >= nBins {
			b = nBins - 1
		}
		counts[b]++
	}
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	fmt.Printf("  histogram (%d bins):\n", nBins)
	for i := 0; i < nBins; i++ {
		lo := mn + float64(i)*binW
		hi := lo + binW
		bar := ""
		if maxCount > 0 {
			stars := counts[i] * barWidth / maxCount
			for j := 0; j < stars; j++ {
				bar += "*"
			}
		}
		fmt.Printf("  [%7.3f, %7.3f) ms : %5d  %s\n", lo, hi, counts[i], bar)
	}
}

// ── Drawing helpers ───────────────────────────────────────────────────────────

// textItem caches a TextLine and only reallocates the GPU texture when the
// string content changes.
type textItem struct {
	tl   *stimuli.TextLine
	text string
}

func (ti *textItem) draw(screen *apparatus.Screen, newText string, x, y float32, color sdl.Color) {
	if newText != ti.text || ti.tl == nil {
		if ti.tl != nil {
			_ = ti.tl.Unload()
		}
		ti.tl = stimuli.NewTextLine(newText, x, y, color)
		ti.text = newText
	}
	_ = ti.tl.Draw(screen)
}

func (ti *textItem) unload() {
	if ti.tl != nil {
		_ = ti.tl.Unload()
		ti.tl = nil
	}
}

// vertBar draws a filled vertical bar whose SDL x-centre is cx, spanning the
// full screen height.
func vertBar(screen *apparatus.Screen, cx, w float32, color sdl.Color) {
	_ = screen.Renderer.SetDrawColor(color.R, color.G, color.B, color.A)
	_ = screen.Renderer.RenderFillRect(&sdl.FRect{
		X: cx - w/2,
		Y: 0,
		W: w,
		H: float32(screen.Height),
	})
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	barWidthFlag := flag.Float64("w", 4, "bar width in pixels")
	speedFlag := flag.Float64("v", 800, "speed in pixels per second")

	exp := control.NewExperimentFromFlags("Tearing Test", control.Black, control.White, hudFontSize)
	defer exp.End()

	barWidth := float32(*barWidthFlag)
	speed := float32(*speedFlag)

	if err := exp.Run(func() error {
		return animLoop(exp, barWidth, speed)
	}); err != nil && !control.IsEndLoop(err) {
		log.Fatalf("run: %v", err)
	}
}

// ── Animation loop ────────────────────────────────────────────────────────────

func animLoop(exp *control.Experiment, initBarWidth, initSpeed float32) error {
	// Disable GC to avoid jitter in the VSYNC-locked loop.
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	screen := exp.Screen
	sw := float32(screen.Width)
	sh := float32(screen.Height)

	barWidth := initBarWidth
	speed := initSpeed

	// barX is the SDL x-coordinate of the bar centre (0 = left edge of screen).
	barX := float32(0)

	hudColor := sdl.Color{R: 200, G: 200, B: 200, A: 255}
	hudY := -sh/2 + 10 // centre-relative: near top of screen

	var hud textItem
	defer hud.unload()

	// Drain stale events before the loop.
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
	}

	var (
		lastTime  = time.Now()
		fpsCnt    int
		fps       float32
		fpsTimer  = time.Now()
		frame     int
		prevT     float64
		intervals []float64
	)

	for {
		// ---- Timing ----------------------------------------------------------------
		now := time.Now()
		dt := float32(now.Sub(lastTime).Seconds())
		if dt > 0.05 {
			dt = 0.05
		}
		lastTime = now

		fpsCnt++
		if elapsed := now.Sub(fpsTimer).Seconds(); elapsed >= 0.5 {
			fps = float32(fpsCnt) / float32(elapsed)
			fpsCnt = 0
			fpsTimer = now
		}

		// ---- Physics ---------------------------------------------------------------
		barX += speed * dt
		// Wrap: re-enter from the left once the bar fully exits the right edge.
		if barX-barWidth/2 > sw {
			barX = -barWidth / 2
		}

		// ---- Render ----------------------------------------------------------------
		_ = screen.Clear()
		vertBar(screen, barX, barWidth, control.White)

		hudText := fmt.Sprintf(
			"FPS: %4.1f   Speed: %.0f px/s [↑↓]   Width: %.0f px [←→]   Quit [ESC/Q]",
			fps, speed, barWidth,
		)
		hud.draw(screen, hudText, 0, hudY, hudColor)

		_ = screen.Update() // blocks until VSYNC

		// ---- Frame-interval measurement (mirrors runJitter) ------------------------
		tA := float64(clock.GetTimeNS()) / 1e6 // ms with sub-ms precision
		if prevT > 0 && frame >= warmup {
			intervals = append(intervals, tA-prevT)
		}
		prevT = tA
		frame++

		// ---- Events ----------------------------------------------------------------
		quit := false
		state := exp.PollEvents(func(e sdl.Event) bool {
			if e.Type != sdl.EVENT_KEY_DOWN {
				return false
			}
			switch e.KeyboardEvent().Key {
			case sdl.K_UP:
				speed += speedStep
				if speed > maxSpeed {
					speed = maxSpeed
				}
			case sdl.K_DOWN:
				speed -= speedStep
				if speed < minSpeed {
					speed = minSpeed
				}
			case sdl.K_RIGHT:
				barWidth++
				if barWidth > maxBarWidth {
					barWidth = maxBarWidth
				}
			case sdl.K_LEFT:
				barWidth--
				if barWidth < minBarWidth {
					barWidth = minBarWidth
				}
			case sdl.K_Q:
				quit = true
			}
			return false
		})

		if state.QuitRequested || quit {
			break
		}
	}

	// ---- Print statistics (same format as the jitter sub-test) -----------------
	s := computeStats(intervals, 16.67) // first pass
	estimatedHz := 0.0
	if s.mean > 0 {
		estimatedHz = 1000.0 / s.mean
		s = computeStats(intervals, s.mean) // recompute late counts against actual mean
	}
	fmt.Printf("\nEstimated refresh rate: %.3f Hz\n", estimatedHz)
	printStats("Frame intervals", s, s.mean)

	return control.EndLoop
}
