#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).
"""
PsychoPy timing tests — counterpart to the goxpyriment Timing-Tests binary.

Implements the same sub-tests and prints statistics in the same format so
results can be compared directly between the two frameworks.

Sub-tests (four tiers — run in this order)
------------------------------------------
Tier 0 — sanity check (no equipment, no measurement):
  check    Verify display flash and audio output   (alias: audio)

Tier 1 — self-contained measurements (computer only):
  display  Frame-interval statistics and true refresh rate  (alias: jitter)
  latency  Audio pipeline latency                           (alias: drain)
  stream   Sequential-stimulus (RSVP) onset/duration accuracy + triggers
  vrr      Variable Refresh Rate sweep: 1 ms to N ms in 1 ms steps

Tier 2 — trigger device characterisation (DLP-IO8-G + oscilloscope):
  trigger  Square-wave output for DLP-IO8-G precision test  (alias: square)

Tier 3 — stimulus timing validation (photodiode + oscilloscope):
  frames   Alternating luminance: visual onset vs. trigger alignment
  flash    Single-frame white flashes: minimum stimulus duration
  tones    Regular tone stream: audio onset jitter over time  (alias: sound)
  av       Audio-visual synchrony with controllable SOA

Tier 4 — response timing:
  rt       Keyboard reaction-time precision test

Usage
-----
  python timing_tests.py --sysinfo
  python timing_tests.py --test display --duration-s 300
  python timing_tests.py --test frames --frames-on 1 --frames-off 2 --cycles 6000
  python timing_tests.py --test frames --frames-on 1 --frames-off 2 --cycles 6000 --gc
  python timing_tests.py --test av --frames-on 12 --frames-off 18 --cycles 1000 --audio-frames 256

Flags mirror the Go binary: -w = windowed, -d N = display index, --gc leaves the
collector running. Note that -d used to mean "windowed" here, the opposite of
the Go binary, which silently turned fullscreen comparisons into windowed ones.

Timing notes
------------
PsychoPy's win.flip() blocks until the next VSYNC boundary (waitBlanking=True),
mirroring SDL's VSync in goxpyriment.  The return value is the flip timestamp
on defaultClock (core.getTime()), captured right after SwapBuffers returns —
the same instant that fillGray() captures as tAfter in the Go binary.

The Python GC is disabled during measurement loops via gc.disable(), mirroring
Go's debug.SetGCPercent(-1). Pass --gc to leave it running and measure its
effect; run each test twice, with and without, for the GC-on/GC-off comparison.

The two are not perfectly symmetric, and the asymmetry belongs in any write-up:
gc.disable() stops CPython's *cyclic* collector only — reference counting keeps
freeing objects deterministically and cannot be switched off — whereas Go's
tracing collector is suspended outright.

The audio backend is pinned (default: ptb) before psychopy.sound is imported,
and --audio-frames maps to the PTB backend's blockSize, the counterpart to the
Go binary's -audio-frames. Without both, audio results characterise the local
PsychoPy install rather than PsychoPy itself.

For the rt test, key timestamps come from psychopy.hardware.keyboard.Keyboard.
With psychtoolbox installed (pip install psychtoolbox), events are timestamped
at hardware-interrupt time (matching goxpyriment's SDL3 nanosecond clock).
Without psychtoolbox, timestamps reflect Python poll-loop time (~1–5 ms jitter).

For the latency test, sounddevice.wait() is used to detect when the audio driver
has consumed all queued PCM data — the Python equivalent of SDL's stream.Queued()==0.

For the vrr test, win.waitBlanking is set to False for the duration so that
win.flip() returns immediately without waiting for VSYNC, matching SDL's
SDL_RENDERER_VSYNC_DISABLED.  On a VRR-capable monitor the panel dynamically
adjusts its refresh interval; on a fixed-rate display errors cluster at frame
multiples.

DLP-IO8-G trigger device
------------------------
Trigger output is optional for frames/flash/av/tones/rt/stream/vrr.  It is
required for trigger (square).  The same ASCII command protocol as
goxpyriment's triggers/dlpio8.go is used:
  Set HIGH pin N : '1'–'8'
  Set LOW  pin N : 'Q','W','E','R','T','Y','U','I'
  Ping           : "'" → device responds 'Q'
Install: pip install pyserial
"""

import argparse
import gc
import math
import platform
import random
import sys
import threading
import traceback
import time

import numpy as np


def _preselect_audio_lib(argv: list) -> str:
    """
    Read --audio-lib from the raw argv before argparse runs.

    PsychoPy resolves its audio backend at `import psychopy.sound` time, so the
    preference has to be set before that import — earlier than the real
    argument parse.
    """
    for i, a in enumerate(argv):
        if a == "--audio-lib" and i + 1 < len(argv):
            return argv[i + 1]
        if a.startswith("--audio-lib="):
            return a.split("=", 1)[1]
    return "ptb"


# Pin the audio backend explicitly. PsychoPy's audio latency differs by roughly
# an order of magnitude between backends (ptb ≪ pygame), so leaving this to
# whatever the local install defaults to would make the tones/av results a
# property of the machine's configuration rather than of PsychoPy — and would
# make the comparison against goxpyriment meaningless.
_AUDIO_LIB = _preselect_audio_lib(sys.argv[1:])

from psychopy import prefs  # noqa: E402

prefs.hardware["audioLib"] = [_AUDIO_LIB]

from psychopy import core, event, logging, sound, visual  # noqa: E402

logging.console.setLevel(logging.WARNING)


# ── DLP-IO8-G trigger device ──────────────────────────────────────────────────
# Same ASCII protocol as Go's triggers/dlpio8.go

_SET_HIGH = [None, b"1", b"2", b"3", b"4", b"5", b"6", b"7", b"8"]
_SET_LOW  = [None, b"Q", b"W", b"E", b"R", b"T", b"Y", b"U", b"I"]


class DLPIO8:
    """DLP-IO8 / DLP-IO8-G USB-CDC trigger device."""

    def __init__(self, port: str):
        import serial
        self._ser = serial.Serial(port, 115200, timeout=0.2)
        if not self._ping():
            self._ser.close()
            raise IOError(f"DLP-IO8-G: no response on {port}")
        self._ser.write(b"\\")  # enable binary read mode

    def _ping(self) -> bool:
        self._ser.reset_input_buffer()
        self._ser.write(b"'")
        for _ in range(3):
            if self._ser.read(1) == b"Q":
                return True
        return False

    def set_high(self, pin: int) -> None:
        self._ser.write(_SET_HIGH[pin])

    def set_low(self, pin: int) -> None:
        self._ser.write(_SET_LOW[pin])

    def all_low(self) -> None:
        for pin in range(1, 9):
            self.set_low(pin)

    def close(self) -> None:
        self.all_low()
        self._ser.close()


class NullTrigger:
    """No-op trigger used when no DLP-IO8-G is found."""
    def set_high(self, pin: int) -> None: pass
    def set_low(self, pin: int) -> None: pass
    def all_low(self) -> None: pass
    def close(self) -> None: pass


