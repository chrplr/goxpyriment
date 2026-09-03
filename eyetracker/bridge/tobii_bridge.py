#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Bridge between goxpyriment's eyetracker package and a Tobii Pro eye tracker.

The Tobii Pro SDK for Python is a native extension (tobii_research_interop.so),
so it cannot be linked into a pure-Go binary. It runs here instead, in its own
process, speaking the line-delimited JSON protocol in eyetracker/protocol.go
over a local socket. The Go client is eyetracker.Bridge; the transport is
bridgelib.

Usage:

    # list the trackers on the network and exit -- do this FIRST at a rig
    python3 tobii_bridge.py --check

    # against real hardware
    python3 tobii_bridge.py --edf-dir /tmp

    # against nothing at all, for developing the experiment
    python3 tobii_bridge.py --simulate

The SDK is a native extension and is not pip-installed. It has to be on
PYTHONPATH; on chrplr's machine ~/.bashrc already puts
~/tobii_eyetracker_pythonlib there, so nothing extra is needed. Elsewhere:

    PYTHONPATH=/path/to/tobii_eyetracker_pythonlib python3 tobii_bridge.py --check

--check says which of the two situations you are in before anything else can
waste time on it.

# How this differs from the EyeLink bridge

Two differences drive the whole design, and both are visible in the protocol.

**There is no tracker-side data file.** An EyeLink Host PC writes an EDF and
`receive_file` pulls it off. Tobii samples exist only inside an SDK callback, so
THIS PROCESS writes the record: `open` names a TSV, the callback fills it at the
tracker's full rate, and `receive_file` copies it where the client asks. The
full-fidelity record therefore never crosses the socket and a slow client cannot
put holes in it. The reduced `sample` events are sent as well, for Latest() and
gaze-contingent loops.

**The SDK draws no calibration targets.** collect_data(x, y) assumes the
participant is already looking at (x, y) and blocks while it samples. So
calibration is driven step by step BY THE CLIENT, which draws the targets in
goxpyriment's own window on the flip clock -- see the calibration_* commands and
control.Experiment.CalibrateTracker. The plain `calibrate` command therefore
fails here, with the alternative named in the message.

# Units and conventions on the wire

`pa` is pupil **DIAMETER in millimetres**. On the EyeLink bridge the same field
is pupil AREA in the tracker's arbitrary units. Nothing in the protocol can
catch a confusion between the two, so the number is written into the TSV header
under its full name and the bridge reports it at open.

Gaze positions are converted from Tobii's normalized active-display-area
coordinates to the TRACKER PIXELS the protocol specifies, using the width and
height the client sends at open:

    x_px = nx * width        y_px = ny * height

