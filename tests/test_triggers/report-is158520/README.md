# FT232H against DLP-IO8 — is158520, 21 August 2026

Six runs of `test_triggers` with both devices on one Analog Discovery 3
acquisition, and the write-up that came out of them.

**Result: the DLP-IO8's TTL edge lags the FT232H's by 179.4 µs (sd 5.7,
n = 322 pulses pooled over three 60 s runs).** The program's own contribution is
0.03 µs median, so the figure is the hardware. It is *not* a constant to carry
between sessions: two of the six datasets sat 40–50 µs higher.

`trigger-skew-report.html` has the full write-up, the figures and the caveats.
It is the page content of an Artifact (no `<html>`/`<head>` wrapper, fonts from
Google Fonts), so it renders best online; opened locally it still reads fine
with fallback fonts.

## Files

Each run is one `.npz` (AD3 edge times, `rise_ch1`/`fall_ch1` = FT232H on AD0,
`rise_ch2`/`fall_ch2` = DLP-IO8 line 1) plus the `.csv` of host-side timings and
its `-info.txt` sidecar, which carries the flags, the host facts and (from
16:38 on) the printed report.

| in the report | .npz | .csv / -info.txt | mode | notes |
|---|---|---|---|---|
| short verification | `ft232h_vs_dlpio8.npz` | `…-163852` | parallel | 12 pulses at 250 ms, first run with both channels working |
| run 1 | `ft232h_vs_dlpio8_60s.npz` | `…-164323` | parallel | 115 pulses; **the last 23 are the slower regime** |
| run 2 | `ft232h_vs_dlpio8_60s_seq.npz` | `…-164623` | **sequential** | 60 810 corrupted samples, 16 ms gap common to both channels |
| run 3 | `ft232h_vs_dlpio8-run3.npz` | `…-165538` | parallel | **capture 40 s against a 60 s train — 72 of 120 pulses**; slower regime throughout |
| run 4 | `ft232h_vs_dlpio8-run4.npz` | `…-170016` | parallel | full coverage, clean |
| run 5 | `ft232h_vs_dlpio8-run5.npz` | `…-170334` | parallel | full coverage, clean; host-side excursions |

Runs 3–5 used the patched `triggers/ft232h_linux.go`; run 1 and run 2 did not.
`ch1` carries one edge more than `ch2` in every run: that extra pulse is the
FT232H's open-time artefact (51.25 ms before the fix, ~0.12 ms after), and it
must be dropped before the two channels are aligned.

## Conditions

Dell Precision 5490, Ubuntu 26.04, kernel 7.0.0-30, Core Ultra 7 165H.
AD3 at 1 MS/s, both channels 0–5 V window, **one absolute threshold of 1.5 V**
(not per-channel 50 % levels — the devices swing 3.3 V and 5 V, so 50 % levels
would put the difference straight into the skew). Grounds common.
`SCHED_FIFO` 50, GC suspended during the train, 5 warm-up pulses per run
excluded from every statistic. No write errors in any run.

## Redoing it

```bash
ad3-capture --seconds 75 --channels 1,2 --threshold 1.5 --out run.npz   # close WaveForms first
go run ./tests/test_triggers \
    -device ft232h:pin=1 -device dlpio8:port=/dev/ttyUSB1,pin=1 \
    -n 115 -isi-ms 500 -width-ms 5
```

Align on `target_ns`: repetition *k* was scheduled at *k* × 500 ms after the
first pulse. Subtract `issue_skew_us` from the wire skew per pulse to get the
device-only figure — in run 5 that removes four excursions that were the
scheduler, and in run 3 it removes nothing, which is how the slower regime is
known to be real.

## Open

What puts the DLP-IO8 into the slower regime (+40–50 µs, several times the
scatter, its pulse width degrading with it while the FT232H's does not) is
unidentified. Two occurrences in six runs. USB scheduling, the FTDI bridge and
the PIC firmware are all downstream of the write syscall, whose cost did not
change; nothing here separates them.
