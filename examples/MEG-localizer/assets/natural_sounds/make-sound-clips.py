#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Cut one short clip out of each natural sound and save it as sound_NN.wav.

    ./make-sound-clips.py

For every source file in this directory:

  1. it is decoded to a common rate and to mono (the sources are a mix of
     44.1 and 48 kHz, mono and stereo; a stimulus set should not be),
  2. the loudest CLIP_SECONDS window is located and taken -- picking the
     window by energy is what keeps the clip on the event rather than on the
     background hiss around it,
  3. a raised-cosine ramp is applied to each end,
  4. the result is written as sound_NN.wav.

Sources shorter than the clip length keep all of what they have and are
padded with silence to the same duration, so every stimulus lasts the same
time in the stream.

Requires numpy, soundfile and ffmpeg (used to decode mp3/flac uniformly).
"""

import subprocess
import sys
from pathlib import Path

import numpy as np
import soundfile as sf

HERE = Path(__file__).resolve().parent

CLIP_SECONDS = 1.6
RAMP_MS = 50
RATE = 44100
WINDOW_MS = 20          # hop for the energy envelope used to find the clip
LEAD_MS = 80            # keep this much before the loudest window's onset
NORMALIZE = None        # None, "peak" or "rms" -- see the note printed at the end
TARGET_RMS = 0.1        # used when NORMALIZE == "rms"
PEAK_CEILING = 0.99


def decode(path):
    """Decode any source to mono float32 at RATE, via ffmpeg."""
    out = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", str(path),
         "-ac", "1", "-ar", str(RATE), "-f", "f32le", "-"],
        capture_output=True, check=True).stdout
    return np.frombuffer(out, dtype=np.float32).astype(np.float64)


def loudest_window(x, n):
    """Start index of the n-sample window with the most energy.

    The window is then nudged earlier by LEAD_MS so that a transient sitting at
    the very start of the loudest window -- a bark, a bell strike -- is not
    clipped by the ramp that follows.
    """
    if len(x) <= n:
        return 0
    hop = int(RATE * WINDOW_MS / 1000)
    energy = np.cumsum(np.concatenate([[0.0], x * x]))
    starts = np.arange(0, len(x) - n + 1, hop)
    best = starts[int(np.argmax(energy[starts + n] - energy[starts]))]
    return int(max(0, best - RATE * LEAD_MS / 1000))


def ramp(x, ms):
    """Raised-cosine fade in and out, in place-safe fashion."""
    n = min(int(RATE * ms / 1000), len(x) // 2)
    if n < 1:
        return x
    w = 0.5 - 0.5 * np.cos(np.pi * np.arange(n) / n)
    y = x.copy()
    y[:n] *= w
    y[-n:] *= w[::-1]
    return y


def main():
    sources = sorted(p for p in HERE.iterdir()
                     if p.suffix.lower() in {".wav", ".mp3", ".flac", ".ogg", ".aiff"}
                     and not p.name.startswith("sound_"))
    if not sources:
        sys.exit("no source sounds found")

    n = int(RATE * CLIP_SECONDS)
    clips, report = [], []
    for path in sources:
        x = decode(path)
        short = len(x) < n
        if short:
            clip = ramp(x, RAMP_MS)
            pad = n - len(clip)
            clip = np.concatenate([np.zeros(pad // 2), clip,
                                   np.zeros(pad - pad // 2)])
        else:
            start = loudest_window(x, n)
            clip = ramp(x[start:start + n], RAMP_MS)
        clips.append(clip)
        report.append((path.name, len(x) / RATE, short,
                       0.0 if short else start / RATE))

    if NORMALIZE == "rms":
        for i, c in enumerate(clips):
            r = np.sqrt(np.mean(c * c))
            if r > 0:
                clips[i] = c * (TARGET_RMS / r)
    # Always ceiling the peak, whatever NORMALIZE says. Several sources sit at
    # full scale and the mono downmix pushes them past it, so writing PCM_16
    # without this clips the loudest transients -- the very events the clip was
    # chosen for.
    for i, c in enumerate(clips):
        m = np.max(np.abs(c))
        if m > PEAK_CEILING:
            clips[i] = c * (PEAK_CEILING / m)

    print(f"{len(clips)} clips of {CLIP_SECONDS}s at {RATE} Hz mono, "
          f"{RAMP_MS} ms raised-cosine ramps"
          + (f", normalised by {NORMALIZE}" if NORMALIZE else ", levels untouched"))
    print(f"{'file':<16}{'rms dBFS':>10}{'peak dBFS':>11}   source")
    for i, (clip, (name, dur, short, start)) in enumerate(zip(clips, report), start=1):
        out = HERE / f"sound_{i:02d}.wav"
        sf.write(out, clip.astype(np.float32), RATE, subtype="PCM_16")
        rms = 20 * np.log10(max(np.sqrt(np.mean(clip * clip)), 1e-12))
        peak = 20 * np.log10(max(np.max(np.abs(clip)), 1e-12))
        where = "whole file, padded" if short else f"from {start:.2f}s"
        print(f"{out.name:<16}{rms:>10.1f}{peak:>11.1f}   {name[:44]} "
              f"({dur:.2f}s, {where})")


if __name__ == "__main__":
    main()
