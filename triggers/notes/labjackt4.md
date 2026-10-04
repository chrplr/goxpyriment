## LabJackT4 (Modbus TCP)

Implements both interfaces. Pure-Go Modbus TCP driver — no SDK or system library required.

**Wiring (T4 digital lines are DIO4–DIO19 only):**
- **outputs** DIO4–DIO11 = FIO4–FIO7 (screw terminals) + EIO0–EIO3 (DB15)
- **inputs** DIO12–DIO19 = EIO4–EIO7 + CIO0–CIO3 (DB15)

`DIO0–DIO3` are **not usable**: on the T4 they are the dedicated analog inputs
AIN0–AIN3. `DIO4–DIO11` are *flexible I/O* that **power up as analog inputs**
and silently ignore digital writes until `DIO_ANALOG_ENABLE` is cleared —
`NewLabJackT4` does that at open.

```go
box, err := triggers.NewLabJackT4("192.168.1.100",
    triggers.WithT4PollInterval(5*time.Millisecond), // optional
    triggers.WithT4Timeout(1*time.Second),            // optional
    triggers.WithT4UnitID(1),                         // optional
    triggers.WithT4OutputBase(4),                     // optional, DIO of output line 0
    triggers.WithT4InputBase(12),                     // optional, DIO of input line 0
)
if err != nil { log.Fatal(err) }
defer box.Close()

box.Send(0b00000001)              // output line 0 (FIO4) HIGH
box.Pulse(0, 5*time.Millisecond)  // FIO4: HIGH for 5 ms, then LOW

_ = box.DrainInputs(ctx)
mask, rt, _ := box.WaitForInput(ctx)
```

**Internal protocol:** Modbus TCP on port 502 via `github.com/goburrow/modbus`.
Only the 32-bit DIO bitmask registers are used (2 Modbus registers each,
big-endian): FC16 (`WriteMultipleRegisters`) for output, FC3
(`ReadHoldingRegisters`) for input. Bit N of every value = DIO N.

| Register | Address | Type | Description |
|----------|---------|------|-------------|
| `DIO_STATE` | 2800 | UINT32 | level of every DIO (0 = LOW, 1 = HIGH) |
| `DIO_DIRECTION` | 2850 | UINT32 | 0 = input, 1 = output |
| `DIO_ANALOG_ENABLE` | 2880 | UINT32 | T4 only: 1 = analog, 0 = digital |
| `DIO_INHIBIT` | 2900 | UINT32 | 1 = ignore writes to that DIO |

Open sequence (order matters — `DIO_INHIBIT` filters the other three writes):
inhibit everything except the 16 owned lines → `DIO_ANALOG_ENABLE = 0` (digital)
→ `DIO_DIRECTION` = output mask → `DIO_STATE = 0` → narrow the inhibit mask to
the outputs alone, so a later `Send` cannot disturb the input lines.

The per-bank 16-bit registers (`FIO_STATE` 2500, `EIO_STATE` 2501,
`FIO_DIRECTION` **2600**, `EIO_DIRECTION` **2601** — note: *not* 2504/2505,
which do not exist and return Modbus exception 2) are unused; their upper 8
bits are inhibit bits.

