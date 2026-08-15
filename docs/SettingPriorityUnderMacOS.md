# Setting priority under macOS

If you are here because of EEG or MEG trigger timing, read
[Minimising trigger-to-stimulus jitter](TriggerJitterForEEGandMEG.md) as well:
priority is necessary but nowhere near sufficient, and the dominant term is not
the one most people expect.

> ### ⚠️ None of this has been measured on macOS
>
> The Linux procedure in [Setting priority under
> Linux](SettingPriorityUnderLinux.md) was verified against `chrt` and `nice` on
> real hardware, and every figure in it comes from a run. **This page has not
> been.** goxpyriment's own Darwin scheduling code
> (`sysinfo/scheduling_darwin.go`) is likewise written against the documented
> BSD/Darwin APIs and never exercised — it says so at runtime, in the `Sched:`
> line of every report.
>
> What follows is the mechanism and the procedure, not a result. [Measure
> it](#how-to-find-out-whether-it-helped-on-your-machine) before quoting a
> number.

---

## The short version

```bash
sudo nice -n -20 ./my_experiment
```

This is worth doing on a dedicated stimulus machine, **with two caveats that are
not cosmetic** — one about what it actually buys you, and one about what `sudo`
does to your data files. Both are below. Read them before adopting this as a
lab habit.

---

## Why a launcher prefix at all

On Linux, a goxpyriment program raises its own priority at startup — it calls
`sysinfo.RaiseToRealTime` and needs no `chrt` prefix.

**On macOS it does not.** `sysinfo/realtime_other.go` deliberately implements
`raiseToRealTime` as an error on every non-Linux platform, so every macOS run
logs this once at startup:

```
real-time scheduling not obtained, continuing at normal priority:
real-time scheduling is not implemented on darwin
```

That message is expected on macOS and is not a fault. The reason it is an error
rather than a silent success is worth stating, because it is the same reason
`nice` is a weaker lever here than it looks:

> macOS has `thread_policy_set`, but […] the setting that matters most for
> stimulus timing is something else entirely […] a per-thread time-constraint
> policy on macOS. Pretending a portable "raise priority" call covers them would
> produce a program that reports success while doing something different on each
> platform.

So on macOS the priority decision is made **outside** the program.
`-no-realtime` and `-realtime-priority` have no effect there beyond suppressing
the message.

---

## Caveat 1: `nice` is not the lever that governs real-time behaviour on Darwin

This is the part where the recommendation should be qualified rather than simply
repeated.

`nice -n -20` lowers the process's nice value, which raises its priority
**within the timeshare scheduling band**. That is a real effect: it makes the
experiment less likely to be preempted by other user-space work, which is
exactly what you want on a machine that is doing anything else at all.

But on Darwin, "will this thread wake on time" is not primarily decided by the
timeshare priority. It is decided by which *scheduling band* the thread is in,
and the band that gives hard wake-up guarantees is entered by setting a
**per-thread time-constraint policy** (`thread_policy_set` with
`THREAD_TIME_CONSTRAINT_POLICY`) — the mechanism Core Audio uses for its render
threads. A negative nice value does not move a thread into that band.

goxpyriment does not set such a policy. `sysinfo/scheduling_darwin.go` is
explicit that it cannot even *see* one:

```
Sched:      policy: SCHED_OTHER (assumed)  nice: -20  [UNTESTED on macOS; cannot
            see per-thread THREAD_TIME_CONSTRAINT_POLICY, so RealTime is not
            authoritative]
```

The practical reading: **expect `nice` to reduce preemption by other processes,
not to give the experiment thread a deadline guarantee.** It is a real
improvement of the "keep other work out of the way" kind, in the same family as
quitting Spotlight-heavy applications before a session — and it is a smaller,
softer effect than `chrt -f 50` is on Linux. Do not carry a Linux expectation
across.

---

## Caveat 2: `sudo` leaves your data files owned by root

This one will bite you, and it has nothing to do with timing.

`sudo nice -n -20 ./my_experiment` runs the **whole experiment** as root, not
just the priority change. Every file it writes is then owned by `root`:

```
-rw-r--r--  1 root  wheel   14523  Aug 15 10:24 Timing-Tests_sub-000_….csv
```

Two consequences. First, the results in `~/goxpy_data/` are no longer yours to
move, edit or delete without `sudo`. Second, and worse, if the directory itself
ends up root-owned, a **later run without `sudo` may fail to write its data
file** — after the participant has done the task.

Mitigations, in order of preference:

1. **Check the ownership after your first `sudo` run**, and fix it once:
   ```bash
   ls -l ~/goxpy_data | head
   sudo chown -R "$USER" ~/goxpy_data
   ```
2. **Point the run somewhere explicit** with `-outdir`, so a mistake is confined
   to one directory rather than to your whole data tree.
3. **Decide per-machine, not per-run.** If a dedicated stimulus Mac always runs
   with `sudo`, that is consistent and fine. Mixing the two on the same data
   directory is what causes trouble.

There is also an identity question worth knowing about: macOS privacy
permissions (screen recording, input monitoring, accessibility) are granted per
user and per application, so a program run as root is not necessarily the same
subject as the same program run as you. If you are prompted for a permission you
thought you had already granted, this is why.

---

## Step 1: run it

```bash
sudo nice -n -20 ./my_experiment
```

`nice` needs root only for **negative** values; `nice -n 5` (lower priority)
works unprivileged. `-20` is the strongest available; the scale runs −20 (most
favourable) to +19.

`renice` can change a process after it has started, which avoids running the
whole experiment as root:

```bash
./my_experiment &
sudo renice -n -20 -p $!
```

The trade is that the first fraction of a second runs at normal priority.
Harmless for an experiment with an instructions screen before the first trial;
not harmless for one that starts measuring immediately.

To make it reproducible across people in a lab, put it in a one-line script
beside the experiment rather than relying on everyone remembering the prefix.

---

## Step 2: verify

Every goxpyriment program's system report carries the state it actually ran
with, so a recorded run is self-labelling:

```bash
Timing-Tests -sysinfo
```

Read the `nice:` field of the `Sched:` line — `-20` if the prefix took, `0` if
it did not.

What the line **cannot** tell you is whether anything is in the time-constraint
band, and it says so rather than guessing. `RealTime` stays `false` on macOS
even if a thread does hold such a policy; a confident "not real-time" would be
an answer this code is not in a position to give.

---

## How to find out whether it helped, on your machine

This is the part that turns the page from advice into a result, and it takes
about ten minutes.

Run the frame-interval test twice, changing only the prefix:

```bash
Timing-Tests -test display -duration-s 300
sudo nice -n -20 Timing-Tests -test display -duration-s 300
```

Then compare the two `-info.txt`/`.csv` pairs. What to read, in order:

1. **Dropped frames and the maximum interval** — the tail, not the average. A
   priority change moves the worst frames; it barely moves the median, so a
   comparison of means will show almost nothing even when the change helped.
2. **The SD of the frame intervals.**
3. **The `Sched:` line in each `-info.txt`**, which records which run was which
   so the two files cannot be mixed up afterwards.

Do it on a machine in the state you will actually run participants on. An idle
Mac hides scheduling problems — that is the single most common way this kind of
comparison produces a false negative. Given caveat 1, a **small or absent**
difference on an idle machine is the outcome to expect; the interesting
comparison is under load.

If you run it, the numbers would be genuinely valuable to this project; there
are currently none for macOS.

---

## Other macOS-specific things worth knowing

Listed as pointers rather than recommendations — none has been verified here,
and on current hardware several of these are plausibly larger terms than `nice`:

- **App Nap and Low Power Mode.** Both throttle background and
  power-constrained work. Low Power Mode on a laptop is the more likely of the
  two to affect a foreground fullscreen application, and it is a single toggle
  in System Settings → Battery. **Run on mains power.**
- **ProMotion / variable refresh rate.** MacBook Pro and Studio Display panels
  can vary their refresh rate. Pin the display to a fixed rate on a stimulus
  machine: a panel that changes its own frame period defeats the assumption
  every frame-counted duration rests on. This is the macOS instance of the
  problem `tests/Timing-Tests -test vrr` exists to probe.
- **Apple silicon efficiency cores.** A thread scheduled onto an E-core behaves
  differently from one on a P-core, and QoS class is what influences that
  placement — another respect in which the meaningful lever on macOS is QoS
  rather than nice.
- **Display mirroring and Sidecar.** Both add a compositing stage between your
  frame and the panel. Turn them off.

Whatever you choose, keep it fixed across a study and record it — a setting that
is invisible afterwards makes two runs incomparable.

---

## See also

- [Setting priority under Linux](SettingPriorityUnderLinux.md) — the measured
  version of this page, and the one to read for the *reasoning*, most of which
  transfers even though the mechanism does not.
- [Minimising trigger-to-stimulus jitter](TriggerJitterForEEGandMEG.md)
- [Timing tests](TimingTests.md) — how to run the measurements above.
