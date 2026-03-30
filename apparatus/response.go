// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package apparatus

import (
	"context"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

// Response is a single input event from any response device.
//
// The Source field identifies which device produced the event. The Code field
// carries a device-specific value:
//   - DeviceKeyboard: SDL keycode (sdl.Keycode cast to uint32)
//   - DeviceMouse:    SDL button index (sdl.BUTTON_LEFT etc.)
//   - DeviceGamepad:  SDL gamepad button (sdl.GamepadButton cast to uint32)
//   - DeviceTTL:      8-bit bitmask of active input lines (bit N = line N)
//
// RT is the elapsed time from the [ResponseDevice.WaitResponse] call to the
// moment the input was detected. Its precision depends on the device:
//   - Precise == true:  RT derived from an SDL3 hardware event timestamp
//     (nanosecond resolution, same clock as Screen.FlipNS). Use this for
//     stimulus-onset-locked reaction time measurements.
//   - Precise == false: RT measured with time.Now() at detection (software
//     poll). Accuracy is limited by the device's poll interval (~5 ms).
type Response struct {
	Source  DeviceKind
	Code    uint32
	RT      time.Duration
	Precise bool
}

// ResponseDevice is the common interface for all participant-input devices.
// It abstracts over SDL-event-driven devices (keyboard, mouse, gamepad) and
// polled TTL devices (MEGTTLBox, DLPIO8).
//
// Implementations:
//   - [KeyboardResponseDevice] — wraps a [Keyboard]
//   - [MouseResponseDevice]    — wraps a [Mouse]
//   - [GamepadResponseDevice]  — wraps a [GamePad]
//   - [TTLResponseDevice]      — wraps any TTL input device (via structural typing)
type ResponseDevice interface {
	// WaitResponse blocks until a response is detected or ctx is cancelled.
	// RT in the returned [Response] is always measured from the call;
	// check Response.Precise to know whether it came from a hardware event
	// timestamp or a software poll.
	WaitResponse(ctx context.Context) (Response, error)

	// DrainResponses discards any pending or latched inputs. Call this
	// before [WaitResponse] to avoid processing stale events from a previous
	// trial.
	DrainResponses(ctx context.Context) error

	// Close releases any device-specific resources. For SDL wrappers this is
	// a no-op; the underlying device manages its own lifecycle.
	Close() error
}

// ── SDL-based response devices ──────────────────────────────────────────────

// KeyboardResponseDevice wraps a [Keyboard] as a [ResponseDevice].
// RT is derived from the SDL3 hardware event timestamp (Precise: true).
type KeyboardResponseDevice struct {
	KB *Keyboard
}

// WaitResponse blocks until any key is pressed or ctx is cancelled.
// It polls in 50 ms slices so that context cancellation is checked regularly.
func (d *KeyboardResponseDevice) WaitResponse(ctx context.Context) (Response, error) {
	startNS := sdl.TicksNS()
	const sliceMS = 50
	for {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		key, ts, err := d.KB.WaitKeysEventRT(nil, sliceMS)
		if err != nil {
			return Response{}, err
		}
		if key != 0 {
			var rt time.Duration
			if ts >= startNS {
				rt = time.Duration(ts - startNS)
			}
			return Response{
				Source:  DeviceKeyboard,
				Code:    uint32(key),
				RT:      rt,
				Precise: true,
			}, nil
		}
		// Slice timed out — loop and re-check ctx.
	}
}

// DrainResponses clears all pending SDL events.
func (d *KeyboardResponseDevice) DrainResponses(_ context.Context) error {
	d.KB.Clear()
	return nil
}

// Close is a no-op; the Keyboard lifecycle is managed by the caller.
func (d *KeyboardResponseDevice) Close() error { return nil }

// MouseResponseDevice wraps a [Mouse] as a [ResponseDevice].
// RT is derived from the SDL3 hardware event timestamp (Precise: true).
type MouseResponseDevice struct {
	M *Mouse
}

// WaitResponse blocks until any mouse button is pressed or ctx is cancelled.
func (d *MouseResponseDevice) WaitResponse(ctx context.Context) (Response, error) {
	startNS := sdl.TicksNS()
	const sliceMS = 50
	for {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		btn, ts, err := d.M.WaitPressEventRT(sliceMS)
		if err != nil {
			return Response{}, err
		}
		if btn != 0 {
			var rt time.Duration
			if ts >= startNS {
				rt = time.Duration(ts - startNS)
			}
			return Response{
				Source:  DeviceMouse,
				Code:    btn,
				RT:      rt,
				Precise: true,
			}, nil
		}
	}
}

// DrainResponses clears pending SDL events.
func (d *MouseResponseDevice) DrainResponses(_ context.Context) error {
	var event sdl.Event
	for sdl.PollEvent(&event) {
	}
	return nil
}

// Close is a no-op.
func (d *MouseResponseDevice) Close() error { return nil }

// GamepadResponseDevice wraps a [GamePad] as a [ResponseDevice].
// RT is derived from the SDL3 hardware event timestamp (Precise: true).
type GamepadResponseDevice struct {
	GP *GamePad
}

// WaitResponse blocks until any gamepad button is pressed or ctx is cancelled.
func (d *GamepadResponseDevice) WaitResponse(ctx context.Context) (Response, error) {
	startNS := sdl.TicksNS()
	const sliceMS = 50
	for {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		btn, ts, err := d.GP.WaitPressEventRT(sliceMS)
		if err != nil {
			return Response{}, err
		}
		if btn != 0 {
			var rt time.Duration
			if ts >= startNS {
				rt = time.Duration(ts - startNS)
			}
			return Response{
				Source:  DeviceGamepad,
				Code:    uint32(btn),
				RT:      rt,
				Precise: true,
			}, nil
		}
	}
}

// DrainResponses clears pending SDL events.
func (d *GamepadResponseDevice) DrainResponses(_ context.Context) error {
	var event sdl.Event
	for sdl.PollEvent(&event) {
	}
	return nil
}

// Close is a no-op.
func (d *GamepadResponseDevice) Close() error { return nil }

// ── TTL response device adapter ─────────────────────────────────────────────

// ttlInputSource is satisfied by any TTL input device without importing the
// triggers package, using Go structural typing.
type ttlInputSource interface {
	ReadAll() (byte, error)
	DrainInputs(ctx context.Context) error
}

// TTLResponseDevice adapts any TTL input device (e.g. MEGTTLBox, DLPIO8) to
// the [ResponseDevice] interface. RT is measured with time.Now() at detection
// (Precise: false), accurate to within one poll interval.
//
// Construct with [NewTTLResponseDevice]; pass the concrete device directly:
//
//	box, _ := triggers.NewMEGTTLBox("/dev/ttyACM0")
//	rd := apparatus.NewTTLResponseDevice(box, 5*time.Millisecond)
type TTLResponseDevice struct {
	src          ttlInputSource
	pollInterval time.Duration
}

// NewTTLResponseDevice wraps a TTL input device as a [ResponseDevice].
// pollInterval controls how often the device is polled while waiting.
func NewTTLResponseDevice(src ttlInputSource, pollInterval time.Duration) *TTLResponseDevice {
	return &TTLResponseDevice{src: src, pollInterval: pollInterval}
}

// WaitResponse polls the TTL device until any input line becomes active or ctx
// is cancelled. RT is measured from the call (software poll, Precise: false).
func (d *TTLResponseDevice) WaitResponse(ctx context.Context) (Response, error) {
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		mask, err := d.src.ReadAll()
		if err != nil {
			return Response{}, err
		}
		if mask != 0 {
			return Response{
				Source:  DeviceTTL,
				Code:    uint32(mask),
				RT:      time.Since(start),
				Precise: false,
			}, nil
		}
		time.Sleep(d.pollInterval)
	}
}

// DrainResponses polls until all TTL input lines are inactive or ctx is done.
func (d *TTLResponseDevice) DrainResponses(ctx context.Context) error {
	return d.src.DrainInputs(ctx)
}

// Close is a no-op; the underlying TTL device manages its own lifecycle.
func (d *TTLResponseDevice) Close() error { return nil }
