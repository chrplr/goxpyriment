// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// demo_spritesheet — cut one image into a grid of stimuli and draw them by
// source rectangle, the idiom Psychtoolbox users know as
// Screen('DrawTexture', win, tex, sourceRect).
//
// The eight cells of assets/sprites.png share a single GPU texture: the sheet
// is decoded and uploaded once, and each sprite is drawn by selecting its
// rectangle within it. The top row shows all eight cells at once; below them,
// one sprite cycles through the same eight clips, which is the animation the
// sheet was drawn for.
//
// Run from the repo root:  go run ./examples/demo_spritesheet -w
package main

import (
	_ "embed"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// The sheet is embedded rather than read from disk: the browser build has no
// filesystem, and one embedded file is all a whole condition needs.
//
//go:embed assets/sprites.png
var sheetPNG []byte

const (
	cols       = 4
	rows       = 2
	frameHold  = 8 // frames each animation step is held, at ~60 Hz
	rowSpacing = 150
)

func main() {
	exp := control.NewExperimentFromFlags("Sprite sheet demo", control.Black, control.White, 24)
	defer exp.End()

	sheet := stimuli.NewSpriteSheetFromMemory(sheetPNG)
	defer sheet.Unload() // the sheet owns the texture; the sprites do not

	var sprites []*stimuli.Sprite
	label := stimuli.NewTextLine("one texture, eight source rectangles — any key to quit",
		0, -260, control.White)
	tick := 0

	err := exp.Run(func() error {
		// Grid uploads the sheet on first call, so it must run on the SDL
		// main thread — inside Run, like every other drawing call.
		if sprites == nil {
			var err error
			sprites, err = sheet.Grid(exp.Screen, cols, rows)
			if err != nil {
				return err
			}
			for i, sp := range sprites {
				sp.SetPosition(control.Point(
					float32(i-len(sprites)/2)*110+55, rowSpacing))
			}
		}

		if err := exp.Screen.Clear(); err != nil {
			return err
		}
		for _, sp := range sprites {
			if err := sp.Draw(exp.Screen); err != nil {
				return err
			}
		}

		// The animation: the same sprite list, one cell at a time, drawn twice
		// the size to show that the clip is independent of the destination.
		current := sprites[(tick/frameHold)%len(sprites)]
		saved := current.GetPosition()
		w, h := current.Width, current.Height
		current.SetPosition(control.Point(0, -60))
		current.Width, current.Height = w*2, h*2
		err := current.Draw(exp.Screen)
		current.SetPosition(saved)
		current.Width, current.Height = w, h
		if err != nil {
			return err
		}

		if err := label.Draw(exp.Screen); err != nil {
			return err
		}
		if err := exp.Screen.Update(); err != nil {
			return err
		}

		tick++
		// Poll with a 1 ms budget, not 0: a zero timeout returns before the
		// queue is looked at even once, so no key would ever be seen.
		key, _, kerr := exp.Keyboard.GetKeyEventTS(nil, 1)
		if kerr != nil {
			return kerr // ESC or quit; Run recovers the sentinel
		}
		if key != 0 {
			return control.EndLoop
		}
		return nil
	})
	if err != nil && !control.IsEndLoop(err) {
		log.Fatalf("demo_spritesheet: %v", err)
	}
}
