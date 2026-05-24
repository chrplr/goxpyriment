# test_linuxgpio

Interactive smoke-test for the Linux GPIO character device driver
(`triggers/linuxgpio_linux.go`). Works on any Linux SBC with GPIO —
Raspberry Pi, Rock Pi, BeagleBone, Jetson, etc.

## Prerequisites

```bash
sudo usermod -aG gpio $USER   # grant access to /dev/gpiochip0; re-login to take effect
```

Requires kernel ≥ 5.10 (GPIO character device v2 API).

## Usage

```bash
# Raspberry Pi — BCM pin defaults (output only):
go run main.go

# With custom output + input pins:
go run main.go -out 17,27,22,5,6,13,19,26 -in 12,16,20,21,4,25,24,23

# Output only (skip input tests):
go run main.go -out 17,27,22,5,6,13,19,26

# Different chip:
go run main.go -chip /dev/gpiochip4 -out 4,5,6,7,8,9,10,11
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-chip` | `/dev/gpiochip0` | GPIO chip device path |
| `-out` | `17,27,22,5,6,13,19,26` | 8 output pin numbers (BCM), comma-separated |
| `-in`  | *(empty)* | 8 input pin numbers (BCM); omit to skip input tests |

## What it tests

| Step | Operation | What to verify |
|------|-----------|----------------|
| 1 | `Send()` with 8 bitmasks | Output pins match each bitmask |
| 2 | `SetHigh` / `SetLow` per line | Each output pin toggles independently |
| 3 | `Pulse(line 0, 100 ms)` | First output pin goes HIGH for ~100 ms then LOW |
| 4 | `ReadAll()` *(if `-in` given)* | Input bitmask printed |
| 5 | `ReadLine()` per line *(if `-in` given)* | Each input pin state printed individually |
| 6 | `WaitForInput()` 2 s window *(if `-in` given)* | Detects first active input line and prints RT |

All output lines are driven LOW on exit.

## Hardware setup (Raspberry Pi example)

```
Output pins (BCM): 17 27 22  5  6 13 19 26  → connect to EEG/MEG STI channel
Input  pins (BCM): 12 16 20 21  4 25 24 23  → connect to response-pad buttons
```

For a self-loopback smoke-test without external equipment, bridge out[0]↔in[0],
…, out[7]↔in[7]. `ReadAll()` should then reflect whatever was last written.

Pin numbers refer to the BCM/GPIO numbering scheme (not the physical header pin
numbers). On non-RPi boards use the chip-relative offset numbers shown in
`gpioinfo /dev/gpiochip0`.
