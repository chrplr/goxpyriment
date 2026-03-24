// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

# triggers package

Hardware trigger interfaces for EEG/MEG event coding and response boxes. All devices implement the `Trigger` interface; `NullTrigger` is always safe to use when no hardware is present.

## Trigger interface

```go
type Trigger interface {
    Send(value byte) error         // set all 8 output lines from bitmask
    SetHigh(pin int) error         // drive single pin HIGH (1-indexed, 1–8)
    SetLow(pin int) error          // drive single pin LOW
    Pulse(pin, durationMs int) error // HIGH for durationMs, then LOW (blocks)
    Close() error                  // set all lines LOW, release resources
}
```

Pins are **1-indexed** (pin 1 = bit 0 of bitmask, pin 8 = bit 7).

## NullTrigger

No-op implementation; all methods return nil. Used as a safe default when no device is detected.

```go
var trig triggers.Trigger = triggers.NullTrigger{}
```

## ParallelPort (Linux LPT)

```go
ports := triggers.AvailableParallelPorts()  // scans /dev/parport0..3
pp := triggers.NewParallelPort("/dev/parport0")
if err := pp.Open(); err != nil { log.Fatal(err) }
defer pp.Close()

pp.Send(0b00000111)     // set pins 1,2,3 HIGH
pp.SetHigh(4)
pp.SetLow(4)
pp.Pulse(1, 10)         // 10 ms pulse on pin 1

status, _ := pp.ReadStatus()  // nACK, BUSY, PAPER-OUT, SELECT, nERROR bits
```

**Prerequisites:** `sudo modprobe ppdev`, user must be in the `lp` group.

Non-Linux platforms return `"not supported"` errors from all methods.

## DLPIO8 (USB digital I/O, DLP-IO8-G)

Auto-detection is the recommended approach:

```go
trig, portName, err := triggers.AutoDetectDLPIO8()
// If no device found: trig = NullTrigger{}, err = nil (safe fallback)
defer trig.Close()
```

Manual construction:

```go
d, err := triggers.NewDLPIO8("/dev/ttyUSB0")
defer d.Close()
d.Send(0b10000001)  // set pins 1 and 8 HIGH
d.ReadPin(3)        // read state of pin 3 (returns 0 or 1)
d.ReadAll()         // read all 8 pins; result[0] = pin 1, result[7] = pin 8
d.AllLow()          // convenience: Send(0x00)
```

**Device details:**
- USB-CDC at 115200 baud; device usually appears as `/dev/ttyUSB0` or `/dev/ttyACM0`.
- Binary mode enabled automatically after init; pin reads return 0x00 or 0x01.
- `Send()` iterates over individual pins (not atomic); suitable for sequential EEG codes.
- Constructor retries ping 3 times to handle USB latency.

## SerialPort (generic UART)

General-purpose serial interface for response boxes, Arduino, etc. Does **not** implement `Trigger`.

```go
ports, _ := triggers.AvailablePorts()   // list all serial ports
sp := triggers.NewSerialPort("/dev/ttyUSB0", 9600)
if err := sp.Open(); err != nil { log.Fatal(err) }
defer sp.Close()

sp.Send(0x42)                    // write single byte
sp.SendLine("GO", false, true)   // write "GO\n"
b, _ := sp.Poll()                // non-blocking read; returns 0 if no data
line, _ := sp.ReadLine()         // blocking read until newline
sp.Clear()                       // flush input buffer
```

## Key conventions

- Always `defer trig.Close()` — this sets all lines LOW and releases the device.
- Use `AutoDetectDLPIO8()` rather than `NewDLPIO8` so the code degrades gracefully on machines without the device.
- For EEG event coding, send the trigger code on stimulus onset and reset to 0 after a short pulse: `pp.Send(code)` → `time.Sleep(10ms)` → `pp.Send(0)`. Or use `pp.Pulse(pin, 10)` for single-pin events.
- `ParallelPort` tracks pin states in a shadow register; `SetHigh`/`SetLow` update it correctly even when only partial byte writes are needed.
