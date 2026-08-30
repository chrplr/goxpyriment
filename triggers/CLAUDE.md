# triggers package

Hardware TTL signal output (EEG/MEG trigger codes) and TTL input (response pads). Lines are **0-indexed (0–7)**; bit N of a bitmask corresponds to line N.

## Interfaces

```go
// OutputTTLDevice — send trigger codes to recording equipment.
type OutputTTLDevice interface {
    Send(mask byte) error                   // all 8 lines from bitmask
    SetHigh(line int) error                 // 0-indexed
    SetLow(line int) error                  // 0-indexed
    Pulse(line int, d time.Duration) error  // HIGH for d, then LOW (blocks)
    AllLow() error
    Close() error
}

// InputTTLDevice — read TTL inputs from response hardware.
type InputTTLDevice interface {
    ReadAll() (byte, error)                                          // bitmask
    ReadLine(line int) (byte, error)                                 // 0 or 1
    WaitForInput(ctx context.Context) (mask byte, rt time.Duration, err error)
    DrainInputs(ctx context.Context) error
    Close() error
}
```

`NullOutputTTLDevice` and `NullInputTTLDevice` are silent no-ops.

## Firing a trigger on a stimulus onset

Use **`FireTriggerSync`**, on the statement after the flip, with nothing in
between:

```go
flipTS, err := exp.ShowTS(stim)
triggers.FireTriggerSync(trig, pin, 5*time.Millisecond)
```

It raises on the calling goroutine and defers only the falling edge, which
carries no information. `FireTrigger` blocks for the whole pulse, so the only
way to call it from a frame loop is `go triggers.FireTrigger(...)` — and
dispatching the *raise* through a goroutine is what costs the measurement:

| how the rising edge is issued | gap from the flip |
|---|---|
| synchronously on the flip thread (`FireTriggerSync`) | **p50 34 µs, max 37 µs** at `SCHED_FIFO` 50 |
| through a goroutine (`go FireTrigger(...)`) | **+0.73 ms, ~1 ms spread** at normal priority under load |

`dur` must be shorter than the interval to the next call on the same device —
the implementations are not internally synchronised, so an overlapping raise and
lower would race on the port.

Neither call makes the edge coincide with the photons. `SDL_RenderPresent`
returns when the driver will accept the *next* frame, so the flip leads the
panel by one to three frames depending on the display stack (measured: kmsdrm
18.91 ms sd 0.113, Wayland 21.75 ms sd 1.344, bare Xorg 35.74 ms sd 0.083 at
60 Hz). That offset is constant per rig and is measured once, recorded and
subtracted in analysis — the library never adjusts a timestamp. See
[docs/TimingTests.md](../docs/TimingTests.md) and
[docs/TriggerJitterForEEGandMEG.md](../docs/TriggerJitterForEEGandMEG.md).

## DLPIO8 (DLP-IO8-G, USB-CDC)

Implements both interfaces. ASCII protocol at 115200 baud.

```go
// Auto-detect (recommended)
out, portName, err := triggers.AutoDetectDLPIO8()
// → NullOutputTTLDevice{} + nil err if not found

// Manual
d, err := triggers.NewDLPIO8("/dev/ttyUSB0")
defer d.Close()
d.Send(0b00000101)                   // lines 0 and 2 HIGH
d.Pulse(0, 10*time.Millisecond)
mask, _ := d.ReadAll()               // bitmask of all 8 input lines
mask, rt, _ := d.WaitForInput(ctx)
```

**Device protocol (internal):** set HIGH pin 1–8 = '1'–'8'; set LOW = 'Q'–'I'; read = 'A'–'K'; ping = '\''; binary mode = '\\'. The public API uses 0-indexed lines; internally translated to 1-indexed for the ASCII commands.

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

## MEGTTLBox (NeuroSpin Arduino Mega)

Implements both interfaces. Binary opcode protocol at 115200 baud.

```go
box, err := triggers.NewMEGTTLBox("/dev/ttyACM0",
    triggers.WithResetDelay(2*time.Second),    // DTR → Arduino reset (default 2 s)
    triggers.WithPollInterval(5*time.Millisecond),
)
defer box.Close()

box.Pulse(0, 5*time.Millisecond)
box.PulseMask(0b00000011, 5*time.Millisecond) // lines 0 and 1
box.Send(0b00000001)                           // persistent set (not a pulse)

_ = box.DrainInputs(ctx)
mask, rt, _ := box.WaitForInput(ctx)
buttons := triggers.DecodeMask(mask)           // []FORPButton
```

