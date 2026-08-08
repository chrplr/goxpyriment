#!/usr/bin/env python3
"""Inline the figures into the report and write a standalone HTML file.

    ./build.py  ->  console-timing-report.html

The source keeps `__SEQ__` / `__WAY__` / `__KMS__` placeholders rather than
700 KB of base64, so the report stays reviewable in a diff and regenerates
whenever the figures change. Published artifacts are served under a strict CSP
that blocks every external host, so the images have to be data URIs -- there is
no version of this that links out to a file.
"""
import base64
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
FIGS = HERE.parent / "figures"
IMAGES = {"__STACK__": "timing-stacks.png",
          "__SEQ__": "timing-sequence-5min.png",
          "__WAY__": "traces-wayland.png",
          "__XORG__": "traces-xorg.png"}

html = (HERE / "console-timing-report.src.html").read_text()
for key, name in IMAGES.items():
    f = FIGS / name
    if not f.exists():
        sys.exit(f"missing figure: {f}")
    html = html.replace(key, base64.b64encode(f.read_bytes()).decode())
if "__" in html.replace("__pycache__", ""):
    sys.exit("a placeholder was left unreplaced")

out = HERE / "console-timing-report.html"
out.write_text(html)
print(f"wrote {out} ({out.stat().st_size / 1e6:.2f} MB)")
