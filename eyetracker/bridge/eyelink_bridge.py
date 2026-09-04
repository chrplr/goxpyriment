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

The bridge is NOT in the timing path and must not be put there. To mark a
stimulus onset in the EDF, send a TTL pulse to the Host PC's parallel port: the
Host timestamps the edge itself, with no socket and no round trip in between.
Use `mark` for trial labels and bookkeeping, never for an onset whose timestamp
is the measurement.
"""

import argparse
import sys
import time

# The transport lives in bridgelib so that every bridge speaks the same wire
# format from the same code. sys.path[0] is this script's own directory when it
# is run by path, which is how the Go tests launch it.
from bridgelib import log, serve_forever

# EyeLink's missing-data sentinel. It is a plausible-looking number, which is
# exactly why every coordinate is checked against it before being sent.
MISSING = -32768.0


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
        import math

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
        if result != 0:
            raise RuntimeError(
                "no usable calibration was stored (result %s%s). The operator "
                "left setup without completing one, or every target was "
                "rejected. Recording now would produce gaze positions that "
                "mean nothing." % (result, ": " + message if message else "")
            )
        log("calibration stored: %s" % (message or "(no message)"))
        return {"points": points or 9, "verified": True,
                "result": result, "message": message}

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

        def put(key, fn, *idx):
            try:
                v = fn()
            except Exception:
                return
            if v is None:
                return
            if idx:
                try:
                    a, b = v
                except Exception:
                    return
                if a > MISSING and b > MISSING:
                    ev[idx[0]] = float(a)
                    ev[idx[1]] = float(b)
                return
            ev[key] = float(v)

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
        return SimTracker()
    try:
        return EyeLinkTracker(args.tracker_host)
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
    p.add_argument("--once", action="store_true", help="exit after one client")
    p.add_argument("-v", "--verbose", action="store_true")
    args = p.parse_args()

    if args.check:
        sys.exit(check(args))

    serve_forever(
        args,
        make_tracker,
        "SIMULATED" if args.simulate else args.tracker_host,
    )


if __name__ == "__main__":
    main()
