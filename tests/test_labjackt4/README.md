# test_labjackt4

Interactive smoke-test for the LabJack T4 driver (`triggers/labjackt4.go`).
Works with any LabJack T4 reachable over Modbus TCP (USB or Ethernet connection).

## Prerequisites

The LabJack T4 must be reachable at its IP address on port 502.  
No special permissions or kernel modules are required.

## Usage

```bash
# Basic (default port 502):
go run main.go -host 192.168.1.100

# Custom port:
go run main.go -host 192.168.1.100 -port 502
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | *(required)* | LabJack T4 IP address |
| `-port` | `502` | Modbus TCP port |

## What it tests

| Step | Operation | What to verify |
|------|-----------|----------------|
| 1 | `Send()` with 8 bitmasks | FIO0–FIO7 match each bitmask |
| 2 | `SetHigh` / `SetLow` per line | Each FIO pin toggles independently |
| 3 | `Pulse(line 0, 100 ms)` | FIO0 goes HIGH for ~100 ms then LOW |
| 4 | `ReadAll()` | EIO0–EIO7 bitmask printed |
| 5 | `ReadLine()` per line | Each EIO pin state printed individually |
| 6 | `WaitForInput()` 2 s window | Detects first active EIO line and prints RT |

All FIO output lines are driven LOW on exit.

## Hardware setup

```
Output pins: FIO0–FIO7  → connect to EEG/MEG STI channel
Input  pins: EIO0–EIO7  → connect to response-pad buttons
```

For a self-loopback smoke-test without external equipment, bridge FIO0↔EIO0,
…, FIO7↔EIO7. `ReadAll()` should then reflect whatever was last written by
`Send()`.

## Wiring (LabJack T4 screw terminals)

| LabJack pin | Role |
|-------------|------|
| FIO0–FIO7   | 8 TTL **outputs** (3.3 V) — trigger codes |
| EIO0–EIO7   | 8 TTL **inputs**  (3.3 V) — response buttons |
| GND         | Common ground |
