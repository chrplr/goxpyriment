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

**Wiring:**
- FIO0–FIO7 → 8 TTL output lines for trigger codes (configured as outputs on open)
- EIO0–EIO7 → 8 TTL input lines for response pads (configured as inputs on open)

```go
box, err := triggers.NewLabJackT4("192.168.1.100",
    triggers.WithT4PollInterval(5*time.Millisecond), // optional
    triggers.WithT4Timeout(1*time.Second),            // optional
    triggers.WithT4UnitID(1),                         // optional
)
if err != nil { log.Fatal(err) }
defer box.Close()

box.Send(0b00000001)              // FIO0 HIGH
box.Pulse(0, 5*time.Millisecond)  // FIO0: HIGH for 5 ms, then LOW

_ = box.DrainInputs(ctx)
mask, rt, _ := box.WaitForInput(ctx)
```

**Internal protocol:** Modbus TCP on port 502 via `github.com/goburrow/modbus`.
FC6 (`WriteSingleRegister`) for output; FC3 (`ReadHoldingRegisters`) for input.

| Register | Address | Description |
|----------|---------|-------------|
| `FIO_STATE` | 2500 | FIO0–FIO7 output bitmask |
| `EIO_STATE` | 2501 | EIO0–EIO7 input bitmask |
| `FIO_DIRECTION` | 2504 | 0x00FF = all FIO lines outputs |
| `EIO_DIRECTION` | 2505 | 0x0000 = all EIO lines inputs |

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
