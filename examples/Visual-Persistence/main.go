// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Visual Persistence Illusion
//
// Reproduces the "Persistence of Vision" illusion from http://TestUFO.com/persistence.
//
// A source image is shown only through thin vertical slits on a black background.
//   - Stare at the fixation cross: you see only disconnected vertical strips.
//   - Track the yellow dot with your eyes: temporal integration reveals the full image.
//
// Controls:
//
//	↑ / ↓      — scroll speed  +/-50 px/s
//	+ / -      — slit width    +/- 1 px
//	[ / ]      — slit gap      -/+ 4 px
//	F          — toggle show-full mode (bypass slits)
//	ESC        — quit
//
// Usage:
//
//	go run main.go [-w] [-d N] [-image <path>] [-speed <px/s>] [-slit-width <px>] [-slit-gap <px>]
package main

import (
	"flag"
	"fmt"
	"math"
	"runtime/debug"
	"time"

	"github.com/Zyko0/go-sdl3/img"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

const (
	speedStep = float32(50)
	minSpeed  = float32(50)
	maxSpeed  = float32(1000)
	minSlitW  = float32(1)
	maxSlitW  = float32(30)
	minGap    = float32(4)
	maxGap    = float32(200)
	dotRadius = float32(10)
)

func main() {
	imagePath := flag.String("image", "", "Image file to show through slits (default: built-in image)")
	initSpeed := flag.Float64("speed", 300, "Initial scroll speed in pixels/second")
	initSlitW := flag.Float64("slit-width", 4, "Initial slit width in pixels")
	initSlitGap := flag.Float64("slit-gap", 36, "Initial gap between slits in pixels")

	exp := control.NewExperimentFromFlags("Visual Persistence Illusion", control.Black, control.White, 20)
	defer exp.End()

	if err := exp.Run(func() error {
		return animLoop(exp, *imagePath,
			float32(*initSpeed), float32(*initSlitW), float32(*initSlitGap))
	}); err != nil && !control.IsEndLoop(err) {
		exp.Fatal("run: %v", err)
	}
}

func animLoop(exp *control.Experiment, imagePath string, speed, slitW, slitGap float32) error {
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	screen := exp.Screen
	sw := float32(screen.Width)
	sh := float32(screen.Height)

	var (
		imgTex *sdl.Texture
		err    error
	)
	if imagePath != "" {
		imgTex, err = loadAndScaleImage(screen, imagePath, int(sw), int(sh))
	} else {
		imgTex, err = buildDefaultImage(screen, int(sw), int(sh))
	}
	if err != nil {
		return fmt.Errorf("source image: %w", err)
	}
	defer imgTex.Destroy()

	imageX := float32(0) // left edge of image in SDL x coords, in [0, sw)
	showFull := false

	// HUD: lazy text cache to avoid recreating textures every frame
	hudColor := sdl.Color{R: 150, G: 150, B: 150, A: 255}
	hudY := sh/2 - 12 // center-relative y near the top
	var hudText string
	var hudLine *stimuli.TextLine
	defer func() {
		if hudLine != nil {
			_ = hudLine.Unload()
		}
	}()

	// Fixation cross for the "stare" condition
	fix := stimuli.NewFixCross(18, 2, control.White)
	fix.SetPosition(sdl.FPoint{X: 0, Y: 0})

	// Drain stale events before starting the loop
	var ev sdl.Event
	for sdl.PollEvent(&ev) {
	}

	lastTime := time.Now()

	for {
		now := time.Now()
		dt := float32(now.Sub(lastTime).Seconds())
		if dt > 0.05 {
			dt = 0.05
		}
		lastTime = now

		// Advance image offset; keep in [0, sw) to avoid float32 drift
		imageX = float32(math.Mod(float64(imageX+speed*dt), float64(sw)))

		_ = screen.Clear()

		if showFull {
			dstRect := sdl.FRect{X: 0, Y: 0, W: sw, H: sh}
			_ = screen.Renderer.RenderTexture(imgTex, nil, &dstRect)
		} else {
			// Draw only the visible slit strips
			period := slitW + slitGap
			srcRect := sdl.FRect{Y: 0, H: sh}
			dstRect := sdl.FRect{Y: 0, H: sh}

			for slitX := float32(0); slitX < sw; slitX += period {
				// Map slit screen position to image source position (with tiling)
				srcX := float32(math.Mod(float64(slitX-imageX), float64(sw)))
				if srcX < 0 {
					srcX += sw
				}
				// Skip any slit that straddles the image wrap boundary
				if srcX+slitW > sw {
					continue
				}
				// Clip slit to screen right edge
				w := slitW
				if slitX+w > sw {
					w = sw - slitX
				}
				srcRect.X, srcRect.W = srcX, w
				dstRect.X, dstRect.W = slitX, w
				_ = screen.Renderer.RenderTexture(imgTex, &srcRect, &dstRect)
			}

			// Fixation cross: staring at it produces the fragmented (non-illusory) percept
			_ = fix.Draw(screen)
		}

		// Tracking dot: moves at the same speed as the image.
		// When the user tracks this dot, temporal integration reveals the hidden image.
		dotSDLX := imageX
		dotSDLY := sh / 2 // vertical center
		_ = screen.Renderer.SetDrawColor(255, 215, 0, 255) // gold
		drawFilledCircle(screen.Renderer, dotSDLX, dotSDLY, dotRadius)

		// HUD
		newHUD := fmt.Sprintf(
			"Speed: %.0f [↑/↓]   Slit: %.0f [+/-]   Gap: %.0f [[ / ]]   Full image [F]   Quit [ESC]",
			speed, slitW, slitGap)
		if newHUD != hudText {
			if hudLine != nil {
				_ = hudLine.Unload()
			}
			hudLine = stimuli.NewTextLine(newHUD, 0, hudY, hudColor)
			hudText = newHUD
		}
		_ = hudLine.Draw(screen)

		_ = screen.PacedFlip()

		// Input handling
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
			case sdl.K_EQUALS, sdl.K_KP_PLUS:
				slitW++
				if slitW > maxSlitW {
					slitW = maxSlitW
				}
			case sdl.K_MINUS, sdl.K_KP_MINUS:
				slitW--
				if slitW < minSlitW {
					slitW = minSlitW
				}
			case sdl.K_RIGHTBRACKET:
				slitGap += 4
				if slitGap > maxGap {
					slitGap = maxGap
				}
			case sdl.K_LEFTBRACKET:
				slitGap -= 4
				if slitGap < minGap {
					slitGap = minGap
				}
			case sdl.K_F:
				showFull = !showFull
			}
			return false
		})

		if state.QuitRequested {
			return control.EndLoop
		}
	}
}

