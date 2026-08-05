# BBTK capture — MEG TTL box pulse timing

Recorded 2026-08-05 on host `is158520`.

## Setup

| | |
|---|---|
| Device under test | Arduino Mega 2560 R3, `/dev/ttyACM0` |
| Firmware | `neurospin-meg-ttl-box` protocol v1, `caps 0x03` (atomic port + timestamps) |
| Instrument | BBTKv3, `/dev/ttyUSB0`, 0.25 ms sampling |
| Wiring | line 0 → D30 → TTLin2, line 1 → D31 → TTLin1, common GND |

Smoothing is irrelevant here — on the BBTK it applies only to `Opto*` and `Mic*`
channels, never to TTL inputs.

## Command

```bash
./run-bbtk.sh                                    # produced these files
./analyse-bbtk.py bbtk-megttlbox-001-events.csv  # produced the numbers below
```

## Sequence

101 events captured, 101 expected — nothing missed or spurious.

| segment | pulses | width | lines |
|---|---|---|---|
| marker | 1 | 100 ms | 0 |
| block A | 20 | 5 ms | 0 |
| block B | 20 | 10 ms | 0 |
| block C | 20 | 20 ms | 0 |
| block D | 20 | 10 ms | 0 **and** 1 together |

## Results

**Pulse width** — device-timed from `millis()`, so the realised width is uniform
on [w−1, w]:

| requested | min | median | max | mean error |
|---|---|---|---|---|
| 5 ms | 3.75 | 4.50 | 5.00 | −0.53 ms |
| 10 ms | 8.25 | 9.25 | 10.25 | −0.68 ms |
| 20 ms | 18.50 | 19.50 | 20.25 | −0.69 ms |

Pulses are **systematically ~0.5–0.7 ms short**. That is a bias, not noise, and
cannot be averaged away: ask for 1 ms more than you need. Spread of 1.25–2.0 ms
is barely above the 1.25 ms floor from truncation plus the BBTK's own sampling,
so firmware jitter is well under a millisecond.

**Inter-line skew (block D)** — one command pulses both lines, and the firmware
writes the port in a single instruction. All 20 trials put both rising edges in
the **same 0.25 ms sample**: skew < 250 µs, measured externally rather than by
the firmware judging itself.

**Onset-to-onset intervals** ran ~1.5 ms longer than requested in every block.
That is per-command host→device latency, and it independently reproduces the
1.44 ms median measured the same day by an unrelated method (firmware timestamp
of a loopback edge vs. host clock).

## Files

| file | contents |
|---|---|
| `bbtk-megttlbox-001-events.csv` | one row per event: type, onset, duration — the file `analyse-bbtk.py` reads |
| `bbtk-megttlbox-001-dscevents.csv` | per-sample state of every BBTK channel |
| `bbtk-megttlbox-001.dat` | raw BBTK download |
| `bbtk-megttlbox.log` | capture-side log (device setup, countdown) |
| `megttlbox-sequence.txt` | stimulus-side log: the schedule as emitted |
