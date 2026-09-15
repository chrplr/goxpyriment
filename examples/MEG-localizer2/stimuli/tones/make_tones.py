#!/usr/bin/env python3
"""Generate narrow-band noise tones for a tonotopy experiment.

Produces N tones whose centre frequencies are log-spaced:

    f_n = f0 * 2**(n * step)          n = 0 .. N-1   (step in octaves)

Each tone is a narrow band of noise (default width: 1/4 semitone) synthesised
as a sum of sinusoids with random phases, uniformly spaced (on a log axis)
inside the band. The band edges are f * 2**(±bandwidth/24) so the band is
centred on f on a log-frequency scale.

Post-processing (see description.md):
  * cos² (raised-cosine) onset/offset ramps, default 10 ms;
  * equal-loudness correction from the ISO 226:2003 contours: every tone is
    scaled so that it sits on the same phon contour (default 60 phon), taking
    1000 Hz as the reference. This corrects for the ear, NOT for the
    headphones/speakers — measure and calibrate the playback chain separately.

Output: 16-bit mono WAV files (tone_0200Hz.wav, …) plus tones.csv listing the
centre frequency, band edges and applied gain of each tone.

Examples:
    python3 make_tones.py                          # 200..3200 Hz, 1-octave steps
    python3 make_tones.py --f0 200 --step 0.6      # 200..1131 Hz
    python3 make_tones.py --no-loudness            # equal RMS instead of equal phon
"""
import argparse
import csv
import os
import wave

import numpy as np

# ---------------------------------------------------------------------------
# ISO 226:2003 equal-loudness contours (Table 1 of the standard).
# Valid for 20 Hz .. 12.5 kHz and 20 .. 90 phon.
# ---------------------------------------------------------------------------
ISO226_F = np.array([
    20, 25, 31.5, 40, 50, 63, 80, 100, 125, 160, 200, 250, 315, 400, 500,
    630, 800, 1000, 1250, 1600, 2000, 2500, 3150, 4000, 5000, 6300, 8000,
    10000, 12500])
ISO226_AF = np.array([
    0.532, 0.506, 0.480, 0.455, 0.432, 0.409, 0.387, 0.367, 0.349, 0.330,
    0.315, 0.301, 0.288, 0.276, 0.267, 0.259, 0.253, 0.250, 0.246, 0.244,
    0.243, 0.243, 0.243, 0.242, 0.242, 0.245, 0.254, 0.271, 0.301])
ISO226_LU = np.array([
    -31.6, -27.2, -23.0, -19.1, -15.9, -13.0, -10.3, -8.1, -6.2, -4.5,
    -3.1, -1.7, -0.2, 0.5, 1.0, 1.2, 1.5, 0.9, 0.4, -0.5,
    -1.3, -1.9, -2.4, -2.4, -1.6, 0.5, 4.0, 5.1, -2.0])
ISO226_TF = np.array([
    78.5, 68.7, 59.5, 51.1, 44.0, 37.5, 31.5, 26.5, 22.1, 17.9,
    14.4, 11.4, 8.6, 6.2, 4.4, 3.0, 2.2, 2.4, 3.5, 1.7,
    -1.3, -4.2, -6.0, -5.4, -1.5, 6.0, 12.6, 13.9, 12.3])


def iso226_spl(freq, phon):
    """Sound pressure level (dB SPL) of a pure tone at `freq` Hz that is
    perceived at `phon` phon, per ISO 226:2003. Table values are interpolated
    on a log-frequency axis."""
    if not (20 <= freq <= 12500):
        raise ValueError(f"{freq} Hz is outside the ISO 226 range (20-12500 Hz)")
    if not (20 <= phon <= 90):
        raise ValueError(f"{phon} phon is outside the ISO 226 range (20-90 phon)")
    lf = np.log10(freq)
    af = np.interp(lf, np.log10(ISO226_F), ISO226_AF)
    lu = np.interp(lf, np.log10(ISO226_F), ISO226_LU)
    tf = np.interp(lf, np.log10(ISO226_F), ISO226_TF)
    a = 4.47e-3 * (10 ** (0.025 * phon) - 1.15) + (0.4 * 10 ** ((tf + lu) / 10 - 9)) ** af
    return (10 / af) * np.log10(a) - lu + 94


def loudness_gain_db(freq, phon, ref=1000.0):
    """dB to add at `freq` so it matches the loudness of `ref` at `phon` phon."""
    return iso226_spl(freq, phon) - iso226_spl(ref, phon)