**Firmware identification.** `NewMEGTTLBox` probes opcode 1 (`get_info`) once at
open and caches the answer; read it back with `box.Info()`:

```go
info := box.Info()
fmt.Println(info)                                   // "firmware v1, caps 0x00"
if info.Has(triggers.MEGCapTimestamps) { /* … */ }  // feature-detect
```

Firmware predating the opcode ignores it *silently* (the sketch's `default:`
branch), so legacy boards are detected by the probe **timing out** — there is no
positive signal. That is reported as `MEGInfo.Legacy`, not as an error. A reply
that arrives without the `MTB` magic **is** an error: something is on the port
that is not a TTL box. Only opcodes 10–16 and 20 may be used against a legacy
box. Capability bits (`MEGCapAtomicPort`, `MEGCapTimestamps`) are declared on
both sides; firmware v1 claims `MEGCapAtomicPort` (opcode 17) and not
`MEGCapTimestamps`. Set a bit in the sketch only when the matching opcode
actually exists — old firmware ignores unknown opcodes *silently*, so a host
that assumes a capability gets no error, just a command that did nothing.

**Wire protocol (opcodes):**

| Opcode | Args | Description |
|--------|------|-------------|
| 1 | — | get_info → `'M','T','B'`, version uint8, caps uint8 |
| 10 | uint16 LE (ms) | set trigger pulse width |
| 11 | uint8 mask | pulse all set lines |
| 12 | uint8 line | pulse single line |
| 13 | uint8 mask | set lines HIGH (persistent) |
| 14 | uint8 mask | set lines LOW (persistent) |
| 15 | uint8 line | set single line HIGH |
| 16 | uint8 line | set single line LOW |
| 17 | uint8 mask | assign all 8 output lines atomically (needs `CAP_ATOMIC_PORT`) |
| 20 | — | read button mask → returns uint8 |
| 21 | — | get_event → `[flags u8][mask u8][micros u32 LE]`, always 6 bytes |
| 22 | — | get_micros → uint32 LE, for clock alignment |
| 23 | — | clear queued events and re-seed the change detector |
| 24 | uint16 LE (µs) | set debounce, 0 disables |

Opcodes 21–24 need `CAP_TIMESTAMPS`. `get_event` replies with a fixed 6 bytes
even when the queue is empty, so the host never has to guess the reply length;
`flags` bit 0 means an event follows, bit 1 means events were dropped.

**`Send` is atomic on firmware ≥ v1 with `CAP_ATOMIC_PORT`.** On the Mega 2560
the output pins D30–D37 are **PORTC7–PORTC0** and the inputs D22–D29 are
**PORTA0–PORTA7**, so each bank is one register. Opcode 17 assigns the whole
output port in a single instruction: no intermediate value ever reaches the pins,
so a code cannot be latched half-written. `readButtons` likewise became one
`PINA` read, sampling all 8 buttons at the same instant rather than smearing them
over ~40 µs of sequential `digitalRead`s. Dropping `digitalWrite`/`digitalRead`
also shrank the sketch from 3360 to 2786 bytes.

**Note the reversal**: PORTC is wired backwards relative to line order (D30 =
PC7 … D37 = PC0), so the firmware's `reverseBits` mirrors the mask. Getting that
wrong mirrors every trigger code, and *nothing in software would show it* — the
commands are accepted either way. `static_assert`s in the sketch fail the build
if `OUT_PINS`/`IN_PINS` are renumbered without updating the mapping.

**Verified on hardware** (2026-08-05, bench Mega 2560 R3):

- *Bit order.* `Send(0x01)` put D30 at 5 V and D37 at 0 V (multimeter).
- *All 16 lines.* `-loopback` passed 8/8 patterns. The asymmetric ones carry the
  proof: `0x0F → 0xF0`, `0x01 → 0xFE`, `0x80 → 0x7F` — a mirrored mapping would
  have returned `0x0F → 0x0F`.
