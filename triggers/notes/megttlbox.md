## MEGTTLBox (NeuroSpin Arduino Mega)

Implements both interfaces. Binary opcode protocol at 115200 baud.

```go
box, err := triggers.NewMEGTTLBox("/dev/ttyACM0",
    triggers.WithResetDelay(2*time.Second),    // DTR → Arduino reset (default 2 s)
    triggers.WithPollInterval(5*time.Millisecond),
)
defer box.Close()

box.Pulse(0, 5*time.Millisecond)
box.PulseMask(0b00000011, 5*time.Millisecond) // lines 0 and 1
box.Send(0b00000001)                           // persistent set (not a pulse)

_ = box.DrainInputs(ctx)
mask, rt, _ := box.WaitForInput(ctx)
buttons := triggers.DecodeMask(mask)           // []FORPButton
```

**Firmware identification.** `NewMEGTTLBox` probes opcode 1 (`get_info`) once at
open and caches the answer; read it back with `box.Info()`:

```go
info := box.Info()
fmt.Println(info)                                   // "firmware v1, caps 0x00"
if info.Has(triggers.MEGCapTimestamps) { /* … */ }  // feature-detect
```

Firmware predating the opcode ignores it *silently* (the sketch's `default:`
branch), so legacy boards are detected by the probe **timing out** — there is no
positive signal. That is reported as `MEGInfo.Legacy`, not as an error. A reply
that arrives without the `MTB` magic **is** an error: something is on the port
that is not a TTL box. Only opcodes 10–16 and 20 may be used against a legacy
box. Capability bits (`MEGCapAtomicPort`, `MEGCapTimestamps`) are declared on
both sides; firmware v1 claims `MEGCapAtomicPort` (opcode 17) and not
`MEGCapTimestamps`. Set a bit in the sketch only when the matching opcode
actually exists — old firmware ignores unknown opcodes *silently*, so a host
that assumes a capability gets no error, just a command that did nothing.

**Wire protocol (opcodes):**

| Opcode | Args | Description |
|--------|------|-------------|
| 1 | — | get_info → `'M','T','B'`, version uint8, caps uint8 |
| 10 | uint16 LE (ms) | set trigger pulse width |
| 11 | uint8 mask | pulse all set lines |
| 12 | uint8 line | pulse single line |
| 13 | uint8 mask | set lines HIGH (persistent) |
| 14 | uint8 mask | set lines LOW (persistent) |
| 15 | uint8 line | set single line HIGH |
| 16 | uint8 line | set single line LOW |
| 17 | uint8 mask | assign all 8 output lines atomically (needs `CAP_ATOMIC_PORT`) |
| 20 | — | read button mask → returns uint8 |
| 21 | — | get_event → `[flags u8][mask u8][micros u32 LE]`, always 6 bytes |
| 22 | — | get_micros → uint32 LE, for clock alignment |
| 23 | — | clear queued events and re-seed the change detector |
| 24 | uint16 LE (µs) | set debounce, 0 disables |

Opcodes 21–24 need `CAP_TIMESTAMPS`. `get_event` replies with a fixed 6 bytes
even when the queue is empty, so the host never has to guess the reply length;
`flags` bit 0 means an event follows, bit 1 means events were dropped.

**`Send` is atomic on firmware ≥ v1 with `CAP_ATOMIC_PORT`.** On the Mega 2560
the output pins D30–D37 are **PORTC7–PORTC0** and the inputs D22–D29 are
**PORTA0–PORTA7**, so each bank is one register. Opcode 17 assigns the whole
output port in a single instruction: no intermediate value ever reaches the pins,
so a code cannot be latched half-written. `readButtons` likewise became one
`PINA` read, sampling all 8 buttons at the same instant rather than smearing them
over ~40 µs of sequential `digitalRead`s. Dropping `digitalWrite`/`digitalRead`
also shrank the sketch from 3360 to 2786 bytes.

**Note the reversal**: PORTC is wired backwards relative to line order (D30 =
PC7 … D37 = PC0), so the firmware's `reverseBits` mirrors the mask. Getting that
wrong mirrors every trigger code, and *nothing in software would show it* — the
commands are accepted either way. `static_assert`s in the sketch fail the build
if `OUT_PINS`/`IN_PINS` are renumbered without updating the mapping.

**Verified on hardware** (2026-08-05, bench Mega 2560 R3):

- *Bit order.* `Send(0x01)` put D30 at 5 V and D37 at 0 V (multimeter).
- *All 16 lines.* `-loopback` passed 8/8 patterns. The asymmetric ones carry the
  proof: `0x0F → 0xF0`, `0x01 → 0xFE`, `0x80 → 0x7F` — a mirrored mapping would
  have returned `0x0F → 0x0F`.
- *Atomicity, internal witness.* `-atomic 30` drove 0x00 → 0xFF thirty times and
  the firmware, which samples those same pins every few µs, reported **exactly one
  event every time**. The two-command fallback would have been caught
  mid-transition. Reproduce with `go run ./tests/test_megttlbox -atomic 30` and
  the full loopback wired.
- *Atomicity, external witness.* The above uses the firmware to judge itself,
  which is circular. A BBTKv3 watching D30 and D31 while one command pulsed both
  recorded **zero skew on all 20 trials** — skew under 250 µs, measured by an
  instrument with no stake in the answer. Reproduce with
  `tests/test_megttlbox/run-bbtk.sh` (block D); details in the device repo's
  `TIMING.md`.