def setup_trigger(port: str | None, pin: int) -> tuple:
    """Open DLP-IO8-G (auto-detecting if port is None). Returns (device, port_name)."""
    if port:
        try:
            d = DLPIO8(port)
            print(f"DLP-IO8-G found on {port} (trigger pin {pin})")
            return d, port
        except Exception as exc:
            print(f"warning: DLP-IO8-G on {port}: {exc} — triggers disabled")
            return NullTrigger(), ""
    try:
        import serial.tools.list_ports
        ports = [p.device for p in serial.tools.list_ports.comports()]
    except ImportError:
        ports = []
    for p in ports:
        try:
            d = DLPIO8(p)
            print(f"DLP-IO8-G auto-detected on {p} (trigger pin {pin})")
            return d, p
        except Exception:
            continue
    print("DLP-IO8-G: not found — trigger output disabled")
    return NullTrigger(), ""


# ── CLI ───────────────────────────────────────────────────────────────────────

_ALL_TESTS = [
    # primary names
    "check", "display", "latency", "stream", "vrr",
    "trigger", "frames", "flash", "tones", "av", "rt",
    # legacy aliases
    "audio", "jitter", "drain", "square", "sound",
]


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    # Not required, so that --sysinfo can run on its own; main() enforces that
    # one of --test / --sysinfo is present.
    p.add_argument("--test", default=None,
                   choices=_ALL_TESTS,
                   metavar="TEST",
                   help=("Sub-test: check|display|latency|stream|vrr|"
                         "trigger|frames|flash|tones|av|rt  "
                         "(legacy aliases: audio=check  jitter=display  "
                         "drain=latency  square=trigger  sound=tones)"))
    # -w / -d carry the same meaning as in the Go binary. They used to be
    # swapped here (-d meant "windowed"), which silently turned a fullscreen
    # comparison into a windowed one — and windowed mode is compositor-throttled,
    # so the timing it produces is not meaningful.
    p.add_argument("-w", action="store_true", dest="windowed",
                   help="Windowed 1024×768 mode (default: fullscreen)")
    p.add_argument("-d", type=int, default=-1, dest="display",
                   help="Display index for fullscreen; -1 = primary (default: -1)")
    p.add_argument("--sysinfo", action="store_true",
                   help="Print system/configuration information and exit")
    p.add_argument("--gc", action="store_true",
                   help=("Leave the Python garbage collector RUNNING during "
                         "measurement loops. By default the cyclic collector is "
                         "suspended with gc.disable(), mirroring Go's "
                         "debug.SetGCPercent(-1); pass --gc to measure its effect "
                         "(run the same test twice, with and without)."))
    p.add_argument("--audio-lib", default="ptb", dest="audio_lib",
                   help=("PsychoPy audio backend preference: ptb|sounddevice|pyo|"
                         "pygame (default: ptb). NOTE: PsychoPy 2026.1.x routes all "
                         "playback through its SpeakerDevice/PsychToolbox layer and "
                         "ignores this for stream creation — verify with --sysinfo "
                         "before relying on it."))
    p.add_argument("--audio-device", type=int, default=None, dest="audio_device",
                   help=("Speaker device index to open (see --sysinfo for the list). "
                         "This, not --audio-lib, is the effective control on "
                         "PsychoPy 2026.1.x. Default: PsychoPy's first-found device."))
    p.add_argument("--audio-frames", type=int, default=0, dest="audio_frames",
                   help=("Audio hardware buffer in sample frames; 0 = backend "
                         "default. Counterpart to the Go binary's -audio-frames "
                         "(ptb backend only, where it sets blockSize)."))
    # trigger
    p.add_argument("--port", default=None,
                   help="Serial port for DLP-IO8-G (default: auto-detect)")
    p.add_argument("--trigger-pin", type=int, default=1, dest="trigger_pin",
                   help="Output pin 1–8 (default: 1)")
    p.add_argument("--trigger-ms", type=float, default=5.0, dest="trigger_ms",
                   help="Trigger pulse duration ms (default: 5)")
    # common
    p.add_argument("--cycles", type=int, default=60,
                   help="Cycles / flashes / trials (default: 60)")
    p.add_argument("--hz", type=float, default=60.0,
                   help="Expected display refresh rate Hz (default: 60)")
    p.add_argument("--warmup", type=int, default=10,
                   help="Frames discarded at start (default: 10)")
    # frames / flash
    p.add_argument("--level-a", type=int, default=0, dest="level_a",
                   help="Dark luminance 0–255 (default: 0)")
    p.add_argument("--level-b", type=int, default=255, dest="level_b",
                   help="Bright luminance 0–255 (default: 255)")
    # Independent bright/dark durations, matching the Go binary. The old
    # --frames-per-phase forced a symmetric square wave and so could not express
    # the 1-on / 2-off stimulus that run-timing-tests.sh uses.
    p.add_argument("--frames-on", type=int, default=1, dest="frames_on",
                   help="Bright frames per cycle [frames / av tests] (default: 1)")
    p.add_argument("--frames-off", type=int, default=9, dest="frames_off",
                   help="Dark frames per cycle [frames / av tests] (default: 9)")
    # av / sound / rt
    p.add_argument("--soa-ms", type=float, default=0.0, dest="soa_ms",
                   help="Visual-to-audio SOA ms; negative=audio first (default: 0)")
    p.add_argument("--iti-ms", type=float, default=1000.0, dest="iti_ms",
                   help="Inter-trial/stimulus interval ms (default: 1000)")
    p.add_argument("--freq-hz", type=float, default=1000.0, dest="freq_hz",
                   help="Tone frequency Hz (default: 1000)")
    p.add_argument("--tone-ms", type=int, default=50, dest="tone_ms",
                   help="Tone duration ms (default: 50)")
    # jitter / square
    p.add_argument("--duration-s", type=float, default=10.0, dest="duration_s",
                   help="Measurement duration seconds (default: 10)")
    p.add_argument("--period-ms", type=float, default=100.0, dest="period_ms",
                   help="Square-wave period ms (default: 100)")
    p.add_argument("--duty", type=float, default=50.0,
                   help="Square-wave duty cycle %% (default: 50)")
    # drain / latency
    p.add_argument("--drain-reps", type=int, default=10, dest="drain_reps",
                   help="Repetitions per tone duration (default: 10)")
    # vrr
    p.add_argument("--vrr-max-ms", type=int, default=50, dest="vrr_max_ms",
                   help="Maximum sweep duration in ms for vrr test (default: 50)")
    return p


# ── Garbage-collector control ─────────────────────────────────────────────────

class suspend_gc:
    """
    Context manager that disables the cyclic garbage collector for the duration
    of a measurement loop, mirroring the Go binary's suspendGC().

    When --gc is passed the collector is deliberately left running so its effect
    on timing can be measured; the context manager is then a no-op. Running the
    same test twice, with and without --gc, yields the GC-on/GC-off comparison.

    Note the asymmetry with Go, which matters when interpreting the comparison:
    gc.disable() stops CPython's *cyclic* collector only. Reference counting
    continues to free objects deterministically and cannot be switched off,
    whereas Go's tracing collector is suspended entirely by
    debug.SetGCPercent(-1).
    """

    def __init__(self, args):
        self._leave_running = bool(getattr(args, "gc", False))

    def __enter__(self):
        if not self._leave_running:
            gc.disable()
        return self

    def __exit__(self, *exc):
        if not self._leave_running:
            gc.enable()
        return False