This assumes normalized (0,0) is the display area's TOP-LEFT corner, which is
Tobii's documented convention and matches the protocol's origin-top-left, +Y-down
pixels. It is not stated in the SDK headers shipped with the SDK, so the display
area corners are logged at open as the evidence, and `docs`/TODO carry a
hardware check that settles it. Getting it wrong mirrors every gaze position
vertically, which looks like a calibration fault rather than a units bug.
"""

import argparse
import math
import os
import queue
import shutil
import sys
import threading
import time

from bridgelib import log, serve_forever

# Room for ~10 s at 1200 Hz binocular before either queue starts refusing. A
# refusal is counted and logged, never silent: a gap in the record that nothing
# reports is worse than no record.
QUEUE_MAX = 12000

# Sent instead of a gaze dict to have the writer thread emit a marker in file
# order, rather than having two threads write to the same file.
_MARK = "mark"

EYES = ("left", "right")


def _tsv_columns():
    """The TSV header, built once so the writer and this list cannot diverge."""
    cols = ["system_time_stamp_us", "device_time_stamp_us"]
    for eye in EYES:
        cols += [
            "%s_gaze_x_norm" % eye,
            "%s_gaze_y_norm" % eye,
            "%s_gaze_x_px" % eye,
            "%s_gaze_y_px" % eye,
            "%s_gaze_valid" % eye,
            "%s_origin_x_mm" % eye,
            "%s_origin_y_mm" % eye,
            "%s_origin_z_mm" % eye,
            "%s_origin_valid" % eye,
            "%s_pupil_diameter_mm" % eye,
            "%s_pupil_valid" % eye,
        ]
    return cols


TSV_COLUMNS = _tsv_columns()


def _num(v):
    """Format a float for the TSV, leaving missing data empty rather than nan.

    An empty field reads as missing in every analysis package. The string "nan"
    reads as missing in some and as a category level in others.
    """
    if v is None:
        return ""
    try:
        f = float(v)
    except (TypeError, ValueError):
        return ""
    if math.isnan(f) or math.isinf(f):
        return ""
    return "%.6g" % f


class GazeRecorder:
    """Writes the full-rate gaze record to a TSV on a thread of its own.

    The SDK calls its callback from an internal thread and a slow callback costs
    samples, so `submit` does nothing but enqueue. All formatting and I/O
    happens on the writer thread.
    """

    def __init__(self):
        self.path = None
        self.meta = []
        self.q = queue.Queue(maxsize=QUEUE_MAX)
        self.thread = None
        self.running = False
        self.dropped = 0
        self.rows = 0
        self._warned = False
        self._f = None

    # -- lifecycle --------------------------------------------------------

    def open(self, path, meta):
        self.path = path
        self.meta = list(meta)
        self._f = open(path, "w", encoding="utf-8")
        for line in self.meta:
            self._f.write("# %s\n" % line)
        self._f.write("\t".join(TSV_COLUMNS) + "\n")
        self._f.flush()

    def start(self):
        if self.running:
            return
        self.running = True
        self.thread = threading.Thread(target=self._run, daemon=True)
        self.thread.start()

    def stop(self):
        """Stop the writer, having drained what is already queued."""
        if not self.running:
            return
        self.running = False
        if self.thread is not None:
            self.thread.join(timeout=5.0)
            self.thread = None
        self._drain()
        if self._f is not None:
            self._f.flush()

    def close(self):
        self.stop()
        if self._f is not None:
            self._f.close()
            self._f = None

    # -- the hot path -----------------------------------------------------

    def submit(self, item):
        """Enqueue a gaze dict or a marker. Called from the SDK's thread."""
        try:
            self.q.put_nowait(item)
        except queue.Full:
            self.dropped += 1
            if not self._warned:
                self._warned = True
                log(
                    "gaze file queue is full; rows are being dropped. The disk "
                    "cannot keep up with the tracker."
                )

    def _run(self):
        while self.running:
            try:
                item = self.q.get(timeout=0.2)
            except queue.Empty:
                continue
            self._write(item)
        self._drain()

    def _drain(self):
        while True:
            try:
                item = self.q.get_nowait()
            except queue.Empty:
                return
            self._write(item)

    def _write(self, item):
        if self._f is None:
            return
        try:
            if isinstance(item, tuple) and item and item[0] == _MARK:
                self._f.write("# MARK\t%d\t%s\n" % (item[1], item[2]))
                self._f.flush()
                return
            self._f.write(self._row(item))
            self.rows += 1
            # Flushing every row costs too much at 1200 Hz; flushing never
            # loses the tail if the process is killed. Once a second is the
            # compromise, and stop() flushes as well.
            if self.rows % 1000 == 0:
                self._f.flush()
        except Exception as exc:
            log("writing the gaze file failed: %s" % exc)
            self._f = None

    def _row(self, d):
        w, h = d["_width"], d["_height"]
        out = [str(d["system_time_stamp"]), str(d["device_time_stamp"])]
        for eye in EYES:
            nx, ny = d["%s_gaze_point_on_display_area" % eye]
            ox, oy, oz = d["%s_gaze_origin_in_user_coordinate_system" % eye]
            gv = int(d["%s_gaze_point_validity" % eye])
            out += [
                _num(nx),
                _num(ny),
                _num(nx * w) if gv else "",
                _num(ny * h) if gv else "",
                str(gv),
                _num(ox),
                _num(oy),
                _num(oz),
                str(int(d["%s_gaze_origin_validity" % eye])),
                _num(d["%s_pupil_diameter" % eye]),
                str(int(d["%s_pupil_validity" % eye])),
            ]
        return "\t".join(out) + "\n"


