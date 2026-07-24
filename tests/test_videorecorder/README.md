# test_videorecorder

Hand-run check of the `triggers.VideoRecorder` client against the BEL_video
networked video recorder (the labelled camera in the EEG room), over TCP/IP.

It walks a full session and prints each step:

1. connect
2. `Start` recording
3. `SetSubject` (`NIP:<id>` — names the saved file)
4. a train of `TRL:<n>` / `CND:<n>` overlay labels
5. `Stop`, then `Close` disconnects

There is **no automated pass/fail**. Watch the recorder's preview window: the
labels should appear burned into the video, and the saved AVI
(`BELv_<subject>_<datetime>.avi`) should contain them.

## Run

```bash
# from the repo root (go.work resolves the workspace)
go run ./tests/test_videorecorder -host 192.168.8.212

# options
go run ./tests/test_videorecorder -host 192.168.8.212 -port 55113 -subject bb0012025 -n 10 -isi 1000
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-host` | *(required)* | recorder host IP address |
| `-port` | `55113` | recorder TCP port |
| `-subject` | `test0001` | subject id (NIP), names the output file |
| `-n` | `5` | number of trial labels |
| `-isi` | `2000` | interval between trials (ms) |
