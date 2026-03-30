// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Multiple Object Tracking (MOT) — Pylyshyn's paradigm
//
// Each trial proceeds in three phases:
//
//	Phase 1 (4 s)  : 10 circles appear stationary; N targets are
//	                 highlighted in red so the participant can memorise them.
//	Phase 2 (varies): All circles turn blue and move at speed specified by
//	                 -speed flag (default 50 px/s) in random directions,
//	                 bouncing off the borders of a black circular playfield
//	                 and each other (elastic collisions, no overlap).
//	Phase 3        : Motion stops. Participant clicks exactly N circles they
//	                 believe were the targets.  Selected circles turn yellow.
//	                 After the Nth click results are shown: correct = green,
//	                 wrong = red, missed target = orange.
//
// Eight trials are run: two for each N ∈ {4, 5, 6, 7}, presented in random
// order.  For each clicked circle the trial log records whether it was a target
// and the running total score.
//
// Controls:
//
//	Left-click  — select / deselect a circle (Phase 3 only)
//	ESC         — quit at any time
//
// Usage:
//
//	go run main.go [-d] [-s <id>] [-speed <px/s>] [-disksize <px>] [-trialduration <ms>]
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ---------------------------------------------------------------------------
// Constants & Variables
// ---------------------------------------------------------------------------

const (
	numCircles = 10
	phase1Ms   = 4000        // target-highlight duration (ms)
	resultMs   = 2000        // result display duration (ms)
	hudH       = float32(44) // HUD strip height at top
)

var (
	circleR       = float32(22)
	minSeparation = circleR * 3  // minimum centre distance during init
	circleSpeed   = float32(100) // px/s
	phase2Ms      = int64(10000) // motion duration (ms)
)

// ---------------------------------------------------------------------------
// Phase enum
// ---------------------------------------------------------------------------

const (
	phaseHighlight = iota // circles stationary, targets in red
	phaseMotion           // all white, moving
	phaseResponse         // stopped, clicking
	phaseResult           // brief result display
)

// ---------------------------------------------------------------------------
// Dot (one circle)
// ---------------------------------------------------------------------------

type dot struct {
	x, y     float32 // centre, center-relative coordinates
	vx, vy   float32 // velocity (px/s)
	isTarget bool
	selected bool
	circ     *stimuli.Circle
}

// dist2 returns the squared distance between two dots' centres.
func dist2(a, b *dot) float32 {
	dx, dy := a.x-b.x, a.y-b.y
	return dx*dx + dy*dy
}

// ---------------------------------------------------------------------------
// Physics helpers
// ---------------------------------------------------------------------------

// bounceCircular reflects a dot's velocity when its centre would leave the
// circular playfield of radius R, and clamps the position.
func bounceCircular(d *dot, R float32) {
	dist := float32(math.Sqrt(float64(d.x*d.x + d.y*d.y)))
	if dist > R && dist > 0 {
		nx, ny := d.x/dist, d.y/dist // normal pointing outwards
		// Velocity component along the outward normal.
		vn := d.vx*nx + d.vy*ny
		if vn > 0 { // moving towards the wall
			// Reflection: v' = v - 2(v.n)n
			d.vx -= 2 * vn * nx
			d.vy -= 2 * vn * ny
		}
		// Clamp to boundary.
		d.x = nx * R
		d.y = ny * R
	}
}

// resolveCollisions performs pairwise elastic collision detection and
// resolution.  Overlapping pairs have their velocities exchanged along the
// collision normal and their positions separated.
func resolveCollisions(dots []dot) {
	minD := float32(2) * circleR
	minD2 := minD * minD

	for i := range dots {
		for j := i + 1; j < len(dots); j++ {
			dx := dots[i].x - dots[j].x
			dy := dots[i].y - dots[j].y
			d2 := dx*dx + dy*dy
			if d2 >= minD2 || d2 == 0 {
				continue
			}

			d := float32(math.Sqrt(float64(d2)))
			nx, ny := dx/d, dy/d

			// Relative velocity projected onto collision normal.
			dvx := dots[i].vx - dots[j].vx
			dvy := dots[i].vy - dots[j].vy
			vn := dvx*nx + dvy*ny

			// Skip if already separating.
			if vn >= 0 {
				continue
			}

			// Exchange momentum along normal (equal masses).
			dots[i].vx -= vn * nx
			dots[i].vy -= vn * ny
			dots[j].vx += vn * nx
			dots[j].vy += vn * ny

			// Push apart to eliminate overlap.
			overlap := (minD - d) * 0.51
			dots[i].x += nx * overlap
			dots[i].y += ny * overlap
			dots[j].x -= nx * overlap
			dots[j].y -= ny * overlap
		}
	}
}

