## NetStation (EGI EEG, ECI over TCP/IP)

Does **not** implement either TTL interface. It marks *named events* in the EEG
stream of an EGI/NetStation host and controls recording remotely, over TCP —
the network equivalent of sending a trigger code, but with 4-char event codes,
durations, and key/value payloads instead of 8 electrical lines.

```go
ns, err := triggers.NewNetStation("134.225.198.12")  // default port 55513
if err != nil { log.Fatal(err) }
defer ns.Close()                 // stops recording + disconnects (blocks ~2 s to flush)

ns.Synchronize()                 // align host clock to ours (call once after connect)
ns.StartRecording()
ns.SendEvent("STIM")             // now, 1 ms, no keys — send near the VSYNC flip
ns.SendEventFull(triggers.Event{ // full form
    Code:     "RESP",
    Start:    flipTime,          // zero = now
    Duration: 2 * time.Millisecond,
    Keys:     []triggers.EventKey{{Code: "corr", Value: 1}},
})
ns.StopRecording()
```

**Protocol (ECI):** on connect the driver advertises the `QNTEL`
(Intel/little-endian) variant and every multi-byte field is encoded
little-endian, so it is portable regardless of the CPU it runs on (do **not**
switch to a native byte order). The handshake is `QNTEL` → `I`+version byte,
then an `A` (attention), matching the NetStation 5-tested reference client.
Commands: `A`+`T`(int32 ms) = synchronize, `B` = start, `E` = stop,
`D`+block = event, `X` = end session. Event block layout: `uint16` size
(`15+12·keys`), `int32` start ms, `int32` duration ms, 4-byte code, `uint8`
label length (0), `uint8` description length (0), `uint8` key count, then per
key: 4-byte code + `"shor"` + `int16` length (2) + `int16` value. Timestamps
are ms from an epoch fixed at connect. Ported from Gergely Csibra's NetStation
MATLAB routines (2006), cross-checked against
[egi-pynetstation](https://github.com/nimh-sfim/egi-pynetstation) (tested on
NetStation 5.3). Options: `WithNSTimeout`.

**Acknowledgements — every command is checked.** The host answers each command
with one byte:

| Byte | Meaning |
|---|---|
| `Z`, `0x01`, `S` | accepted |
| `F` | command refused |
| `R` | no recording device ready (no session open / no amplifier) |

Only `Z` is documented; the rest come from the reference clients' testing. The
driver used to read the ack and **discard** it, so a `B` the host had refused
still reported success and the run continued to an unusable file. `checkAck`
now turns `F`/`R`/unknown into errors.

**The .mff file is not ours to control.** ECI has nine commands and none of
them names the output file, selects a format or finalizes it — NetStation
Acquisition writes the bundle. What the client *can* do wrong is leave a
recording open: an .mff containing `Acquiring.xml` and no `info.xml` (events
and signal both unreadable) is EGI's documented signature of an acquisition
that never completed. Hence:

- `Close` sends `E` before `X` when a recording is still open, and **returns**
  any failure instead of discarding it — always check or log it.
- Callers must not `log.Fatal` between `StartRecording` and `Close`:
  `log.Fatal` skips deferred functions, so the host keeps acquiring. Use the
  `run() error` pattern in `tests/test_netstation/main.go`, which also traps
  Ctrl-C.

Protocol behaviour is covered by `triggers/netstation_test.go`, which drives
the client against an in-process fake ECI host — no amplifier needed.

