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

Implements both interfaces. **Untested on hardware** — written from the
[datasheet](https://www.dlpdesign.com/usb/dlp-io20-ds-v11.pdf) rev 1.1.

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
`echo 1 | sudo tee /sys/bus/usb-serial/devices/ttyUSB0/latency_timer`.

**5 V logic**, unlike the LabJack T4's 3.3 V. Inputs have no pull-ups, so patch
a pin to +5V or GND — a floating input reads unpredictably.

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

**Wire protocol (opcodes):**

| Opcode | Args | Description |
|--------|------|-------------|
| 10 | uint16 LE (ms) | set trigger pulse width |
| 11 | uint8 mask | pulse all set lines |
| 12 | uint8 line | pulse single line |
| 13 | uint8 mask | set lines HIGH (persistent) |
| 14 | uint8 mask | set lines LOW (persistent) |
| 15 | uint8 line | set single line HIGH |
| 16 | uint8 line | set single line LOW |
| 20 | — | read button mask → returns uint8 |

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
switch to a native byte order). Commands: `A`+`T`(int32 ms) = synchronize,
`B` = start, `E` = stop, `D`+block = event, `X` = end session. Each command
reads a one-byte ack. Event block layout: `uint16` size (`15+12·keys`),
`int32` start ms, `int32` duration ms, 4-byte code, `int16` label length (0),
`uint8` key count, then per key: 4-byte code + `"shor"` + `int16` length (2) +
`int16` value. Timestamps are ms from an epoch fixed at connect. Ported from
Gergely Csibra's NetStation MATLAB routines (2006). Options: `WithNSTimeout`.

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
