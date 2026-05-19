// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-developed by Álvaro Cabana <almadana@gmail.com> with Cursor (2026).
// Distributed under the GNU General Public License v3.

//go:build !js

package control

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/stimuli"
)

// SelectAudioRecordingDevice shows a numbered menu of available microphones
// and blocks until the participant confirms a choice.
//
// title is shown above the list (e.g. "Select microphone"). Navigation matches
// [stimuli.Menu.Get]: UP/DOWN, ENTER/SPACE, digit keys 1–9, ESC to cancel.
//
// When SDL enumerates at least one physical device, the first enumerated device
// (not "System default") is highlighted initially.
func (e *Experiment) SelectAudioRecordingDevice(title string) (stimuli.RecordingDevice, error) {
	devices, err := stimuli.ListRecordingDevices()
	if err != nil {
		return stimuli.RecordingDevice{}, err
	}
	if len(devices) == 0 {
		return stimuli.RecordingDevice{}, fmt.Errorf("control: no audio recording devices available")
	}

	labels := make([]string, len(devices))
	for i, d := range devices {
		labels[i] = d.Name
	}

	caption := title
	if caption == "" {
		caption = "Select microphone"
	}
	caption += "\n\n↑↓ move   ENTER confirm   1–9 quick pick   ESC cancel"

	menu := stimuli.NewMenu(labels)
	menu.Caption = caption
	menu.TextColor = sdl.Color{R: 170, G: 170, B: 170, A: 255}
	menu.HighlightColor = e.ForegroundColor
	menu.Pos = Point(0, -40)

	initialSel := 0
	if len(devices) > 1 {
		initialSel = 1
	}

	idx, err := menu.Get(e.Screen, e.Keyboard, initialSel)
	if err != nil {
		return stimuli.RecordingDevice{}, err
	}
	if idx < 0 || idx >= len(devices) {
		return stimuli.RecordingDevice{}, fmt.Errorf("control: recording device selection cancelled")
	}
	return devices[idx], nil
}