# ---------------------------------------------------------------------------
# Synthesis
# ---------------------------------------------------------------------------
def narrow_band_noise(fc, bandwidth_semitones, duration, sr, rng, n_components=64):
    """Sum of `n_components` sinusoids, log-spaced within ±bandwidth/2 of fc,
    with random phases. Returned at unit RMS."""
    half = bandwidth_semitones / 24.0          # half-width in octaves
    freqs = fc * 2 ** np.linspace(-half, half, n_components)
    phases = rng.uniform(0, 2 * np.pi, n_components)
    t = np.arange(int(round(duration * sr))) / sr
    x = np.sin(2 * np.pi * np.outer(t, freqs) + phases).sum(axis=1)
    return x / np.sqrt(np.mean(x ** 2))


def cos2_ramps(n, ramp_s, sr):
    """Envelope with cos² onset and offset ramps of `ramp_s` seconds."""
    r = int(round(ramp_s * sr))
    env = np.ones(n)
    if r > 0:
        ramp = np.sin(np.linspace(0, np.pi / 2, r)) ** 2   # 0 → 1, cos²-shaped
        env[:r] = ramp
        env[-r:] = ramp[::-1]
    return env


def write_wav(path, x, sr):
    y = np.clip(np.round(x * 32767), -32768, 32767).astype("<i2")
    with wave.open(path, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(sr)
        w.writeframes(y.tobytes())


# ---------------------------------------------------------------------------
def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0],
                                 formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    ap.add_argument("--f0", type=float, default=200.0, help="lowest centre frequency (Hz)")
    ap.add_argument("--step", type=float, default=1.0, help="spacing between tones (octaves)")
    ap.add_argument("--n", type=int, default=5, help="number of tones")
    ap.add_argument("--duration", type=float, default=0.5, help="tone duration (s)")
    ap.add_argument("--bandwidth", type=float, default=0.25, help="spectral width (semitones)")
    ap.add_argument("--ramp", type=float, default=0.010, help="cos² onset/offset ramp (s)")
    ap.add_argument("--phon", type=float, default=60.0,
                    help="ISO 226 loudness contour used for equalisation")
    ap.add_argument("--no-loudness", action="store_true",
                    help="skip ISO 226 correction (all tones at equal RMS)")
    ap.add_argument("--peak", type=float, default=-3.0,
                    help="peak level of the loudest file (dBFS); others are relative")
    ap.add_argument("--sr", type=int, default=44100, help="sample rate (Hz)")
    ap.add_argument("--seed", type=int, default=0, help="random seed for the noise phases")
    ap.add_argument("--outdir", default=".", help="output directory")
    args = ap.parse_args()

    rng = np.random.default_rng(args.seed)
    freqs = [args.f0 * 2 ** (i * args.step) for i in range(args.n)]
    gains = [0.0 if args.no_loudness else loudness_gain_db(f, args.phon) for f in freqs]

    env = cos2_ramps(int(round(args.duration * args.sr)), args.ramp, args.sr)
    signals = [narrow_band_noise(f, args.bandwidth, args.duration, args.sr, rng) * env
               * 10 ** (g / 20) for f, g in zip(freqs, gains)]

    # One global scale factor so that relative levels are preserved and the
    # loudest file peaks at --peak dBFS.
    scale = 10 ** (args.peak / 20) / max(np.abs(s).max() for s in signals)

    os.makedirs(args.outdir, exist_ok=True)
    rows = []
    for f, g, s in zip(freqs, gains, signals):
        s = s * scale
        name = f"tone_{f:05.0f}Hz.wav"
        write_wav(os.path.join(args.outdir, name), s, args.sr)
        lo, hi = f * 2 ** (-args.bandwidth / 24), f * 2 ** (args.bandwidth / 24)
        rms_dbfs = 20 * np.log10(np.sqrt(np.mean(s ** 2)))
        peak_dbfs = 20 * np.log10(np.abs(s).max())
        rows.append(dict(file=name, freq_hz=round(f, 2), band_lo_hz=round(lo, 2),
                         band_hi_hz=round(hi, 2), iso226_gain_db=round(g, 2),
                         rms_dbfs=round(rms_dbfs, 2), peak_dbfs=round(peak_dbfs, 2)))

    with open(os.path.join(args.outdir, "tones.csv"), "w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=rows[0].keys())
        w.writeheader()
        w.writerows(rows)

    print(f"{args.n} tones, {args.duration*1000:.0f} ms, {args.bandwidth} semitone band, "
          f"{args.ramp*1000:.0f} ms cos² ramps, "
          + ("no loudness correction" if args.no_loudness else f"ISO 226 @ {args.phon:.0f} phon"))
    print(f"{'file':<18}{'freq':>9}{'band':>20}{'gain dB':>9}{'RMS dBFS':>10}{'peak dBFS':>11}")
    for r in rows:
        print(f"{r['file']:<18}{r['freq_hz']:>9.1f}"
              f"{r['band_lo_hz']:>9.1f}-{r['band_hi_hz']:<10.1f}"
              f"{r['iso226_gain_db']:>9.2f}{r['rms_dbfs']:>10.2f}{r['peak_dbfs']:>11.2f}")


if __name__ == "__main__":
    main()
