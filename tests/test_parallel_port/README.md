# test_parallel_port

Interactive smoke-test for the Linux LPT parallel port driver (`triggers/parallel_linux.go`).

## Prerequisites

```bash
sudo modprobe ppdev
sudo usermod -aG lp $USER   # re-login to take effect
```

## Usage

```bash
go run main.go /dev/parport0
```

If no device is given the program lists accessible ports and exits.

## What it tests

| Step | Operation | What to verify |
|------|-----------|----------------|
| 1 | `Send()` with 8 bitmasks (0x00–0xFF range) | Data pins D0–D7 match each bitmask |
| 2 | `SetHigh` / `SetLow` per line | Each data line toggles independently |
| 3 | `Pulse(line 0, 100 ms)` | D0 goes HIGH for ~100 ms then LOW |
| 4 | `ReadStatus()` | Status register byte printed with per-bit decode |

All data lines are driven LOW on exit.

## Hardware setup

Connect a breakout board or logic analyser to the DB-25 connector to observe
the data lines (pins 2–9 = D0–D7) and status lines (pins 10–13, 15).
A loopback cable (data → status) lets you verify round-trip read/write without
external equipment.
