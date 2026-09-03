# test_mmbts

Interactive smoke-test for the NEUROSPEC MMBT-S driver (`triggers/mmbts.go`).

The MMBT-S is **output-only and silent** — it never answers on its serial line —
so nothing here can be checked automatically. The program drives known patterns
and you confirm them, either on the box's own LED or on an instrument.

## Prerequisites

Read/write access to the serial port. On Linux the box appears as
`/dev/ttyACM0` (a generic Arduino Micro, USB `2341:8037`):

```bash
sudo usermod -aG dialout $USER   # then log in again
ls -l /dev/ttyACM*
```

On macOS it is `/dev/tty.usbmodemXXXX`, on Windows a `COMx` port.

## Check the P/S switch first

The runtime mode is set by the hardware switch next to the USB-C socket, is
read by the box at reset, and **cannot be queried over the serial link**. Tell
the program which way it is set:

| Switch | `-mode` | Behaviour |
|---|---|---|
| `P` (factory setting) | `p` (default) | The firmware clears the output port 8 ms after each byte. Every pulse is 8 ms wide whatever width is requested, and codes sent closer together than 8 ms are queued and delayed. |
| `S` | `s` | A byte latches on the port until the next one is written; writing `0` pulls every line LOW. The host controls the width. |

If `-mode` disagrees with the switch nothing fails — the lines simply behave
differently from what is printed.

## Usage

```bash
go run ./tests/test_mmbts -list                          # list serial ports and exit
go run ./tests/test_mmbts -device /dev/ttyACM0           # the default sequence
go run ./tests/test_mmbts -device /dev/ttyACM0 -mode s   # switch set to "S"
go run ./tests/test_mmbts -device /dev/ttyACM0 -set 0xAA # drive one mask and hold it
go run ./tests/test_mmbts -device /dev/ttyACM0 -cycle    # square wave until Ctrl-C
```

The program prints the wiring and asks for confirmation before the first edge
(`-no-prompt` skips the question). If the box is plugged into a recording that
is running, these pulses land in it as trigger codes.

## What it tests

| Step | Operation | What to verify |
|------|-----------|----------------|
| 1 | `Send()` with 8 bitmasks | D-Sub 25 pins 2–9 match each bitmask; the green LED follows bit 1, so it lights on the odd codes (`0xFF`, `0x55`, `0x0F`, `0x01`) |
| 2 | `SetHigh` / `SetLow` per line | Each pin toggles independently (in mode `p` the firmware drops it after 8 ms) |
| 3 | `Pulse()` ×10 on `-line` | Ten pulses; ~8 ms wide in mode `p`, `-width` wide in mode `s` |

`-set` and `-cycle` replace the sequence with a single held mask and a square
wave respectively — the patterns to put a scope or a photodiode capture on.

All lines are driven LOW on exit.

## Hardware setup

Output socket: female D-Sub 25, standard LPT pinout.

| Pin | Function |
|---|---|
| 2–9 | bits 1–8 (5 V HIGH, 0 V LOW) |
| 20–25 | ground |
| 1, 10–19 | not connected |

Bit *N* of the byte (0-indexed, as `SetHigh(N)` takes it) is **pin N+2**.

Without an instrument, the green LED beside the socket is enough to tell a
working link from a dead one: it follows bit 1.

## Reference

NEUROSPEC AG, *MMBT-S Interface Box Manual*, v2.3 (2024) — runtime modes §2.3,
technical specifications §3.1, pinout §3.2, warnings §3.3, delay measurement
§3.4. <https://www.neurospec.com>

Two warnings from that manual are enforced or documented in the driver: the
baud rate is fixed at 9600 (opening the port at 1200 baud is the Arduino
bootloader touch and makes the box disappear until it is replugged), and the
output port is inactive until a host opens it.
