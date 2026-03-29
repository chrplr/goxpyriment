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

## Key conventions

- Always `defer dev.Close()` — drives all lines LOW and releases the port.
- For `OutputTTLDevice`, send the trigger as close as possible to the `exp.ShowNS` VSYNC flip; latency is typically <1 ms.
- For `InputTTLDevice`, call `DrainInputs(ctx)` before `WaitForInput(ctx)` between trials to clear latched presses.
- To use a MEGTTLBox or DLPIO8 as a `ResponseDevice` in the `io` package: `io.NewTTLResponseDevice(box, 5*time.Millisecond)`.
