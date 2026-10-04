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