- *Atomicity, internal witness.* `-atomic 30` drove 0x00 → 0xFF thirty times and
  the firmware, which samples those same pins every few µs, reported **exactly one
  event every time**. The two-command fallback would have been caught
  mid-transition. Reproduce with `go run ./tests/test_megttlbox -atomic 30` and
  the full loopback wired.
- *Atomicity, external witness.* The above uses the firmware to judge itself,
  which is circular. A BBTKv3 watching D30 and D31 while one command pulsed both
  recorded **zero skew on all 20 trials** — skew under 250 µs, measured by an
  instrument with no stake in the answer. Reproduce with
  `tests/test_megttlbox/run-bbtk.sh` (block D); details in the device repo's
  `TIMING.md`.

Legacy firmware without the capability falls back to two commands — `13`
(set-high) then `14` (set-low) — written in a **single** `Write` so both ride the
same USB transfer. Issued as separate writes they could land in different USB
frames, leaving the port at `previous | mask` for up to a frame, which is a
valid-looking but wrong code. Sharing one transfer narrows that window to
firmware parse time but does not close it; reflashing does.

**Pulse width is timed on the device**, unlike the DLP boxes where
`defaultPulse` sleeps on the host and absorbs OS scheduling jitter. The firmware
sets `g_pulse_end = millis() + width` and drops the line from its main loop, so
the width is quantised to `millis()` resolution. `Pulse` also sleeps host-side
for `dur` to honour the blocking contract, concurrently with the device's own
timing.

Measured on a BBTKv3 (2026-08-05): pulses come out **~0.5–0.7 ms short**, never
long by more than a sample. That is `millis()` truncation as predicted — the
realised width is uniform on [w−1, w]. It is a systematic **bias, not noise**, so
a paradigm needing a true 5 ms pulse should request 6 ms.

