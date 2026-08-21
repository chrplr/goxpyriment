// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// demo_getinfo demonstrates control.GetParticipantInfo: a graphical setup
// dialog that collects participant demographics and monitor characteristics
// before the experiment window is opened.
//
// Between them the fields cover all four widgets the dialog can draw -- text,
// number, select and checkbox -- so this is also what to run when checking the
// dialog itself on an unfamiliar display: a console with no window manager, a
// HiDPI panel. The select row is the one to click carefully there, since it is
// the only widget whose hit-testing divides a row between several targets.
//
// Usage:
//
//	go run ./examples/demo_getinfo
package main

import (
	"fmt"
	"log"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

func main() {
	// Build the field list: standard participant + monitor fields, a select
	// field, plus the fullscreen toggle.  Add or remove InfoField entries as
	// needed.
	//
	// A FieldSelect is a row of buttons rather than something to type in, for
	// a setting with a handful of known answers -- which ear, which hand, which
	// protocol table to run. Its value is the option itself, not an index.
	// Keep the list short: the options share one row, so each one drawn makes
	// them all narrower.
	fields := append(control.StandardFields,
		control.InfoField{
			Name:    "response_hand",
			Label:   "Responding hand",
			Type:    control.FieldSelect,
			Options: []string{"left", "right", "both"},
			Default: "right",
		},
		control.FullscreenField)

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
			"Responding hand: %s\n\n"+
			"Press any key to quit.",
		info["subject_id"], info["age"],
		info["gender"], info["handedness"],
		info["screen_width_cm"], info["viewing_distance_cm"],
		info["refresh_rate_hz"],
		info["response_hand"],
	)

	tb := stimuli.NewTextBox(msg, 800, control.Origin(), control.White)

	_ = exp.Run(func() error {
		exp.Show(tb)
		exp.Keyboard.Wait()
		return control.EndLoop
	})
}
