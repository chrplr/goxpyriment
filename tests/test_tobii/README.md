# test_tobii

Checks a Tobii Pro tracker reached through `tobii_bridge.py`, and measures what
it actually delivers.

## What it measures

**1. Which way up are the coordinates (`-corners`).** Tobii reports gaze in
normalized active-display-area coordinates, and the SDK headers shipped with the
SDK never say where the origin is. Tobii's published documentation says
top-left, and the bridge assumes it, but an assumption is not a measurement. So
this mode shows a target in each corner and the centre, averages the gaze during
each, and prints the measured normalized position beside the expected one —
then states the verdict by comparing the residual against a mirrored
expectation:

```
target          expected nx,ny    measured nx,ny    samples
centre          0.50, 0.50        0.503, 0.497     587
top-left        0.10, 0.10        0.112, 0.104     583
...
mean |Y error| as-is: 0.011    if Y were mirrored: 0.788
the normalized origin is TOP-LEFT, as the bridge assumes
```

Get this wrong and every gaze position in every recording is mirrored
vertically, which reads as a bad calibration rather than a units bug. **Run it
first**, and put its output in the commit that settles the question.

**2. Calibration.** Nothing in the Tobii SDK draws a target: `collect_data(x,y)`
assumes the participant is already looking at `(x,y)` and blocks while it
samples. goxpyriment draws the targets itself, through
`control.Experiment.CalibrateTracker`, on its own flip clock and in its own
window. The stored result — including any target that yielded no usable data,
which names the corner the participant never looked at — goes into the
`-info.txt`.

**3. Sample rate.** Counted over a measured interval and reported *with* the
count and the duration, not as a bare rate. The bridge emits one sample event
per eye, so both the event rate and the implied binocular gaze rate are printed:
reporting only one of them is how a 600 Hz tracker gets written up as 1200 Hz. A
nonzero `Dropped()` is a finding — it means the record has holes.

**4. Clock offset and drift.** Sampled before and after the run. Tobii's
`system_time_stamp` is `CLOCK_MONOTONIC` microseconds on the *bridge's* machine
(measured: mean offset −3.2 µs against `clock_gettime(CLOCK_MONOTONIC)`, n=2000,
min −19.8, max +4.7), so with the bridge running on the display machine the
offset should be a constant rather than a drifting rate. The drift figure is
what tests that claim.

## No tracker-side data file

This is the structural difference from the EyeLink. An EyeLink Host PC writes an
EDF and `receive_file` pulls it off; Tobii samples exist only inside an SDK
callback. So **the bridge writes the record**: a full-field TSV at the tracker's
full rate, with both timestamps, both eyes, gaze in normalized *and* pixel
coordinates, gaze origin in mm, and pupil diameter — plus a `#`-commented header
carrying the model, serial, rate, display area and the screen resolution the
pixel columns were derived from. `-fetch` copies it here.

The reduced `sample` events go over the socket as well, for `Latest()` and
gaze-contingent loops. The full-fidelity record therefore never crosses the
socket, and a slow client cannot put holes in it.

The CSV *this program* writes is a different and much coarser thing: one row per
fixation trial, measured from this side.

## Pupil units differ between makes

`Sample.PupilArea` carries pupil **diameter in millimetres** here (roughly 2–8).
On the EyeLink bridge the same field is pupil **area** in the tracker's own
arbitrary units (in the thousands). Nothing in the type can catch a confusion
between the two, so the unit is written into the gaze file's header and into the
run's `-info.txt`. Comparing pupil sizes across makes without converting is
meaningless.

## Running it

First — this is the step that can waste a whole rig session, so nothing else
comes before it:

```bash
python3 eyetracker/bridge/tobii_bridge.py --check
```

The SDK is a native extension and is not pip-installed. It has to be on
`PYTHONPATH`; on this machine `~/.bashrc` already puts
`~/tobii_eyetracker_pythonlib` there, so no extra setting is needed and
`--check` confirms it (`tobii_research: importable, SDK version 2.1.0.1`). On
another machine, prefix the command with
`PYTHONPATH=/path/to/tobii_eyetracker_pythonlib`.

`--check` reports whether the SDK imports, what trackers are reachable, and each
one's model, serial, firmware, available sample rates, modes and display area,
then exits.

Once that looks right, start the bridge in one terminal:

```bash
python3 eyetracker/bridge/tobii_bridge.py --edf-dir /tmp
```

and in another:

```bash
# settle the coordinate origin first
go run ./tests/test_tobii -w -s 999 -corners

# then a calibrated run with a live gaze dot
go run ./tests/test_tobii -w -s 999 -calibrate -gaze -fetch /tmp/gaze.tsv
```

With no hardware at all:

```bash
python3 eyetracker/bridge/tobii_bridge.py --simulate
go run ./tests/test_tobii -w -s 999
```

The simulator reports both eyes with a small vergence offset and blinks
regularly, so the per-eye split and the invalid-sample path are both exercised;
it says so in the hello event, in the gaze file's header, and on this program's
output, because a simulated run mistaken for a real one is worse than no run.

## Flags

| Flag | Meaning |
|---|---|
| `-bridge` | address of `tobii_bridge.py` (default `127.0.0.1:5010`) |
| `-tracker-address` | tracker URI, e.g. `tet-tcp://169.254.1.2` (empty: the bridge's choice) |
| `-file` | name of the gaze TSV the bridge writes |
| `-fetch` | after the run, copy the gaze file here |
| `-corners` | measure the coordinate origin, then exit |
| `-calibrate` | calibrate before recording (goxpyriment draws the targets) |
| `-points` | calibration targets: 3, 5, 9 or 13 |
| `-dwell` | ms each calibration target is shown before it is sampled |
| `-gaze` | show a live gaze dot until a key is pressed |
| `-trials`, `-hold`, `-isi` | fixation trials and their timing |
| `-rate-secs` | seconds over which to measure the sample rate (0 skips it) |
| `-sync` | clock round trips per synchronisation |

Plus the usual `-w` (windowed), `-d N` (display index) and `-s <id>` (subject).

## What this test cannot tell you

The gaze-to-target distance it prints is accuracy **only if the participant
actually fixated each target**. On a simulated run, or with an empty chair, it
is a number about a Lissajous curve. The output says so; do not quote it
otherwise.

Tobii's TTL input (`EYETRACKER_EXTERNAL_SIGNAL`) is not used here. That is the
stream that timestamps an edge at the tracker, and it is the right way to mark a
stimulus onset — the same argument as the EyeLink's parallel-port `INPUT`
events, and the reason `Mark` must not be used for an onset whose timestamp is
the measurement. Wiring and measuring it is a separate job.
