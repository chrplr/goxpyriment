// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// VideoRecorder controls a networked behavioral video recorder over TCP/IP —
// the "BEL_video" system used in the NeuroSpin EEG room. A camera PC films the
// participant, burns event labels (subject id, trial, condition, …) into the
// footage as an overlay, and saves an AVI; this type is the *client* that
// starts/stops that recording and pushes the labels. The video capture and
// encoding stay on the camera PC.
//
// Like [NetStation] and [SerialPort] it implements neither TTL interface — it
// is an event-marker/recording-control client for an external recorder, not an
// electrical trigger line.
//
// Session:
//
//	vr, err := triggers.NewVideoRecorder("192.168.8.212") // default port 55113
//	if err != nil { log.Fatal(err) }
//	defer vr.Close()               // stops recording + disconnects
//
//	vr.Start()
//	vr.SetSubject("bb0012025")     // names the output file
//	vr.Label("TRL", "001")         // overlaid until the next label or timeout
//	vr.Label("CND", "007")
//	vr.Stop()
//
// Protocol: plain ASCII messages, no framing and no acknowledgement (unlike
// NetStation). "START" begins, "STOP" finalizes, and any other message is a
// "KEY:VALUE" overlay label ("NIP:<id>" is special — it names the file). Ported
// from videoComm.m / videoCommClient.py.
//
// Because the server has no message framing (it reads one socket buffer per
// captured frame), two messages sent too close together can arrive coalesced
// and be mis-parsed. VideoRecorder therefore waits [SendGap] after each message
// (default 50 ms, matching the reference client); see [WithVRSendGap].
type VideoRecorder struct {
	conn      net.Conn
	host      string
	timeout   time.Duration
	sendGap   time.Duration
	recording bool
}

const (
	vrDefaultPort    = 55113
	vrDefaultTimeout = 2 * time.Second
	vrDefaultSendGap = 50 * time.Millisecond
	vrMsgStart       = "START"
	vrMsgStop        = "STOP"
	vrSubjectKey     = "NIP"
)

// VideoRecorderOption configures a [VideoRecorder] at construction time.
type VideoRecorderOption func(*VideoRecorder)

// WithVRTimeout sets the TCP dial timeout. Default: 2 s.
func WithVRTimeout(d time.Duration) VideoRecorderOption {
	return func(vr *VideoRecorder) { vr.timeout = d }
}

// WithVRSendGap sets the pause inserted after every message to keep the
// unframed server from coalescing consecutive messages into one read. Default:
// 50 ms. Lowering it risks mis-parsed labels; 0 disables the pause entirely.
func WithVRSendGap(d time.Duration) VideoRecorderOption {
	return func(vr *VideoRecorder) { vr.sendGap = d }
}

// NewVideoRecorder opens a TCP connection to the recorder host. host may be
// "192.168.8.212" (port 55113 assumed) or "192.168.8.212:55113". The connection
// is opened but recording is not started; call [VideoRecorder.Start].
func NewVideoRecorder(host string, opts ...VideoRecorderOption) (*VideoRecorder, error) {
	vr := &VideoRecorder{
		timeout: vrDefaultTimeout,
		sendGap: vrDefaultSendGap,
	}
	for _, opt := range opts {
		opt(vr)
	}
	if !strings.Contains(host, ":") {
		host = fmt.Sprintf("%s:%d", host, vrDefaultPort)
	}
	vr.host = host

	conn, err := net.DialTimeout("tcp", host, vr.timeout)
	if err != nil {
		return nil, fmt.Errorf("videorecorder: connect %s: %w", host, err)
	}
	vr.conn = conn
	return vr, nil
}

// Start begins recording. No-op if already recording.
func (vr *VideoRecorder) Start() error {
	if vr.recording {
		return nil
	}
	if err := vr.send(vrMsgStart); err != nil {
		return fmt.Errorf("videorecorder: Start: %w", err)
	}
	vr.recording = true
	return nil
}

// SetSubject sends the participant identifier ("NIP:<id>"), which the recorder
// uses to name the saved file. Send it after [VideoRecorder.Start]. The id must
// not contain ':' or a newline.
func (vr *VideoRecorder) SetSubject(id string) error {
	return vr.Label(vrSubjectKey, id)
}

// Label sends a "KEY:VALUE" overlay label that the recorder burns into the
// video until the next label or its display timeout. key must be non-empty and
// contain no ':' or newline; value must contain no newline (a ':' inside value
// is allowed — only the first ':' separates key from value).
func (vr *VideoRecorder) Label(key, value string) error {
	if key == "" {
		return fmt.Errorf("videorecorder: Label: empty key")
	}
	if strings.ContainsAny(key, ":\r\n") {
		return fmt.Errorf("videorecorder: Label: key %q must not contain ':' or newline", key)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("videorecorder: Label: value %q must not contain a newline", value)
	}
	if err := vr.send(key + ":" + value); err != nil {
		return fmt.Errorf("videorecorder: Label %q: %w", key, err)
	}
	return nil
}

// Stop finalizes the recording. No-op if not recording.
func (vr *VideoRecorder) Stop() error {
	if !vr.recording {
		return nil
	}
	if err := vr.send(vrMsgStop); err != nil {
		return fmt.Errorf("videorecorder: Stop: %w", err)
	}
	vr.recording = false
	return nil
}

// Recording reports whether recording is currently active (per the last
// successful Start/Stop call).
func (vr *VideoRecorder) Recording() bool { return vr.recording }

// Close stops recording (if active) and closes the TCP connection. Safe to call
// multiple times.
func (vr *VideoRecorder) Close() error {
	if vr.conn == nil {
		return nil
	}
	if vr.recording {
		_ = vr.send(vrMsgStop)
		vr.recording = false
	}
	err := vr.conn.Close()
	vr.conn = nil
	return err
}

// send writes one ASCII message and pauses for sendGap so the unframed server
// does not coalesce it with the next one. The protocol is fire-and-forget: the
// server sends no acknowledgement, so only the write error is surfaced.
func (vr *VideoRecorder) send(msg string) error {
	if vr.conn == nil {
		return fmt.Errorf("host not connected")
	}
	if _, err := vr.conn.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if vr.sendGap > 0 {
		time.Sleep(vr.sendGap)
	}
	return nil
}