def gc_label(args) -> str:
    """Describe the collector state, for report headers."""
    return "on" if getattr(args, "gc", False) else "suspended"


# ── Helpers ───────────────────────────────────────────────────────────────────

def make_tone(args, freq_hz: float, secs: float, volume: float = 0.8):
    """
    Build a Sound, honouring --audio-frames where the backend supports it.

    Go's -audio-frames sets the SDL hardware buffer in sample frames; the PTB
    backend's equivalent is blockSize (per-sound, unlike other backends). Other
    backends have no equivalent and ignore the setting.
    """
    kwargs = dict(value=freq_hz, secs=secs, volume=volume,
                  sampleRate=44100, stereo=True)
    if args.audio_frames > 0:
        kwargs["blockSize"] = args.audio_frames
    if args.audio_device is not None:
        kwargs["speaker"] = args.audio_device
    try:
        return sound.Sound(**kwargs)
    except Exception as exc:
        raise SystemExit(
            f"\naudio: could not open an output stream.\n"
            f"  {type(exc).__name__}: {exc}\n\n"
            f"PsychoPy 2026.1.x opens every sound through its SpeakerDevice /\n"
            f"PsychToolbox layer, so --audio-lib does NOT change this — pick a\n"
            f"different device instead:\n"
            f"    python timing_tests.py --sysinfo       # lists device indices\n"
            f"    python timing_tests.py --test check --audio-device N\n\n"
            f"If every device fails, PsychoPy has no usable audio session here.\n"
            f"Check that a sound server is running for this login session\n"
            f"(pipewire/pulseaudio) and that the shell can reach it.\n"
            f"Record the device index used in the report — audio numbers are only\n"
            f"comparable across machines that opened the same kind of device.\n"
        ) from exc


def level_to_psychopy(level: int) -> float:
    """Convert 0–255 luminance byte to PsychoPy's [-1, 1] color space."""
    return (level / 127.5) - 1.0


def sleep_until(target_t: float) -> None:
    """
    Sleep until core.getTime() reaches target_t, busy-spinning the last 500 µs.
    Mirrors Go's sleepUntil() in main.go.
    """
    remaining = target_t - core.getTime()
    if remaining > 0.0005:
        core.wait(remaining - 0.0005)
    while core.getTime() < target_t:
        pass


def trigger_pulse_async(trig, pin: int, duration_s: float) -> None:
    """Fire a trigger pulse in a daemon thread. Mirrors Go's goroutine approach."""
    def _low():
        time.sleep(duration_s)
        trig.set_low(pin)
    threading.Thread(target=_low, daemon=True).start()


def _ptb_available() -> bool:
    try:
        import psychtoolbox  # noqa: F401
        return True
    except ImportError:
        return False


def _flip(win) -> float:
    """win.flip() with fallback to core.getTime() for backends that return None."""
    t = win.flip()
    return t if t is not None else core.getTime()


# ── Statistics ────────────────────────────────────────────────────────────────

