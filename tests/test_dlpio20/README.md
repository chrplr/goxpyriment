# test_dlpio20

Interactive smoke-test for the DLP-IO20 driver (`triggers/dlpio20.go`).

> **Untested on hardware.** The driver was written from the
> [datasheet](https://www.dlpdesign.com/usb/dlp-io20-ds-v11.pdf) (rev 1.1) and
> has never been run against a real DLP-IO20. Treat the first run as a
> bring-up: start with `-hold` and a multimeter before trusting it in an
> experiment.

## Prerequisites

The DLP-IO20 appears as an FTDI virtual COM port (`/dev/ttyUSB0` on Linux,
`COMn` on Windows). On Linux:

```bash
sudo usermod -a -G dialout $USER    # log out and back in
sudo dmesg -w                       # then plug the device in
#   FTDI USB Serial Device converter now attached to ttyUSB0
```

Read round-trips are dominated by the FTDI latency timer (16 ms by default).
Lower it before timing-sensitive work:

```bash
echo 1 | sudo tee /sys/bus/usb-serial/devices/ttyUSB0/latency_timer
```

## Usage

```bash
go run .                        # auto-detect the port
go run . -device /dev/ttyUSB0   # explicit port
go run . -list                  # list serial ports and exit
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-device` | *(auto-detect)* | Serial port, e.g. `/dev/ttyUSB0` or `COM3` |
| `-list` | off | List available serial ports and exit |
| `-hold` | off | Walk the 8 output lines, holding each HIGH until you press Enter |
| `-set <mask>` | off | Drive one output bitmask and hold it until Enter/Ctrl-C |
| `-watch` | off | Live display of the input lines until Enter/Ctrl-C |

`-hold`, `-set` and `-watch` are mutually exclusive; each replaces the automatic
sequence. All of them drive every output LOW on exit, including on Ctrl-C.

## Channel windows

The DLP-IO20 has 17 usable digital channels but the TTL interfaces are 8-bit,
so lines 0–7 address a window of 8 channels:

| Interface line | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 |
|---|---|---|---|---|---|---|---|---|
| **output** | AN0 | AN1 | AN2 | AN3 | AN4 | AN5 | AN6 | AN7 |
| **input** | AN8 | AN9 | AN10 | AN11 | AN12 | AN13 | RB6 | RB7 |

Remap with `triggers.WithIO20OutputChannels` / `WithIO20InputChannels`. Channels
outside the windows (RA4, and the relay drivers P5–P7) stay reachable through
`SetChannelHigh` / `SetChannelLow` / `ReadChannel`; the automatic run exercises
RA4 that way.

## Measuring with a multimeter

The automatic sequence holds each output level for only 30–100 ms — far too
briefly for a DMM. Use the static modes:

```bash
# Walk the outputs one at a time; each stays HIGH until you press Enter.
go run . -hold

# Drive a fixed pattern and hold it indefinitely.
go run . -set 0xAA      # also accepts 0b10101010 or 170
```

Measure between the named terminal and a GND terminal. The DLP-IO20 is a 5 V
system, so HIGH is **≈5 V** (unlike the LabJack T4's 3.3 V).

## Testing an input line

Unlike the LabJack T4, the DLP-IO20's inputs have no internal pull-ups, so an
unconnected pin floats and reads unpredictably. Patch the pin to **+5V** for a
1 or to **GND** for a 0 — the datasheet's own example wires a morse key from
AN4 to +5V. `-watch` redraws whenever the bitmask changes:

```bash
go run . -watch
#   0x00  00000000   high: (none)
#   0x10  00010000   high: 4=AN12       ← AN12 patched to +5V
```

## Self-loopback

Connect output line N to input line N — AN0→AN8, AN1→AN9, AN2→AN10, AN3→AN11,
AN4→AN12, AN5→AN13, AN6→RB6, AN7→RB7 — and `ReadAll()` should mirror whatever
`Send()` last wrote.

## What it tests

| Step | Operation | What to verify |
|------|-----------|----------------|
| 1 | `Send()` with 8 bitmasks | AN0–AN7 match each bitmask |
| 2 | `SetHigh` / `SetLow` per line | Each output pin toggles independently |
| 3 | `Pulse(line 0, 100 ms)` | AN0 goes HIGH for ~100 ms then LOW |
| 4 | `SetChannelHigh/Low(RA4)` | A channel outside the output window drives |
| 5 | `ReadAll()` | Input window bitmask printed |
| 6 | `ReadLine()` per line | Each input pin state printed individually |
| 7 | `WaitForInput()` 2 s window | Detects first active input line and prints RT |

## Timing caveat

Every channel needs its own 5-byte packet, so `Send()` issues 8 packets — the
lines change over ~3.5 ms of wire time plus USB latency, not simultaneously.
For sub-millisecond trigger onsets use `SetHigh` on a single line, or a
parallel port / LabJack T4.
