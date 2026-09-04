# test_eyelink

Checks an SR Research EyeLink reached through `eyelink_bridge.py`, and measures
what the bridge costs.

## What it measures

Every trial flips a white patch, then marks that flip twice by two different
routes:

1. **A TTL pulse**, raised on the flip thread immediately after the flip, on
   whatever device `-trigger` selects.
2. **A message through the bridge**, sent straight after. The Host PC records it
   in the EDF as a `MSG`, timestamped when it arrives.

The two do **not** currently meet in the same file — see *Wiring* below. So what
this test actually reports is measured on **this machine**: the flip → TTL gap,
and the full round trip of a bridge `Mark` (send, Python, tracker, reply). The
round trip is an upper bound on the bridge's one-way latency, and it is the
number that decides whether a `MSG` can be used to time a stimulus. It cannot:
three runs of 10–20 trials on the MEG rig gave a median of 600–719 µs and a
maximum of 1207 µs, against a frame of 8.3 ms at 120 Hz. Mark stimulus onsets
with `triggers.FireTriggerSync`, and use `MSG` only for labelling.

It also synchronises the two clocks before and after the run, so the difference
gives the drift, and streams gaze samples throughout to check the link is alive.

## Wiring

On the MEG rig the TTL and the gaze are joined in the **MEG acquisition**, not
in the EDF:

```
stim PC ──TTL out────── MEG STI channel   (stimulus onsets)
EyeLink Host PC ──X / Y / pupil (analog)── MEG MISC channels  (gaze)
stim PC ──ethernet───── EyeLink Host PC   (the SDK link, via the bridge)
```

Both signals are therefore on the MEG's clock, and stimulus onset is aligned
with gaze in the MEG file. The EDF is a second, independent record of the gaze,
carrying the bridge's `MSG` marks but **not** the TTL.

**Nothing is connected to the EyeLink Host's parallel-port input.** Holding
`0x00` and then `0xFF` on all eight lines of the TTL box both read `INPUT 127`
in the EDF — 127 is the idle value of an unconnected port. Until a cable runs
from a TTL source to the Host's DB25, the EDF contains no usable `INPUT` events,
and the Host-clock comparison of an `INPUT` against its `MSG` — the tracker
measuring the bridge's latency itself, rather than this side inferring it — is
not available. The bridge does ask for `INPUT` in `file_event_filter` at open,
so that measurement becomes possible the moment the cable exists, with no change
to this program.

## Running it

### Prequesites

* The Display (Stim) PC, on which you are going to run the commands listed below, must have the eyelink SDK installed as well as a Python environment where pylink ins installed.

* the eyetracking camera and illuminator must be powered on and connected to th Eyelink Host PC.

* The Eyelink Host PC must be running the calibration/acquisition program `C:\ELCL\EXE\ELCL.EXE`. 

* To setup the eyetracking camera, run `track` on the Stim PC.


### Launch the bridge on the Stim PC

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


### Run the test program in another terminal on the stim PC

(from the repo root)

```bash
# no TTL device — exercises the bridge only
go run ./tests/test_eyelink -w -s 999

# Run the calibration
go run ./test/test_eyelink -calibrate

# see the gaze on screen first, to confirm the stream and the calibration
go run ./tests/test_eyelink -w -s 999 -gaze

# with the MEG TTL box: pulses reach the MEG STI channel, and the run also
# measures the flip → TTL gap
go run ./tests/test_eyelink -s 999 -device megttlbox:port=/dev/ttyACM0,pin=1 \
    -trials 50 -fetch /tmp/goxtest.edf

# or through a parallel port — pin=N is the data line, 1-8 as D0-D7
go run ./tests/test_eyelink -s 999 -device parallel:port=/dev/parport0,pin=1 \
    -trials 50 -fetch /tmp/goxtest.edf

# the second LPT, driving D3 (DB25 pin 5)
go run ./tests/test_eyelink -s 999 -device parallel:port=/dev/parport1,pin=4

```

#### Naming the TTL device

