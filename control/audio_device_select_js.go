// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

//go:build js

package control

import (
	"errors"

	"github.com/chrplr/goxpyriment/stimuli"
)

// SelectAudioRecordingDevice is not supported on JS/WASM.
func (e *Experiment) SelectAudioRecordingDevice(title string) (stimuli.RecordingDevice, error) {
	return stimuli.RecordingDevice{}, errors.New("control: SelectAudioRecordingDevice is not supported in the JS/WASM build")
}