Legacy firmware without the capability falls back to two commands — `13`
(set-high) then `14` (set-low) — written in a **single** `Write` so both ride the
same USB transfer. Issued as separate writes they could land in different USB
frames, leaving the port at `previous | mask` for up to a frame, which is a
valid-looking but wrong code. Sharing one transfer narrows that window to
firmware parse time but does not close it; reflashing does.

**Pulse width is timed on the device**, unlike the DLP boxes where
`defaultPulse` sleeps on the host and absorbs OS scheduling jitter. The firmware
sets `g_pulse_end = millis() + width` and drops the line from its main loop, so
the width is quantised to `millis()` resolution. `Pulse` also sleeps host-side
for `dur` to honour the blocking contract, concurrently with the device's own
timing.

Measured on a BBTKv3 (2026-08-05): pulses come out **~0.5–0.7 ms short**, never
long by more than a sample. That is `millis()` truncation as predicted — the
realised width is uniform on [w−1, w]. It is a systematic **bias, not noise**, so
a paradigm needing a true 5 ms pulse should request 6 ms.

Full measurement tables, the host→device latency figures, and what remains
unmeasured live with the device, not here:
**[neurospin-meg-ttl-box/TIMING.md](https://github.com/neurospin/neurospin-meg-ttl-box/blob/main/TIMING.md)**
(raw captures in that repo's `measurements/`). They characterise the hardware and
firmware, of which this package is only one client — keep them in one place so
the numbers cannot drift.

**Reaction times: use the event API, not `WaitForInput`.** `WaitForInput` polls
`ReadAll` every 5 ms and reports *elapsed host time*, so its resolution is the
poll interval plus USB jitter — it can only ever say "the press had happened by
the time I asked". Firmware advertising `MEGCapTimestamps` instead samples PINA
every loop iteration (a few µs) and records `micros()` at the transition; the
host drains those events later and the timestamp is unaffected by when it got
round to asking.

```go
box.SetDebounce(0)                     // default; use >0 for mechanical buttons
box.DrainEvents()                      // between trials, instead of DrainInputs
ev, err := box.WaitForPressTS(ctx)     // ev.TS is a host-clock instant
rt := ev.TS.Sub(stimulusOnset)         // subtract your own onset timestamp
if box.EventsDropped() { /* queue overflowed: treat the trial as suspect */ }
```

`PollEvent` is the non-blocking form; it returns releases as well as presses
(`ev.Pressed()` distinguishes them). Resolution floor is `micros()`, which ticks
in 4 µs steps at 16 MHz.

**Clock alignment.** `ev.TS` is the device's `micros()` translated to the host
clock. `NewMEGTTLBox` calls `SyncClock` at open; it brackets the device's reading
between two host readings across 7 round trips and keeps the tightest. Device
timestamps are decoded as a **signed 32-bit µs delta** from the sync point, which
makes the ~71.6 min `micros()` wrap self-correcting as long as readings stay
within ±35.8 min of a sync — `megResyncAfter` re-syncs every 20 min to guarantee
it. Call `SyncClock` explicitly only after stepping the host clock.

**Measured (2026-08-05, bench Mega 2560 R3, jumper D30→D22, 50 trials).**
`(firmware timestamp of the edge) − (host clock immediately before the write)`:
**min 802 µs, median 1.44 ms, max 2.05 ms**. That ~1.25 ms spread is one USB
full-speed frame (1 ms) plus the ~174 µs two command bytes take on the
16u2↔2560 UART — it is *host→device* latency, not timestamp error.

What this does and does not buy you. The firmware's reading is taken within a
loop iteration of the edge, so the *event* is timestamped to a few µs. Turning
that into a host instant costs the accuracy of the clock offset, which is
bounded by the asymmetry of the sync round trip — a few hundred µs, not 4 µs. So
RT accuracy is sub-millisecond rather than microsecond, against the 5 ms-plus
floor of `WaitForInput`. Reproduce with
`go run ./tests/test_megttlbox -rtloop 50`.

**Queue overflow is not silent.** The firmware holds 32 events (160 bytes); if
the host does not drain them it sets a sticky flag, surfaced by `EventsDropped()`
(read-and-clear). That means presses were *lost*, not delayed, so the trial
should be treated as suspect. In practice it takes a mechanical button
chattering — hence `SetDebounce`, which is **off by default** because fibre-optic
pads do not bounce and suppressing real transitions is worse than reporting extra
ones.

**Inputs are `INPUT_PULLUP`** and reported with `invert=true`: a line pulled LOW
reads as 1 ("pressed"), an unconnected line reads 0. Unlike the DLP-IO20, an
unwired box cannot generate spurious `WaitForInput` triggers. For a jumpered
loopback (D30→D22 …) this means `ReadAll() == ^mask` after `Send(mask)`.


## FORPButton

```go
// Each constant is the 0-indexed line number = bit position in the bitmask.
triggers.FORPLeftBlue    // 0, D22, STI007
triggers.FORPLeftYellow  // 1, D23, STI008
triggers.FORPLeftGreen   // 2, D24, STI009
triggers.FORPLeftRed     // 3, D25, STI010
triggers.FORPRightBlue   // 4, D26, STI012
triggers.FORPRightYellow // 5, D27, STI013
triggers.FORPRightGreen  // 6, D28, STI014
triggers.FORPRightRed    // 7, D29, STI015

buttons := triggers.DecodeMask(mask)  // []FORPButton, ordered low→high bit
fmt.Println(buttons[0])               // "left blue"
```

