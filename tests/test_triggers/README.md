# test_triggers

Fires the same pulse train on **two or more TTL output devices at once**, so an
oscilloscope can measure how far apart their edges actually land.

Every device in `triggers/` has been characterised alone — `test_megttlbox`,
`test_labjackt4`, `test_linuxgpio`, `test_parallel_port`, `test_ft232h`, and
`Timing-Tests -trigger-device X`. Nothing put two of them on one timebase. This
program does: the devices are pulsed on one absolute schedule, and the
instrument reads the difference between the channels, so its own clock cancels
and neither device's numbers have to be reconciled with the other's.

```bash
# parallel port against a DLP-IO8: 40 pulses, 5 ms wide, every 500 ms
go run ./tests/test_triggers \
    -device parallel:pin=1 \
    -device dlpio8:port=/dev/ttyUSB0,pin=1

# what naive experiment code does: both writes from one thread, in order
go run ./tests/test_triggers -sequential \
    -device parallel:pin=1 -device dlpio8:port=/dev/ttyUSB0,pin=1

# no instrument: let a MEG TTL box timestamp the other devices' edges itself
go run ./tests/test_triggers -loopback 3 \
    -device parallel:pin=1 -device gpio:pin=1 -device megttlbox:port=/dev/ttyACM0

# check the program itself on a machine with no trigger hardware
go run ./tests/test_triggers -device null:pin=1 -device null:pin=2 -no-prompt
```

## Naming devices

One repeatable `-device KIND[:key=value,...]` flag. Parameters are separated by
commas, so the GPIO pin list joins with `+`:

| kind | parameters | probe point for `pin=N` |
|---|---|---|
| `dlpio8` | `port=` (empty → auto-detect) | terminal *N* on the block, 5 V |
| `dlpio20` | `port=` (empty → auto-detect) | `AN`*(N−1)*, 5 V |
| `megttlbox` | `port=` (**required**) | Arduino Mega `D`*(29+N)*, 5 V |
| `parallel` | `port=` (empty → first accessible) | DB25 pin *N*+1 (= `D`*(N−1)*), 5 V |
| `gpio` | `chip=`, `pins=17+27+22+5+6+13+19+26` | the *N*th entry of `pins=`, **3.3 V** |
| `ft232h` | — | `AD`*(N−1)*, **3.3 V** |
| `labjackt4` | `host=` (**required**) | `FIO4`…`EIO3` for *N*=1…8, **3.3 V** |
| `null` | — | nothing at all — for testing this program |

`pin=` is 1–8, as the hardware is labelled, and defaults to 1. The program
prints the resolved probe point for every device before it fires anything, and
waits for Enter (`-no-prompt` skips the wait).

Two specs of the same kind with different pins are legal and useful: that
measures the skew between two lines of one device.

A device that will not open is **fatal** here. `Timing-Tests` degrades to a
silent no-op device to save the visual half of its measurement; in a comparison
run that would put a flat trace on a scope channel, and the loss would only be
discovered when the capture was read.

## Wiring

- One instrument channel per device output, **grounds common** — between the
  devices and with the instrument. Two boxes on separate grounds can show a
  skew that is entirely an artefact of the reference.
- Check the instrument's logic threshold before a long capture: `gpio`,
  `ft232h` and `labjackt4` swing 0–3.3 V, the rest 0–5 V.
- An Analog Discovery 3 has two analog channels. For more than two devices use
  its digital (logic-analyser) inputs, which do not care about the 3.3/5 V
  difference as long as the threshold is set once for all of them.
- On FTDI-based devices, lower the latency timer before a run that also reads
  inputs: `echo 1 | sudo tee /sys/bus/usb-serial/devices/ttyUSB0/latency_timer`.

## How it fires

By default each device gets its **own OS thread**, locked, optionally at
`SCHED_FIFO` priority (`-realtime-priority`, default 50). All threads wait on
the *same absolute deadline* and busy-spin the last 1.5 ms into it; they are
never woken through a channel, because a per-repetition broadcast would
re-serialise exactly what is being measured. The GC is suspended for the whole
train unless `-gc` is given.

`-sequential` instead issues every write from one thread in `-device` order.
That is what experiment code pulsing two boxes in a row does, and the difference
between the two modes is what the first device's blocking write costs the one
behind it.

`-warmup N` pulses (default 5) are fired before the recorded block. They are
written to the data file with `phase=warmup` and `rep` counting −N…−1, and left
out of the statistics: the first write to a USB device wakes a driver and is
measurably different from the rest.

## `-loopback N` — a skew number without an instrument

The Nth device becomes a **witness**: it fires nothing, and its inputs record
the other devices' edges. Only a MEG TTL box with `CAP_TIMESTAMPS` qualifies —
every other input path in `triggers/` polls at ~5 ms plus USB jitter, which
cannot resolve the quantity being measured; anything else is refused rather than
quietly reported.

Wire source *i* (in `-device` order, witness skipped) to witness input line *i*
— `D22` is line 1 — or say otherwise with `-loopback-map 3,1`.

**The polarity is inverted, by design of the box**: its inputs are
`INPUT_PULLUP` and reported with `invert=true`, so a bit *set* means the line is
LOW. An idle source therefore reads as "pressed", and a source's **rising** edge
appears as a bit **clearing**. The program reads it that way.

Why this number is trustworthy even though the box's absolute host alignment is
only good to a few hundred µs: both edges are stamped by the same `micros()`
counter, sampled every firmware loop iteration, so the device→host clock offset
cancels in the *difference*. Resolution floor is 4 µs. If `EventsDropped()` comes
back set, edges were **lost, not delayed**, and the run is marked suspect.

## Output

`$HOME/goxpy_data/test_triggers_sub-999_date-*.csv` (`-outdir` to change), with
the run's conditions and the printed report in the companion `-info.txt`:
devices and their resolved lines, ISI, width, mode, real-time priority, GC
state, and the host/kernel facts from `sysinfo`.

One row per (repetition × device):

| column | meaning |
|---|---|
| `rep`, `phase` | repetition index; `warmup` rows have a negative `rep` |
| `dev_index`, `dev_kind`, `dev_label`, `line` | which device, which line |
| `target_ns` | the repetition's scheduled onset, from the first pulse |
| `pre_high_ns`, `post_high_ns` | host clock either side of the `SetHigh` call |
| `pre_low_ns`, `post_low_ns` | host clock either side of the `SetLow` call |
| `write_high_us` | how long the `SetHigh` call blocked |
| `issue_skew_us` | `pre_high` minus the reference device's, same repetition |
| `width_us` | host-side interval between the two calls |
| `err` | non-empty if a write failed — read this before using a run |
| `wit_*` | witness timestamps and skew, in `-loopback` runs only |

**These are host-side numbers.** They say when the program issued each write,
not when the edge reached the wire. The instrument's trace is the measurement;
the CSV says whether a skew seen there came from the device or from the host
having issued the writes late. Use `target_ns` to match a scope edge to a row:
repetition *k* was scheduled at *k* × ISI after the first pulse.

## Related

- `docs/TriggerJitterForEEGandMEG.md` — what to do about the jitter once it is
  measured
- `tests/test_photodiode_latency` — TTL against light, for the absolute
  flip→photons figure
- `tests/Timing-Tests` — the single-device timing harness this borrows its
  device handling from (`tests/internal/trigdev`)
