# test_netstation

Hand-run check of the `triggers.NetStation` driver against a real EGI /
NetStation EEG host, using the ECI protocol over TCP/IP.

It walks a full session and prints each step:

1. connect + ECI handshake
2. `Synchronize` (align the host clock)
3. `StartRecording`
4. `SendEvent("STIM")` — plain event (now, 1 ms, no keys)
5. `SendEventFull("RESP", …)` — explicit onset + key/value payloads
6. a train of numbered `T<n>` events
7. `StopRecording`, then `Close` disconnects the ECI session

There is **no automated pass/fail**. Watch the NetStation host: recording should
start, the events should land on the timeline in order, and it should stop
cleanly.

Whatever happens — an error mid-run, or Ctrl-C — the recording is stopped and
the session ended before the program exits. That matters: a recording left open
produces an `.mff` that cannot be reopened (`Acquiring.xml` present, `info.xml`
missing, events and signal both unreadable).

## If the .mff will not open

The ECI protocol has no command that names the output file, chooses its format
or finalizes it — NetStation Acquisition writes the bundle, so the file's format
version is a property of the host, not of this program. If a recording is
unreadable, check, in order:

1. **Is `info.xml` missing and `Acquiring.xml` present?** (An `.mff` is a
   directory; on macOS use *Show Package Contents*.) That means the recording
   was never finalized. EGI's File Validator (Net Station 5.4, under
   Applications ▸ EGI ▸ Utilities) removes `Acquiring.xml` and repairs it.
2. **Did the run report an error?** Since the acknowledgement fix, a host that
   refuses to start or stop recording produces a visible error. A clean run that
   prints `closed cleanly` did send `E` then `X`.
3. **Was the bundle copied between machines?** An `.mff` is a directory. Copying
   it with a tool that treats it as a single file can strip its contents.
4. **Is the reader current?** MFF support in EEGLAB (`mffmatlabio`) and
   MNE-Python has changed across Net Station versions; an old reader can fail on
   a perfectly valid recent file.

## Testing without an amplifier

`triggers/netstation_test.go` drives the client against an in-process fake ECI
host, covering the handshake, refused acknowledgements, the event datagram
layout and the teardown order:

```bash
go test ./triggers/ -run NetStation -v
```

## Run

```bash
# from the repo root (go.work resolves the workspace)
go run ./tests/test_netstation -host 134.225.198.12

# options
go run ./tests/test_netstation -host 134.225.198.12 -port 55513 -n 20 -isi 250
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-host` | *(required)* | NetStation host IP address |
| `-port` | `55513` | ECI TCP port |
| `-n` | `10` | number of numbered `T<n>` events |
| `-isi` | `500` | inter-event interval (ms) |