def gaze_events(d, width, height):
    """Convert one Tobii gaze dict into one protocol event per eye.

    An eye whose gaze point is invalid is reported with "valid": false and NO
    coordinates. That is not just tidiness: Tobii writes nan into an invalid
    gaze point, json.dumps emits a bare NaN for it, and Go's encoding/json
    rejects that outright -- one blink would kill the connection. The client
    treats a sample without coordinates as invalid, which is what a blink is.
    """
    t = d["system_time_stamp"] / 1000.0  # microseconds -> milliseconds
    out = []
    for eye in EYES:
        ev = {"ev": "sample", "t": t, "eye": eye}
        pd = d["%s_pupil_diameter" % eye]
        if int(d["%s_pupil_validity" % eye]) and pd is not None \
                and not math.isnan(pd):
            ev["pa"] = float(pd)  # DIAMETER in mm -- see the module docstring
        if int(d["%s_gaze_point_validity" % eye]):
            nx, ny = d["%s_gaze_point_on_display_area" % eye]
            if not (math.isnan(nx) or math.isnan(ny)):
                ev["x"] = float(nx) * width
                ev["y"] = float(ny) * height
        if "x" not in ev:
            ev["valid"] = False
        out.append(ev)
    return out


# --------------------------------------------------------------------------
# Tracker back ends
# --------------------------------------------------------------------------


