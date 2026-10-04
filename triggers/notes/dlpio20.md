## DLPIO20 (DLP-IO20, USB-CDC)

Implements both interfaces. Written from the
[datasheet](https://www.dlpdesign.com/usb/dlp-io20-ds-v11.pdf) rev 1.1 and
**partly verified on hardware** (2026-08-05, `/dev/ttyUSB0`):

- *Confirmed — output.* Digital output on `AN0`–`AN7` via `Send`, measured with a
  multimeter against GND: `0xAA` then `0x55` gave a clean 5 V / 0 V on every line
  and inverted correctly, so all eight drive both ways and the channel-code
  mapping is right (an off-by-one would have shifted the pattern by one terminal).
- *Confirmed — input.* Packet framing and the ping (`'Y'`). With GND patched to
  `AN8`, `ReadAll` returned `0xFE`; moving the wire to `AN12` returned `0xEF`.
  The driven line reads `0` at the right bit position, so reads reflect the real
  pin and the input mapping holds across the window.
- *Not confirmed.* `RA4`, the `P5`–`P7` relay drivers, and every timing figure
  below (estimated from the baud rate, not measured).

Binary *packet* protocol, not the IO8's single ASCII bytes: byte 0 is the
packet length **including itself**.

| Command | Packet | Returns |
|---|---|---|
| Ping | `02 27` | `'Y'` (0x59) — the IO8 answers `'Q'`, so the two never cross-detect |
| Digital I/O | `05 35 <ch> <dir> <val>` | 1 byte, **only** when `dir` = `0x01` (input) |

`dir` is `0x00` output / `0x01` input, `val` is `0x00` low / `0x01` high. Every
command carries a direction, so channels are reconfigured per call — there is
no direction register to set up at open. Byte 4 must be present even in input
mode, where it is ignored.

**Channels** (`IO20Channel`, code = datasheet channel number):

| Code | Name | Notes |
|---|---|---|
| `0x00`–`0x0D` | `IO20_AN0`–`IO20_AN13` | digital I/O, also analog-capable |
| `0x0E` | `IO20_RA4` | digital I/O |
| `0x0F`–`0x11` | `IO20_P5`–`IO20_P7` | relay drivers (Darlington) — **not TTL**, cannot be read |
| `0x12` | `IO20_RB7` | note the inversion: RB7 is `0x12`… |
| `0x13` | `IO20_RB6` | …and RB6 is `0x13` |

**8-line windows.** The TTL interfaces are 8-bit but the device has 17 usable
digital channels, so interface lines 0–7 address a *window*:

- outputs (default): `AN0`–`AN7`
- inputs (default): `AN8`–`AN13`, `RB6`, `RB7`

```go
d, err := triggers.NewDLPIO20("/dev/ttyUSB0",
    triggers.WithIO20OutputChannels(triggers.IO20_AN0, /* …8 total… */ triggers.IO20_AN7),
    triggers.WithIO20InputChannels(triggers.IO20_AN8, /* …8 total… */ triggers.IO20_RB7),
    triggers.WithIO20PollInterval(5*time.Millisecond),
    triggers.WithIO20ReadTimeout(200*time.Millisecond),
)
defer d.Close()

d.Send(0b00000101)                  // lines 0,2 → AN0, AN2
d.Pulse(0, 5*time.Millisecond)

// Any channel, in or out of the windows:
d.SetChannelHigh(triggers.IO20_RA4)
v, _ := d.ReadChannel(triggers.IO20_AN12)

out, port, err := triggers.AutoDetectDLPIO20()   // → NullOutputTTLDevice{} if absent
```

A group must be exactly 8 channels, without duplicates, and no channel may be
in both groups — `NewDLPIO20` rejects all three.

**Timing.** There is no write-all command: `Send` issues 8 packets (~3.5 ms of
wire time at 115200 baud plus USB latency), so lines do *not* change
simultaneously. Prefer `SetHigh` on a single line for trigger onsets. On Linux
the FTDI latency timer (16 ms default) dominates reads:
`echo 1 | sudo tee /sys/bus/usb-serial/devices/ttyUSB0/latency_timer`. That cost
is per *round trip*, and `ReadAll` makes 8 of them — with the timer left at its
16 ms default, a `WaitForInput` was observed returning `rt = 128 ms`, i.e. one
full `ReadAll` sweep, regardless of the 5 ms poll interval. Lower the timer
before using the input path for anything reaction-time-like.

**Reads leave the channel an input.** Direction travels with every command, so
`ReadChannel` (and hence `ReadLine`/`ReadAll`/`WaitForInput`) reconfigures the
channel to input mode and *nothing switches it back*. After any read, those pins
float until the next write — measuring one then reads an arbitrary mid-rail
voltage (~2 V observed), which is the pin floating, not a fault. `Close` drives
only the **output** window LOW; previously-read channels are left as inputs.

**5 V logic**, unlike the LabJack T4's 3.3 V. Inputs have no pull-ups, so patch
a pin to +5V or GND — a floating input reads unpredictably.

**Never leave an input line floating.** This is not cosmetic: floating lines were
observed flipping between reads (`0xEF`/`0xEB`/`0xE7` on an otherwise idle board,
while the one GND-tied line stayed rock steady). Since `WaitForInput` returns as
soon as *any* line in the window goes active, unused floating lines make it fire
on noise — an early bench run reported a confident `mask=0x7F` with nothing
connected at all. Tie every unused input to GND with a pull-down, or narrow the
window with `WithIO20InputChannels` so it only covers lines you have wired.

