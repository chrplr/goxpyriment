#!/usr/bin/env python3
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Distributed under the GNU General Public License v3.
"""
PsychoPy timing tests — counterpart to the goxpyriment Timing-Tests binary.

Implements the same sub-tests and prints statistics in the same format so
results can be compared directly between the two frameworks.

Sub-tests
---------
  jitter   Pure frame-interval statistics       (no hardware needed)
  frames   Alternating luminance cycles          (photodiode recommended)
  flash    Single-frame bright flashes           (photodiode recommended)
  av       Audio-visual synchrony                (oscilloscope recommended)
  square   DLP-IO8-G square wave                 (DLP-IO8-G + oscilloscope required)
  sound    Tone stream onset-jitter              (no hardware needed)
  rt       Keyboard reaction time                (keyboard required)
  drain    Audio pipeline latency                (no hardware needed)

Usage
-----
  python timing_tests.py --test jitter [options]
  python timing_tests.py --test rt --cycles 60 -d
  python timing_tests.py --test frames --level-a 0 --level-b 255 --frames-per-phase 2 --cycles 120

Timing notes
------------
PsychoPy's win.flip() blocks until the next VSYNC boundary (waitBlanking=True),
mirroring SDL's VSync in goxpyriment.  The return value is the flip timestamp
on defaultClock (core.getTime()), captured right after SwapBuffers returns —
the same instant that fillGray() captures as tAfter in the Go binary.

The Python GC is disabled during measurement loops via gc.disable(), mirroring
Go's debug.SetGCPercent(-1).

For the rt test, key timestamps come from psychopy.hardware.keyboard.Keyboard.
With psychtoolbox installed (pip install psychtoolbox), events are timestamped
at hardware-interrupt time (matching goxpyriment's SDL3 nanosecond clock).
Without psychtoolbox, timestamps reflect Python poll-loop time (~1–5 ms jitter).

For the drain test, sounddevice.wait() is used to detect when the audio driver
has consumed all queued PCM data — the Python equivalent of SDL's stream.Queued()==0.

DLP-IO8-G trigger device
------------------------
Trigger output is optional for frames/flash/av/sound/rt.  It is required for
square.  The same ASCII command protocol as goxpyriment's triggers/dlpio8.go
is used:
  Set HIGH pin N : '1'–'8'
  Set LOW  pin N : 'Q','W','E','R','T','Y','U','I'
  Ping           : "'" → device responds 'Q'
Install: pip install pyserial
"""

import argparse
import gc
import math
import random
import sys
import threading
import time

import numpy as np
from psychopy import core, event, logging, sound, visual

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

def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    p.add_argument("--test", required=True,
                   choices=["frames", "flash", "av", "jitter",
                            "square", "sound", "rt", "drain"],
                   metavar="TEST",
                   help="Sub-test: frames|flash|av|jitter|square|sound|rt|drain")
    p.add_argument("-d", action="store_true",
                   help="Windowed 1024×768 developer mode (default: fullscreen)")
    p.add_argument("--screen", type=int, default=0,
                   help="Screen index for fullscreen (default: 0)")
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
    p.add_argument("--frames-per-phase", type=int, default=2, dest="frames_per_phase",
                   help="Frames per dark/bright phase (default: 2)")
    p.add_argument("--isi-frames", type=int, default=60, dest="isi_frames",
                   help="Dark frames between flashes (default: 60)")
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
    # drain
    p.add_argument("--drain-reps", type=int, default=10, dest="drain_reps",
                   help="Repetitions per tone duration (default: 10)")
    return p


