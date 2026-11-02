# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""A stand-in for pylink, for testing eyelink_bridge.py's stepwise relay.

It plays the Host PC's setup routine as the bridge expects it to behave: "c"
starts a calibration that shows the configured targets in the configured
sequence, Enter accepts a target and then the result, Esc leaves a
calibration and then setup. That is a model of the Host, not the Host: it
proves the relay's threading and bookkeeping, and only a run against a real
tracker (eyelink_bridge.py --check-calibration) proves the model.
"""

import time

ENTER_KEY = 13
ESC_KEY = 27
NO_REPLY = 1000

_display = None
drawn = []  # every (x, y) the Host asked to be drawn, for the test to inspect


class KeyInput:
    def __init__(self, key, mod=0):
        self.key = key
        self.mod = mod


class EyeLinkCustomDisplay:
    def __init__(self):
        pass


def openGraphicsEx(display):
    global _display
    _display = display


def closeGraphics():
    global _display
    _display = None


def pumpDelay(ms):
    time.sleep(ms / 1000.0)


class EyeLink:
    def __init__(self, host):
        self.cmds = {}
        self.result = NO_REPLY
        self.msg = ""

    def openDataFile(self, name):
        pass

    def setOfflineMode(self):
        pass

    def sendCommand(self, c):
        k, _, v = c.partition("=")
        self.cmds[k.strip()] = v.strip()

    def sendMessage(self, m):
        pass

    def getTrackerVersionString(self):
        return "FAKE EYELINK"

    def close(self):
        pass

    def doTrackerSetup(self):
        d = _display
        targets = [tuple(int(v) for v in p.split(","))
                   for p in self.cmds["calibration_targets"].split()]
        order = [int(v) for v in self.cmds["calibration_sequence"].split(",")]
        assert self.cmds.get("generate_default_targets") == "NO"
        state, i = "setup", 0
        deadline = time.time() + 30
        while time.time() < deadline:
            for k in d.get_input_key() or []:
                code = k.key
                if state == "setup":
                    if code == ord("c"):
                        state, i = "cal", 0
                        d.erase_cal_target()
                        drawn.append(targets[order[0]])
                        d.draw_cal_target(*targets[order[0]])
                    elif code == ESC_KEY:
                        return 0
                elif state == "cal":
                    if code == ENTER_KEY:
                        d.erase_cal_target()
                        i += 1
                        if i < len(order):
                            drawn.append(targets[order[i]])
                            d.draw_cal_target(*targets[order[i]])
                        else:
                            state = "result"
                            self.msg = "calibration_result: GOOD (fake)"
                    elif code == ESC_KEY:
                        state = "setup"
                elif state == "result":
                    if code in (ENTER_KEY, ESC_KEY):
                        if code == ENTER_KEY:
                            self.result = 0
                        state = "setup"
        raise RuntimeError("fake doTrackerSetup: no Esc within 30 s")

    def getCalibrationResult(self):
        return self.result

    def getCalibrationMessage(self):
        return self.msg
