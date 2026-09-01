# test_eyelink

Checks an SR Research EyeLink reached through `eyelink_bridge.py`, and measures
what the bridge costs.

## What it measures

Every trial flips a white patch, then does the same thing twice by two
different routes:

1. **A TTL pulse**, raised on the flip thread immediately after the flip. The
   EyeLink Host PC records the edge on its parallel port as an `INPUT` event,
   timestamped by the Host itself.
2. **A message through the bridge**, sent straight after. The Host records it as
   a `MSG`, timestamped when it arrives.

Both land in the same EDF on the same clock, so the gap between each `INPUT` and
the `MSG` that follows it *is* the bridge's added latency — measured by the
tracker, not inferred from this side. That is the number that decides whether a
message can ever be used to time a stimulus. (It cannot; this test is how you
show by how much.)

It also synchronises the two clocks before and after the run, so the difference
gives the drift, and streams gaze samples throughout to check the link is alive.

## Wiring

```
display PC ──TTL out── EyeLink Host PC parallel port (INPUT)
display PC ──ethernet── EyeLink Host PC (the SDK link, via the bridge)
```

The Host records TTL edges as `INPUT` events only when the EDF keeps them; the
bridge asks for that at open (`file_event_filter` includes `INPUT`).

## Running it

Start the bridge in one terminal:

```bash
python3 eyetracker/bridge/eyelink_bridge.py --tracker-host 100.1.1.1

# or, with no hardware in the room:
python3 eyetracker/bridge/eyelink_bridge.py --simulate
```

Check the SDK and the Host answer before anything else:

```bash
python3 eyetracker/bridge/eyelink_bridge.py --check --tracker-host 100.1.1.1
```

Then, from the repo root:

```bash
# no TTL device — exercises the bridge only
go run ./tests/test_eyelink -w -s 999

# the real measurement, through a parallel port
go run ./tests/test_eyelink -s 999 -trigger parport -device /dev/parport0 \
    -trials 50 -fetch /tmp/goxtest.edf

# or through the MEG TTL box (Arduino-based, /dev/ttyACM0 by default)
go run ./tests/test_eyelink -s 999 -trigger megttl -device /dev/ttyACM0 \
    -trials 50 -fetch /tmp/goxtest.edf

# see the gaze on screen first, to confirm the stream and the calibration
go run ./tests/test_eyelink -w -s 999 -gaze
```

Useful flags: `-trigger none|parport|dlpio8|dlpio20|megttl`, `-device`, `-line`,
`-pulse`, `-trials`, `-isi`, `-calibrate`, `-points`, `-fetch`, `-sync`.
`-w` runs windowed, `-s` sets the subject ID. Escape aborts between trials.

## Samples, and what "dropped" means

The trial loop drains the sample buffer every trial and records `n_samples`,
which is the link's own pulse: a trial that receives far fewer samples than the
tracker's rate times the ISI means the stream stalled.

A non-zero `dropped` count refers to **this program's** ring buffer overflowing
between drains. The EDF on the Host PC is written by the Host and is complete
regardless. Nothing in this test reads gaze from the buffer for timing, so a
drop never affects a figure reported here — it is a symptom to explain, not a
hole in the data.

## Reading the results

The CSV written by the run holds, per trial, the flip timestamp, the flip → TTL
gap, and the full bridge round trip, all measured on this machine.

The EDF holds the other half. Convert and compare:

```bash
edf2asc /tmp/goxtest.edf
grep -E '^(INPUT|MSG)' /tmp/goxtest.asc | head -20
```

Each `INPUT` should be followed by the `MSG` for the same trial. The difference
between their timestamps is the bridge latency on the Host clock.

## Known gap

Calibration graphics. `-calibrate` asks the tracker to run its own setup, which
needs somewhere to draw the targets; pylink's built-in graphics are not
available everywhere and goxpyriment owns the display here. The test reports
this and carries on — gaze *positions* are then meaningless, but every timing
figure above is unaffected, since none of them depends on where the eye is.
