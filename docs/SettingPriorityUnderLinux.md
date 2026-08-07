Here are the step-by-step instructions to set up the `goxpyriment` group with those high-priority privileges.

---

### Step 1: Create the Group
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

**goxpyriment programs ask for themselves.** Any experiment built with
`NewExperimentFromFlags` requests priority 50 at startup, so once Steps 1-4 are
done there is nothing further to do — including when the program is launched by
clicking its icon, where no command-line prefix is possible. If the grant is not
in place it says so and continues at normal priority rather than refusing to run:

```
real-time scheduling not obtained, continuing at normal priority: real-time
scheduling is not permitted for this user (RLIMIT_RTPRIO is 0). ...
```

Two flags control it:

```bash
./my-experiment -no-realtime               # do not ask at all
./my-experiment -realtime-priority 20      # ask for something other than 50
```

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
that means `-no-realtime` was passed, since otherwise it asks on its own; for
anything else it means no `chrt` prefix. Without the line in the data you would
be left comparing timing distributions and guessing which had happened.

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
  two, and the one this warning was originally written about.

Either way: keep a terminal with `top` or `htop` open while developing, and use
`-no-realtime` when stepping through code in a debugger — a breakpoint hit on a
real-time thread can leave the desktop unresponsive until the process is killed.

The kernel's real-time throttle (`/proc/sys/kernel/sched_rt_runtime_us`, 950 ms
per second by default) is a backstop, not a licence: it stops a runaway task
locking the machine completely, but a machine at 95 % real-time occupancy is not
usable.

> **Note on the grant itself:** goxpyriment only uses `rtprio`. The `nice -20`
> and `memlock unlimited` lines in Step 2 are there because they are commonly
> wanted alongside it and cost nothing, not because anything here requires them.
> Granting `rtprio` alone is enough if you prefer the smaller privilege.