// drawFilledCircle draws a filled circle at SDL coordinates (cx, cy) with the given radius.
func drawFilledCircle(r *sdl.Renderer, cx, cy, radius float32) {
	for y := -radius; y <= radius; y++ {
		dx := float32(math.Sqrt(float64(radius*radius - y*y)))
		_ = r.RenderLine(cx-dx, cy+y, cx+dx, cy+y)
	}
}

// loadAndScaleImage loads an image file and scales it to (w, h) in a render-target texture.
func loadAndScaleImage(screen *apparatus.Screen, path string, w, h int) (*sdl.Texture, error) {
	surface, err := img.Load(path)
	if err != nil {
		return nil, err
	}
	defer surface.Destroy()

	srcTex, err := screen.Renderer.CreateTextureFromSurface(surface)
	if err != nil {
		return nil, err
	}
	defer srcTex.Destroy()

	dstTex, err := screen.Renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_TARGET, w, h)
	if err != nil {
		return nil, err
	}

	if err := screen.Renderer.SetRenderTarget(dstTex); err != nil {
		dstTex.Destroy()
		return nil, err
	}
	defer screen.Renderer.SetRenderTarget(nil)

	dstRect := sdl.FRect{X: 0, Y: 0, W: float32(w), H: float32(h)}
	if err := screen.Renderer.RenderTexture(srcTex, nil, &dstRect); err != nil {
		_ = screen.Renderer.SetRenderTarget(nil)
		dstTex.Destroy()
		return nil, err
	}

	return dstTex, nil
}

// buildDefaultImage generates a high-contrast source texture when no image file is given.
func buildDefaultImage(screen *apparatus.Screen, w, h int) (*sdl.Texture, error) {
	tex, err := screen.Renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_TARGET, w, h)
	if err != nil {
		return nil, err
	}

	if err := screen.Renderer.SetRenderTarget(tex); err != nil {
		tex.Destroy()
		return nil, err
	}
	defer screen.Renderer.SetRenderTarget(nil)

	fw, fh := float32(w), float32(h)

	// Dark blue background
	_ = screen.Renderer.SetDrawColor(15, 15, 50, 255)
	_ = screen.Renderer.Clear()

	// Colorful horizontal bands occupying the central 40% of the image height
	bands := []sdl.Color{
		{R: 200, G: 40, B: 40, A: 255},
		{R: 200, G: 120, B: 20, A: 255},
		{R: 200, G: 200, B: 20, A: 255},
		{R: 40, G: 180, B: 40, A: 255},
		{R: 20, G: 180, B: 200, A: 255},
		{R: 60, G: 60, B: 200, A: 255},
		{R: 160, G: 40, B: 200, A: 255},
	}
	topY := fh * 0.30
	bandH := fh * 0.40 / float32(len(bands))
	for i, c := range bands {
		_ = screen.Renderer.SetDrawColor(c.R, c.G, c.B, c.A)
		rect := sdl.FRect{X: 0, Y: topY + float32(i)*bandH, W: fw, H: bandH}
		_ = screen.Renderer.RenderFillRect(&rect)
	}

	// White border framing the bands
	_ = screen.Renderer.SetDrawColor(255, 255, 255, 255)
	border := sdl.FRect{X: 8, Y: topY - 8, W: fw - 16, H: float32(len(bands))*bandH + 16}
	_ = screen.Renderer.RenderRect(&border)

	// Text overlay — temporarily redirect center-relative coordinates into the texture
	oldOffset := screen.CanvasOffset
	screen.CanvasOffset = &sdl.FPoint{X: fw / 2, Y: fh / 2}
	defer func() { screen.CanvasOffset = oldOffset }()

	bigFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 56)
	if err == nil {
		defer bigFont.Close()
		t1 := stimuli.NewTextLine("VISUAL PERSISTENCE", 0, 36, control.White)
		t1.Font = bigFont
		_ = t1.Draw(screen)
		_ = t1.Unload()

		t2 := stimuli.NewTextLine("ILLUSION", 0, -36, control.White)
		t2.Font = bigFont
		_ = t2.Draw(screen)
		_ = t2.Unload()
	}

	smFont, err := control.FontFromMemory(assets_embed.InconsolataFont, 22)
	if err == nil {
		defer smFont.Close()
		hint := stimuli.NewTextLine("← track the dot with your eyes →", 0, -(fh/2 - 22),
			sdl.Color{R: 220, G: 220, B: 60, A: 255})
		hint.Font = smFont
		_ = hint.Draw(screen)
		_ = hint.Unload()
	}

	return tex, nil
}