def compute_stats(vals: list, target_ms: float) -> dict | None:
    """Same computation as Go's computeStats()."""
    n = len(vals)
    if n == 0:
        return None
    arr = np.array(vals, dtype=float)
    mean = float(arr.mean())
    sd = float(arr.std(ddof=1)) if n > 1 else 0.0
    mn, mx = float(arr.min()), float(arr.max())
    s = np.sort(arr)
    p5 = float(s[n * 5 // 100])
    p95 = float(s[min(n - 1, n * 95 // 100)])
    devs = np.abs(arr - target_ms)
    return {
        "n": n, "mean": mean, "sd": sd, "min": mn, "max": mx,
        "p5": p5, "p95": p95,
        "late05": int((devs > 0.5).sum()),
        "late1": int((devs > 1.0).sum()),
        "vals": list(arr),
    }


def print_histogram(vals: list, n_bins: int = 10, bar_width: int = 40) -> None:
    """10-bin ASCII histogram matching Go's printHistogram() format exactly."""
    n = len(vals)
    if n == 0:
        return
    arr = np.array(vals, dtype=float)
    mn, mx = float(arr.min()), float(arr.max())
    bin_w = (mx - mn) / n_bins if mx > mn else 1.0
    counts = [0] * n_bins
    for v in arr:
        b = min(int((v - mn) / bin_w), n_bins - 1)
        counts[b] += 1
    max_count = max(counts) if max(counts) > 0 else 1
    print(f"  histogram ({n_bins} bins):")
    for i in range(n_bins):
        lo = mn + i * bin_w
        hi = lo + bin_w
        bar = "*" * (counts[i] * bar_width // max_count)
        print(f"  [{lo:7.3f}, {hi:7.3f}) ms : {counts[i]:5d}  {bar}")


def print_stats(label: str, s: dict, target_ms: float) -> None:
    """Print statistics in the same format as Go's printStats()."""
    print(f"\n── {label} ───────────────────────────────")
    print(f"  n       : {s['n']}")
    print(f"  target  : {target_ms:.3f} ms")
    print(f"  mean    : {s['mean']:.3f} ms")
    print(f"  SD      : {s['sd']:.3f} ms")
    print(f"  min/max : {s['min']:.3f} / {s['max']:.3f} ms")
    print(f"  p5/p95  : {s['p5']:.3f} / {s['p95']:.3f} ms")
    print(f"  >0.5 ms : {s['late05']} ({100 * s['late05'] / s['n']:.1f} %)")
    print(f"  >1.0 ms : {s['late1']} ({100 * s['late1'] / s['n']:.1f} %)")
    print_histogram(s["vals"])


# ── Test: frames ──────────────────────────────────────────────────────────────

def run_frames(win, trig, args) -> None:
    """
    Alternate a bright phase of --frames-on frames with a dark phase of
    --frames-off frames, for --cycles cycles. A trigger pulse is fired on the
    first frame of each bright phase.

    Reports the same two quantities as the Go binary, against the *measured*
    mean rather than a nominal target derived from --hz:

      bright_duration_ms = first dark flip − first bright flip
      period_ms          = this bright onset − previous bright onset

    The single-frame flash case is simply --frames-on 1, which is why `flash`
    is now an alias for this test rather than a separate implementation.

    Matches: Timing-Tests -test frames -level-a N -level-b N
             -frames-on N -frames-off N -cycles N [-w] [-d N] [-gc]
    """
    frames_on, frames_off = args.frames_on, args.frames_off
    print(f"frames: level-a={args.level_a} level-b={args.level_b}"
          f" frames-on={frames_on} frames-off={frames_off}"
          f" cycles={args.cycles} warmup={args.warmup}")

    col_a = level_to_psychopy(args.level_a)
    col_b = level_to_psychopy(args.level_b)
    bright_durations: list[float] = []
    periods: list[float] = []
    prev_bright_start: float | None = None
    is_null = isinstance(trig, NullTrigger)

    with suspend_gc(args):
        for cycle in range(args.cycles):
            # ── Bright phase ──────────────────────────────────────────────────
            # Re-draw every frame so double-buffering never shows the other colour.
            t_bright_start = None
            for f in range(frames_on):
                win.color = [col_b, col_b, col_b]
                t_a = _flip(win)
                if f == 0:
                    t_bright_start = t_a
                    if not is_null:
                        trig.set_high(args.trigger_pin)
                        trigger_pulse_async(trig, args.trigger_pin,
                                            args.trigger_ms / 1000)

            # ── Dark phase ────────────────────────────────────────────────────
            t_dark_start = None
            for f in range(frames_off):
                win.color = [col_a, col_a, col_a]
                t_a = _flip(win)
                if f == 0:
                    t_dark_start = t_a

            if event.getKeys(keyList=["escape"]):
                print("  (stopped early by ESC)")
                break

            # ── Record measurements ───────────────────────────────────────────
            bright_dur_ms = (t_dark_start - t_bright_start) * 1000
            period_ms = 0.0
            if prev_bright_start is not None:
                period_ms = (t_bright_start - prev_bright_start) * 1000

            if cycle >= args.warmup:
                bright_durations.append(bright_dur_ms)
                if period_ms > 0:
                    periods.append(period_ms)

            prev_bright_start = t_bright_start

    # Deviation is reported against the measured mean, as the Go binary does,
    # so no --hz is needed and a wrong nominal refresh rate cannot skew it.
    s_dur = compute_stats(bright_durations, 0)
    if s_dur:
        s_dur = compute_stats(bright_durations, s_dur["mean"])
        print_stats(f"Bright-phase duration (frames-on={frames_on})",
                    s_dur, s_dur["mean"])
    s_per = compute_stats(periods, 0)
    if s_per:
        s_per = compute_stats(periods, s_per["mean"])
        print_stats(f"Period (frames-on={frames_on} + frames-off={frames_off})",
                    s_per, s_per["mean"])


# ── Test: av ──────────────────────────────────────────────────────────────────

def run_av(win, trig, args) -> None:
    """
    Present periodic visual flashes paired with tones at a configurable SOA.

    The bright phase lasts --frames-on frames and the tone duration matches that
    duration (frames-on × refresh period, derived from --hz). The dark ITI
    between stimuli lasts --frames-off frames. --iti-ms and --tone-ms are not
    used by this test, exactly as in the Go binary.

    t_audio_queued_ms is when play() was called (PCM handed to the driver), not
    the acoustic onset; the true delay is what the BBTK microphone measures.

    Matches: Timing-Tests -test av -soa-ms N -freq-hz N
             -frames-on N -frames-off N -cycles N [-w] [-d N]
    """
    frames_on, frames_off = args.frames_on, args.frames_off
    frame_ms = 1000.0 / args.hz
    tone_dur_ms = int(round(frames_on * frame_ms))

    # SOA=0 uses callOnFlip so the tone is triggered by the flip itself rather
    # than by a subsequent Python statement. This is the closest counterpart to
    # the Go binary's PlaySyncedWithFlip: it removes interpreter scheduling
    # jitter between the flip returning and play() being called.
    sync_method = "callOnFlip" if args.soa_ms == 0 else "sequential"
    print(f"av: soa={args.soa_ms:.1f} ms  freq={args.freq_hz:.0f} Hz"
          f"  tone={tone_dur_ms} ms (frames-on={frames_on} × {frame_ms:.2f} ms)"
          f"  frames-off={frames_off}  cycles={args.cycles}  sync={sync_method}")

    tone = make_tone(args, args.freq_hz, tone_dur_ms / 1000.0)
    # Warm up: the first play carries driver start-up cost; discard it.
    tone.play(); core.wait(0.02); tone.stop(); core.wait(0.05)

    soa_s = abs(args.soa_ms) / 1000.0
    audio_first = args.soa_ms < 0
    col_a = level_to_psychopy(args.level_a)
    col_b = level_to_psychopy(args.level_b)
    is_null = isinstance(trig, NullTrigger)

    print(f"{'trial':>6}  {'t_vis_before_ms':>16}  {'t_vis_after_ms':>15}"
          f"  {'t_audio_queued_ms':>18}  {'soa_actual_ms':>14}")

    def hold_bright(remaining: int) -> None:
        """Redraw bright for the remaining frames so the panel never flips dark."""
        for _ in range(remaining):
            win.color = [col_b, col_b, col_b]
            _flip(win)

    with suspend_gc(args):
        for trial in range(args.cycles):
            if audio_first:
                t_audio_queued = core.getTime()
                tone.play()
                core.wait(soa_s)
                win.color = [col_b, col_b, col_b]
                t_vis_before = core.getTime()
                t_vis_after = _flip(win)
                if not is_null:
                    trigger_pulse_async(trig, args.trigger_pin, args.trigger_ms / 1000)
                hold_bright(frames_on - 1)
            elif soa_s == 0:
                win.color = [col_b, col_b, col_b]
                t_vis_before = core.getTime()
                win.callOnFlip(tone.play)
                t_vis_after = _flip(win)
                # Audio was launched by the flip itself; onset lags by at most
                # one callback period.
                t_audio_queued = t_vis_after
                if not is_null:
                    trigger_pulse_async(trig, args.trigger_pin, args.trigger_ms / 1000)
                hold_bright(frames_on - 1)
            else:
                win.color = [col_b, col_b, col_b]
                t_vis_before = core.getTime()
                t_vis_after = _flip(win)
                if not is_null:
                    trigger_pulse_async(trig, args.trigger_pin, args.trigger_ms / 1000)
                core.wait(soa_s)
                t_audio_queued = core.getTime()
                tone.play()
                hold_bright(frames_on - 1)

            soa_actual_ms = (t_audio_queued - t_vis_after) * 1000
            print(f"{trial:>6}  {t_vis_before * 1000:>16.3f}  {t_vis_after * 1000:>15.3f}"
                  f"  {t_audio_queued * 1000:>18.3f}  {soa_actual_ms:>14.1f}")

            # Dark phase: frames-off frames as ITI between stimuli.
            for _ in range(frames_off):
                win.color = [col_a, col_a, col_a]
                _flip(win)

            if event.getKeys(keyList=["escape"]):
                print("  (stopped early by ESC)")
                break

    tone.stop()
    print(f"\nav: {args.cycles} trials complete.  Check the BBTK/oscilloscope "
          f"for acoustic onset.")


# ── Test: jitter ──────────────────────────────────────────────────────────────

def run_jitter(win, args) -> None:
    """
    Flip a mid-gray screen continuously for args.duration_s seconds and record
    the wall-clock interval between consecutive flip returns.

    interval = t_after_flip[i] − t_after_flip[i−1], mirroring Go's tAfter.

    Matches: go run main.go -test jitter -duration-s N -warmup N [-d]
    """
    n_approx = int(args.duration_s * args.hz)
    print(f"jitter: ~{n_approx} frames over {args.duration_s:.1f} s"
          f"  warmup={args.warmup}  (ESC to stop early)")

    intervals: list[float] = []
    prev_t: float | None = None
    frame = 0

    with suspend_gc(args):
        t_start = core.getTime()
        deadline = t_start + args.duration_s

        while core.getTime() < deadline:
            t_flip = _flip(win)

            if prev_t is not None:
                interval_ms = (t_flip - prev_t) * 1000.0
                if frame >= args.warmup:
                    intervals.append(interval_ms)
            prev_t = t_flip
            frame += 1

            if event.getKeys(keyList=["escape"]):
                print("  (stopped early by ESC)")
                break

    if not intervals:
        print("No intervals recorded.")
        return

    s = compute_stats(intervals, 16.67)
    estimated_hz = 1000.0 / s["mean"] if s["mean"] > 0 else 0.0
    s = compute_stats(intervals, s["mean"])
    print(f"\nEstimated refresh rate: {estimated_hz:.3f} Hz"
          f"  (use --hz {estimated_hz:.2f} for frame targets)")
    print_stats("Frame intervals", s, s["mean"])


# ── Test: square ──────────────────────────────────────────────────────────────

def run_square(win, trig, args) -> None:
    """
    Drive a square wave on DLP-IO8-G pin for args.duration_s seconds.
    Requires a DLP-IO8-G; exits immediately if none is found.

    Uses a busy-spin approach (sleep until 500 µs before target, then spin)
    to minimise overshoot, matching Go's sleepUntil().

    Matches: go run main.go -test square -period-ms N -duty N
             -duration-s N -trigger-pin N
    """
    if isinstance(trig, NullTrigger):
        print("square test requires a DLP-IO8-G (no device found)")
        sys.exit(1)

    period_s = args.period_ms / 1000.0
    high_dur_s = period_s * args.duty / 100.0
    expected_cycles = int(args.duration_s / period_s)
    print(f"square: period={args.period_ms:.1f} ms  duty={args.duty:.0f} %%"
          f"  pin={args.trigger_pin}  duration={args.duration_s:.0f} s"
          f"  (~{expected_cycles} cycles)")

    status = visual.TextStim(
        win,
        text=(f"Square wave: {args.period_ms:.1f} ms period, {args.duty:.0f}% duty,"
              f" pin {args.trigger_pin} — press ESC to stop"),
        height=24, color=[1, 1, 1])
    status.draw()
    win.flip()

    rise_jitter: list[float] = []
    fall_jitter: list[float] = []
    t_start = core.getTime()
    deadline = t_start + args.duration_s
    cycle = 0

    try:
        while core.getTime() < deadline:
            # ── Rising edge ──────────────────────────────────────────────────
            target_rise = t_start + cycle * period_s
            sleep_until(target_rise)
            t_rise = core.getTime()
            trig.set_high(args.trigger_pin)
            rise_jitter.append((t_rise - target_rise) * 1000)

            # ── Falling edge ─────────────────────────────────────────────────
            target_fall = target_rise + high_dur_s
            sleep_until(target_fall)
            t_fall = core.getTime()
            trig.set_low(args.trigger_pin)
            fall_jitter.append((t_fall - target_fall) * 1000)

            cycle += 1

            if event.getKeys(keyList=["escape"]):
                print("  (stopped early by ESC)")
                break

            # Idle until 2 ms before the next rising edge
            next_rise = t_start + cycle * period_s
            slack = next_rise - core.getTime() - 0.002
            if slack > 0:
                core.wait(slack)
    finally:
        trig.set_low(args.trigger_pin)

    print_stats("Rising-edge jitter (ms from target)",
                compute_stats(rise_jitter, 0), 0)
    print_stats("Falling-edge jitter (ms from target)",
                compute_stats(fall_jitter, 0), 0)


# ── Test: sound ───────────────────────────────────────────────────────────────

def run_sound(win, trig, args) -> None:
    """
    Play a long regular tone stream and report onset-jitter statistics.

    actual_onset_ms is when tone.play() was called (PCM queued to driver),
    not the acoustic onset.  Acoustic onset = actual_onset + pipeline_latency.
    Use the drain test to measure pipeline latency on your system.

    If a DLP-IO8-G is connected, the trigger line is held high for the whole
    stream (high before the first tone, low after the last), matching the Go
    version. It is deliberately *not* pulsed per tone: the previous version
    called core.wait(trigger_ms) between set_high and set_low inside the timing
    loop, injecting a 5 ms block immediately after every tone onset.

    Matches: Timing-Tests -test tones -cycles N -freq-hz N -tone-ms N -iti-ms N
    """
    tone_dur_s = args.tone_ms / 1000.0
    isi_dur_s = args.iti_ms / 1000.0
    soa_s = tone_dur_s + isi_dur_s
    soa_ms = soa_s * 1000.0
    is_null = isinstance(trig, NullTrigger)
    trig_dur_s = args.trigger_ms / 1000.0

    print(f"sound: {args.cycles} tones  {args.freq_hz:.0f} Hz"
          f"  {args.tone_ms} ms on  {args.iti_ms:.0f} ms ISI"
          f"  SOA {soa_ms:.0f} ms  total ~{args.cycles * soa_s:.1f} s"
          + (f"  trigger pin {args.trigger_pin}" if not is_null else ""))

    tone = make_tone(args, args.freq_hz, tone_dur_s)

    # Warm up: first play has driver startup overhead; discard it.
    tone.play(); core.wait(0.02); tone.stop(); core.wait(0.05)

    status = visual.TextStim(
        win,
        text=(f"Audio timing: {args.cycles} × {args.freq_hz:.0f} Hz tones,"
              f" {args.tone_ms} ms on / {args.iti_ms:.0f} ms ISI — ESC to stop"),
        height=24, color=[1, 1, 1])
    status.draw()
    win.flip()

    onset_errors: list[float] = []
    ioi_vals: list[float] = []
    prev_actual_ms: float | None = None
    aborted = False

    with suspend_gc(args):
        # Trigger brackets the whole stream, as in the Go version.
        if not is_null:
            trig.set_high(args.trigger_pin)
        stream_start = core.getTime()
        for i in range(args.cycles):
            target_onset_s = i * soa_s

            # Wait until target onset time
            while core.getTime() - stream_start < target_onset_s:
                core.wait(0.0005)
                if event.getKeys(keyList=["escape"]):
                    aborted = True
                    break
            if aborted:
                print("  (stopped early by ESC)")
                break

            actual_onset_s = core.getTime() - stream_start
            tone.play()

            onset_error_ms = (actual_onset_s - target_onset_s) * 1000
            actual_ms = actual_onset_s * 1000
            onset_errors.append(onset_error_ms)
            if prev_actual_ms is not None:
                ioi_vals.append(actual_ms - prev_actual_ms)
            prev_actual_ms = actual_ms

            # Wait remainder of on-phase + ISI
            deadline_s = stream_start + target_onset_s + soa_s
            while core.getTime() < deadline_s:
                core.wait(0.001)
                if event.getKeys(keyList=["escape"]):
                    aborted = True
                    break
            if aborted:
                print("  (stopped early by ESC)")
                break
        if not is_null:
            trig.set_low(args.trigger_pin)

    tone.stop()
    print_stats("Onset error vs target (ms)", compute_stats(onset_errors, 0), 0)
    print_stats("Inter-onset interval (ms)", compute_stats(ioi_vals, soa_ms), soa_ms)


# ── Test: rt ──────────────────────────────────────────────────────────────────

def run_rt(win, trig, args) -> None:
    """
    Measure keyboard reaction time for args.cycles trials.

    Each trial: jittered blank ITI → single-frame white flash → wait for key.

    RT = key event timestamp − flip timestamp, both on the same clock.
    With psychtoolbox, key.tDown reflects the hardware interrupt time;
    without it, timestamps have Python poll-loop jitter (~1–5 ms).

    In goxpyriment, both timestamps use SDL3's nanosecond clock (hardware
    interrupt precision on both sides).  This test lets you compare the
    two approaches on the same hardware/display setup.

    Matches: go run main.go -test rt -cycles N -iti-ms N [-d]
    """
    n_trials = args.cycles
    mean_iti_s = args.iti_ms / 1000.0
    print(f"rt: {n_trials} trials  mean ITI {args.iti_ms:.0f} ms  press any key each flash")

    # Use hardware keyboard for best timestamp precision
    use_hw_kb = False
    kb = None
    try:
        from psychopy.hardware.keyboard import Keyboard as HwKeyboard
        try:
            kb = HwKeyboard(backend="ptb") if _ptb_available() else HwKeyboard()
            use_hw_kb = True
            print(f"  keyboard backend: {'ptb (hardware timestamps)' if _ptb_available() else 'default'}")
        except Exception as exc:
            print(f"  note: HwKeyboard failed ({exc}), using event module (lower precision)")
    except ImportError:
        print("  note: psychopy.hardware.keyboard unavailable, using event module")

    instr = visual.TextStim(
        win, text="Press any key as fast as possible when the screen flashes white.",
        pos=(0, 50), height=24, color=[1, 1, 1])
    hint = visual.TextStim(
        win, text="(press SPACE to start)", pos=(0, -50), height=24, color=[0.5, 0.5, 0.5])
    instr.draw(); hint.draw(); win.flip()
    event.waitKeys(keyList=["space"])

    col_dark = level_to_psychopy(0)
    col_bright = level_to_psychopy(255)
    is_null = isinstance(trig, NullTrigger)
    rt_values: list[float] = []

    with suspend_gc(args):
        for i in range(n_trials):
            # Jittered ITI ± 50 %
            iti_s = mean_iti_s * (1.0 + (random.random() - 0.5))
            win.color = [col_dark, col_dark, col_dark]
            _flip(win)
            core.wait(iti_s)

            if not is_null:
                trig.set_high(args.trigger_pin)

            # Prepare RT measurement clock
            if use_hw_kb:
                kb.clearEvents()
                t_reset = core.getTime()
                kb.clock.reset()  # zero kb.clock at approximately t_reset

            # White flash
            win.color = [col_bright, col_bright, col_bright]
            t_flip = _flip(win)

            if not is_null:
                trigger_pulse_async(trig, args.trigger_pin, args.trigger_ms / 1000)

            # Compute how far after the clock reset the flip landed
            flip_delay = (t_flip - t_reset) if use_hw_kb else 0.0

            # Wait for keypress
            if use_hw_kb:
                # keys[0].rt = time from kb.clock.reset() to key event
                # rt_from_flip = keys[0].rt - flip_delay
                keys = kb.waitKeys(maxWait=5.0, waitRelease=False, clear=False)
                if not keys:
                    print(f"  trial {i:3d}: timeout")
                    continue
                rt_ms = (keys[0].rt - flip_delay) * 1000
            else:
                # Fallback: record poll time (lower precision)
                raw = event.waitKeys(maxWait=5.0)
                if not raw:
                    print(f"  trial {i:3d}: timeout")
                    continue
                rt_ms = (core.getTime() - t_flip) * 1000

            rt_values.append(rt_ms)
            print(f"  trial {i:3d}  RT = {rt_ms:.1f} ms")

    if not rt_values:
        print("No RT data collected.")
        return
    print_stats("RT (ms, event-timestamp method)", compute_stats(rt_values, 0), 0)


# ── Test: drain ───────────────────────────────────────────────────────────────

def run_drain(win, args) -> None:
    """
    Measure audio pipeline latency by timing how long the OS driver takes to
    consume pre-generated PCM data after sounddevice.play() is called.

    drain_ms = time from sd.play() to sd.wait() returning.
    pipeline_latency ≈ mean(drain_ms) − nominal_ms.

    sd.wait() returns when the software buffer is empty (last sample sent to
    DAC), mirroring Go's spin-poll on stream.Queued()==0.  DAC/amplifier
    latency (~0–2 ms) is not captured here.

    Matches: go run main.go -test drain -drain-reps N [-freq-hz N]
    """
    try:
        import sounddevice as sd
    except ImportError:
        print("drain test requires sounddevice: pip install sounddevice")
        sys.exit(1)

    durations_ms = [25, 50, 100, 200, 500]
    reps = args.drain_reps
    freq = args.freq_hz
    sample_rate = 44100

    print(f"drain: freq={freq:.0f} Hz  reps={reps}  durations={durations_ms} ms")

    status = visual.TextStim(
        win,
        text=f"Audio drain test: {freq:.0f} Hz tone, {reps} reps — please wait…",
        height=24, color=[1, 1, 1])
    status.draw()
    win.flip()

    for dur_ms in durations_ms:
        dur_s = dur_ms / 1000.0
        t = np.linspace(0, dur_s, int(sample_rate * dur_s), endpoint=False)
        mono = (0.8 * np.sin(2 * math.pi * freq * t)).astype(np.float32)
        stereo = np.column_stack([mono, mono])

        drain_vals: list[float] = []
        aborted = False
        for rep in range(reps):
            core.wait(0.05)  # 50 ms silence between reps

            t_play = core.getTime()
            sd.play(stereo, sample_rate, blocking=False)
            sd.wait()  # blocks until all queued bytes are consumed by the DAC
            drain_ms_val = (core.getTime() - t_play) * 1000
            overhead_ms = drain_ms_val - dur_ms
            drain_vals.append(drain_ms_val)

            print(f"  {dur_ms:3d} ms  rep {rep:2d}:"
                  f"  drain={drain_ms_val:.1f} ms  overhead={overhead_ms:+.1f} ms")

            if event.getKeys(keyList=["escape"]):
                aborted = True
                print("  (stopped early by ESC)")
                break

        s = compute_stats(drain_vals, float(dur_ms))
        print()
        print_stats(
            f"Drain time for {dur_ms} ms tone (latency = mean − target)",
            s, float(dur_ms))
        print(f"  pipeline latency ≈ {s['mean'] - dur_ms:.1f} ms")

        if aborted:
            break


# ── Test: check ───────────────────────────────────────────────────────────────

def run_check(win, args) -> None:
    """
    Combined display and audio sanity check — no measurement, no equipment needed.

    Shows a bright white screen for 1 second (watch for the flash on the monitor),
    then plays a buzzer followed by a ping (listen through speakers/headphones).

    Matches: go run main.go -test check [-d]
    """
    print("check: verifying display and audio output"
          " — watch for a bright flash, then listen for two sounds")

    # ── Step 1: bright flash ──────────────────────────────────────────────────
    label = visual.TextStim(
        win,
        text="DISPLAY CHECK — you should see this bright screen for ~1 second.",
        height=24, color=[-1, -1, -1])  # black text on white background
    win.color = [1, 1, 1]
    label.draw()
    win.flip()
    core.wait(1.0)

    # Brief dark transition so the boundary is clearly visible.
    win.color = [-1, -1, -1]
    win.flip()
    core.wait(0.3)

    # ── Step 2: buzzer ────────────────────────────────────────────────────────
    msg1 = visual.TextStim(win, text="AUDIO CHECK — listen for a buzzer…",
                           height=24, color=[1, 1, 1])
    win.color = [-1, -1, -1]
    msg1.draw()
    win.flip()
    print(f"check: playing buzzer… (backend={_AUDIO_LIB}"
          + (f", {args.audio_frames} sample frames)" if args.audio_frames > 0 else ")"))
    buzzer = make_tone(args, 200, 0.5)
    buzzer.play()
    core.wait(1.0)

    # ── Step 3: ping ──────────────────────────────────────────────────────────
    msg2 = visual.TextStim(win, text="AUDIO CHECK — …then a ping.",
                           height=24, color=[1, 1, 1])
    win.color = [-1, -1, -1]
    msg2.draw()
    win.flip()
    print("check: playing ping…")
    ping = make_tone(args, 880, 0.1)
    ping.play()
    core.wait(1.0)

    print("check: done. Did you see the bright flash and hear both sounds?"
          " If yes, proceed to the measurement tests.")


# ── Test: stream ───────────────────────────────────────────────────────────────

def run_stream(win, trig, args) -> None:
    """
    Measure timing accuracy of sequential (RSVP-style) stimulus presentations.

    Each element: args.frames_on bright frames then args.frames_off dark
    frames.  Two statistics are reported:
      - Duration error  : actual on-duration − target on-duration (ms)
      - SOA error       : actual onset-to-onset interval − target SOA (ms)

    First args.warmup elements are excluded from statistics (GPU pipeline warm-up).

    If a DLP-IO8-G is connected, a trigger pulse is sent at the onset of every
    bright phase so software timestamps can be validated against a photodiode.

    Matches: Timing-Tests -test stream -cycles N -frames-on N
             -frames-off N -hz N -warmup N [-w] [-d N]
    """
    on_frames = args.frames_on
    off_frames = args.frames_off
    target_frame_ms = 1000.0 / args.hz
    target_on_ms = on_frames * target_frame_ms
    target_off_ms = off_frames * target_frame_ms
    target_soa_ms = target_on_ms + target_off_ms
    n = args.cycles
    is_null = isinstance(trig, NullTrigger)

    print(f"stream: {n} elements  on={on_frames} frames ({target_on_ms:.2f} ms)"
          f"  off={off_frames} frames ({target_off_ms:.2f} ms)"
          f"  SOA={target_soa_ms:.2f} ms  hz={args.hz:.2f}  warmup={args.warmup}"
          + (f"  trigger pin {args.trigger_pin} ({args.trigger_ms} ms pulse)"
             if not is_null else ""))

    col_a = level_to_psychopy(args.level_a)
    col_b = level_to_psychopy(args.level_b)

    status = visual.TextStim(
        win,
        text=(f"Stream timing: {n} elements, {on_frames} on / {off_frames} off frames"
              " — press ESC to stop"),
        height=24, color=[1, 1, 1])
    status.draw()
    win.flip()
    core.wait(0.5)

    duration_errors: list[float] = []
    interval_errors: list[float] = []
    prev_onset_t: float | None = None

    with suspend_gc(args):
        for elem in range(n):
            # ── ON phase ──────────────────────────────────────────────────────
            if not is_null:
                trig.set_high(args.trigger_pin)

            t_onset: float | None = None
            for f in range(on_frames):
                win.color = [col_b, col_b, col_b]
                t_flip = _flip(win)
                if f == 0:
                    t_onset = t_flip
                    if not is_null:
                        trigger_pulse_async(trig, args.trigger_pin,
                                            args.trigger_ms / 1000)

            # ── OFF phase (ISI) ───────────────────────────────────────────────
            t_offset: float | None = None
            for f in range(off_frames):
                win.color = [col_a, col_a, col_a]
                t_flip = _flip(win)
                if f == 0:
                    t_offset = t_flip

            # ── Statistics ────────────────────────────────────────────────────
            duration_ms = (t_offset - t_onset) * 1000
            duration_error = duration_ms - target_on_ms

            interval_ms = 0.0
            interval_error = 0.0
            if prev_onset_t is not None:
                interval_ms = (t_onset - prev_onset_t) * 1000
                interval_error = interval_ms - target_soa_ms
                if elem >= args.warmup:
                    interval_errors.append(interval_error)

            if elem >= args.warmup:
                duration_errors.append(duration_error)

            prev_onset_t = t_onset

            if event.getKeys(keyList=["escape"]):
                print("  (stopped early by ESC)")
                break

    print_stats(f"Duration error (target {target_on_ms:.2f} ms)",
                compute_stats(duration_errors, 0), 0)
    if interval_errors:
        print_stats(f"SOA error (target {target_soa_ms:.2f} ms)",
                    compute_stats(interval_errors, 0), 0)


# ── Test: vrr ─────────────────────────────────────────────────────────────────

def run_vrr(win, trig, args) -> None:
    """
    VRR (Variable Refresh Rate) sweep: present stimuli for 1 ms to vrr_max_ms in
    1 ms steps, args.cycles reps per step, with VSync disabled.

    On a VRR-capable monitor, duration errors should be small (<0.5 ms) across
    the entire sweep, confirming arbitrary-duration stimulus presentation works.
    On a non-VRR display, errors cluster at multiples of the frame period (±half
    a frame).  VRR panels have a supported refresh range (e.g. 48–144 Hz =
    6.9–20.8 ms); outside this range the panel reverts to fixed-rate behaviour,
    revealing the VRR window directly from the data.

    VSync is disabled via win.waitBlanking = False for the duration of the test
    (restored on exit), matching SDL's SDL_RENDERER_VSYNC_DISABLED.

    Matches: go run main.go -test vrr -vrr-max-ms N -cycles N [-d]
    """
    max_ms = args.vrr_max_ms
    reps = args.cycles
    is_null = isinstance(trig, NullTrigger)
    trig_dur_s = args.trigger_ms / 1000.0

    print(f"vrr: sweep 1–{max_ms} ms in 1 ms steps  reps={reps}"
          f"  level-a={args.level_a}  level-b={args.level_b}"
          + (f"  trigger pin {args.trigger_pin} ({args.trigger_ms} ms pulse)"
             if not is_null else ""))
    print("vrr: disabling VSync — use a VRR-capable monitor for meaningful sub-frame durations")

    col_a = level_to_psychopy(args.level_a)
    col_b = level_to_psychopy(args.level_b)

    win.waitBlanking = False  # disable VSync — flip() returns immediately
    # Let the driver settle after the change.
    core.wait(0.1)

    status = visual.TextStim(
        win,
        text=f"VRR sweep: 1–{max_ms} ms, {reps} reps — press ESC to stop",
        height=24, color=[1, 1, 1])
    status.draw()
    win.flip()
    core.wait(0.5)

    aborted = False
    with suspend_gc(args):
        for target_ms in range(1, max_ms + 1):
            target_s = target_ms / 1000.0
            duration_errors: list[float] = []

            for rep in range(reps):
                # ── ISI: blank screen ────────────────────────────────────────
                win.color = [col_a, col_a, col_a]
                win.flip()
                core.wait(0.2)

                # ── Onset: bright screen ─────────────────────────────────────
                if not is_null:
                    trig.set_high(args.trigger_pin)
                win.color = [col_b, col_b, col_b]
                t_onset = _flip(win)

                # ── Hold for exactly target_s using busy-wait ─────────────────
                sleep_until(t_onset + target_s)

                # ── Offset: blank screen ─────────────────────────────────────
                win.color = [col_a, col_a, col_a]
                t_offset = _flip(win)

                if not is_null:
                    trigger_pulse_async(trig, args.trigger_pin, trig_dur_s)

                # ── Log ───────────────────────────────────────────────────────
                actual_ms = (t_offset - t_onset) * 1000
                duration_error = actual_ms - target_ms
                duration_errors.append(duration_error)

                print(f"  {target_ms:3d} ms  rep {rep:2d}:"
                      f"  actual={actual_ms:6.3f} ms  error={duration_error:+6.3f} ms")

                if event.getKeys(keyList=["escape"]):
                    print("  (stopped early by ESC)")
                    aborted = True
                    break

            s = compute_stats(duration_errors, 0)
            if s:
                print(f"── {target_ms:3d} ms: mean={s['mean']:+.3f} ms  SD={s['sd']:.3f} ms")

            if aborted:
                break
        win.waitBlanking = True
        print("vrr: VSync re-enabled")


# ── Window factory ─────────────────────────────────────────────────────────────

def make_window(args) -> visual.Window:
    """
    Create a PsychoPy window matching goxpyriment's display setup.

    color=(0,0,0) in [-1,1] space = RGB(128,128,128) = the mid-gray used by
    fillGray(exp, 128) in the Go binary.
    waitBlanking=True (default) makes win.flip() VSYNC-locked, matching SDL VSync.
    """
    kwargs = dict(
        color=[0, 0, 0],
        colorSpace="rgb",
        units="pix",
        allowGUI=False,
        waitBlanking=True,
        useFBO=True,
    )
    if args.windowed:
        return visual.Window(size=[1024, 768], fullscr=False, **kwargs)
    # -d -1 means "primary", which is screen 0 for PsychoPy.
    screen = 0 if args.display < 0 else args.display
    return visual.Window(fullscr=True, screen=screen, **kwargs)


def print_sysinfo(args) -> None:
    """
    Print machine and configuration information, the counterpart to the Go
    binary's -sysinfo. Capture this on every machine so a report can be
    attributed to a configuration after the fact.
    """
    import psychopy

    uname = platform.uname()
    print("── System ─────────────────────────────────────────────────────────")
    print(f"Host:       {uname.node}")
    print(f"System:     {uname.system} {uname.release} ({uname.machine})")
    print(f"Version:    {uname.version}")
    print(f"Processor:  {uname.processor or 'unknown'}")
    print()
    print("── Software ───────────────────────────────────────────────────────")
    print(f"Python:     {platform.python_version()} ({platform.python_implementation()})")
    print(f"PsychoPy:   {psychopy.__version__}")
    print(f"numpy:      {np.__version__}")
    print(f"audioLib:   {_AUDIO_LIB}")
    print(f"audioFrames: {args.audio_frames if args.audio_frames > 0 else 'backend default'}")
    try:
        import psychtoolbox  # noqa: F401
        print("psychtoolbox: available (hardware timestamps)")
    except ImportError:
        print("psychtoolbox: NOT installed (key/flip timestamps are poll-based)")
    try:
        import sounddevice  # noqa: F401
        print("sounddevice: available")
    except ImportError:
        print("sounddevice: NOT installed (the latency/drain test needs it)")
    print()
    print("── Speaker devices (index → --audio-device N) ─────────────────────")
    try:
        from psychopy.hardware.speaker import SpeakerDevice
        devs = SpeakerDevice.getAvailableDevices()
        if not devs:
            print("  (none found)")
        for d in devs:
            print(f"  [{int(d.get('index', -1)):>3}] {d.get('deviceName', d.get('name', '?'))}")
    except Exception as exc:
        print(f"  could not enumerate: {type(exc).__name__}: {exc}")
    print()
    print("── Measurement settings ───────────────────────────────────────────")
    print(f"gc:         {gc_label(args)} during measurement loops")
    print(f"display:    {'windowed 1024x768' if args.windowed else f'fullscreen, screen {0 if args.display < 0 else args.display}'}")


# ── Main ──────────────────────────────────────────────────────────────────────

def main() -> None:
    args = build_parser().parse_args()

    if args.sysinfo:
        print_sysinfo(args)
        return

    if args.test is None:
        build_parser().error("one of --test or --sysinfo is required")

    if args.audio_frames > 0 and _AUDIO_LIB != "ptb":
        print(f"warning: --audio-frames is only honoured by the ptb backend; "
              f"ignored for {_AUDIO_LIB!r}", file=sys.stderr)

    # Resolve legacy aliases to their canonical names. `flash` is now an alias
    # for `frames` (the single-frame case is just --frames-on 1), matching the
    # Go binary, rather than a separate implementation.
    _aliases = {
        "audio": "check",
        "jitter": "display",
        "drain": "latency",
        "square": "trigger",
        "sound": "tones",
        "flash": "frames",
    }
    test = _aliases.get(args.test, args.test)

    # Record the collector state alongside the results so GC-on and GC-off runs
    # cannot be confused during analysis.
    print(f"gc: {gc_label(args)} during measurement loops")
    print(f"audio: {_AUDIO_LIB}"
          + (f", {args.audio_frames} sample frames" if args.audio_frames > 0 else ""))

    status = 0
    win = make_window(args)

    needs_trigger = test in ("check", "frames", "flash", "av", "trigger",
                             "tones", "rt", "stream", "vrr")
    trig: DLPIO8 | NullTrigger = NullTrigger()
    if needs_trigger:
        trig, _ = setup_trigger(args.port, args.trigger_pin)

    try:
        match test:
            # ── Tier 0: sanity check ─────────────────────────────────────────
            case "check":
                run_check(win, args)
            # ── Tier 1: self-contained measurements ──────────────────────────
            case "display":
                run_jitter(win, args)
            case "latency":
                run_drain(win, args)
            case "stream":
                run_stream(win, trig, args)
            case "vrr":
                run_vrr(win, trig, args)
            # ── Tier 2: trigger device characterisation ───────────────────────
            case "trigger":
                run_square(win, trig, args)
            # ── Tier 3: stimulus timing validation ────────────────────────────
            case "frames":
                run_frames(win, trig, args)
            case "tones":
                run_sound(win, trig, args)
            case "av":
                run_av(win, trig, args)
            # ── Tier 4: response timing ───────────────────────────────────────
            case "rt":
                run_rt(win, trig, args)
    except SystemExit as exc:
        # make_tone() and friends raise SystemExit carrying an actionable
        # message; surface it rather than letting cleanup swallow it.
        msg = str(exc)
        if msg and msg not in ("0", "None"):
            print(msg, file=sys.stderr)
        status = exc.code if isinstance(exc.code, int) and exc.code else 1
    except Exception:
        traceback.print_exc()
        status = 1
    finally:
        # Cleanup must not mask the failure. core.quit() raises SystemExit(0),
        # so calling it here would discard the in-flight exception and make a
        # crashed run exit 0 — indistinguishable from a good one in a report.
        for close in (trig.close, win.close):
            try:
                close()
            except Exception:
                pass

    if status:
        sys.exit(status)
    core.quit()


if __name__ == "__main__":
    main()
