#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Bridge between goxpyriment's eyetracker package and an SR Research EyeLink.

SR Research publishes no network protocol for the EyeLink: the only supported
API is the C library, wrapped for Python as pylink. goxpyriment is pure Go and
intends to stay that way, so the SDK runs here, in its own process, and speaks a
small line-delimited JSON protocol over a local socket. The Go client is
eyetracker.Bridge; the protocol is specified in eyetracker/protocol.go.

Usage:

    # against real hardware (needs pylink on PYTHONPATH)
    python3 eyelink_bridge.py --tracker-host 100.1.1.1

    # against nothing at all, for developing the experiment
    python3 eyelink_bridge.py --simulate

    # check the SDK is importable and the Host PC answers, then exit
    python3 eyelink_bridge.py --check --tracker-host 100.1.1.1

    # check the stepwise calibration relay against the Host, then exit
    python3 eyelink_bridge.py --check-calibration --width 1920 --height 1080

Calibration. By default the bridge offers the stepwise calibration commands
(calibration_enter ...): the Host PC sequences the targets and the client draws
them, in its own window and through its display transform. --no-stepwise
withdraws them, and the client falls back to `calibrate`, where pylink draws
the targets in a window of its own.

The bridge is NOT in the timing path and must not be put there. To mark a
stimulus onset in the EDF, send a TTL pulse to the Host PC's parallel port: the
Host timestamps the edge itself, with no socket and no round trip in between.
Use `mark` for trial labels and bookkeeping, never for an onset whose timestamp
is the measurement.
"""

import argparse
import math
import queue
import sys
import threading
import time

# The transport lives in bridgelib so that every bridge speaks the same wire
# format from the same code. sys.path[0] is this script's own directory when it
# is run by path, which is how the Go tests launch it.
from bridgelib import log, serve_forever

# EyeLink's missing-data sentinel. It is a plausible-looking number, which is
# exactly why every coordinate is checked against it before being sent.
MISSING = -32768.0


# --------------------------------------------------------------------------
# Stepwise calibration: the client draws, the Host PC sequences
# --------------------------------------------------------------------------
#
# The Host PC runs the calibration sequence itself: it decides which target
# comes next and when one is accepted, and asks for each to be drawn. pylink
# hands those requests to a "custom display" object; the relay below records
# them instead of drawing, and the client — goxpyriment, which draws through
# its display transform — puts the same target on screen. For that to mean
# anything the Host must be told the targets beforehand, which is why
# calibration_enter takes the client's list.

# HV9 and HV13 give each target index a role, and the Host's fit may rely on
# it, so the targets are sent in the Host's own slot order whatever order the
# client shows them in; calibration_sequence then reproduces the client's
# order. Slots 0-8 are SR Research's documented HV9 layout (centre, top,
# bottom, left, right, then the corners top-left, top-right, bottom-left,
# bottom-right); 9-12 are HV13's inner corners in the same corner order.
#
# NOT YET CHECKED ON A TRACKER: that the Host fits on these roles and
# that HV13 numbers its inner corners this way. `--check-calibration` prints
# what the Host asks for, which settles both.
_EDGE = {(0, 0): 0, (0, -1): 1, (0, 1): 2, (-1, 0): 3, (1, 0): 4}
_CORNER = {(-1, -1): 0, (1, -1): 1, (-1, 1): 2, (1, 1): 3}
STEPWISE_COUNTS = (9, 13)


def _sign(v, eps=0.05):
    return 0 if abs(v) < eps else (1 if v > 0 else -1)


def host_slots(points):
    """Map each normalized (x, y) target to its HV9/HV13 slot index.

    The roles are read relative to the layout itself — its centre and
    half-extents on each axis — not to the centre of the screen: the client
    moves or shrinks the pattern where the display transform or the
    photodiode square would cut a target (control.Experiment.
    CalibrationPoints), and the centre target is then off the screen centre.

    Raises ValueError for a layout the Host's HV models cannot represent:
    a count other than 9 or 13, two targets in one slot, or a target that
    is neither on the pattern's centre lines nor on one of its diagonals.
    """
    n = len(points)
    if n not in STEPWISE_COUNTS:
        raise ValueError(
            "an EyeLink calibrates on HV9 or HV13 only when the client draws "
            "the targets; got %d points" % n)
    xs = [p[0] for p in points]
    ys = [p[1] for p in points]
    cx, cy = (min(xs) + max(xs)) / 2, (min(ys) + max(ys)) / 2
    ex, ey = (max(xs) - min(xs)) / 2, (max(ys) - min(ys)) / 2
    if ex <= 0 or ey <= 0:
        raise ValueError("targets %s do not span both axes" % (points,))
    slots = []
    for x, y in points:
        ux, uy = (x - cx) / ex, (y - cy) / ey  # in [-1, 1]
        sx, sy = _sign(ux), _sign(uy)
        if (sx, sy) in _EDGE:
            slot = _EDGE[(sx, sy)]
        elif abs(abs(ux) - abs(uy)) < 0.05:
            # On a diagonal: an outer corner (5-8), or an inner one (9-12).
            inner = abs(ux) < 0.95
            slot = (9 if inner else 5) + _CORNER[(sx, sy)]
        else:
            raise ValueError(
                "target (%.3f, %.3f) is neither on a centre line nor on a "
                "diagonal of the layout; HV%d has no slot for it" % (x, y, n))
        slots.append(slot)
    if sorted(slots) != list(range(n)):
        raise ValueError(
            "targets %s do not fill the HV%d slots once each (got slots %s)"
            % (points, n, slots))
    return slots


def host_commands(points, width, height):
    """The Host commands that make it present exactly these targets, in
    this order, at these pixels. Returns (commands, pixels by display order).
    """
    slots = host_slots(points)
    n = len(points)
    # The same rounding as the client's drawing: normalized times the size
    # in pixels (eyetracker.Geometry, then ToCentre).
    px = [(int(round(x * width)), int(round(y * height))) for x, y in points]
    by_slot = [None] * n
    for slot, p in zip(slots, px):
        by_slot[slot] = p
    targets = " ".join("%d,%d" % p for p in by_slot)
    seq = ",".join(str(s) for s in slots)
    cmds = [
        "calibration_type = HV%d" % n,
        "generate_default_targets = NO",
        "enable_automatic_calibration = NO",
        "calibration_samples = %d" % n,
        "calibration_sequence = %s" % seq,
        "calibration_targets = %s" % targets,
        # Validation on the Host (V on its keyboard) uses the same
        # transform-correct targets rather than the default ones.
        "validation_samples = %d" % n,
        "validation_sequence = %s" % seq,
        "validation_targets = %s" % targets,
    ]
    return cmds, px


class _CalState:
    """What the relay has seen of the Host's calibration, shared with the
    command thread under one condition variable."""

    ENTER = 13
    ESC = 27

    def __init__(self, points, px):
        self.points = list(points)
        self.px = list(px)
        self.accepted = [False] * len(points)
        self.cond = threading.Condition()
        self.keys = queue.Queue()
        self.target = None   # (x, y) the Host is showing, None when erased
        self.draws = 0       # draw_cal_target calls so far
        self.finished = False
        self.error = None
        self.thread = None

    def key(self, code):
        self.keys.put(code)

    def index_of(self, x, y):
        for i, p in enumerate(self.points):
            if abs(p[0] - x) < 1e-6 and abs(p[1] - y) < 1e-6:
                return i
        raise ValueError("(%.3f, %.3f) is not one of the targets given to "
                         "calibration_enter" % (x, y))


def _make_relay(pylink, cal):
    """A pylink custom display that draws nothing and records the targets.

    Built here rather than at import because its base class comes from
    pylink, which --simulate does not need.
    """
    enter = getattr(pylink, "ENTER_KEY", cal.ENTER)
    esc = getattr(pylink, "ESC_KEY", cal.ESC)
    cal.ENTER, cal.ESC = enter, esc

    class Relay(pylink.EyeLinkCustomDisplay):
        def __init__(self):
            pylink.EyeLinkCustomDisplay.__init__(self)

        def draw_cal_target(self, x, y):
            log("Host: draw target at (%.0f, %.0f)" % (x, y))
            with cal.cond:
                cal.target = (float(x), float(y))
                cal.draws += 1
                cal.cond.notify_all()

        def erase_cal_target(self):
            with cal.cond:
                cal.target = None
                cal.cond.notify_all()

        def clear_cal_display(self):
            self.erase_cal_target()

        def get_input_key(self):
            try:
                k = cal.keys.get_nowait()
            except queue.Empty:
                time.sleep(0.001)
                return None
            log("relay: key %r to the Host" % k)
            return [pylink.KeyInput(k, 0)]

        def alert_printf(self, msg):
            log("Host alert: %s" % msg)

        def setup_image_display(self, width, height):
            return 1

        # Nothing is drawn: the camera image stays on the Host PC's monitor.
        def setup_cal_display(self): pass
        def exit_cal_display(self): pass
        def record_abort_hide(self): pass
        def play_beep(self, beepid): pass
        def image_title(self, text): pass
        def draw_image_line(self, width, line, totlines, buff): pass
        def set_image_palette(self, r, g, b): pass
        def exit_image_display(self): pass
        def draw_line(self, x1, y1, x2, y2, colorindex): pass
        def draw_lozenge(self, x, y, width, height, colorindex): pass

        def get_mouse_state(self):
            return ((0, 0), 0)

    return Relay()


# --------------------------------------------------------------------------
# Tracker back ends
# --------------------------------------------------------------------------


class SimTracker:
    """A fake tracker that reports a slowly drifting gaze.

    It exists so the whole protocol, the Go client and the experiment can be
    exercised with no hardware present. Every response it gives is honest about
    being fake: `simulated` is true in the hello event, and the Go client logs a
    warning and exposes it so the run can be labelled in the data file.
    """

    simulated = True
    name = "eyelink-sim"

    def __init__(self, rate=1000.0):
        self.rate = rate
        self.t0 = time.monotonic()
        self.width = 1920
        self.height = 1080
        self.recording = False
        self.marks = []

    def tracker_time(self):
        return (time.monotonic() - self.t0) * 1000.0

    def open(self, host, edf, width, height):
        self.width, self.height = width, height
        return {"edf": edf or "sim.edf", "host": host or "simulated"}

    def calibrate(self, points):
        time.sleep(0.2)
        return {"points": points or 9, "simulated": True}

    # -- stepwise calibration (see host_commands) -------------------------
    #
    # The simulator plays the Host's part honestly: it sequences the targets
    # it was given and refuses a collect for any other one, so a client that
    # draws out of order fails here as it would on the rig.

    def calibration_enter(self, points=None):
        if not points:
            raise ValueError(
                "calibration_enter needs the client's target list: the Host "
                "sequences the targets and must know where they are drawn")
        _, px = host_commands(points, self.width, self.height)
        self.cal_points = list(points)
        self.cal_next = 0
        self.cal_accepted = [False] * len(points)
        log("simulated stepwise calibration: %d targets at %s" % (len(points), px))
        return {"points": len(points)}

    def _require_cal(self):
        if getattr(self, "cal_points", None) is None:
            raise RuntimeError("not in calibration mode: send calibration_enter first")

    def calibration_collect(self, x, y):
        self._require_cal()
        if self.cal_next >= len(self.cal_points):
            raise RuntimeError("every target has already been accepted")
        want = self.cal_points[self.cal_next]
        if abs(want[0] - x) > 1e-6 or abs(want[1] - y) > 1e-6:
            raise RuntimeError(
                "the tracker is waiting for target %d at (%.3f, %.3f), not "
                "(%.3f, %.3f)" % (self.cal_next + 1, want[0], want[1], x, y))
        time.sleep(0.05)
        self.cal_accepted[self.cal_next] = True
        self.cal_next += 1
        return {"x": x, "y": y}

    def calibration_discard(self, x, y):
        self._require_cal()
        return {"x": x, "y": y}

    def calibration_compute(self):
        self._require_cal()
        done = all(self.cal_accepted)
        return {
            "status": "calibration_status_success" if done
            else "calibration_status_failure",
            "message": "SIMULATED calibration, %d/%d targets"
            % (sum(self.cal_accepted), len(self.cal_points)),
            "points": [{"x": p[0], "y": p[1], "samples": int(a), "used": int(a)}
                       for p, a in zip(self.cal_points, self.cal_accepted)],
        }

    def calibration_leave(self):
        self.cal_points = None
        return {}

    def start_recording(self):
        self.recording = True

    def stop_recording(self):
        self.recording = False

    def mark(self, text):
        self.marks.append((self.tracker_time(), text))

    def receive_file(self, path):
        with open(path, "w") as f:
            f.write("** SIMULATED EyeLink data file - contains no gaze data **\n")
            for t, text in self.marks:
                f.write("MSG\t%.0f\t%s\n" % (t, text))
        return {"path": path, "simulated": True}

    def poll(self):
        """Return a list of protocol event dicts. Never blocks for long."""
        if not self.recording:
            time.sleep(0.005)
            return []
        t = self.tracker_time()
        # A Lissajous path, so the simulated gaze moves in a way that is
        # obviously synthetic on screen and never sits still.
        x = self.width / 2 + 0.3 * self.width * math.sin(t / 900.0)
        y = self.height / 2 + 0.3 * self.height * math.sin(t / 1330.0)
        time.sleep(1.0 / self.rate)
        return [{"ev": "sample", "t": t, "eye": "right", "x": x, "y": y, "pa": 1000.0}]

    def close(self):
        self.recording = False


class EyeLinkTracker:
    """The real thing, through pylink."""

    simulated = False
    name = "eyelink"

    def __init__(self, host):
        import pylink  # imported here so --simulate needs no SDK

        self.pylink = pylink
        self.host = host or "100.1.1.1"
        self.el = None
        self.recording = False
        self.edf = None
        self.width, self.height = 1920, 1080
        self._cal = None
        self._eye_names = {0: "left", 1: "right", 2: "binocular"}

    # -- lifecycle --------------------------------------------------------

    def open(self, host, edf, width, height):
        if host:
            self.host = host
        self.el = self.pylink.EyeLink(self.host)
        edf = edf or "gox.edf"
        # The Host PC enforces an 8.3-style name. Failing here with the reason
        # beats a Host-side refusal that surfaces as a generic RuntimeError.
        stem = edf.rsplit(".", 1)[0]
        if len(stem) > 8 or not stem.replace("_", "").isalnum():
            raise ValueError(
                "EDF name %r is invalid: the Host PC allows at most 8 "
                "characters of letters, digits and underscore before the "
                "extension" % edf
            )
        self.el.openDataFile(edf)
        self.edf = edf

        self.el.setOfflineMode()
        self.width, self.height = width, height
        # The tracker must be told the screen it is looking at, in pixels, or
        # every gaze coordinate it reports is scaled against the wrong frame.
        self.el.sendCommand("screen_pixel_coords = 0 0 %d %d" % (width - 1, height - 1))
        self.el.sendMessage("DISPLAY_COORDS 0 0 %d %d" % (width - 1, height - 1))
        # Ask for samples and parsed events over the link as well as in the
        # file; without the link half, poll() below has nothing to read.
        self.el.sendCommand(
            "file_event_filter = LEFT,RIGHT,FIXATION,SACCADE,BLINK,MESSAGE,BUTTON,INPUT"
        )
        self.el.sendCommand(
            "link_event_filter = LEFT,RIGHT,FIXATION,SACCADE,BLINK,MESSAGE,BUTTON,INPUT"
        )
        self.el.sendCommand("file_sample_data = LEFT,RIGHT,GAZE,AREA,GAZERES,STATUS,INPUT")
        self.el.sendCommand("link_sample_data = LEFT,RIGHT,GAZE,AREA,GAZERES,STATUS,INPUT")
        return {
            "edf": edf,
            "host": self.host,
            "version": str(self.el.getTrackerVersionString()),
        }

    def calibrate(self, points):
        if points:
            self.el.sendCommand("calibration_type = HV%d" % points)
        # Let the targets advance on their own once the eye holds still.
        # Without this the operator must accept every point by hand, and with
        # no eye in front of the camera the sequence stops on target 1 with no
        # indication of why.
        try:
            self.el.enableAutoCalibration()
            self.el.setAutoCalibrationPacing(1000)
        except Exception as exc:  # older pylink; hand pacing still works
            log("automatic calibration pacing unavailable: %s" % exc)

        # doTrackerSetup needs somewhere to draw the targets. pylink ships a
        # built-in graphics environment on some platforms and not others, so
        # report a missing one plainly rather than hanging with a blank screen.
        if not hasattr(self.pylink, "openGraphics"):
            raise RuntimeError(
                "calibration graphics are not available in this bridge: pylink "
                "has no openGraphics on this platform. Calibrate from the SR "
                "Research display software, or run this bridge on a machine "
                "where pylink's graphics work. Everything else (recording, "
                "markers, sample streaming) is unaffected."
            )
        # A graphics environment may still be up from an earlier calibration:
        # this bridge outlives any one run of the experiment, and a run
        # interrupted while pylink owned the display leaves its window behind.
        # pylink then refuses the next openGraphics with "unable to use
        # OpenGraphics when a previous Core Graphics is active", and the
        # calibration is silently skipped for the rest of the bridge's life.
        # Closing first costs nothing when nothing is open.
        self._close_graphics(quiet=True)
        self.pylink.openGraphics()
        try:
            self.el.doTrackerSetup()
        finally:
            self._close_graphics()

        # doTrackerSetup returns whenever the operator leaves setup, whether or
        # not anything was calibrated, so its return is no evidence. Ask the
        # tracker what it actually stored: a session recorded against an absent
        # or stale calibration looks entirely normal until the gaze is
        # analysed, which is far too late to notice.
        try:
            result = self.el.getCalibrationResult()
            message = (self.el.getCalibrationMessage() or "").strip()
        except Exception as exc:
            log("cannot read the calibration result: %s" % exc)
            return {"points": points or 9, "verified": False, "message": ""}
        # The result codes come from the EyeLink C API. OK_RESULT is 0.
        # NO_REPLY is 1000 and means "the tracker has no result pending" — NOT
        # that the calibration failed: doTrackerSetup consumes the reply of the
        # last operation, so a session that ends with a good validation returns
        # here with nothing left to report. Reading 1000 as a failure rejected
        # calibrations that had just validated at 0.47 deg average error.
        # The message is then the evidence, and it names what was run.
        no_reply = getattr(self.pylink, "NO_REPLY", 1000)
        low = message.lower()
        by_message = "validation_result" in low or "calibration_result" in low
        if result == 0 or (result == no_reply and by_message):
            log("calibration stored: %s (result %s)" % (message or "(no message)", result))
            return {"points": points or 9, "verified": True,
                    "result": result, "message": message}
        raise RuntimeError(
            "no usable calibration was stored (result %s%s). The operator "
            "left setup without completing one, or every target was "
            "rejected. Recording now would produce gaze positions that "
            "mean nothing." % (result, ": " + message if message else "")
        )

    def _close_graphics(self, quiet=False):
        """Tear down pylink's graphics environment, tolerating its absence.

        A failure here must not replace whatever exception is already on its
        way out of doTrackerSetup, and must not stop the bridge: the next
        openGraphics is the thing that would notice, and it now closes first.
        """
        close = getattr(self.pylink, "closeGraphics", None)
        if close is None:
            return
        try:
            close()
        except Exception as exc:
            if not quiet:
                log("closeGraphics failed: %s (continuing)" % exc)

    # -- stepwise calibration: the client draws ----------------------------
    #
    # doTrackerSetup runs on a worker thread with a relay display (below),
    # and every pylink call of the calibration happens on that thread: the
    # command thread only queues keys for the relay to return and waits on
    # the targets it records. The keys are the Host's own setup keys — "c"
    # starts a calibration, Enter accepts a target and then the result, Esc
    # leaves setup — exactly what an operator would press.

    CAL_TIMEOUT = 5.0  # s to wait for the Host to ask for, or move past, a target

    def calibration_enter(self, points=None):
        if not points:
            raise ValueError(
                "calibration_enter needs the client's target list: the Host "
                "sequences the targets and must know where they are drawn")
        if self.recording:
            raise RuntimeError("stop recording before calibrating")
        if getattr(self, "_cal", None) is not None:
            self.calibration_leave()
        cmds, px = host_commands(points, self.width, self.height)
        self.el.setOfflineMode()
        for c in cmds:
            self.el.sendCommand(c)
        log("stepwise calibration: %s" % "; ".join(cmds))

        cal = _CalState(points, px)
        relay = _make_relay(self.pylink, cal)
        self._cal = cal

        def run():
            try:
                self._close_graphics(quiet=True)
                self.pylink.openGraphicsEx(relay)
                self.el.doTrackerSetup()
            except Exception as exc:
                cal.error = exc
                log("doTrackerSetup failed: %s" % exc)
            finally:
                self._close_graphics(quiet=True)
                with cal.cond:
                    cal.finished = True
                    cal.cond.notify_all()

        cal.thread = threading.Thread(target=run, daemon=True)
        cal.thread.start()
        cal.key(ord("c"))
        return {"points": len(points)}

    def _require_cal(self):
        cal = getattr(self, "_cal", None)
        if cal is None:
            raise RuntimeError("not in calibration mode: send calibration_enter first")
        return cal

    def calibration_collect(self, x, y):
        cal = self._require_cal()
        i = cal.index_of(x, y)
        want = cal.px[i]
        with cal.cond:
            # The Host must be showing this target, at these pixels, before
            # it can be accepted: anything else is a client drawing one thing
            # while the Host collects another.
            ok = cal.cond.wait_for(
                lambda: cal.finished or cal.target is not None
                and abs(cal.target[0] - want[0]) <= 1.5
                and abs(cal.target[1] - want[1]) <= 1.5,
                timeout=self.CAL_TIMEOUT)
            if cal.finished:
                raise RuntimeError("the Host left setup: %s" % (cal.error or "no reason given"))
            if not ok:
                raise RuntimeError(
                    "the Host is not showing target %d at %s (it shows %s)"
                    % (i + 1, want, cal.target))
            seq = cal.draws
        cal.key(cal.ENTER)
        last = i == len(cal.px) - 1
        with cal.cond:
            ok = cal.cond.wait_for(
                lambda: cal.finished or cal.draws != seq
                or (last and cal.target is None),
                timeout=self.CAL_TIMEOUT)
        if not ok:
            raise RuntimeError(
                "target %d was not accepted within %.0f s: is the pupil and the "
                "corneal reflection tracked (see the Host PC screen)?"
                % (i + 1, self.CAL_TIMEOUT))
        cal.accepted[i] = True
        return {"x": x, "y": y}

    def calibration_discard(self, x, y):
        # A target the Host did not accept left nothing to throw away, and it
        # is still waiting on it — which is what the client's retry wants.
        cal = self._require_cal()
        i = cal.index_of(x, y)
        if cal.accepted[i]:
            log("discarding an accepted target is not supported; keeping it")
        return {"x": x, "y": y}

    def calibration_compute(self):
        cal = self._require_cal()
        # Accept the result screen, then leave setup.
        cal.key(cal.ENTER)
        time.sleep(0.5)
        cal.key(cal.ESC)
        cal.thread.join(10.0)
        if cal.thread.is_alive():
            log("doTrackerSetup did not return after Esc; reading the result anyway")
        points = [{"x": p[0], "y": p[1], "samples": int(a), "used": int(a)}
                  for p, a in zip(cal.points, cal.accepted)]
        try:
            result = self.el.getCalibrationResult()
            message = (self.el.getCalibrationMessage() or "").strip()
        except Exception as exc:
            return {"status": "calibration_status_failure",
                    "message": "cannot read the result: %s" % exc, "points": points}
        # See calibrate() for why NO_REPLY with a result message is a success.
        no_reply = getattr(self.pylink, "NO_REPLY", 1000)
        low = message.lower()
        ok = all(cal.accepted) and (
            result == 0 or (result == no_reply and "calibration_result" in low))
        log("stepwise calibration result %s: %s" % (result, message or "(no message)"))
        return {"status": "calibration_status_success" if ok
                else "calibration_status_failure",
                "message": message, "result": result, "points": points}

    def calibration_leave(self):
        cal = getattr(self, "_cal", None)
        self._cal = None
        if cal is None:
            return {}
        # Esc leaves a calibration in progress, a second one leaves setup.
        # Both go at once: a key left over once setup has returned lands in a
        # queue nobody reads any more (each calibration_enter has its own).
        for _ in range(3):
            if not cal.thread.is_alive():
                break
            cal.key(cal.ESC)
            cal.key(cal.ESC)
            cal.thread.join(2.0)
        if cal.thread.is_alive():
            log("doTrackerSetup is still running after Esc; leaving it behind")
        try:
            self.el.setOfflineMode()
        except Exception as exc:
            log("setOfflineMode after calibration: %s" % exc)
        return {}

    def start_recording(self):
        # (file_samples, file_events, link_samples, link_events)
        err = self.el.startRecording(1, 1, 1, 1)
        if err:
            raise RuntimeError("startRecording returned %s" % err)
        # The tracker needs a moment before the link carries data; without it
        # the first samples of every trial are missing.
        self.pylink.pumpDelay(100)
        self.recording = True

    def stop_recording(self):
        self.pylink.pumpDelay(100)
        self.el.stopRecording()
        self.recording = False

    def mark(self, text):
        self.el.sendMessage(text)

    def tracker_time(self):
        return float(self.el.trackerTime())

    def receive_file(self, path):
        self.el.setOfflineMode()
        self.pylink.pumpDelay(500)
        self.el.closeDataFile()
        self.el.receiveDataFile(self.edf, path)
        return {"path": path, "edf": self.edf}

    def close(self):
        if self.el is None:
            return
        try:
            if self._cal is not None:
                self.calibration_leave()
            if self.recording:
                self.el.stopRecording()
            self.el.setOfflineMode()
            self.el.close()
        finally:
            self.el = None

    # -- the sample pump --------------------------------------------------

    def poll(self):
        """Drain everything the link has to offer, as protocol event dicts."""
        if not self.recording:
            time.sleep(0.005)
            return []
        out = []
        pl = self.pylink
        # Bounded so one call cannot monopolise the thread if the link is
        # backed up; the caller comes straight back for more.
        for _ in range(256):
            kind = self.el.getNextData()
            if not kind:
                break
            data = self.el.getFloatData()
            if data is None:
                continue
            if kind == pl.SAMPLE_TYPE:
                ev = self._sample(data)
                if ev:
                    out.append(ev)
            else:
                ev = self._event(kind, data)
                if ev:
                    out.append(ev)
        if not out:
            # Nothing waiting: yield rather than spin a core at 100%.
            time.sleep(0.0005)
        return out

    def _eye_of(self, data):
        try:
            return self._eye_names.get(data.getEye(), "unknown")
        except Exception:
            return "unknown"

    def _sample(self, s):
        try:
            t = float(s.getTime())
        except Exception:
            return None
        eye_obj, eye = None, "unknown"
        # A binocular recording delivers both; report the right eye, and say
        # which one it was rather than leaving the consumer to assume.
        if s.isRightSample():
            eye_obj, eye = s.getRightEye(), "right"
        elif s.isLeftSample():
            eye_obj, eye = s.getLeftEye(), "left"
        if eye_obj is None:
            return None
        try:
            x, y = eye_obj.getGaze()
            pa = float(eye_obj.getPupilSize())
        except Exception:
            return None
        valid = x > MISSING and y > MISSING
        ev = {"ev": "sample", "t": t, "eye": eye, "pa": pa}
        if valid:
            ev["x"] = float(x)
            ev["y"] = float(y)
        else:
            # Send no coordinates at all rather than the sentinel. The client
            # marks a sample without them invalid, so a blink cannot be read as
            # a gaze at (-32768, -32768).
            ev["valid"] = False
        return ev

    def _event(self, kind, e):
        pl = self.pylink
        names = {
            pl.STARTFIX: "fix_start",
            pl.ENDFIX: "fix_end",
            pl.STARTSACC: "sacc_start",
            pl.ENDSACC: "sacc_end",
            pl.STARTBLINK: "blink_start",
            pl.ENDBLINK: "blink_end",
        }
        name = names.get(kind)
        if name is None:
            return None
        ev = {"ev": name, "eye": self._eye_of(e)}

        # Every conversion is inside the try, not just the call. pylink's
        # getters are not uniform — getAmplitude() returns a PAIR of degrees,
        # one per axis, where getPeakVelocity() returns a number — and a
        # float() against the wrong shape propagated out of poll(), where
        # bridgelib stops the sample pump. One saccade then ended gaze
        # streaming for the rest of the session:
        #
        #   [bridge] poll failed: float() argument must be a string or a real
        #   number, not 'tuple'
        #
        # A field that will not convert is dropped and named; the stream is
        # worth more than any one of them.
        def put(key, fn, *idx):
            try:
                v = fn()
                if v is None:
                    return
                if idx:
                    a, b = v
                    if a > MISSING and b > MISSING:
                        ev[idx[0]] = float(a)
                        ev[idx[1]] = float(b)
                    return
                if isinstance(v, (tuple, list)):
                    # An amplitude given per axis; report the magnitude, which
                    # is what "saccade amplitude" means everywhere else.
                    ev[key] = math.hypot(float(v[0]), float(v[1]))
                    return
                ev[key] = float(v)
            except Exception as exc:
                log("dropping %s from a %s event: %s" % (key or idx, name, exc))

        put("start", e.getStartTime)
        if name.endswith("_end"):
            put("end", e.getEndTime)
        put(None, e.getStartGaze, "sx", "sy")
        if name.endswith("_end"):
            put(None, e.getEndGaze, "ex", "ey")
        if name == "fix_end":
            put(None, e.getAverageGaze, "ax", "ay")
        if name == "sacc_end":
            put("ampl", e.getAmplitude)
            put("pvel", e.getPeakVelocity)
        return ev


# --------------------------------------------------------------------------
# Server
# --------------------------------------------------------------------------


def make_tracker(args):
    if args.simulate:
        t = SimTracker()
        t.stepwise = not args.no_stepwise
        return t
    try:
        t = EyeLinkTracker(args.tracker_host)
        t.stepwise = not args.no_stepwise
        return t
    except ImportError:
        log(
            "pylink is not importable. Install the EyeLink Developer Kit and "
            "put its pylink on PYTHONPATH, or run with --simulate to work "
            "without hardware."
        )
        raise


def check(args):
    """Verify the SDK and the Host PC without starting a server."""
    ok = True
    try:
        import pylink  # noqa: F401

        log("pylink: importable")
    except ImportError as exc:
        log("pylink: NOT importable (%s)" % exc)
        return 1
    try:
        t = EyeLinkTracker(args.tracker_host)
        info = t.open("", "chk.edf", 1920, 1080)
        log("tracker: connected, %s" % info.get("version", "?"))
        log("tracker time: %.0f ms" % t.tracker_time())
        t.close()
    except Exception as exc:
        log("tracker: FAILED (%s)" % exc)
        ok = False
    return 0 if ok else 1


# eyetracker.StandardPoints(9), in the same order, for --check-calibration.
CHECK_POINTS = [(0.5, 0.5), (0.1, 0.1), (0.9, 0.1), (0.1, 0.9), (0.9, 0.9),
                (0.5, 0.1), (0.1, 0.5), (0.9, 0.5), (0.5, 0.9)]


def check_calibration(args):
    """Drive one stepwise calibration from the terminal, logging every request
    the Host makes, to check the relay against a real tracker.

    Nothing is drawn on the stimulus screen: the participant (or anyone in
    front of the camera) looks at roughly where each target is announced, and
    the operator presses Enter here to accept it. What it checks is the
    mechanism: that the Host asks for our targets, in our order, at our
    pixels, that Enter accepts them, and what result comes back.
    """
    t = EyeLinkTracker(args.tracker_host)
    t.stepwise = True
    t.open("", "calchk.edf", args.width, args.height)
    try:
        print(t.calibration_enter(CHECK_POINTS))
        for i, (x, y) in enumerate(CHECK_POINTS):
            input("target %d/%d at (%.0f, %.0f) px: look there, then press "
                  "Enter here " % (i + 1, len(CHECK_POINTS),
                                   x * args.width, y * args.height))
            try:
                print(t.calibration_collect(x, y))
            except Exception as exc:
                print("collect failed: %s" % exc)
        res = t.calibration_compute()
        print("result:", res.get("status"), res.get("result"))
        print("message:", res.get("message"))
    finally:
        t.calibration_leave()
        t.close()
    return 0


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--host", default="127.0.0.1", help="address to listen on")
    p.add_argument("--port", type=int, default=5010, help="port to listen on")
    p.add_argument(
        "--tracker-host", default="100.1.1.1", help="EyeLink Host PC address"
    )
    p.add_argument(
        "--simulate",
        action="store_true",
        help="invent gaze data instead of talking to hardware",
    )
    p.add_argument(
        "--check",
        action="store_true",
        help="verify pylink and the Host PC, then exit",
    )
    p.add_argument(
        "--no-stepwise",
        action="store_true",
        help="do not offer stepwise calibration (the client then falls back "
        "to pylink's own setup window, which ignores the display transform)",
    )
    p.add_argument(
        "--check-calibration",
        action="store_true",
        help="run one stepwise calibration from the terminal, logging what the "
        "Host asks for, then exit",
    )
    p.add_argument("--width", type=int, default=1920,
                   help="screen width in pixels, for --check-calibration")
    p.add_argument("--height", type=int, default=1080,
                   help="screen height in pixels, for --check-calibration")
    p.add_argument("--once", action="store_true", help="exit after one client")
    p.add_argument("-v", "--verbose", action="store_true")
    args = p.parse_args()

    if args.check:
        sys.exit(check(args))
    if args.check_calibration:
        sys.exit(check_calibration(args))

    serve_forever(
        args,
        make_tracker,
        "SIMULATED" if args.simulate else args.tracker_host,
    )


if __name__ == "__main__":
    main()
