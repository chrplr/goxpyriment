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
they are worth being able to tell apart after the fact. The last one is this
setup not being in place; the middle one is it being in place but the program not
having been started under `chrt`. Without the line in the data you would be left
comparing timing distributions and guessing which had happened.

---

### ⚠️ A Friendly "Warning"
Since you are giving your code the ability to run at `-20` niceness (the highest possible priority), a "busy loop" in your Go code (like `for { }` without a sleep) could potentially **freeze your entire desktop**. 

Because your code is now more "important" than the mouse driver or the window manager, the OS won't interrupt your code to let you click "Stop." Always keep a terminal open with a `top` or `htop` window ready, or test your logic with lower priority first!

