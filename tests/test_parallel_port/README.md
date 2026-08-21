# test_parallel_port

Interactive smoke-test for the Linux LPT parallel port driver (`triggers/parallel_linux.go`).

## Prerequisites

```bash
sudo modprobe ppdev
sudo usermod -aG lp $USER   # the GROUP lp — rw access to /dev/parport0
sudo rmmod lp               # the MODULE lp — the printer driver
```

The last two are different things that happen to share a name: `lp` the Unix
group owns the device node, `lp` the kernel module is the parallel *printer*
driver. Both it and `ppdev` can be registered on one port, and then `Open`'s
`PPCLAIM` goes through `parport_claim_or_block` — if `lp` is holding the port,
the ioctl blocks in **uninterruptible sleep**. The process then survives Ctrl-C
*and* `kill -9` until `lp` releases it. It is intermittent, because `lp` only
holds the port some of the time.

Check for it in `dmesg`:

```
[    2.929160] lp0: using parport0 (interrupt-driven).   ← lp is attached
```

To keep it away across reboots:

```bash
echo 'blacklist lp' | sudo tee /etc/modprobe.d/blacklist-lp.conf
```

That also leaves the port's IRQ unarmed, which costs nothing: `ppdev` writes
do not use the interrupt.

## Usage

```bash
go run main.go /dev/parport0
```

If no device is given the program lists accessible ports and exits.

## What it tests

| Step | Operation | What to verify |
|------|-----------|----------------|
| 1 | `Send()` with 8 bitmasks (0x00–0xFF range) | Data pins D0–D7 match each bitmask |
| 2 | `SetHigh` / `SetLow` per line | Each data line toggles independently |
| 3 | `Pulse(line 0, 100 ms)` | D0 goes HIGH for ~100 ms then LOW |
| 4 | `ReadStatus()` | Status register byte printed with per-bit decode |

All data lines are driven LOW on exit.

## Hardware setup

Connect a breakout board or logic analyser to the DB-25 connector to observe
the data lines (pins 2–9 = D0–D7) and status lines (pins 10–13, 15).
A loopback cable (data → status) lets you verify round-trip read/write without
external equipment.