class TobiiTracker:
    """The real thing, through the Tobii Pro SDK."""

    simulated = False
    name = "tobii"

    def __init__(self, address="", rate=0, edf_dir=""):
        import tobii_research  # imported here so --simulate needs no SDK

        self.tr = tobii_research
        self.address = address
        self.rate = rate
        self.edf_dir = edf_dir
        self.et = None
        self.recording = False
        self.width = 1920
        self.height = 1080
        self.rec = GazeRecorder()
        self.wire = queue.Queue(maxsize=QUEUE_MAX)
        self.wire_dropped = 0
        self.cal = None
        self._wire_warned = False

    # -- lifecycle --------------------------------------------------------

    def _find(self):
        found = self.tr.find_all_eyetrackers()
        if not found:
            raise RuntimeError(
                "no Tobii eye tracker was found. Check it is powered and on "
                "the same network, and that it appears in Tobii Pro Eye "
                "Tracker Manager."
            )
        if not self.address:
            return found[0]
        for et in found:
            if et.address == self.address:
                return et
        raise RuntimeError(
            "no tracker at %r; found %s"
            % (self.address, ", ".join(e.address for e in found))
        )

    def open(self, host, edf, width, height):
        if host:
            self.address = host
        self.et = self._find()
        self.width, self.height = width, height

        if self.rate:
            available = self.et.get_all_gaze_output_frequencies()
            if self.rate not in available:
                raise ValueError(
                    "this tracker cannot sample at %g Hz; it offers %s"
                    % (self.rate, ", ".join(str(f) for f in available))
                )
            self.et.set_gaze_output_frequency(float(self.rate))
        freq = self.et.get_gaze_output_frequency()

        da = self.et.get_display_area()
        # The corners are the evidence for the normalized-coordinate origin the
        # module docstring assumes. Logging them means a rig session can settle
        # the question from the log rather than by argument.
        log(
            "display area (mm): top_left=%s top_right=%s bottom_left=%s "
            "%.1f x %.1f" % (da.top_left, da.top_right, da.bottom_left,
                             da.width, da.height)
        )

        path = self._data_path(edf)
        self.rec.open(path, self._meta(freq, da, path))
        log("writing gaze data to %s" % path)

        return {
            "path": path,
            "model": self.et.model,
            "serial": self.et.serial_number,
            "address": self.et.address,
            "firmware": self.et.firmware_version,
            "freq": freq,
            "pupil_units": "diameter_mm",
            "display_area_mm": [da.width, da.height],
        }

    def _data_path(self, edf):
        name = edf or "tobii_gaze.tsv"
        if self.edf_dir:
            name = os.path.join(self.edf_dir, os.path.basename(name))
        return name

    def _meta(self, freq, da, path):
        return [
            "goxpyriment tobii_bridge gaze record",
            "bridge\t%s" % self.name,
            "simulated\tfalse",
            "written\t%s" % time.strftime("%Y-%m-%dT%H:%M:%S"),
            "file\t%s" % path,
            "model\t%s" % self.et.model,
            "serial\t%s" % self.et.serial_number,
            "address\t%s" % self.et.address,
            "firmware\t%s" % self.et.firmware_version,
            "gaze_output_frequency_hz\t%s" % freq,
            "sdk_version\t%s" % self.tr.__version__,
            # Without these two the pixel columns cannot be re-derived and the
            # file cannot be compared with one recorded on another screen.
            "screen_width_px\t%d" % self.width,
            "screen_height_px\t%d" % self.height,
            "display_area_mm\t%.2f\t%.2f" % (da.width, da.height),
            "display_area_top_left_mm\t%s" % (da.top_left,),
            "display_area_bottom_right_mm\t%s" % (da.bottom_right,),
            "timestamps\tsystem_time_stamp is CLOCK_MONOTONIC microseconds; "
            "device_time_stamp is the tracker's own clock",
            "pupil_units\tdiameter in millimetres",
            "normalized_origin\tassumed top-left of the display area",
        ]

    def close(self):
        if self.et is None:
            self.rec.close()
            return
        try:
            if self.recording:
                self.stop_recording()
        finally:
            self.rec.close()
            self.et = None

    # -- recording --------------------------------------------------------

    def _on_gaze(self, d):
        """The SDK's callback. Must do as little as possible and return."""
        d["_width"] = self.width
        d["_height"] = self.height
        self.rec.submit(d)
        try:
            self.wire.put_nowait(d)
        except queue.Full:
            self.wire_dropped += 1
            if not self._wire_warned:
                self._wire_warned = True
                log("socket queue is full; the client is not draining fast enough")

    def start_recording(self):
        if self.recording:
            return
        self.rec.start()
        self.et.subscribe_to(
            self.tr.EYETRACKER_GAZE_DATA, self._on_gaze, as_dictionary=True
        )
        self.recording = True

    def stop_recording(self):
        if not self.recording:
            return
        self.et.unsubscribe_from(self.tr.EYETRACKER_GAZE_DATA, self._on_gaze)
        self.recording = False
        self.rec.stop()
        if self.rec.dropped or self.wire_dropped:
            log(
                "dropped %d file rows and %d socket samples this recording"
                % (self.rec.dropped, self.wire_dropped)
            )

    def mark(self, text):
        self.rec.submit((_MARK, self.tr.get_system_time_stamp(), text))

    def tracker_time(self):
        return self.tr.get_system_time_stamp() / 1000.0

    def receive_file(self, path):
        self.rec.stop()
        src = self.rec.path
        if not src or not os.path.exists(src):
            raise RuntimeError("no gaze file was written; was open() called?")
        if path and os.path.abspath(path) != os.path.abspath(src):
            shutil.copyfile(src, path)
        return {"path": path or src, "source": src, "rows": self.rec.rows}

    def poll(self):
        if not self.recording:
            time.sleep(0.005)
            return []
        out = []
        # Bounded so one call cannot monopolise the thread when the queue is
        # backed up; the caller comes straight back for more.
        for _ in range(256):
            try:
                d = self.wire.get_nowait()
            except queue.Empty:
                break
            out.extend(gaze_events(d, self.width, self.height))
        if not out:
            # Nothing waiting: yield rather than spin a core at 100%.
            time.sleep(0.002)
        return out

    # -- calibration ------------------------------------------------------
    #
    # The SDK draws nothing, so the client puts each target on screen and calls
    # calibration_collect when the participant is looking at it. collect_data
    # BLOCKS while it samples -- up to 10 s per the SDK headers -- which is why
    # the Go client sends these with no request timeout.

    def calibrate(self, points):
        raise RuntimeError(
            "the Tobii SDK draws no calibration targets, so this bridge "
            "cannot run a calibration on its own. Drive it with "
            "control.Experiment.CalibrateTracker, which draws the targets in "
            "goxpyriment's own window on the flip clock, or calibrate in "
            "Tobii Pro Eye Tracker Manager (a calibration stored there "
            "persists in the tracker)."
        )

    def calibration_enter(self):
        self.cal = self.tr.ScreenBasedCalibration(self.et)
        self.cal.enter_calibration_mode()
        return {}

    def _require_cal(self):
        if self.cal is None:
            raise RuntimeError(
                "not in calibration mode: send calibration_enter first"
            )
        return self.cal

    def calibration_collect(self, x, y):
        status = self._require_cal().collect_data(x, y)
        # A point the tracker refused is not an error here: the client decides
        # whether to retry it or drop it, and only it knows what is on screen.
        return {"status": str(status), "x": x, "y": y}

    def calibration_discard(self, x, y):
        self._require_cal().discard_data(x, y)
        return {"x": x, "y": y}

    def calibration_compute(self):
        result = self._require_cal().compute_and_apply()
        points = []
        for p in result.calibration_points:
            px, py = p.position_on_display_area
            used = 0
            for s in p.calibration_samples:
                for eye in (s.left_eye, s.right_eye):
                    if eye.validity == self.tr.VALIDITY_VALID_AND_USED:
                        used += 1
            points.append(
                {
                    "x": px,
                    "y": py,
                    "samples": len(p.calibration_samples),
                    "used": used,
                }
            )
        status = str(result.status)
        log("calibration computed: %s, %d points" % (status, len(points)))
        return {"status": status, "points": points}

    def calibration_leave(self):
        if self.cal is not None:
            self.cal.leave_calibration_mode()
            self.cal = None
        return {}