Full measurement tables, the host→device latency figures, and what remains
unmeasured live with the device, not here:
**[neurospin-meg-ttl-box/TIMING.md](https://github.com/neurospin/neurospin-meg-ttl-box/blob/main/TIMING.md)**
(raw captures in that repo's `measurements/`). They characterise the hardware and
firmware, of which this package is only one client — keep them in one place so
the numbers cannot drift.

**Reaction times: use the event API, not `WaitForInput`.** `WaitForInput` polls
`ReadAll` every 5 ms and reports *elapsed host time*, so its resolution is the
poll interval plus USB jitter — it can only ever say "the press had happened by
the time I asked". Firmware advertising `MEGCapTimestamps` instead samples PINA
every loop iteration (a few µs) and records `micros()` at the transition; the
host drains those events later and the timestamp is unaffected by when it got
round to asking.

```go
box.SetDebounce(0)                     // default; use >0 for mechanical buttons
box.DrainEvents()                      // between trials, instead of DrainInputs
ev, err := box.WaitForPressTS(ctx)     // ev.TS is a host-clock instant
rt := ev.TS.Sub(stimulusOnset)         // subtract your own onset timestamp
if box.EventsDropped() { /* queue overflowed: treat the trial as suspect */ }
```

`PollEvent` is the non-blocking form; it returns releases as well as presses
(`ev.Pressed()` distinguishes them). Resolution floor is `micros()`, which ticks
in 4 µs steps at 16 MHz.

**Clock alignment.** `ev.TS` is the device's `micros()` translated to the host
clock. `NewMEGTTLBox` calls `SyncClock` at open; it brackets the device's reading
between two host readings across 7 round trips and keeps the tightest. Device
timestamps are decoded as a **signed 32-bit µs delta** from the sync point, which
makes the ~71.6 min `micros()` wrap self-correcting as long as readings stay
within ±35.8 min of a sync — `megResyncAfter` re-syncs every 20 min to guarantee
it. Call `SyncClock` explicitly only after stepping the host clock.

**Measured (2026-08-05, bench Mega 2560 R3, jumper D30→D22, 50 trials).**
`(firmware timestamp of the edge) − (host clock immediately before the write)`:
**min 802 µs, median 1.44 ms, max 2.05 ms**. That ~1.25 ms spread is one USB
full-speed frame (1 ms) plus the ~174 µs two command bytes take on the
16u2↔2560 UART — it is *host→device* latency, not timestamp error.

What this does and does not buy you. The firmware's reading is taken within a
loop iteration of the edge, so the *event* is timestamped to a few µs. Turning
that into a host instant costs the accuracy of the clock offset, which is
bounded by the asymmetry of the sync round trip — a few hundred µs, not 4 µs. So
RT accuracy is sub-millisecond rather than microsecond, against the 5 ms-plus
floor of `WaitForInput`. Reproduce with
`go run ./tests/test_megttlbox -rtloop 50`.

**Queue overflow is not silent.** The firmware holds 32 events (160 bytes); if
the host does not drain them it sets a sticky flag, surfaced by `EventsDropped()`
(read-and-clear). That means presses were *lost*, not delayed, so the trial
should be treated as suspect. In practice it takes a mechanical button
chattering — hence `SetDebounce`, which is **off by default** because fibre-optic
pads do not bounce and suppressing real transitions is worse than reporting extra
ones.

**Inputs are `INPUT_PULLUP`** and reported with `invert=true`: a line pulled LOW
reads as 1 ("pressed"), an unconnected line reads 0. Unlike the DLP-IO20, an
unwired box cannot generate spurious `WaitForInput` triggers. For a jumpered
loopback (D30→D22 …) this means `ReadAll() == ^mask` after `Send(mask)`.

## FT232HTrigger (Adafruit FT232H, Linux)

Implements both interfaces. Pure-Go driver via Linux usbfs — no libftdi or D2XX required.

**Wiring:**
- AD0–AD7 (D-bus) → 8 TTL output lines for trigger codes
- AC0–AC7 (C-bus) → 8 TTL input lines for response pads

```go
box, err := triggers.NewFT232H(
    triggers.WithFT232HPollInterval(5*time.Millisecond), // optional
)
if err != nil { log.Fatal(err) }
defer box.Close()

box.Send(0b00000001)               // line 0 HIGH (persistent)
box.Pulse(0, 5*time.Millisecond)   // line 0: HIGH for 5 ms, then LOW
box.AllLow()

_ = box.DrainInputs(ctx)
mask, rt, _ := box.WaitForInput(ctx)

// Auto-detect (falls back to NullOutputTTLDevice if not found):
out, err := triggers.AutoDetectFT232H()
defer out.Close()
```

**Prerequisites (Linux):**
- `ftdi_sio` kernel module must not hold the device: `sudo rmmod ftdi_sio`
- User needs rw access to `/dev/bus/usb/BBB/DDD`; add a udev rule or join `plugdev`:
  ```
  ACTION=="add", SUBSYSTEM=="usb", ATTRS{idVendor}=="0403", ATTRS{idProduct}=="6014", MODE="0666", GROUP="plugdev"
  ```

**Internal protocol:** MPSSE mode via direct usbfs ioctls (VID=0x0403, PID=0x6014). GPIO commands: `SET_BITS_LOW` (0x80) for AD0–AD7, `GET_BITS_HIGH` (0x83) for AC0–AC7. Every bulk IN packet is prefixed by 2 modem-status bytes.

## FORPButton

```go
// Each constant is the 0-indexed line number = bit position in the bitmask.
triggers.FORPLeftBlue    // 0, D22, STI007
triggers.FORPLeftYellow  // 1, D23, STI008
triggers.FORPLeftGreen   // 2, D24, STI009
triggers.FORPLeftRed     // 3, D25, STI010
triggers.FORPRightBlue   // 4, D26, STI012
triggers.FORPRightYellow // 5, D27, STI013
triggers.FORPRightGreen  // 6, D28, STI014
triggers.FORPRightRed    // 7, D29, STI015

buttons := triggers.DecodeMask(mask)  // []FORPButton, ordered low→high bit
fmt.Println(buttons[0])               // "left blue"
```

## LinuxGPIOTrigger (Raspberry Pi and other Linux SBCs)

Implements both interfaces via the Linux GPIO character device v2 API (kernel ≥ 5.10). Works on any Linux SBC with GPIO — Raspberry Pi, Rock Pi, BeagleBone, Jetson, etc. No libraries or kernel modules required beyond read-write access to `/dev/gpiochip0`.

**Wiring:** any 8 GPIO pins for output, any other 8 for input. Pin numbers are chip-relative offsets (= BCM numbers on Raspberry Pi).

```go
box, err := triggers.NewLinuxGPIOTrigger(
    triggers.WithGPIOOutputPins([8]int{17, 27, 22, 5, 6, 13, 19, 26}),
    triggers.WithGPIOInputPins([8]int{12, 16, 20, 21, 4, 25, 24, 23}),
    triggers.WithGPIOChip("/dev/gpiochip0"),          // optional, this is default
    triggers.WithGPIOPollInterval(5*time.Millisecond), // optional
)
if err != nil { log.Fatal(err) }
defer box.Close()

box.Send(0b00000001)              // pin 17 HIGH
box.Pulse(0, 5*time.Millisecond)  // pin 17: HIGH for 5 ms, then LOW

_ = box.DrainInputs(ctx)
mask, rt, _ := box.WaitForInput(ctx)
```

Output-only and input-only configurations are both valid; omit the unused option.

**Prerequisites:**
- Kernel ≥ 5.10 (GPIO character device v2 API)
- User in the `gpio` group or `/dev/gpiochip0` accessible: `sudo usermod -aG gpio $USER`

**Internal protocol:** `GPIO_V2_GET_LINE_IOCTL` (0xC250B407) to claim 8 lines, `GPIO_V2_LINE_SET_VALUES_IOCTL` (0xC010B40E) and `GPIO_V2_LINE_GET_VALUES_IOCTL` (0xC010B40F) for atomic byte read/write. The `init()` function panic-checks struct size 592 at startup to catch any layout drift.

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

## ParallelPort (Linux LPT)

Implements `OutputTTLDevice`. Uses ppdev ioctl (`/dev/parport0..3`).

**Unload the `lp` module, not just the `lp` group.** Two prerequisites share
that name and only one of them is about permissions: the *group* `lp` owns the
device node, while the *module* `lp` is the parallel printer driver. Both `lp`
and `ppdev` can be registered on one port, and `Open`'s `PPCLAIM` then goes
through `parport_claim_or_block` — with `lp` holding the port the ioctl blocks
in **uninterruptible sleep**, surviving Ctrl-C and `kill -9` alike. Intermittent,
since `lp` holds the port only some of the time. Diagnosed 2026-08-21 on a PCIe
LPT card whose `-blink` run had to be ended by powering the machine off.

`dmesg` names it: `lp0: using parport0 (interrupt-driven).` Fix with
`sudo rmmod lp`, made permanent with a `blacklist lp` in `/etc/modprobe.d/`.
That also leaves the port's IRQ unarmed, at no cost — ppdev writes do not use
the interrupt.

```go
pp := triggers.NewParallelPort("/dev/parport0")
if err := pp.Open(); err != nil { log.Fatal(err) }
defer pp.Close()
pp.Send(0b00000111)   // lines 0,1,2 HIGH
pp.Pulse(0, 10*time.Millisecond)
status, _ := pp.ReadStatus()   // status register (Linux only)
```

**Prerequisites:** `sudo modprobe ppdev`; user in `lp` group.

## SerialPort (generic UART)

Does **not** implement either TTL interface. General-purpose serial wrapper.

```go
sp := triggers.NewSerialPort("/dev/ttyUSB0", 9600)
sp.Open(); defer sp.Close()
sp.Send(0x42); sp.SendLine("GO", false, true)
b, _ := sp.Poll()
line, _ := sp.ReadLine()
```

## NetStation (EGI EEG, ECI over TCP/IP)

Does **not** implement either TTL interface. It marks *named events* in the EEG
stream of an EGI/NetStation host and controls recording remotely, over TCP —
the network equivalent of sending a trigger code, but with 4-char event codes,
durations, and key/value payloads instead of 8 electrical lines.

```go
ns, err := triggers.NewNetStation("134.225.198.12")  // default port 55513
if err != nil { log.Fatal(err) }
defer ns.Close()                 // stops recording + disconnects (blocks ~2 s to flush)

ns.Synchronize()                 // align host clock to ours (call once after connect)
ns.StartRecording()
ns.SendEvent("STIM")             // now, 1 ms, no keys — send near the VSYNC flip
ns.SendEventFull(triggers.Event{ // full form
    Code:     "RESP",
    Start:    flipTime,          // zero = now
    Duration: 2 * time.Millisecond,
    Keys:     []triggers.EventKey{{Code: "corr", Value: 1}},
})
ns.StopRecording()
```

**Protocol (ECI):** on connect the driver advertises the `QNTEL`
(Intel/little-endian) variant and every multi-byte field is encoded
little-endian, so it is portable regardless of the CPU it runs on (do **not**
switch to a native byte order). The handshake is `QNTEL` → `I`+version byte,
then an `A` (attention), matching the NetStation 5-tested reference client.
Commands: `A`+`T`(int32 ms) = synchronize, `B` = start, `E` = stop,
`D`+block = event, `X` = end session. Event block layout: `uint16` size
(`15+12·keys`), `int32` start ms, `int32` duration ms, 4-byte code, `uint8`
label length (0), `uint8` description length (0), `uint8` key count, then per
key: 4-byte code + `"shor"` + `int16` length (2) + `int16` value. Timestamps
are ms from an epoch fixed at connect. Ported from Gergely Csibra's NetStation
MATLAB routines (2006), cross-checked against
[egi-pynetstation](https://github.com/nimh-sfim/egi-pynetstation) (tested on
NetStation 5.3). Options: `WithNSTimeout`.

**Acknowledgements — every command is checked.** The host answers each command
with one byte:

| Byte | Meaning |
|---|---|
| `Z`, `0x01`, `S` | accepted |
| `F` | command refused |
| `R` | no recording device ready (no session open / no amplifier) |

Only `Z` is documented; the rest come from the reference clients' testing. The
driver used to read the ack and **discard** it, so a `B` the host had refused
still reported success and the run continued to an unusable file. `checkAck`
now turns `F`/`R`/unknown into errors.

**The .mff file is not ours to control.** ECI has nine commands and none of
them names the output file, selects a format or finalizes it — NetStation
Acquisition writes the bundle. What the client *can* do wrong is leave a
recording open: an .mff containing `Acquiring.xml` and no `info.xml` (events
and signal both unreadable) is EGI's documented signature of an acquisition
that never completed. Hence:

- `Close` sends `E` before `X` when a recording is still open, and **returns**
  any failure instead of discarding it — always check or log it.
- Callers must not `log.Fatal` between `StartRecording` and `Close`:
  `log.Fatal` skips deferred functions, so the host keeps acquiring. Use the
  `run() error` pattern in `tests/test_netstation/main.go`, which also traps
  Ctrl-C.

Protocol behaviour is covered by `triggers/netstation_test.go`, which drives
the client against an in-process fake ECI host — no amplifier needed.

## VideoRecorder (BEL_video, labelled camera over TCP/IP)

Does **not** implement either TTL interface. Client for the NeuroSpin EEG-room
**video recorder** ("BEL_video"): a camera PC films the participant, burns event
labels into the footage, and saves an AVI. This type starts/stops that recording
and pushes the labels — the capture/encoding stay on the camera PC. Same family
as `NetStation` (event-marker + recording-control over TCP), *not* an eye tracker.

```go
vr, err := triggers.NewVideoRecorder("192.168.8.212")  // default port 55113
if err != nil { log.Fatal(err) }
defer vr.Close()             // stops recording + disconnects

vr.Start()
vr.SetSubject("bb0012025")   // "NIP:<id>" — names the output file
vr.Label("TRL", "001")       // "KEY:VALUE" overlay until next label / timeout
vr.Label("CND", "007")
vr.Stop()
```

**Protocol:** plain ASCII, **no framing and no ack** (fire-and-forget, unlike
NetStation). `START` begins, `STOP` finalizes, any other message is a
`KEY:VALUE` overlay label (`NIP:<id>` names the file). Because the server reads
one socket buffer per captured frame, messages sent too close together can
arrive coalesced and be mis-parsed; `VideoRecorder` waits `SendGap` (default
50 ms) after each message (`WithVRSendGap` to tune, 0 to disable). Options:
`WithVRTimeout`, `WithVRSendGap`. Ported from videoComm.m / videoCommClient.py.

## Key conventions

- Always `defer dev.Close()` — drives all lines LOW and releases the port.
- For `OutputTTLDevice`, send the trigger as close as possible to the `exp.ShowNS` VSYNC flip; latency is typically <1 ms.
- For `InputTTLDevice`, call `DrainInputs(ctx)` before `WaitForInput(ctx)` between trials to clear latched presses.
- To use a MEGTTLBox or DLPIO8 as a `ResponseDevice` in the `apparatus` package: `apparatus.NewTTLResponseDevice(box, 5*time.Millisecond)`.
