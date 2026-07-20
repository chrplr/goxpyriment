// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// test_images_embedded displays every PNG file in the pictures/ folder, each
// centered on the screen for one second, using goxpyriment/stimuli.
//
// The images are embedded into the binary at compile time via //go:embed, so
// the resulting executable is fully self-contained and can be distributed
// without the pictures/ folder.
//
// Usage:
//
//	go run main.go -w
//
// Flags: -w windowed mode, -d N display index, -s N subject ID.
package main

import (
	"embed"
	"log"
	"sort"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// pictures holds the embedded image files. The //go:embed directive bundles
// every PNG under pictures/ into the binary at compile time.
//
//go:embed pictures/*.png
var pictures embed.FS

func main() {
	exp := control.NewExperimentFromFlags(
		"Image Display Test",
		control.Gray, control.White, 32,
	)
	defer exp.End()

	// Enumerate the embedded PNG files.
	entries, err := pictures.ReadDir("pictures")
	if err != nil {
		log.Fatalf("cannot read embedded pictures: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		log.Fatal("no embedded PNG files found")
	}
	sort.Strings(names)

	// Build and preload one Picture per embedded file, centered at the screen
	// origin. PreloadVisualOnScreen returns an error if the data cannot be
	// decoded or the texture cannot be created (e.g. the image exceeds the
	// GPU's maximum texture size) — fail loudly rather than silently showing a
	// black screen.
	stims := make([]stimuli.VisualStimulus, 0, len(names))
	for _, name := range names {
		data, err := pictures.ReadFile("pictures/" + name)
		if err != nil {
			log.Fatalf("cannot read embedded %s: %v", name, err)
		}
		pic := stimuli.NewPictureFromMemory(data, 0, 0)
		if err := stimuli.PreloadVisualOnScreen(exp.Screen, pic); err != nil {
			log.Fatalf("cannot load %s: %v", name, err)
		}
		log.Printf("loaded %s (%.0fx%.0f px)", name, pic.Width, pic.Height)
		stims = append(stims, pic)
	}

	exp.Run(func() error {
		for i, pic := range stims {
			log.Printf("showing %s", names[i])
			if err := exp.ShowTimed(pic, 1000); err != nil {
				return err
			}
		}
		return control.EndLoop
	})
}
