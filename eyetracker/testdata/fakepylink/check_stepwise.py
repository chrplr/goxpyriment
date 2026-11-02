# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

"""Drive EyeLinkTracker's stepwise calibration against the fake pylink.

Run by TestEyeLinkRelayAgainstFakePylink with the bridge directory and this
directory on sys.path. Prints OK on success; any failure raises.
"""

import sys

import eyelink_bridge as eb
import pylink

W, H = 1280, 800
P9 = [(0.5, 0.5), (0.1, 0.1), (0.9, 0.1), (0.1, 0.9), (0.9, 0.9),
      (0.5, 0.1), (0.1, 0.5), (0.9, 0.5), (0.5, 0.9)]

t = eb.EyeLinkTracker("fake")
t.CAL_TIMEOUT = 2.0
t.open("", "t.edf", W, H)

# A target out of the Host's order is refused, naming both.
t.calibration_enter(P9)
try:
    t.calibration_collect(*P9[1])
    raise SystemExit("an out-of-order collect was accepted")
except RuntimeError as exc:
    assert "not showing target" in str(exc), exc
t.calibration_leave()
assert t._cal is None

# The whole sequence, in the client's order, at the client's pixels.
pylink.drawn.clear()
t.calibration_enter(P9)
for p in P9:
    t.calibration_collect(*p)
res = t.calibration_compute()
t.calibration_leave()
assert res["status"] == "calibration_status_success", res
assert "GOOD" in res["message"], res
want = [(int(round(x * W)), int(round(y * H))) for x, y in P9]
assert pylink.drawn == want, (pylink.drawn, want)
assert all(p["used"] == 1 for p in res["points"]), res

# Leaving in the middle ends the setup thread.
t.calibration_enter(P9)
t.calibration_collect(*P9[0])
cal = t._cal
t.calibration_leave()
assert not cal.thread.is_alive(), "doTrackerSetup still running after leave"

t.close()
print("OK")
sys.exit(0)
