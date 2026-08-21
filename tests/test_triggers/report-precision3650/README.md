# DLP-IO8 against the parallel port — Precision 3650, 21 August 2026

Does a USB trigger box lag enough to matter for MEG? Three runs of
`test_triggers` with both devices fired on one schedule and both edges read in
one Analog Discovery 3 acquisition.

**Result: the DLP-IO8's edge follows the parallel port's by 185.9 µs
(sd 12.6, n = 900 pulses). The worst single pulse of the campaign was 254.8 µs.
Nothing exceeded 500 µs, let alone the 1 ms budget — a 3.9× margin.** The host
issued both writes within 0.03 µs (median), so the figure is the hardware.

`meg-trigger-report.html` is the full write-up with the figures and the caveats.
It is the page content of an Artifact (no `<html>` wrapper, fonts from Google
Fonts), so it renders best online; locally it still reads fine.

Cross-check worth knowing: the same box measured against an **FT232H** on a
different machine the same day lagged by 177–181 µs (see `../report-is158520/`).
Two independent references, ~6 µs apart.

## Files

Each run is one `.npz` (AD3 edge times: `rise_ch1`/`fall_ch1` = parallel port D0,
`rise_ch2`/`fall_ch2` = DLP-IO8 line 1) plus the host-side `.csv` and its
`-info.txt` sidecar.

| run | .npz | .csv / -info.txt | notes |
|---|---|---|---|
| 1 | `parallel_vs_dlpio8_run1.npz` | `…-201706` | 352 corrupted samples; carries all five outliers |
| 2 | `parallel_vs_dlpio8_run2.npz` | `…-202139` | 16 383 lost samples, in a gap common to both channels |
| 3 | `parallel_vs_dlpio8_run3.npz` | `…-202847` | clean |

Each run: 305 pulses (5 warm-up, excluded) at 500 ms, 5 ms wide, parallel mode,
GC suspended, no write errors.

**A common record gap cancels in the channel difference; a one-channel gap would
not.** Run 2's gap sits at the same index on both channels, which was checked
rather than assumed — the skew is quoted from it, but no per-channel time axis is.

## Conditions

Dell Precision 3650 Tower, Intel i5-10600K, Ubuntu 24.04.4, kernel 7.0.0-28.
Parallel port `/dev/parport0`, D0 = DB25 pin 2. DLP-IO8 auto-detected at
`/dev/serial/by-id/usb-DLP_Design_DLP-IO8_12345678-if00-port0`, terminal 1.
AD3 at 1 MS/s, both channels 0–5 V, **one absolute threshold of 2.5 V** — both
devices swing 5 V, so mid-swing is the same fraction of each edge. Grounds are
the desktop's throughout; the box is USB-powered from it.

The AD3 was on a **different machine** from the trigger devices. That is fine for
the channel-to-channel skew, which is the question — the instrument's clock
cancels — but it means no per-device host→wire latency can be recovered here,
since the two hosts' clocks cannot be aligned.

## Two caveats for anyone quoting these numbers

- **The CPU governor was not pinned**: `host cpu_mhz` reads 3600, 3600 and 3256
  against a 4800 maximum. A figure dominated by UART wire time should not care,
  but that is an expectation; a run under
  `cpupower frequency-set -g performance` would settle it.
- **The files record the real-time *request*, not the grant.** All three asked
  for `SCHED_FIFO 50`. The pooled host issue skew (sd 1.63 µs) is strong evidence
  it was granted, but it is inference. `test_triggers` now reads the policy back
  per firing thread and records `t realtime_obtained:` beside the request, so a
  repeat states it outright.

## Redoing it

```bash
# on the machine with the AD3 (close WaveForms first — it holds the device)
ad3-capture --seconds 180 --channels 1,2 --threshold 2.5 --out run.npz

# on the machine with the devices
lsmod | grep '^lp' && sudo rmmod lp     # or the parallel claim can hang the box
go run ./tests/test_triggers -no-prompt \
    -device parallel:pin=1 -device dlpio8:pin=1 \
    -n 300 -isi-ms 500 -width-ms 5
```

Skew is `rise_ch2 − rise_ch1`; subtract the CSV's `issue_skew_us` per pulse to
remove the host's share. Align rows to edges with `target_ns`: repetition *k*
was scheduled at *k* × 500 ms after the first pulse.

## Open

Five pulses of 900 sit outside 150–230 µs, all in run 1. Four are consecutive
(t = 63.7–65.7 s, +234 to +255 µs) — the DLP-IO8 briefly late, matching
behaviour seen on the other machine. The fifth reads −52 µs, which is the
**parallel port's** rising edge arriving ~240 µs late: its pulse is 4756 µs wide
against a 4999 µs median, with the falling edge on schedule and the host write
issued on time. Run 1 also had 352 corrupted samples, so that one event cannot
be separated from a misplaced edge. Either way the reference is not perfectly
deterministic, which is worth knowing before treating it as ground truth.
