## VideoRecorder (BEL_video, labelled camera over TCP/IP)

Does **not** implement either TTL interface. Client for the NeuroSpin EEG-room
**video recorder** ("BEL_video"): a camera PC films the participant, burns event
labels into the footage, and saves an AVI. This type starts/stops that recording
and pushes the labels — the capture/encoding stay on the camera PC. Same family
as `NetStation` (event-marker + recording-control over TCP), *not* an eye tracker.

```go
vr, err := triggers.NewVideoRecorder("192.168.8.212")  // default port 55113
if err != nil { log.Fatal(err) }
defer vr.Close()             // stops recording + disconnects

vr.Start()
vr.SetSubject("bb0012025")   // "NIP:<id>" — names the output file
vr.Label("TRL", "001")       // "KEY:VALUE" overlay until next label / timeout
vr.Label("CND", "007")
vr.Stop()
```

**Protocol:** plain ASCII, **no framing and no ack** (fire-and-forget, unlike
NetStation). `START` begins, `STOP` finalizes, any other message is a
`KEY:VALUE` overlay label (`NIP:<id>` names the file). Because the server reads
one socket buffer per captured frame, messages sent too close together can
arrive coalesced and be mis-parsed; `VideoRecorder` waits `SendGap` (default
50 ms) after each message (`WithVRSendGap` to tune, 0 to disable). Options:
`WithVRTimeout`, `WithVRSendGap`. Ported from videoComm.m / videoCommClient.py.

