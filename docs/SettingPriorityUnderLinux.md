Here are the step-by-step instructions to set up the `goxpyriment` group with those high-priority privileges.

Steps 1-5 cover real-time priority. [Step 6](#step-6-two-other-groups-the-same-machine-usually-needs)
covers the `input` and `video` groups, which a machine needs before it can read
keyboards or drive the display from a bare console — the configuration the
timing measurements recommend, and the one where their absence first shows.

---

If you are here because of EEG or MEG trigger timing, read
[Minimising trigger-to-stimulus jitter](TriggerJitterForEEGandMEG.md) as well:
real-time priority is necessary but nowhere near sufficient, and the dominant
term is not the one most people expect.

**On another platform?** See [Setting priority under
Windows](SettingPriorityUnderWindows.md) or [Setting priority under
macOS](SettingPriorityUnderMacOS.md). Both use a different mechanism, and
neither has been measured — this page is the only one whose figures come from
real runs, and most of its *reasoning* transfers even where the commands do not.

## Step 1: Create the Group
First, you need to create the group in the system database.

```bash
sudo groupadd goxpyriment
```

### Step 2: Create the Limits Configuration
Linux stores these specific "privilege" rules in `/etc/security/limits.d/`. You should create a new file specifically for your group so it doesn't mess with other system settings.

1.  **Open a new config file:**
    ```bash
    sudo nano /etc/security/limits.d/99-goxpyriment.conf
    ```
2.  **Paste the following lines into the file:**
    ```text
    @goxpyriment - nice -20
    @goxpyriment - rtprio 50
    @goxpyriment - memlock unlimited
    ```
3.  **Save and Exit:** Press `Ctrl + O`, then `Enter` to save, and `Ctrl + X` to exit.
4.  **Check the text actually landed:**
    ```bash
    grep rtprio /etc/security/limits.d/*.conf
    ```
    Worth the two seconds. An editor opened *without* `sudo` — or a graphical
    editor that cannot write to `/etc` — may fail to save without making it
    obvious. The only later symptom is `ulimit -r` still returning `0` after a
    re-login, which is easy to misread as "the limits system doesn't work" rather
    than "the file was never written".

> **Why a new file rather than `/etc/security/limits.conf`?**
> That file belongs to the `libpam-modules` package and can be replaced on
> upgrade, silently taking your setting with it. Anything you add under
> `/etc/security/limits.d/` is yours and survives.

> **Why not simply join an existing group such as `audio`?**
> Tempting, because on many systems `audio` already carries an rtprio grant. But
> that grant is installed by the jackd package and can be revoked by
> `dpkg-reconfigure -p high jackd2`, and it hands out far more than you need
> (`rtprio 95`, `memlock unlimited`). The real objection is subtler: an
> experiment that has quietly lost real-time priority behaves exactly like one
> that still has it, right up until you look at the timing data. A group of your
> own cannot be switched off by another package's maintainer script.

### Step 3: Add Yourself (and others) to the Group
Simply creating the group isn't enough; you have to tell Linux which users belong to it. Replace `$USER` with a specific username if you are adding someone else.

```bash
sudo usermod -aG goxpyriment $USER
```

### Step 4: Apply the Changes
**Important:** Linux only checks group memberships and limits when a user **logs in**. 

* You **must** log out of your Linux session and log back in.
* Alternatively, you can run `su - $USER` in your terminal to start a sub-shell with the new permissions for testing.

### Step 5: Actually Use It

The grant only makes real-time priority *available* — nothing runs at it until
something asks.

**goxpyriment programs ask for themselves.** `Experiment.Initialize()` requests
priority 50 at startup, so once Steps 1-4 are done there is nothing further to
do — including when the program is launched by clicking its icon, where no
command-line prefix is possible. If the grant is not in place it says so and
continues at normal priority rather than refusing to run:

```
real-time scheduling not obtained, continuing at normal priority: real-time
scheduling is not permitted for this user (RLIMIT_RTPRIO is 0). ...
```

Two flags control it:

```bash
./my-experiment -no-realtime               # do not ask at all
./my-experiment -realtime-priority 20      # ask for something other than 50
```

A program with no flags — one built with `NewExperiment` + `Initialize` rather
than `NewExperimentFromFlags` — sets the field directly instead:

```go
exp := control.NewExperiment(...)
exp.RealTimePriority = 0     // decline; the -no-realtime equivalent
```

Every run records what it ended up with, as `sys sched_policy` in its
`-info.txt`. Read it from there rather than assuming: a run at `SCHED_OTHER` and
one at `SCHED_FIFO` are not comparable, so a study should not mix them.

**For anything else**, or to override, use `chrt`:

```bash
chrt -f 50 ./some-other-program
```

`-f` selects SCHED_FIFO and `50` is the priority. **It must not exceed the
`rtprio` value granted in Step 2** — asking for more fails with "Operation not
permitted", which is the same error you get when the grant is missing entirely,
so it is easy to misdiagnose. Keep the two numbers equal unless you have a
reason not to.

The policy is inherited by child processes, so one `chrt` covers the whole run.
It does not persist to your shell or to the next command; that is deliberate.
You *can* make a shell real-time with `chrt -f -p 50 $$`, but do not: every
command you then type runs above most system threads, and a mistyped one is
very hard to interrupt.

### Step 6: Two other groups the same machine usually needs

Real-time priority is not the only permission a stimulus machine wants, and the
other two bite in exactly the configuration the timing measurements recommend:
running without a display server, from a virtual console.

```bash
sudo usermod -aG input $USER      # /dev/input/event*  — keyboards, mice, gamepads
sudo usermod -aG video $USER      # /dev/dri/card*     — KMS/DRM output and vblank
```

Both need a full logout and login, the same as Step 4.

**Why it only shows up on the console.** In a desktop session `systemd-logind`
attaches an ACL granting the active user access to that seat's devices — the
`+` at the end of the permission bits is the ACL:

```
crw-rw----+ 1 root video  226,   0 /dev/dri/card0
crw-rw---- 1 root input   13,  64 /dev/input/event0
```

So everything works in a desktop session and the same program run from a bare VT
finds the nodes closed to it. Group membership is not seat-dependent and covers
both.

**`input`** — without it SDL prints a line per device it cannot open while it
enumerates for joysticks:

```
Error: could not open /dev/input/event3
```

Harmless in itself, and a visual-only run is unaffected. It matters for
**reaction times**: with evdev unavailable SDL falls back to reading the console
tty, which is not the same input path as the one a desktop run uses. We have not
measured the difference — which is the reason to remove the ambiguity rather
than to reason about it. Never compare RT distributions between a console run
and a desktop run without checking both used the same path.

> ⚠️ A user in `input` can read every keystroke on the machine, in every
> session, including other users'. On a dedicated stimulus box that is a fair
> trade; on a shared machine it is not, and the alternative is to make sure the
> VT login creates a proper logind session so the ACLs are applied
> (`loginctl session-status` should show it active on `seat0`).

**`video`** — needed to open `/dev/dri/card*`, which is both how SDL drives the
display under `kmsdrm` and how the vblank backend reads hardware timestamps
(`GOXPY_VBLANK=on`, see `vblank/drm_linux.go`). If GPU rendering then fails
while output works, add `render` as well: the two are separate nodes with
separate groups.

```
crw-rw----+ 1 root video  226,   0 /dev/dri/card0        # video
crw-rw----+ 1 root render 226, 128 /dev/dri/renderD128   # render
```

**`audio`** — usually **not** needed, despite looking like the same case. The
nodes are protected identically:

```
crw-rw----+ 1 root audio  116,   8 /dev/snd/pcmC0D0p
```

but the resemblance stops there. SDL does not normally open them: it talks to
PipeWire or PulseAudio over a socket in your own runtime directory, and the
sound server holds the device. So audio keeps working from a console where the
display and the keyboard do not, and joining `audio` changes nothing.

It matters only if you bypass the server and open ALSA directly
(`SDL_AUDIO_DRIVER=alsa` with no server running) — which our own measurements
argue against on other grounds: on a Raspberry Pi 4 with the server stopped and
the device opened directly, **every one of 463 tones was torn**. If you are in
that configuration, the missing group is not your biggest problem.

Do not join `audio` to obtain real-time priority either — see [the note in Step
2](#step-2-create-the-limits-configuration) on why that grant is the wrong one
to rely on. PipeWire gets its own real-time threads through RealtimeKit, not
through the group.

Verify the same way as the rest of this page — from the data, not from memory.
A run that obtained the vblank clock says so in its `-info.txt`:

```
# sys vblank_backend: Linux DRM vblank (card /dev/dri/card2, crtc 0, ...)
```

and a run that could not falls back silently to a software-derived onset, which
is a difference of several parts per million in a long block.

---

### How to Verify it Worked
Once you’ve logged back in, you can verify that your Go program (or any process) will have these rights.

**1. Check Group Membership:**
Run the command `groups`. You should see `goxpyriment` in the list.

**2. Check the Memory Lock Limit:**
Run this command to see your current "Max locked memory" limit:
```bash
ulimit -l
```
If it says **`unlimited`**, you are good to go!

**3. Check Real-Time Priority:**
Run:
```bash
ulimit -r
```
It should return **`50`**. If it still returns `0`, the grant is not in effect —
either the file was never written, or you have not fully logged out and back in.

**4. Check from inside your experiment.**
Any goxpyriment program's system report includes a `Sched:` line, so a recorded
run carries its own evidence:

```
Sched:      policy: SCHED_FIFO  priority: 50  REAL-TIME
Sched:      policy: SCHED_OTHER  nice: 0  (real-time available up to 50, not used)
Sched:      policy: SCHED_OTHER  nice: 0  (real-time NOT available to this user)
```

Those three lines are three different situations with three different fixes, and
they are worth being able to tell apart after the fact.

The **last** is this setup not being in place — go back to Step 2. The **middle**
is the setup working but the program not having asked: for a goxpyriment program
that means `-no-realtime` was passed or `RealTimePriority` was set to 0, since
otherwise it asks on its own; for anything else it means no `chrt` prefix.
Without the line in the data you would be left comparing timing distributions and
guessing which had happened.

---

### ⚠️ A Friendly "Warning"

Once this grant is in place, goxpyriment programs run at real-time priority **by
default** — you no longer have to remember a `chrt` prefix, and equally you no
longer get a reminder that you are asking for it. A busy loop (`for { }` with no
sleep, or a spin-wait) is then running above the window manager and the input
handling, and the OS will not interrupt it to let you click "Stop".

Two things bound the damage, and it is worth knowing which one you are relying
on:

- **In-program elevation raises only one thread** — the experiment's own. The Go
  runtime's other threads, including the garbage collector, stay at normal
  priority, so a spinning goroutine occupies one core rather than all of them.
- **`chrt -f 50 prog` raises the whole process**, because the policy is set
  before `exec` and every thread inherits it. That is the more dangerous of the
  two, and the one this warning is mostly about.

Either way: keep a terminal with `top` or `htop` open while developing, and use
`-no-realtime` when stepping through code in a debugger — a breakpoint hit on a
real-time thread can leave the desktop unresponsive until the process is killed.

The kernel's real-time throttle (`/proc/sys/kernel/sched_rt_runtime_us`, 950 ms
per second by default) is a backstop, not a licence: it stops a runaway task
locking the machine completely, but a machine at 95 % real-time occupancy is not
usable.

---

### ⚠️ The throttle also wrecks the timing you asked for

That backstop is not only a protection against you — it is a hazard to the very
thing real-time priority was requested for. Once a `SCHED_FIFO` thread reaches
100 % duty, the kernel stops it for the remaining 50 ms of the second, and that
suspension lands wherever it lands: in the middle of a pulse, a frame, or a
response window.

Measured on a 22-core Linux 7.0 host, with a pinned `SCHED_FIFO 50` thread
spinning continuously:

| host state | stalls | each | when they landed |
|---|---|---|---|
| idle | **0 in 20 s** | — | — |
| under `stress-ng --cpu 20` | **24 in 25 s** | 51.0 ms | 0.999, 2.000, 3.001, 4.002 s … |

One per second, to the millisecond — the throttle period exactly. **Load is a
necessary condition**, which is the worst way for a fault to behave: on an idle
runqueue the kernel borrows unused real-time bandwidth from the other CPUs and
the limit is never reached. So it does not happen on the quiet machine you
develop on, and does happen on the loaded one you run participants on. A pulse
train that spun through its inter-trial gaps at `chrt -f 50` lost up to
**49.63 ms** on 23 of 1000 trials this way — about ten times the 4.75 ms
worst-case spread that real-time priority was bought to remove in the first
place.

**The rule that avoids it: sleep, and spin only the last millisecond or two.**
A wait that sleeps most of its duration never approaches the limit, and the
short spin at the end recovers the precision `time.Sleep` cannot give on its
own. goxpyriment's frame pacing works this way (`apparatus.paceToFrame`), which
takes a 60 Hz present loop from ~100 % duty to ~10 % with no loss of landing
accuracy. If you write your own wait, write it the same way — a bare
`for time.Now().Before(deadline) {}` over a whole trial is the shape that gets
throttled.

If you genuinely need a thread at 100 % duty — a spin-wait on a panel fast
enough that there is nothing left worth sleeping — the limit can be lifted:

```bash
sudo sysctl kernel.sched_rt_runtime_us=-1        # until reboot
```

That removes the backstop along with the throttle, so a runaway real-time loop
will then hold a CPU with nothing left to take it back. Reasonable on a
dedicated stimulus machine; not on a laptop you also read mail on.

---

### ⚠️ The speed the CPU runs at is not fixed either

Real-time priority decides **when** your thread runs. It says nothing about how
fast the core runs once it does, nor how long the core takes to wake up, and
both default to saving power rather than responding quickly:

- **The frequency governor.** `powersave` and `schedutil` raise the clock only
  after they observe load, so the first work after an idle gap runs at a lower
  clock than the work after it.
- **Idle states.** A deeply idle core takes tens to hundreds of microseconds to
  come back. An experiment that sleeps between frames is idle by design — which
  is precisely the pattern that pays this cost, and the pattern the previous
  section tells you to adopt.

To pin the clock for a session:

```bash
sudo cpupower frequency-set -g performance     # package: linux-tools-common / cpupower
cpupower frequency-info | grep -i "current policy"
```

It does not survive a reboot. Make it persistent through your distribution's
usual mechanism (a systemd unit, or `GOVERNOR=performance` in
`/etc/default/cpufrequtils` on Debian and Ubuntu) rather than by remembering to
type it, since the failure mode is silent.

**Unlike the rest of this page, this section is not backed by a measurement of
its own.** It is here because of one unexplained observation: on 2026-08-19, in
a 1010-cycle run on a workstation whose onsets were anchored on kernel DRM
vblanks, the host loop's cadence changed by **10 ppm** 316 s in, while the panel
it was driving held its own period to 0.27 ppm across the same instant. Nothing
in the recorded configuration changed. The frequency governor is a *candidate*
for that, not a diagnosis — the honest status is that the run has not been
repeated with the governor pinned, and until it has, this is a cheap precaution
rather than a fix.

Cheap is the operative word: it costs one command and some electricity, and the
alternative is discovering afterwards that two halves of a session were not
comparable.

**Check it from the data rather than from memory.** Every run records the clock
it saw at start-up in its `-info.txt`:

```
# host cpu_mhz: 3600 (max 4800)
```

A run that starts well below its maximum was not on a pinned clock. As with
`sys sched_policy`, the value is in the file so that two runs can be compared
afterwards without anyone having to recall what the machine was doing.

If you take this further — holding `/dev/cpu_dma_latency` open at 0 to keep
cores out of deep idle is the usual next step — measure the pair before and
after and add the numbers here. That is what the rest of this page is made of.

---

> **Note on the grant itself:** goxpyriment only uses `rtprio`. The `nice -20`
> and `memlock unlimited` lines in Step 2 are there because they are commonly
> wanted alongside it and cost nothing, not because anything here requires them.
> Granting `rtprio` alone is enough if you prefer the smaller privilege.

