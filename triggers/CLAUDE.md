# triggers package

Hardware TTL signal output (EEG/MEG trigger codes) and TTL input (response pads). Lines are **0-indexed (0–7)**; bit N of a bitmask corresponds to line N.

## Firing a trigger on a stimulus onset

Use **`FireTriggerSync`**, on the statement after the flip, with nothing in
between:

```go
flipTS, err := exp.ShowTS(stim)
triggers.FireTriggerSync(trig, pin, 5*time.Millisecond)
```

It raises on the calling goroutine and defers only the falling edge, which
carries no information. `FireTrigger` blocks for the whole pulse, so the only
way to call it from a frame loop is `go triggers.FireTrigger(...)` — and
dispatching the *raise* through a goroutine is what costs the measurement:

| how the rising edge is issued | gap from the flip |
|---|---|
| synchronously on the flip thread (`FireTriggerSync`) | **p50 34 µs, max 37 µs** at `SCHED_FIFO` 50 |
| through a goroutine (`go FireTrigger(...)`) | **+0.73 ms, ~1 ms spread** at normal priority under load |

`dur` must be shorter than the interval to the next call on the same device —
the implementations are not internally synchronised, so an overlapping raise and
lower would race on the port.

Neither call makes the edge coincide with the photons. `SDL_RenderPresent`
returns when the driver will accept the *next* frame, so the flip leads the
panel by one to three frames depending on the display stack (measured: kmsdrm
18.91 ms sd 0.113, Wayland 21.75 ms sd 1.344, bare Xorg 35.74 ms sd 0.083 at
60 Hz). That offset is constant per rig and is measured once, recorded and
subtracted in analysis — the library never adjusts a timestamp. See
[docs/TimingTests.md](../docs/TimingTests.md) and
[docs/TriggerJitterForEEGandMEG.md](../docs/TriggerJitterForEEGandMEG.md).

## Opening a device by name (open.go)

`triggers.Open(spec)` parses and opens a device named in one string. It is what
`-ttl` (control's onset marker) and `tests/internal/trigdev`'s `-device` use.
`DeviceSpecUsage` documents the syntax:

- **Format:** `KIND[:PORT][,key=value,...]`, or `KIND[:key=value,...]`.
- **First bare value:** the value right after the colon is the kind's address:
  the port, the host for labjackt4, the chip for gpio.
- **Auto-detection:** `dlpio8`, `dlpio20` and `parallel` auto-detect when no
  port is given or the port is `auto`.
- **Kinds:** dlpio8, dlpio20, megttlbox, mmbts, parallel, gpio, ft232h,
  labjackt4 and null. `parport` and `megttl` are accepted as aliases.
- **`pin=N`** (1-8) is accepted and validated, but `Open` ignores it: a device is
  opened whole.
- **Failure:** an open failure is an error. It never falls back to a null device.

For marking stimulus onsets in an experiment, prefer the library's onset marker
(`control.MarkerFlags`, `exp.Marker.Arm`) over calling the device yourself. It
does all of the following:
- writes the code right after the flip
- serialises the writes
- clears the lines on a timer
- counts failures
- draws the matching photodiode square

## Per-device notes

Wiring, protocol, timing measurements and quirks for each device are in
`triggers/notes/` — read the one for the device you are working on:

| Device | Notes | Source |
|---|---|---|
| DLP-IO8-G (USB-CDC) | `notes/dlpio8.md` | `dlpio8.go` |
| DLP-IO20 (USB-CDC) | `notes/dlpio20.md` | `dlpio20.go`, `dlp_ports.go` |
| NeuroSpin MEG TTL box, FORP buttons | `notes/megttlbox.md` | `megttlbox.go` |
| NEUROSPEC MMBT-S | `notes/mmbts.md` | `mmbts.go` |
| Adafruit FT232H (Linux) | `notes/ft232h.md` | `ft232h*.go` |
| Linux GPIO (Raspberry Pi, SBCs) | `notes/linuxgpio.md` | `linuxgpio*.go` |
| LabJack T4 (Modbus TCP) | `notes/labjackt4.md` | `labjackt4.go` |
| Parallel port (Linux LPT) | `notes/parallel.md` | `parallel*.go` |
| Generic serial UART | `notes/serial.md` | `serial.go` |
| EGI NetStation (ECI over TCP) | `notes/netstation.md` | `netstation.go` |
| BEL video recorder (TCP) | `notes/videorecorder.md` | `videorecorder.go` |

## Key conventions

- Always `defer dev.Close()` — drives all lines LOW and releases the port.
- For `OutputTTLDevice`, send the trigger as close as possible to the `exp.ShowTS` VSYNC flip; latency is typically <1 ms.
- For `InputTTLDevice`, call `DrainInputs(ctx)` before `WaitForInput(ctx)` between trials to clear latched presses.
- To use a MEGTTLBox or DLPIO8 as a `ResponseDevice` in the `apparatus` package: `apparatus.NewTTLResponseDevice(box, 5*time.Millisecond)`.
