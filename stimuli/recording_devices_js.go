// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build js

package stimuli

import (
	"errors"

	"github.com/Zyko0/go-sdl3/sdl"
)

// RecordingDevice describes one SDL3 audio input device available for capture.
type RecordingDevice struct {
	ID   sdl.AudioDeviceID
	Name string
}

// ListRecordingDevices is not supported on JS/WASM.
func ListRecordingDevices() ([]RecordingDevice, error) {
	return nil, errors.New("stimuli: ListRecordingDevices is not supported in the JS/WASM build")
}
