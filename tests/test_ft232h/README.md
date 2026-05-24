# test_ft232h

Interactive smoke-test for the Adafruit FT232H driver (`triggers/ft232h_linux.go`).

## Prerequisites

```bash
sudo rmmod ftdi_sio              # detach kernel UART driver if loaded
sudo usermod -aG plugdev $USER   # grant access to /dev/bus/usb; re-login to take effect
```

Alternatively, add a udev rule so the device is always accessible without root:

```
# /etc/udev/rules.d/99-ft232h.rules
ACTION=="add", SUBSYSTEM=="usb", ATTRS{idVendor}=="0403", ATTRS{idProduct}=="6014", MODE="0666", GROUP="plugdev"
```

## Usage

```bash
go run main.go
```

The program auto-detects the first FT232H on the system. No arguments needed.

## What it tests

| Step | Operation | What to verify |
|------|-----------|----------------|
| 1 | `Send()` with 8 bitmasks | AD0–AD7 (D-bus) match each bitmask |
| 2 | `SetHigh` / `SetLow` per line | Each AD pin toggles independently |
| 3 | `Pulse(line 0, 100 ms)` | AD0 goes HIGH for ~100 ms then LOW |
| 4 | `ReadAll()` | AC0–AC7 (C-bus) bitmask printed |
| 5 | `ReadLine()` per line | Each AC pin state printed individually |
| 6 | `WaitForInput()` (2 s window) | Detects first active C-bus line and prints RT |

All output lines are driven LOW on exit.

## Hardware setup

| FT232H pin | Role |
|------------|------|
| AD0–AD7    | 8 TTL **outputs** — connect to recording equipment or logic analyser |
| AC0–AC7    | 8 TTL **inputs**  — connect to response buttons or loopback wires |

For a self-loopback smoke-test without external equipment, bridge AD0↔AC0,
AD1↔AC1, …, AD7↔AC7. Step 4 (`ReadAll`) should then reflect whatever was last
written by `Send()`.
