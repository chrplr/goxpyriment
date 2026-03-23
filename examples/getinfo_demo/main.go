// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

// getinfo_demo demonstrates control.GetParticipantInfo: a graphical setup
// dialog that collects participant demographics and monitor characteristics
// before the experiment window is opened.
//
// Usage:
//
//	go run examples/getinfo_demo/main.go
package main

import (
	"fmt"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	// Build the field list: standard participant + monitor fields, plus the
	// fullscreen toggle.  Add or remove InfoField entries as needed.
	fields := append(control.StandardFields, control.FullscreenField)

	info, err := control.GetParticipantInfo("Demo Experiment", fields)
	if err != nil {
		log.Fatalf("Setup cancelled: %v", err)
	}

	// Use the fullscreen checkbox to decide the window mode.
	fullscreen := info["fullscreen"] == "true"
	width, height := 0, 0
	if !fullscreen {
		width, height = 1024, 768
	}

	exp := control.NewExperiment("Demo Experiment", width, height, fullscreen,
		control.Black, control.White, 32)
	if err := exp.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer exp.End()

	// Store the collected info on the experiment for later access.
	exp.Info = info

	// Display a summary of what was entered.
	msg := fmt.Sprintf(
		"Subject: %s    Age: %s\nGender: %s    Handedness: %s\n\n"+
			"Screen: %s cm    Distance: %s cm\nRefresh: %s Hz\n\n"+
			"Press any key to quit.",
		info["subject_id"], info["age"],
		info["gender"], info["handedness"],
		info["screen_width_cm"], info["viewing_distance_cm"],
		info["refresh_rate_hz"],
	)

	tb := stimuli.NewTextBox(msg, 800, control.Origin(), control.White)

	_ = exp.Run(func() error {
		exp.Show(tb)
		exp.Keyboard.Wait()
		return control.EndLoop
	})
}
