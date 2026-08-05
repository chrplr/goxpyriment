# test_labjackt4

Interactive smoke-test for the LabJack T4 driver (`triggers/labjackt4.go`).
Works with any LabJack T4 reachable over Modbus TCP (USB or Ethernet connection).

## Prerequisites

The LabJack T4 must be reachable at its IP address on port 502.  
No special permissions or kernel modules are required.

## Usage

```bash
# Basic (default port 502):
go run . -host 192.168.1.100

# Custom port:
go run . -host 192.168.1.100 -port 502
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | *(required)* | LabJack T4 IP address |
| `-port` | `502` | Modbus TCP port |
| `-hold` | off | Walk the 8 output lines, holding each HIGH until you press Enter |
| `-set <mask>` | off | Drive one output bitmask and hold it until Enter/Ctrl-C |
| `-watch` | off | Live display of the input lines until Enter/Ctrl-C |

`-hold`, `-set` and `-watch` are mutually exclusive; each replaces the automatic
sequence. All of them drive every output LOW on exit, including on Ctrl-C.

## Measuring with a multimeter

The automatic sequence holds each output level for only 30–100 ms — far too
briefly for a DMM, which needs 0.3–1 s to settle. Use the static modes instead:

```bash
# Walk the outputs one at a time; each stays HIGH until you press Enter.
go run . -host 192.168.1.100 -hold

# Drive a fixed pattern and hold it indefinitely (0xAA = alternating lines).
go run . -host 192.168.1.100 -set 0xAA      # also accepts 0b10101010 or 170
```

`-set` prints the expected level of every terminal, so you can tick them off:

```
  line  terminal  DIO   expected
  0     FIO4      DIO4   LOW  (≈0 V)
  1     FIO5      DIO5   HIGH (≈3.3 V)
  ...
```

Measure between the named terminal and any GND terminal. HIGH is **3.3 V**, not
5 V — the T-series drives 3.3 V logic (its *inputs* are 5 V tolerant).

## Testing an input line

The input lines idle HIGH on the T4's internal pull-ups, so an unconnected pin
reads 1. **Patch the pin to GND** to see it change; patching to 3.3/5 V shows
nothing new. `-watch` redraws whenever the bitmask changes:

```bash
go run . -host 192.168.1.100 -watch
#   0xFF  11111111   low: (none)
#   0xFB  11111011   low: 2=EIO6        ← EIO6 patched to GND
```

## What it tests

| Step | Operation | What to verify |
|------|-----------|----------------|
| 1 | `Send()` with 8 bitmasks | DIO4–DIO11 match each bitmask |
| 2 | `SetHigh` / `SetLow` per line | Each output pin toggles independently |
| 3 | `Pulse(line 0, 100 ms)` | FIO4 goes HIGH for ~100 ms then LOW |
| 4 | `ReadAll()` | DIO12–DIO19 bitmask printed |
| 5 | `ReadLine()` per line | Each input pin state printed individually |
| 6 | `WaitForInput()` 2 s window | Detects first active input line and prints RT |

All output lines are driven LOW on exit.

## Hardware setup

```
Output pins: DIO4–DIO11  (FIO4–FIO7, EIO0–EIO3) → connect to EEG/MEG STI channel
Input  pins: DIO12–DIO19 (EIO4–EIO7, CIO0–CIO3) → connect to response-pad buttons
```

For a self-loopback smoke-test without external equipment, bridge output line N
to input line N: FIO4↔EIO4, FIO5↔EIO5, FIO6↔EIO6, FIO7↔EIO7, EIO0↔CIO0,
EIO1↔CIO1, EIO2↔CIO2, EIO3↔CIO3. `ReadAll()` should then reflect whatever was
last written by `Send()`.

## Wiring (LabJack T4)

| LabJack pin | Connector | Role |
|-------------|-----------|------|
| FIO4–FIO7 (DIO4–DIO7)   | screw terminals | output lines 0–3 (3.3 V) — trigger codes |
| EIO0–EIO3 (DIO8–DIO11)  | DB15 | output lines 4–7 (3.3 V) — trigger codes |
| EIO4–EIO7 (DIO12–DIO15) | DB15 | input lines 0–3 (3.3 V) — response buttons |
| CIO0–CIO3 (DIO16–DIO19) | DB15 | input lines 4–7 (3.3 V) — response buttons |
| GND         | both | Common ground |

**Note:** the T4 has no digital DIO0–DIO3 — those terminals are the dedicated
analog inputs AIN0–AIN3. FIO4–FIO7 and EIO0–EIO3 are *flexible I/O* that power
up as analog inputs; the driver switches them to digital mode when it opens the
device.
