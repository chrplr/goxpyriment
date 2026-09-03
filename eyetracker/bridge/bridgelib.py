#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Transport shared by every goxpyriment eyetracker bridge.

The Go client is eyetracker.Bridge and the wire format is specified in
eyetracker/protocol.go: one JSON object per line, UTF-8, newline-terminated, in
both directions.

Nothing here knows about any particular tracker. A bridge is a module that
defines a back-end class and calls serve_forever() with a factory for it, which
is what `eyetracker/CLAUDE.md` means by "a bridge for another tracker is a new
script, not a new Go package".

# The back-end contract

A back end is any object with these attributes:

    name        str  — reported in the hello event, e.g. "eyelink", "tobii"
    simulated   bool — true if it invents data; the client warns and records it

and these methods:

    open(host, edf, width, height) -> dict
    calibrate(points)              -> dict
    start_recording()
    stop_recording()
    mark(text)
    tracker_time()                 -> float, milliseconds
    receive_file(path)             -> dict
    close()
    poll()                         -> list of protocol event dicts

poll() must return promptly whether or not it has anything, and must sleep
briefly when it has nothing: it is called in a loop, and a poll() that returns
an empty list without yielding spins a core at 100%.

# Optional: stepwise calibration

A tracker whose SDK draws no calibration targets cannot implement calibrate()
usefully, because the client has to put each target on screen. Such a back end
instead defines any of

    calibration_enter()
    calibration_collect(x, y)      — x, y NORMALIZED on the display area
    calibration_discard(x, y)
    calibration_compute()          -> dict with "status" and "points"
    calibration_leave()

and the corresponding commands become available. A back end that defines none
of them rejects those commands by name, so an experiment driving the wrong
tracker gets an error that says so rather than a silent no-op.
"""

import json
import queue
import socket
import sys
import threading
import time

# The protocol version this transport speaks. It must match protoVersion in
# eyetracker/protocol.go; the client fails at Open on a mismatch rather than
# puzzlingly three commands later.
PROTO = 1

# The stepwise-calibration commands, dispatched to the back end by name. Held
# as a set rather than tested with a startswith() so that a typo on the wire is
# an unknown command rather than a getattr against an arbitrary attribute.
CALIBRATION_COMMANDS = (
    "calibration_enter",
    "calibration_collect",
    "calibration_discard",
    "calibration_compute",
    "calibration_leave",
)


def log(msg):
    print("[bridge] %s" % msg, file=sys.stderr, flush=True)


class Session:
    """One connected client. Commands on one thread, samples on another."""

    def __init__(self, conn, tracker, verbose=False):
        self.conn = conn
        self.tracker = tracker
        self.verbose = verbose
        self.out = queue.Queue(maxsize=100000)
        self.running = True
        self.polling = False

    # -- writing ----------------------------------------------------------

    def send(self, obj):
        try:
            self.out.put_nowait(obj)
        except queue.Full:
            # Dropping is better than blocking the sample pump on a slow
            # client, but it must never be silent.
            log("output queue full; dropped a message")

    def writer(self):
        f = self.conn.makefile("w", encoding="utf-8", newline="\n")
        while self.running:
            try:
                obj = self.out.get(timeout=0.2)
            except queue.Empty:
                continue
            try:
                f.write(json.dumps(obj) + "\n")
                f.flush()
            except Exception as exc:
                log("write failed: %s" % exc)
                self.running = False
                return

    def pump(self):
        while self.running:
            if not self.polling:
                time.sleep(0.005)
                continue
            try:
                for ev in self.tracker.poll():
                    self.send(ev)
            except Exception as exc:
                log("poll failed: %s" % exc)
                self.send({"ev": "log", "level": "error", "msg": "poll: %s" % exc})
                self.polling = False

    # -- commands ---------------------------------------------------------

    def calibration(self, cmd, args):
        """Dispatch a stepwise-calibration command to the back end."""
        fn = getattr(self.tracker, cmd, None)
        if fn is None:
            raise ValueError(
                "%s does not support %s: this tracker calibrates through its "
                "own setup routine, so use the `calibrate` command instead"
                % (self.tracker.name, cmd)
            )
        if cmd in ("calibration_collect", "calibration_discard"):
            return fn(float(args["x"]), float(args["y"]))
        return fn()

    def handle(self, req):
        cmd = req.get("cmd", "")
        args = req.get("args") or {}
        t = self.tracker

        if cmd == "open":
            return t.open(
                args.get("host", ""),
                args.get("edf", ""),
                int(args.get("width", 1920)),
                int(args.get("height", 1080)),
            )
        if cmd == "calibrate":
            return t.calibrate(int(args.get("points", 0)))
        if cmd in CALIBRATION_COMMANDS:
            return self.calibration(cmd, args)
        if cmd == "start_recording":
            t.start_recording()
            self.polling = True
            return {}
        if cmd == "stop_recording":
            self.polling = False
            t.stop_recording()
            return {}
        if cmd == "mark":
            t.mark(str(args.get("text", "")))
            return {}
        if cmd == "tracker_time":
            return {"time": t.tracker_time()}
        if cmd == "receive_file":
            self.polling = False
            return t.receive_file(str(args.get("path", "")))
        if cmd == "close":
            self.polling = False
            t.close()
            self.running = False
            return {}
        raise ValueError("unknown command %r" % cmd)

    def serve(self):
        threading.Thread(target=self.writer, daemon=True).start()
        threading.Thread(target=self.pump, daemon=True).start()

        self.send(
            {
                "ev": "hello",
                "bridge": self.tracker.name,
                "proto": PROTO,
                "simulated": self.tracker.simulated,
            }
        )

        f = self.conn.makefile("r", encoding="utf-8")
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                req = json.loads(line)
            except Exception as exc:
                log("undecodable request: %s" % exc)
                continue
            rid = req.get("id", 0)
            if self.verbose:
                log("<- %s" % line)
            try:
                result = self.handle(req)
                self.send({"id": rid, "ok": True, "result": result or {}})
            except Exception as exc:
                log("%s failed: %s" % (req.get("cmd"), exc))
                self.send({"id": rid, "ok": False, "error": str(exc)})
            if not self.running:
                break
        # Give the writer a moment to flush the final response before the
        # socket goes away, so a client waiting on the reply to `close` sees it
        # rather than a connection reset.
        time.sleep(0.1)
        self.running = False


def serve_forever(args, make_tracker, label=""):
    """Accept clients one at a time, serving each with a fresh back end.

    args supplies host, port, once and verbose. make_tracker is called per
    connection and returns the back end; building it late means a tracker that
    failed to open does not poison the next client. label appears in the
    listening line, so the operator can see which device this bridge is for.
    """
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind((args.host, args.port))
    srv.listen(1)
    log("listening on %s:%d (%s)" % (args.host, args.port, label or "?"))

    while True:
        conn, addr = srv.accept()
        # Nagle would coalesce samples into ~40 ms bursts, which is invisible
        # in the data and fatal for a gaze-contingent loop.
        conn.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
        log("client connected from %s:%d" % addr)
        tracker = make_tracker(args)
        session = Session(conn, tracker, args.verbose)
        try:
            session.serve()
        except Exception as exc:
            log("session ended: %s" % exc)
        finally:
            session.running = False
            try:
                tracker.close()
            except Exception:
                pass
            conn.close()
            log("client disconnected")
        if args.once:
            return
