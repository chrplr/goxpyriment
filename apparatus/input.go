// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package apparatus

import "github.com/Zyko0/go-sdl3/sdl"

// DeviceKind identifies which input device generated an InputEvent.
type DeviceKind int

const (
	DeviceKeyboard DeviceKind = iota
	DeviceMouse
	DeviceGamepad
	DeviceTTL // TTL input device (MEGTTLBox, DLPIO8, etc.) — polled, not event-driven
)

// InputEvent is a unified representation of a single input event from any
// device (keyboard, mouse, or gamepad). It is returned by
// Experiment.WaitAnyEventTS so that callers can wait for the first response
// regardless of which device the participant uses.
//
// Inspect the Device field to determine which response fields are populated:
//
//	switch ev.Device {
//	case apparatus.DeviceKeyboard:
//	    // ev.Key contains the pressed key
//	case apparatus.DeviceMouse:
//	    // ev.Button contains the mouse button index (sdl.BUTTON_LEFT etc.)
//	case apparatus.DeviceGamepad:
//	    // ev.GamepadButton contains the gamepad button
//	}
//
// TimestampNS is always set to the SDL3 hardware event timestamp in
// nanoseconds (same clock as Screen.FlipTS and GetKeyEventTS), suitable
// for computing reaction time:
//
//	onset, _ := exp.ShowTS(stim)
//	ev, _ := exp.WaitAnyEventTS(keys, true, -1)
//	rtNS := int64(ev.TimestampNS - onset)
type InputEvent struct {
	Device        DeviceKind
	Key           sdl.Keycode       // non-zero when Device == DeviceKeyboard
	Button        uint32            // non-zero when Device == DeviceMouse
	GamepadButton sdl.GamepadButton // non-zero when Device == DeviceGamepad
	TimestampNS   uint64            // SDL3 hardware timestamp, nanoseconds
}
