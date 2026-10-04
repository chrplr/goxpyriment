## MMBTS (NEUROSPEC MMBT-S, USB-CDC)

Implements `OutputTTLDevice` only — the box has no inputs and never answers on
its serial line. One byte at **9600 baud** drives the eight TTL lines: bit N →
line N. That silence is why there is no auto-detect: the box cannot be probed,
and it enumerates as a plain Arduino Micro (USB `2341:8037`), indistinguishable
from any other Arduino-based box on the machine.

```go
box, err := triggers.NewMMBTS("/dev/ttyACM0",
    triggers.WithMMBTSMode(triggers.MMBTSPulseMode), // default; the factory switch setting
)
defer box.Close()

box.Send(0b00000011)                 // whole port in one byte
box.SetHigh(0)                       // rewrites the full mask from the shadow
box.Pulse(0, 5*time.Millisecond)     // width honoured in simple mode only
```

**Runtime mode is a hardware switch and cannot be read back.** The P/S switch
next to the USB-C socket is sampled by the box at reset. `WithMMBTSMode` tells
the driver what it is set to; it does not set it, and a mismatch fails silently
— the lines just behave differently from the code.

| Switch | `MMBTSMode` | Behaviour |
|---|---|---|
| `P` (factory) | `MMBTSPulseMode` (default) | The firmware clears the port 8 ms after each byte. **Every trigger is 8 ms wide** whatever duration `Pulse`, `FireTrigger` or `FireTriggerSync` is given, and codes sent less than 8 ms apart are queued behind the 64-byte receive buffer and delayed 8 ms each. |
| `S` | `MMBTSSimpleMode` | A byte latches until the next one is written; `0` pulls every line LOW. The host times the width; the firmware's USB polling caps the update rate near 5 kHz. |

**Shadow mask.** The box takes a whole byte, not per-line commands, so
`SetHigh`/`SetLow` rewrite the full mask from a shadow of it. In pulse mode that
shadow expires after the pulse width: a `SetHigh` more than 8 ms after the last
write starts from zero rather than resurrecting a code the recording has already
seen fall.

**D-Sub 25 pinout** (female, standard LPT):

| Pin | Function |
|---|---|
| 2–9 | bits 1–8 — 5 V HIGH, 0 V LOW |
| 20–25 | ground |
| 1, 10–19 | not connected |

Bit *N* as `SetHigh(N)` takes it (0-indexed) is **pin N+2**. The green LED beside
the socket follows bit 1, so it lights on odd codes — the only feedback the box
gives, and enough to tell a working link from a dead one without an amplifier.

**Never change the baud rate.** 9600 is fixed in firmware and not exposed by the
driver: opening the port at 1200 baud is the Arduino bootloader touch, and the
box resets and disappears from the system until it is replugged (manual §3.3).

**Prerequisites.** Read/write access to the port — on Linux the `dialout` group
(`sudo usermod -aG dialout $USER`, then log in again). The output port is
inactive until a host opens it; `NewMMBTS` writes a zero at open to leave it in
the documented initial state.

**Verified on hardware:** not yet. The driver is written from the manual (v2.3,
2024) and the unit tests pin the bytes it puts on the wire, but nothing has been
driven into an instrument here. Confirm with
`go run ./tests/test_mmbts -device /dev/ttyACM0` before an experiment depends on
it, and record the result in this block.

The only latency figure available is the **manufacturer's**, not a measurement
from this repo: mean 92.1 µs, min 54.0 µs, max 172.2 µs, sd 17.2 µs, n = 1000,
against an on-board LPT port on a Windows 10 desktop (manual §3.4). Measure the
number for your own rig — with `tests/test_photodiode_latency` or a scope on
pins 2–9 — before quoting one.

