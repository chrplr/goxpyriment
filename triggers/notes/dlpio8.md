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