class TobiiSimTracker:
    """A fake Tobii that reports a slowly drifting binocular gaze.

    It exists so the whole protocol, the Go client and an experiment can be
    exercised with no hardware present. Every response is honest about being
    fake: `simulated` is true in the hello event, the Go client warns and
    exposes it, and the TSV says so in its header.

    It reports BOTH eyes, with a small horizontal offset between them, because
    the per-eye split is the part of this bridge most easily got wrong and a
    one-eyed simulator would never exercise it. It also blinks, so that the
    invalid-sample path -- the one that must never put a nan on the wire -- is
    covered too.
    """

    simulated = True
    name = "tobii-sim"

    def __init__(self, rate=600.0, edf_dir=""):
        self.rate = rate
        self.edf_dir = edf_dir
        self.t0 = time.monotonic()
        self.width = 1920
        self.height = 1080
        self.recording = False
        self.rec = GazeRecorder()
        self.cal = None
        self.cal_points = []

    def _stamp_us(self):
        return int((time.monotonic() - self.t0) * 1e6)

    def tracker_time(self):
        return self._stamp_us() / 1000.0

    def open(self, host, edf, width, height):
        self.width, self.height = width, height
        name = edf or "tobii_sim_gaze.tsv"
        if self.edf_dir:
            name = os.path.join(self.edf_dir, os.path.basename(name))
        self.rec.open(
            name,
            [
                "** SIMULATED Tobii gaze record - contains no real gaze data **",
                "bridge\t%s" % self.name,
                "simulated\ttrue",
                "written\t%s" % time.strftime("%Y-%m-%dT%H:%M:%S"),
                "file\t%s" % name,
                "gaze_output_frequency_hz\t%g" % self.rate,
                "screen_width_px\t%d" % width,
                "screen_height_px\t%d" % height,
                "pupil_units\tdiameter in millimetres",
                "normalized_origin\tassumed top-left of the display area",
            ],
        )
        return {
            "path": name,
            "model": "simulated",
            "serial": "SIM-0",
            "address": host or "simulated",
            "firmware": "0",
            "freq": self.rate,
            "pupil_units": "diameter_mm",
        }

    def calibrate(self, points):
        raise RuntimeError(
            "the Tobii SDK draws no calibration targets; drive the calibration "
            "with control.Experiment.CalibrateTracker (this simulator accepts "
            "the calibration_* commands)."
        )

    def calibration_enter(self):
        self.cal = True
        self.cal_points = []
        return {}

    def calibration_collect(self, x, y):
        if not self.cal:
            raise RuntimeError("not in calibration mode: send calibration_enter first")
        # Real hardware blocks here while it samples. Sleeping a little keeps
        # the client's no-timeout path honest without slowing the tests down.
        time.sleep(0.05)
        self.cal_points.append((x, y))
        return {"status": "calibration_status_success", "x": x, "y": y}

    def calibration_discard(self, x, y):
        self.cal_points = [p for p in self.cal_points if p != (x, y)]
        return {"x": x, "y": y}

    def calibration_compute(self):
        if not self.cal:
            raise RuntimeError("not in calibration mode: send calibration_enter first")
        return {
            "status": "calibration_status_success",
            "points": [
                {"x": x, "y": y, "samples": 30, "used": 60}
                for (x, y) in self.cal_points
            ],
        }

    def calibration_leave(self):
        self.cal = None
        return {}

    def start_recording(self):
        self.rec.start()
        self.recording = True

    def stop_recording(self):
        self.recording = False
        self.rec.stop()

    def mark(self, text):
        self.rec.submit((_MARK, self._stamp_us(), text))

    def receive_file(self, path):
        self.rec.stop()
        src = self.rec.path
        if path and os.path.abspath(path) != os.path.abspath(src):
            shutil.copyfile(src, path)
        return {"path": path or src, "source": src, "rows": self.rec.rows,
                "simulated": True}

    def _gaze(self):
        """One synthetic gaze dict, in the SDK's dictionary form."""
        t = self._stamp_us()
        ts = t / 1e6
        # A Lissajous path, so the gaze moves in a way that is obviously
        # synthetic on screen and never sits still.
        nx = 0.5 + 0.3 * math.sin(ts / 0.9)
        ny = 0.5 + 0.3 * math.sin(ts / 1.33)
        # Blink for 40 ms out of every 400 ms. That is far more often than
        # anyone blinks, and deliberately so: the invalid-sample path is the
        # one that must never put a nan on the wire (Go's encoding/json
        # rejects the bare NaN that json.dumps writes for one, so a single
        # leaked blink drops the connection). At this rate a test that
        # collects a few hundred samples is certain to cover it.
        blink = (ts % 0.4) < 0.04
        d = {
            "device_time_stamp": t,
            "system_time_stamp": t,
            "_width": self.width,
            "_height": self.height,
        }
        for eye, dx in (("left", -0.01), ("right", 0.01)):
            valid = 0 if blink else 1
            d["%s_gaze_point_on_display_area" % eye] = (
                (nx + dx, ny) if valid else (float("nan"), float("nan"))
            )
            d["%s_gaze_point_in_user_coordinate_system" % eye] = (0.0, 0.0, 600.0)
            d["%s_gaze_point_validity" % eye] = valid
            d["%s_gaze_origin_in_user_coordinate_system" % eye] = (
                -30.0 if eye == "left" else 30.0,
                20.0,
                600.0,
            )
            d["%s_gaze_origin_validity" % eye] = valid
            d["%s_pupil_diameter" % eye] = (
                3.5 + 0.2 * math.sin(ts) if valid else float("nan")
            )
            d["%s_pupil_validity" % eye] = valid
        return d

    def poll(self):
        if not self.recording:
            time.sleep(0.005)
            return []
        d = self._gaze()
        self.rec.submit(d)
        time.sleep(1.0 / self.rate)
        return gaze_events(d, self.width, self.height)

    def close(self):
        self.recording = False
        self.rec.close()