// ---------------------------------------------------------------------------
// Initialisation
// ---------------------------------------------------------------------------

// initDots places numCircles dots randomly (without overlap) inside a circular
// area of radius R and assigns random unit-speed velocities. nTargets of them
// are marked as targets.
func initDots(nTargets int, R float32) []dot {
	dots := make([]dot, numCircles)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := range dots {
		for {
			// Polar distribution for uniform disk sampling.
			r := float32(math.Sqrt(rng.Float64())) * R
			angle := rng.Float64() * 2 * math.Pi
			cx := r * float32(math.Cos(angle))
			cy := r * float32(math.Sin(angle))

			ok := true
			for j := 0; j < i; j++ {
				dx, dy := cx-dots[j].x, cy-dots[j].y
				if dx*dx+dy*dy < minSeparation*minSeparation {
					ok = false
					break
				}
			}
			if ok {
				dots[i].x, dots[i].y = cx, cy
				break
			}
		}

		// Random unit-speed velocity.
		angle := rng.Float64() * 2 * math.Pi
		dots[i].vx = circleSpeed * float32(math.Cos(angle))
		dots[i].vy = circleSpeed * float32(math.Sin(angle))
		dots[i].circ = stimuli.NewCircle(circleR, control.White)
	}

	// Designate targets (first nTargets after a shuffle).
	perm := rng.Perm(numCircles)
	for _, idx := range perm[:nTargets] {
		dots[idx].isTarget = true
	}
	return dots
}

// ---------------------------------------------------------------------------
// Drawing
// ---------------------------------------------------------------------------

var (
	colorWhite  = sdl.Color{R: 255, G: 255, B: 255, A: 255}
	colorRed    = sdl.Color{R: 230, G: 50, B: 50, A: 255}
	colorYellow = sdl.Color{R: 255, G: 220, B: 0, A: 255}
	colorGreen  = sdl.Color{R: 60, G: 210, B: 60, A: 255}
	colorOrange = sdl.Color{R: 255, G: 140, B: 0, A: 255}
	colorGray   = sdl.Color{R: 90, G: 90, B: 90, A: 255}
	colorDim    = sdl.Color{R: 40, G: 40, B: 40, A: 255}
)

// drawOutlineCircle draws a thin ring of the given colour at radius+3 around
// a dot centre (used to show selection and result).
func drawOutlineCircle(screen *apparatus.Screen, cx, cy, r float32, color sdl.Color) {
	or_ := r + 3
	_ = screen.Renderer.SetDrawColor(color.R, color.G, color.B, color.A)
	sdlCX, sdlCY := screen.CenterToSDL(cx, cy)
	steps := int(2*math.Pi*float64(or_)) + 1
	for i := 0; i <= steps; i++ {
		angle := float64(i) / float64(steps) * 2 * math.Pi
		px := sdlCX + or_*float32(math.Cos(angle))
		py := sdlCY - or_*float32(math.Sin(angle)) // SDL y is inverted
		_ = screen.Renderer.RenderPoint(px, py)
	}
}

// drawDots renders all circles in colours appropriate for the current phase.
func drawDots(screen *apparatus.Screen, dots []dot, phase int) {
	for i := range dots {
		d := &dots[i]
		d.circ.SetPosition(sdl.FPoint{X: d.x, Y: d.y})

		switch phase {
		case phaseHighlight:
			if d.isTarget {
				d.circ.Color = colorRed
			} else {
				d.circ.Color = colorWhite
			}
		case phaseMotion:
			d.circ.Color = colorWhite
		case phaseResponse:
			if d.selected {
				d.circ.Color = colorYellow
			} else {
				d.circ.Color = colorGray
			}
		case phaseResult:
			switch {
			case d.selected && d.isTarget:
				d.circ.Color = colorGreen
			case d.selected && !d.isTarget:
				d.circ.Color = colorRed
			case !d.selected && d.isTarget:
				d.circ.Color = colorOrange
			default:
				d.circ.Color = colorDim
			}
		}

		_ = d.circ.Draw(screen)

		// Selection ring during response phase.
		if phase == phaseResponse && d.selected {
			drawOutlineCircle(screen, d.x, d.y, circleR, colorYellow)
		}
	}
}

