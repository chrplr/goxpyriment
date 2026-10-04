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

