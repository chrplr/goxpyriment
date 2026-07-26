// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// test_images loads every PNG file in the current directory and displays each
// one, centered on the screen, for one second, using goxpyriment/stimuli.
//
// Usage:
//
//	go run main.go -w
//
// Flags: -w windowed mode, -d N display index, -s N subject ID.
package main

import (
	"log"
	"path/filepath"
	"sort"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	exp := control.NewExperimentFromFlags(
		"Image Display Test",
		control.Gray, control.White, 32,
	)
	defer exp.End()

	// Discover the PNG files in the current working directory.
	paths, err := filepath.Glob("pictures/*.png")
	if err != nil {
		log.Fatalf("cannot scan for PNG files: %v", err)
	}
	if len(paths) == 0 {
		log.Fatal("no PNG files found in the current directory")
	}
	sort.Strings(paths)

	// Build and preload one Picture per file, centered at the screen origin.
	// PreloadVisualOnScreen returns an error if the file cannot be decoded or
	// the texture cannot be created (e.g. the image exceeds the GPU's maximum
	// texture size) — fail loudly rather than silently showing a black screen.
	pictures := make([]stimuli.VisualStimulus, 0, len(paths))
	for _, p := range paths {
		pic := stimuli.NewPicture(p, 0, 0)
		if err := stimuli.PreloadVisualOnScreen(exp.Screen, pic); err != nil {
			log.Fatalf("cannot load %s: %v", p, err)
		}
		log.Printf("loaded %s (%.0fx%.0f px)", p, pic.Width, pic.Height)
		pictures = append(pictures, pic)
	}

	exp.Run(func() error {
		for i, pic := range pictures {
			log.Printf("showing %s", paths[i])
			if err := exp.ShowTimed(pic, 1000); err != nil {
				return err
			}
		}
		return control.EndLoop
	})
}
