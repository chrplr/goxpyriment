// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build !js

package stimuli

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
)

// RecordingDevice describes one SDL3 audio input device available for capture.
type RecordingDevice struct {
	ID   sdl.AudioDeviceID
	Name string
}

// ListRecordingDevices returns the system default entry followed by every
// device reported by [sdl.GetAudioRecordingDevices].
func ListRecordingDevices() ([]RecordingDevice, error) {
	list := []RecordingDevice{{
		ID:   sdl.AUDIO_DEVICE_DEFAULT_RECORDING,
		Name: "System default",
	}}

	ids, err := sdl.GetAudioRecordingDevices()
	if err != nil {
		return list, err
	}
	for _, id := range ids {
		name, err := id.Name()
		if err != nil || name == "" {
			name = fmt.Sprintf("Recording device %d", id)
		}
		list = append(list, RecordingDevice{ID: id, Name: name})
	}
	return list, nil
}
