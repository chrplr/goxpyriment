## SerialPort (generic UART)

Does **not** implement either TTL interface. General-purpose serial wrapper.

```go
sp := triggers.NewSerialPort("/dev/ttyUSB0", 9600)
sp.Open(); defer sp.Close()
sp.Send(0x42); sp.SendLine("GO", false, true)
b, _ := sp.Poll()
line, _ := sp.ReadLine()
```