// ---------------------------------------------------------------------------
// HUD text
// ---------------------------------------------------------------------------

type cachedText struct {
	tl   *stimuli.TextLine
	text string
}

func (ct *cachedText) draw(screen *apparatus.Screen, s string, x, y float32, col sdl.Color) {
	if s != ct.text || ct.tl == nil {
		if ct.tl != nil {
			_ = ct.tl.Unload()
		}
		ct.tl = stimuli.NewTextLine(s, x, y, col)
		ct.text = s
	}
	_ = ct.tl.Draw(screen)
}
func (ct *cachedText) free() {
	if ct.tl != nil {
		_ = ct.tl.Unload()
		ct.tl = nil
	}
}

// ---------------------------------------------------------------------------
// Single trial
// ---------------------------------------------------------------------------

func runTrial(exp *control.Experiment, trialID, nTargets int) error {
	screen := exp.Screen
	sh := float32(screen.Height)

	containerR := sh / 3
	bounceR := containerR - circleR
	dots := initDots(nTargets, bounceR)

	container := stimuli.NewCircle(containerR, control.Black)
	container.SetPosition(sdl.FPoint{X: 0, Y: 0})

	// HUD text (cached).
	var hud cachedText
	defer hud.free()

	hudY := sh/2 - hudH/2 // centre of the HUD strip (center-relative coords)

	phase := phaseHighlight
	phaseStart := clock.GetTime()
	nSelected := 0

	// Disable GC during the tight animation phases.
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	// Drain stale events.
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
	}

	lastTime := time.Now()

	for {
		now := time.Now()
		dt := float32(now.Sub(lastTime).Seconds())
		if dt > float32(0.05) {
			dt = float32(0.05)
		}
		lastTime = now

		elapsed := clock.GetTime() - phaseStart

		// ── Phase transitions ───────────────────────────────────────────────
		switch phase {
		case phaseHighlight:
			if elapsed >= phase1Ms {
				phase = phaseMotion
				phaseStart = clock.GetTime()
			}
		case phaseMotion:
			if elapsed >= phase2Ms {
				phase = phaseResponse
				phaseStart = clock.GetTime()
				// Stop all circles.
				for i := range dots {
					dots[i].vx, dots[i].vy = 0, 0
				}
			}
		case phaseResult:
			if elapsed >= resultMs {
				return nil // trial complete
			}
		}

		// ── Physics (motion phase only) ─────────────────────────────────────
		if phase == phaseMotion {
			for i := range dots {
				dots[i].x += dots[i].vx * dt
				dots[i].y += dots[i].vy * dt
				bounceCircular(&dots[i], bounceR)
			}
			resolveCollisions(dots)
		}

		// ── Render ──────────────────────────────────────────────────────────
		_ = screen.Clear()

		// Draw black container disk.
		_ = container.Draw(screen)

		drawDots(screen, dots, phase)

		// HUD text.
		var hudText string
		switch phase {
		case phaseHighlight:
			remaining := (phase1Ms - elapsed) / 1000
			hudText = fmt.Sprintf(
				"Trial %d  —  Remember the %d RED circles!  (%d s)",
				trialID, nTargets, remaining+1,
			)
		case phaseMotion:
			remaining := (phase2Ms - elapsed) / 1000
			hudText = fmt.Sprintf(
				"Trial %d  —  Track the targets…  (%d s remaining)",
				trialID, remaining+1,
			)
		case phaseResponse:
			hudText = fmt.Sprintf(
				"Trial %d  —  Click %d circles you tracked  (%d/%d selected)",
				trialID, nTargets, nSelected, nTargets,
			)
		case phaseResult:
			score := 0
			for i := range dots {
				if dots[i].selected && dots[i].isTarget {
					score++
				}
			}
			hudText = fmt.Sprintf(
				"Trial %d  —  Score: %d / %d     Green=correct  Red=wrong  Orange=missed",
				trialID, score, nTargets,
			)
		}
		hud.draw(screen, hudText, 0, hudY, colorWhite)

		_ = screen.Update()

		// ── Events ──────────────────────────────────────────────────────────
		quit := false
		clickX, clickY := float32(0), float32(0)
		clicked := false

		state := exp.PollEvents(func(e sdl.Event) bool {
			if e.Type == sdl.EVENT_MOUSE_BUTTON_DOWN &&
				e.MouseButtonEvent().Button == 1 {
				// Use screen.MousePosition() for correct HiDPI / logical-size
				// coordinate conversion (mirrors CenterToSDL inverse).
				clickX, clickY = screen.MousePosition()
				clicked = true
			}
			return false
		})
		if state.QuitRequested {
			quit = true
		}

		if quit {
			return control.EndLoop
		}

		// ── Response handling ────────────────────────────────────────────────
		if phase == phaseResponse && clicked {
			for i := range dots {
				dx := clickX - dots[i].x
				dy := clickY - dots[i].y
				if dx*dx+dy*dy <= circleR*circleR {
					if dots[i].selected {
						dots[i].selected = false
						nSelected--
					} else if nSelected < nTargets {
						dots[i].selected = true
						nSelected++
					}
					break
				}
			}

			// Auto-advance when exactly N circles are selected.
			if nSelected == nTargets {
				// Record this trial's data.
				score := 0
				for i := range dots {
					if dots[i].selected && dots[i].isTarget {
						score++
					}
				}
				for i := range dots {
					isTarget := 0
					if dots[i].isTarget {
						isTarget = 1
					}
					wasSelected := 0
					if dots[i].selected {
						wasSelected = 1
					}
					exp.Data.Add(
						trialID,
						nTargets,
						i+1,
						isTarget,
						wasSelected,
						score,
						clock.GetTime(),
					)
				}
				phase = phaseResult
				phaseStart = clock.GetTime()
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Instructions screen
// ---------------------------------------------------------------------------

func showInstructions(exp *control.Experiment, totalTrials int) error {
	text := fmt.Sprintf(
		"Multiple Object Tracking\n\n"+
			"You will see %d trials.\n\n"+
			"Phase 1: Some circles will turn RED — memorise them!\n"+
			"Phase 2: All circles turn white and start moving.\n"+
			"         Keep tracking the ones that were red.\n"+
			"Phase 3: All circles stop. Click the ones you tracked.\n\n"+
			"Use the LEFT mouse button to select / deselect circles.\n"+
			"Results are shown briefly after each trial.\n\n"+
			"Press any key to begin.  ESC to quit.",
		totalTrials,
	)
	box := stimuli.NewTextBox(text, 1600, control.FPoint{X: 0, Y: 0}, control.White)
	_ = exp.Screen.Clear()
	_ = box.Draw(exp.Screen)
	_ = exp.Screen.Update()
	_, err := exp.Keyboard.Wait()
	return err
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	speed := flag.Float64("speed", 100, "Circle speed in px/s")
	exp := control.NewExperimentFromFlags("MOT", control.Gray, control.White, 32)
	defer exp.End()

	circleSpeed = float32(*speed)

	exp.AddDataVariableNames([]string{
		"trial_id", "n_targets", "circle_index",
		"is_target", "was_selected", "score", "timestamp_ms",
	})

	// Build trial list: 2 repetitions × {4,5,6,7} in random order.
	nList := []int{2, 3, 4, 5, 6, 2, 3, 4, 5, 6}
	rand.Shuffle(len(nList), func(i, j int) { nList[i], nList[j] = nList[j], nList[i] })

	if err := exp.Run(func() error {
		if err := showInstructions(exp, len(nList)); err != nil {
			return err
		}

		for trialIdx, n := range nList {
			if err := runTrial(exp, trialIdx+1, n); err != nil {
				return err
			}
		}
		return control.EndLoop
	}); err != nil && !control.IsEndLoop(err) {
		log.Fatalf("experiment: %v", err)
	}
}
