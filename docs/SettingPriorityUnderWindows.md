# Setting priority under Windows

If you are here because of EEG or MEG trigger timing, read
[Minimising trigger-to-stimulus jitter](TriggerJitterForEEGandMEG.md) as well:
priority is necessary but nowhere near sufficient, and the dominant term is not
the one most people expect.

> ### ⚠️ None of this has been measured on Windows
>
> The Linux procedure in [Setting priority under
> Linux](SettingPriorityUnderLinux.md) was verified against `chrt` and `nice` on
> real hardware, and every figure in it comes from a run. **This page has not
> been.** goxpyriment's own Windows scheduling code
> (`sysinfo/scheduling_windows.go`) is likewise written against the documented
> Win32 API and never exercised — it says so at runtime, in the `Sched:` line of
> every report.
>
> What follows is the mechanism and the procedure, not a result. [Measure
> it](#how-to-find-out-whether-it-helped-on-your-machine) before quoting a
> number.

---

## The short version

From **`cmd.exe`** (not PowerShell — see below):

```bat
start /high my_experiment.exe
```

That is the recommendation. `/realtime` also exists and is discussed below, but
it is **not** the default suggestion, for reasons that are worth reading before
you use it.

---

## Why a launcher prefix at all

On Linux, a goxpyriment program raises its own priority at startup — it calls
`sysinfo.RaiseToRealTime` and needs no `chrt` prefix, which is what makes the
Linux setup a one-time system configuration rather than a thing to remember.

**On Windows it does not.** `sysinfo/realtime_other.go` deliberately implements
`raiseToRealTime` as an error on every non-Linux platform, so every Windows run
logs this once at startup:

```
real-time scheduling not obtained, continuing at normal priority:
real-time scheduling is not implemented on windows
```

That message is expected on Windows and is not a fault. It is there because the
alternative — a portable "raise priority" call that quietly does something
different on each OS — would report success while giving you a different
guarantee on every machine. Windows has no `SCHED_FIFO` and no POSIX priority to
raise; it has *priority classes*, which are a different mechanism with different
consequences.

So on Windows the priority decision is made **outside** the program, by however
you launch it. `-no-realtime` and `-realtime-priority` have no effect there
beyond suppressing the message.

---

## Step 1: launch through `start`

`start` is a **`cmd.exe` built-in command**, and its priority switches are the
simplest way to set a process's priority class at launch:

| switch | priority class |
|---|---|
| `/low` | `IDLE_PRIORITY_CLASS` |
| `/belownormal` | `BELOW_NORMAL_PRIORITY_CLASS` |
| `/normal` | `NORMAL_PRIORITY_CLASS` (the default) |
| `/abovenormal` | `ABOVE_NORMAL_PRIORITY_CLASS` |
| `/high` | `HIGH_PRIORITY_CLASS` |
| `/realtime` | `REALTIME_PRIORITY_CLASS` |

A shortcut or a `.bat` file next to the experiment is the practical way to make
this reproducible, so that a run started by someone else gets the same treatment
as one started by you:

```bat
@echo off
rem run_experiment.bat — always launch at HIGH, never at NORMAL by accident
start /high /wait my_experiment.exe %*
```

`/wait` keeps the console window attached so you can see the program's own log
output, including the `Sched:` line.

### ⚠️ In PowerShell, `start` is a different command

This is the trap most likely to catch you, because it fails *quietly*:

```powershell
start /high my_experiment.exe        # NOT what you want in PowerShell
```

In PowerShell, `start` is an alias for `Start-Process`, which has no `/high`
switch and no priority parameter at all. Depending on the version you will get
an error about an unexpected argument, or `/high` will be passed through as an
*argument to your program* — which goxpyriment's flag parser will reject, but a
different program might silently ignore. Either way the priority class is
`NORMAL`.

Two ways out. Call `cmd` explicitly:

```powershell
cmd /c start /high my_experiment.exe
```

or start the process and set the class afterwards:

```powershell
$p = Start-Process .\my_experiment.exe -PassThru
$p.PriorityClass = 'High'
```

The second raises the priority a moment *after* the process starts, so the very
beginning of the run is at `NORMAL`. That is harmless for an experiment with an
instructions screen and a keypress before the first trial, and not harmless for
one that starts measuring immediately.

---

## Step 2: decide about `/realtime` — the honest answer is "probably not"

`REALTIME_PRIORITY_CLASS` is not the Windows equivalent of Linux's `SCHED_FIFO
50`. It is considerably more aggressive, and Microsoft's own documentation warns
against it: threads in this class run above most operating-system threads,
including ones handling disk flushing and mouse and keyboard input. A busy loop
there can make the machine unresponsive in a way that `/high` cannot.

goxpyriment's frame pacing sleeps most of each wait and spins only the last
couple of milliseconds (`apparatus.paceToFrame`), specifically so that it does
not sit at 100 % duty — so the worst case is less likely than it would be with a
naive spin-wait. But "less likely" is the whole claim, and it is not one this
project has tested on Windows.

Two further reasons to prefer `/high` as the default:

- **`/realtime` needs administrator privileges** (`SeIncreaseBasePriorityPrivilege`).
  Without them the process is created at `HIGH_PRIORITY_CLASS` instead — and
  **no error is reported**. You get the priority you would have got from
  `/high`, while believing you got something stronger. This is the same
  failure shape the Linux page warns about, where a missing `rtprio` grant
  produces a run that looks identical to one that has it.
- **`/high` is very likely where the return curve flattens.** The gap between
  `NORMAL` and `HIGH` is the one that stops ordinary background work preempting
  the experiment. The gap between `HIGH` and `REALTIME` mostly buys precedence
  over the OS itself, which is not what a dropped frame usually comes from.

If you do use `/realtime`, launch the shell as Administrator, and **check the
`Sched:` line afterwards** rather than assuming it took.

---

## Step 3: the thing that probably matters more — timer resolution

For millisecond stimulus timing on Windows, the system timer resolution is
plausibly a larger term than the priority class, and it is the reason
`sysinfo/scheduling_windows.go` collects it alongside the priority:

> The timer resolution matters more than the priority class for millisecond
> stimulus timing […] the default tick has historically been ~15.6 ms, and a
> process that has not raised it cannot place an event to the millisecond
> however high its priority.

A process that has not requested a finer tick can have its sleeps rounded to the
nearest ~15.6 ms, which is roughly one 60 Hz frame — no priority class rescues
that. The Go runtime does request high-resolution timers on modern Windows, so
in practice you are likely to see a much smaller number, but that is a runtime
implementation detail rather than a guarantee, which is why goxpyriment
**measures** it (`NtQueryTimerResolution`) instead of assuming it.

Read it off the report:

```
Sched:      policy: HIGH_PRIORITY_CLASS  nice: 0  [UNTESTED on Windows; no POSIX
            policy, priority class shown instead; timer resolution 0.500 ms]
```

If that figure reads ~15.6 ms rather than ~1 ms or below, **stop and fix that
first** — no amount of priority will compensate, and any timing you record until
you do is quantised to a frame.

---

## Step 4: verify

Every goxpyriment program's system report carries the state it actually ran
with, so a recorded run is self-labelling. Print it without running an
experiment:

```bat
Timing-Tests -sysinfo
```

and look at the `Sched:` line:

| what you see | what it means |
|---|---|
| `policy: NORMAL_PRIORITY_CLASS` | the prefix did not take — most likely the PowerShell trap above |
| `policy: HIGH_PRIORITY_CLASS` | `/high` worked |
| `policy: REALTIME_PRIORITY_CLASS … REAL-TIME` | `/realtime` worked *and* you had the privilege |
| `policy: unknown` | `GetPriorityClass` failed; report it as a bug |

Note the asymmetry that makes checking worth the two seconds: asking for
`/realtime` **without** administrator rights shows `HIGH_PRIORITY_CLASS` here,
which is exactly what a successful `/high` shows. The line tells you what you
got, not what you asked for.

The `[UNTESTED on Windows …]` note is printed deliberately and will stay until
someone runs the verification below on real hardware.

---

## How to find out whether it helped, on your machine

This is the part that turns the page from advice into a result, and it takes
about ten minutes. Nothing above substitutes for it, because the answer depends
on your GPU driver, your compositor settings and what else the machine is doing.

Run the frame-interval test twice, changing only the launcher:

```bat
Timing-Tests -test display -duration-s 300
start /high Timing-Tests -test display -duration-s 300
```

Then compare the two `-info.txt`/`.csv` pairs. What to read, in order:

1. **Dropped frames and the maximum interval** — the tail, not the average. A
   priority change moves the worst frames; it barely moves the median, so a
   comparison of means will show almost nothing even when the change helped.
2. **The SD of the frame intervals.**
3. **The `Sched:` line in each `-info.txt`**, which records which run was which
   so the two files cannot be mixed up afterwards.

Do it on a machine in the state you will actually run participants on. An idle
machine hides scheduling problems — that is the single most common way this kind
of comparison produces a false negative.

If you run it, the numbers would be genuinely valuable to this project; there
are currently none for Windows.

---

## Other Windows-specific things worth knowing

Listed as pointers rather than recommendations — none has been verified here:

- **Fullscreen optimizations.** Windows 10/11 may run an apparently exclusive
  fullscreen window through the desktop compositor instead. That is the same
  class of problem as the compositor issues documented for Linux in
  [TimingTests.md](TimingTests.md), and it can be disabled per-executable in the
  file's Properties → Compatibility tab.
- **Variable refresh rate / G-Sync / FreeSync.** Turn it off on a stimulus
  machine. A panel that varies its own frame period defeats the assumption every
  frame-counted duration rests on.
- **Power plan.** "High performance" rather than "Balanced"; core parking and
  aggressive frequency scaling both add wake-up latency.
- **Windows Game Mode and background apps.** Both change scheduling in ways that
  are documented only loosely. Whatever you choose, keep it fixed across a study
  and record it — a setting that is invisible afterwards makes two runs
  incomparable.

---

## See also

- [Setting priority under Linux](SettingPriorityUnderLinux.md) — the measured
  version of this page, and the one to read for the *reasoning*, most of which
  transfers even though the mechanism does not.
- [Minimising trigger-to-stimulus jitter](TriggerJitterForEEGandMEG.md)
- [Timing tests](TimingTests.md) — how to run the measurements above.
