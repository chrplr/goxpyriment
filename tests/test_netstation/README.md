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