`-device` takes one `KIND[:key=value,...]` spec, the same syntax as
`test_triggers` (`tests/internal/trigdev`); `-h` prints the full list. The kinds
that matter here are `megttlbox:port=…`, `parallel:port=…`, and `null` (the
default: no hardware, bridge only). Every kind also takes `pin=N`, **numbered
1-8 as printed on the hardware** — on a parallel port `pin=1` is D0, DB25 pin 2,
and `pin=N` is D(N-1), DB25 pin N+1; ground is any of DB25 pins 18-25, 5 V logic.

Leaving `port=` out of a `parallel:` spec takes the first accessible port. On
the stim PC, which has two, the run prints which one it chose and what the
alternatives were — but name the port explicitly rather than relying on the
enumeration order.

The port must be usable from userspace: `sudo modprobe ppdev`, rw access to the
node (`sudo usermod -aG lp $USER`, then log in again), and — if `dmesg` says
`lp0: using parport0` — `sudo rmmod lp`. With the `lp` printer driver holding
the port, the claim can block in uninterruptible sleep and the process cannot
be killed. See `triggers/parallel.go` for the details.

The device, the pin and the spec as typed are written into the run's
`-info.txt`, so a data file says what produced its triggers.

Other useful flags: `-pulse` (TTL width, 5 ms), `-trials`, `-isi`, `-frames`,
`-calibrate`, `-points`, `-fetch`, `-sync`. `-w` runs windowed, `-s` sets the
subject ID. Escape aborts between trials.

#### Samples, and what "dropped" means

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
gap, and the full bridge round trip, all measured on this machine. That is where
every latency figure this test reports comes from.

The EDF holds the marks. Convert it and check they are all there, one per trial:

```bash
edf2asc /tmp/goxtest.edf
grep '^MSG' /tmp/goxtest.asc | grep TRIAL | head -20
```

`INPUT` lines, if any appear, are the Host's unconnected port idling at 127 and
mean nothing (see *Wiring*). With a cable to the Host's DB25 they would become
the real measurement: each `INPUT` followed by its trial's `MSG`, the difference
being the bridge's latency on the Host clock.

To check the TTL side, look at the MEG STI channel: one pulse per trial, at the
onset of the patch.

## Calibration

`-calibrate` asks the tracker to run its own setup routine. pylink draws the
targets from the **bridge process**, so they appear over whatever goxpyriment
has on screen; the tracker owns the display until the operator leaves setup.

The keys are the EyeLink's own standard set, handled by pylink and not by
goxpyriment:

| Key | Action |
|---|---|
| `C` | start calibration |
| `V` | start validation (after a calibration) |
| `D` | drift check / correct |
| `A` | auto-threshold the pupil |
| `Enter` | accept the current target during calibration; toggle the camera image in setup |
| `Esc` | leave setup — **the run starts recording immediately** |

Between operations the screen shows the camera image, the eye video. That is
where the routine sits when it is not drawing targets, so a calibration and then
a validation each end by returning there: seeing the eye again is the normal
"done" state, not a failure. **Press Esc to leave.** There is no second keypress
to start the acquisition — the bridge reads back what was stored, the program
starts the recording and the first trial's blank ISI follows at once.

That Esc goes to pylink, not to goxpyriment, and does not abort the run — unlike
Esc everywhere else in the framework, including `control.CalibrateTracker`,
which is the path a tracker whose SDK draws nothing (a Tobii) takes.

The bridge enables automatic pacing, so targets advance on their own once the
eye holds still. That also means **an empty chair sits on target 1 for ever** —
there is no fixation to accept. Seat the participant before calibrating.

Calibrating against a windowed (`-w`) screen is only useful for checking that
the mechanism runs. Gaze coordinates are meaningless unless the calibration and
the experiment used the same display geometry, so calibrate fullscreen.

The tracker's setup routine exits identically whether or not anything was
calibrated, so the bridge asks it afterwards what it actually stored and fails
if the answer is "nothing". The tracker's own summary, including the validation
error when the operator ran one, is printed and is available from
`Bridge.CalibrationMessage`. Confirm it in the EDF too — a completed
calibration writes `!CAL` records:

```bash
grep '!CAL' /tmp/goxtest.asc
```

No `!CAL` lines means no calibration was stored, whatever the screen appeared
to show.