# ── Helpers ───────────────────────────────────────────────────────────────────

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
    Alternate between two luminance levels for args.cycles complete dark/bright
    cycles.  A trigger pulse is sent at the first frame of each bright phase.

    Matches: go run main.go -test frames -level-a N -level-b N
             -frames-per-phase N -cycles N [-d]
    """
    target_ms = args.frames_per_phase * 1000.0 / args.hz
    print(f"frames: level-a={args.level_a} level-b={args.level_b}"
          f" frames-per-phase={args.frames_per_phase}"
          f" cycles={args.cycles} hz={args.hz:.2f} warmup={args.warmup}")

    col_a = level_to_psychopy(args.level_a)
    col_b = level_to_psychopy(args.level_b)
    intervals: list[float] = []
    prev_t: float | None = None
    frame = 0
    warmup_ticks = args.warmup * 2 * args.frames_per_phase
    is_null = isinstance(trig, NullTrigger)

    gc.disable()
    try:
        for cycle in range(args.cycles):
            for phase in range(2):
                is_bright = phase == 1
                col = col_b if is_bright else col_a
                for f in range(args.frames_per_phase):
                    triggered = is_bright and f == 0
                    if triggered and not is_null:
                        trig.set_high(args.trigger_pin)

                    win.color = [col, col, col]
                    t_flip = _flip(win)

                    if triggered and not is_null:
                        trigger_pulse_async(trig, args.trigger_pin,
                                            args.trigger_ms / 1000)

                    if prev_t is not None:
                        interval_ms = (t_flip - prev_t) * 1000
                        if frame >= warmup_ticks:
                            intervals.append(interval_ms)
                    prev_t = t_flip
                    frame += 1

                    if event.getKeys(keyList=["escape"]):
                        print("  (stopped early by ESC)")
                        print_stats("Frame intervals",
                                    compute_stats(intervals, target_ms), target_ms)
                        return
    finally:
        gc.enable()

    print_stats("Frame intervals", compute_stats(intervals, target_ms), target_ms)


# ── Test: flash ───────────────────────────────────────────────────────────────

def run_flash(win, trig, args) -> None:
    """
    Present a single bright frame every isi_frames dark frames for args.cycles
    flashes, recording flash-to-flash interval statistics.

    Matches: go run main.go -test flash -isi-frames N -cycles N [-d]
    """
    expected_ms = (args.isi_frames + 1) * 1000.0 / args.hz
    print(f"flash: level-a={args.level_a} level-b={args.level_b}"
          f" isi-frames={args.isi_frames} cycles={args.cycles}"
          f" hz={args.hz:.2f} warmup={args.warmup}")

    col_a = level_to_psychopy(args.level_a)
    col_b = level_to_psychopy(args.level_b)
    flash_intervals: list[float] = []
    prev_flash_t: float | None = None
    is_null = isinstance(trig, NullTrigger)

    gc.disable()
    try:
        for flash in range(args.cycles):
            for _ in range(args.isi_frames):
                win.color = [col_a, col_a, col_a]
                _flip(win)
                if event.getKeys(keyList=["escape"]):
                    print("  (stopped early by ESC)")
                    print_stats("Flash intervals",
                                compute_stats(flash_intervals, expected_ms), expected_ms)
                    return

            if not is_null:
                trig.set_high(args.trigger_pin)
            win.color = [col_b, col_b, col_b]
            t_flip = _flip(win)
            if not is_null:
                trigger_pulse_async(trig, args.trigger_pin, args.trigger_ms / 1000)

            if prev_flash_t is not None:
                interval_ms = (t_flip - prev_flash_t) * 1000
                if flash >= args.warmup:
                    flash_intervals.append(interval_ms)
            prev_flash_t = t_flip
    finally:
        gc.enable()

    print_stats("Flash intervals", compute_stats(flash_intervals, expected_ms), expected_ms)


# ── Test: av ──────────────────────────────────────────────────────────────────

def run_av(win, trig, args) -> None:
    """
    Present cycles of a white-screen flash paired with a pure sine tone at a
    configurable SOA.

    t_audio_queued_ms is when tone.play() was called (PCM data pushed to the
    driver buffer), not the acoustic onset.  The actual delay must be measured
    with an oscilloscope (audio line-out → channel 1, photodiode → channel 2).

    Matches: go run main.go -test av -soa-ms N -freq-hz N -tone-ms N
             -iti-ms N -cycles N [-d]
    """
    print(f"av: soa={args.soa_ms:.1f} ms  iti={args.iti_ms:.0f} ms"
          f"  tone={args.freq_hz:.0f} Hz / {args.tone_ms} ms  cycles={args.cycles}")

    tone = sound.Sound(value=args.freq_hz, secs=args.tone_ms / 1000,
                       volume=0.8, sampleRate=44100, stereo=True)
    soa_s = abs(args.soa_ms) / 1000.0
    audio_first = args.soa_ms < 0
    col_a = level_to_psychopy(args.level_a)
    col_b = level_to_psychopy(args.level_b)
    is_null = isinstance(trig, NullTrigger)
    # ITI: one dark flip then sleep for the remainder (matches Go's approach)
    iti_remainder_s = max(0.0, args.iti_ms / 1000 - 1.0 / args.hz)

    print(f"{'trial':>6}  {'t_vis_after_ms':>16}  {'t_audio_queued_ms':>18}  {'soa_actual_ms':>14}")

    for trial in range(args.cycles):
        if audio_first:
            t_audio_queued = core.getTime()
            tone.play()
            core.wait(soa_s)
            if not is_null:
                trig.set_high(args.trigger_pin)
            win.color = [col_b, col_b, col_b]
            t_vis_after = _flip(win)
            if not is_null:
                trigger_pulse_async(trig, args.trigger_pin, args.trigger_ms / 1000)
        else:
            if not is_null:
                trig.set_high(args.trigger_pin)
            win.color = [col_b, col_b, col_b]
            t_vis_after = _flip(win)
            if not is_null:
                trigger_pulse_async(trig, args.trigger_pin, args.trigger_ms / 1000)
            if soa_s > 0:
                core.wait(soa_s)
            t_audio_queued = core.getTime()
            tone.play()

        soa_actual_ms = (t_audio_queued - t_vis_after) * 1000
        print(f"{trial:>6}  {t_vis_after * 1000:>16.3f}  {t_audio_queued * 1000:>18.3f}"
              f"  {soa_actual_ms:>14.1f}")

        win.color = [col_a, col_a, col_a]
        _flip(win)
        if iti_remainder_s > 0:
            core.wait(iti_remainder_s)

        if event.getKeys(keyList=["escape"]):
            print("  (stopped early by ESC)")
            break

    tone.stop()
    print(f"\nav: {args.cycles} trials complete.  "
          f"Check oscilloscope for audio latency.")


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

    gc.disable()
    try:
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
    finally:
        gc.enable()

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

    If a DLP-IO8-G is connected, a trigger pulse is sent just before each
    tone's play() call — same as the Go version.

    Matches: go run main.go -test sound -cycles N -freq-hz N -tone-ms N -iti-ms N
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

    tone = sound.Sound(value=args.freq_hz, secs=tone_dur_s,
                       volume=0.8, sampleRate=44100, stereo=True)

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

    gc.disable()
    try:
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

            if not is_null:
                trig.set_high(args.trigger_pin)
            actual_onset_s = core.getTime() - stream_start
            tone.play()
            if not is_null:
                core.wait(trig_dur_s)
                trig.set_low(args.trigger_pin)

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
    finally:
        gc.enable()

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

    gc.disable()
    try:
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
    finally:
        gc.enable()

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
    if args.d:
        return visual.Window(size=[1024, 768], fullscr=False, **kwargs)
    return visual.Window(fullscr=True, screen=args.screen, **kwargs)


# ── Main ──────────────────────────────────────────────────────────────────────

def main() -> None:
    args = build_parser().parse_args()
    win = make_window(args)

    needs_trigger = args.test in ("frames", "flash", "av", "square", "sound", "rt")
    trig: DLPIO8 | NullTrigger = NullTrigger()
    if needs_trigger:
        trig, _ = setup_trigger(args.port, args.trigger_pin)

    try:
        match args.test:
            case "jitter":
                run_jitter(win, args)
            case "frames":
                run_frames(win, trig, args)
            case "flash":
                run_flash(win, trig, args)
            case "av":
                run_av(win, trig, args)
            case "square":
                run_square(win, trig, args)
            case "sound":
                run_sound(win, trig, args)
            case "rt":
                run_rt(win, trig, args)
            case "drain":
                run_drain(win, args)
    finally:
        trig.close()
        win.close()
        core.quit()


if __name__ == "__main__":
    main()