# --------------------------------------------------------------------------
# Server
# --------------------------------------------------------------------------


def make_tracker(args):
    if args.simulate:
        return TobiiSimTracker(rate=args.rate or 600.0, edf_dir=args.edf_dir)
    try:
        return TobiiTracker(
            address=args.tracker_address, rate=args.rate, edf_dir=args.edf_dir
        )
    except ImportError:
        log(
            "tobii_research is not importable. Point PYTHONPATH at the Tobii "
            "Pro SDK for Python (for example "
            "PYTHONPATH=~/tobii_eyetracker_pythonlib), or run with --simulate "
            "to work without hardware."
        )
        raise


def check(args):
    """List the trackers on the network without starting a server.

    This is the step that can waste a whole rig session, so it exists to be run
    first: it reports whether the SDK imports, what is reachable, and what each
    device will do, then exits.
    """
    try:
        import tobii_research as tr
    except ImportError as exc:
        log("tobii_research: NOT importable (%s)" % exc)
        log("point PYTHONPATH at the Tobii Pro SDK for Python.")
        return 1
    log("tobii_research: importable, SDK version %s" % tr.__version__)
    log("system time stamp: %d us (CLOCK_MONOTONIC)" % tr.get_system_time_stamp())

    found = tr.find_all_eyetrackers()
    if not found:
        log("no eye tracker found.")
        return 1
    for i, et in enumerate(found):
        log("--- tracker %d ---" % i)
        log("  address:   %s" % et.address)
        log("  model:     %s" % et.model)
        log("  name:      %s" % et.device_name)
        log("  serial:    %s" % et.serial_number)
        log("  firmware:  %s" % et.firmware_version)
        log("  runtime:   %s" % et.runtime_version)
        try:
            log("  frequency: %s Hz (available: %s)" % (
                et.get_gaze_output_frequency(),
                ", ".join(str(f) for f in et.get_all_gaze_output_frequencies()),
            ))
        except Exception as exc:
            log("  frequency: unavailable (%s)" % exc)
        try:
            log("  modes:     %s" % ", ".join(et.get_all_eye_tracking_modes()))
        except Exception as exc:
            log("  modes:     unavailable (%s)" % exc)
        try:
            da = et.get_display_area()
            log("  display:   %.1f x %.1f mm, top_left=%s bottom_right=%s"
                % (da.width, da.height, da.top_left, da.bottom_right))
        except Exception as exc:
            log("  display:   unavailable (%s)" % exc)
        log("  capabilities: %s" % ", ".join(et.device_capabilities))
    return 0


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--host", default="127.0.0.1", help="address to listen on")
    p.add_argument("--port", type=int, default=5010, help="port to listen on")
    p.add_argument(
        "--tracker-address",
        default="",
        help="tracker URI, e.g. tet-tcp://169.254.1.2 (default: the first found)",
    )
    p.add_argument(
        "--rate",
        type=float,
        default=0,
        help="gaze output frequency in Hz (default: leave the tracker's own)",
    )
    p.add_argument(
        "--edf-dir",
        default="",
        help="directory for the gaze TSV (default: alongside this process)",
    )
    p.add_argument(
        "--simulate",
        action="store_true",
        help="invent gaze data instead of talking to hardware",
    )
    p.add_argument(
        "--check",
        action="store_true",
        help="list the trackers on the network, then exit",
    )
    p.add_argument("--once", action="store_true", help="exit after one client")
    p.add_argument("-v", "--verbose", action="store_true")
    args = p.parse_args()

    if args.check:
        sys.exit(check(args))

    serve_forever(
        args,
        make_tracker,
        "SIMULATED" if args.simulate else (args.tracker_address or "first found"),
    )


if __name__ == "__main__":
    main()
