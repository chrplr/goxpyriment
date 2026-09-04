// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package eyetracker

// The bridge protocol.
//
// One JSON object per line, UTF-8, newline-terminated, in both directions. Line
// framing rather than a length prefix so that the stream can be read by eye and
// replayed with netcat when something goes wrong at 9pm in a scanner room.
//
// # Client to bridge: requests
//
//	{"id":1,"cmd":"open","args":{"host":"100.1.1.1","edf":"tst.edf","width":1920,"height":1080}}
//	{"id":2,"cmd":"calibrate","args":{"points":9}}
//	{"id":3,"cmd":"start_recording"}
//	{"id":4,"cmd":"mark","args":{"text":"TRIALID 1"}}
//	{"id":5,"cmd":"stop_recording"}
//	{"id":6,"cmd":"tracker_time"}
//	{"id":7,"cmd":"receive_file","args":{"path":"/tmp/tst.edf"}}
//	{"id":8,"cmd":"close"}
//
// id is a positive integer chosen by the client and unique among outstanding
// requests. The bridge must echo it back. Commands may be pipelined; the bridge
// may answer out of order.
//
// # Client to bridge: stepwise calibration
//
// Optional, and only for a tracker whose SDK draws no calibration targets, so
// that the client must draw them itself — a Tobii. A bridge that does not
// support these rejects them by name, which is the answer an experiment driving
// the wrong tracker needs to see.
//
//	{"id":2,"cmd":"calibration_enter"}
//	{"id":3,"cmd":"calibration_collect","args":{"x":0.5,"y":0.5}}
//	{"id":4,"cmd":"calibration_discard","args":{"x":0.5,"y":0.5}}
//	{"id":5,"cmd":"calibration_compute"}
//	{"id":6,"cmd":"calibration_leave"}
//
// x and y are NORMALIZED on the tracker's active display area — (0,0) top-left,
// (1,1) bottom-right — not the tracker pixels that samples use, and not
// goxpyriment's centre-origin coordinates.
//
// calibration_collect blocks in the bridge while the tracker samples, up to
// about ten seconds, so the client sends it with no request timeout. The target
// must already be on screen and fixated when it is sent; that ordering is the
// whole point of driving calibration from the client, since it puts the target
// onset on the client's flip clock.
//
// calibration_compute answers with the tracker's own verdict and per-target
// counts, so that a target the participant never looked at can be named:
//
//	{"id":5,"ok":true,"result":{"status":"calibration_status_success",
//	 "points":[{"x":0.5,"y":0.5,"samples":30,"used":57}]}}
//
// These commands are additive: a bridge and a client that both speak protocol 1
// interoperate whether or not either knows about them, so they do not bump
// protoVersion.
//
// # Bridge to client: responses
//
//	{"id":1,"ok":true,"result":{...}}
//	{"id":1,"ok":false,"error":"could not connect to tracker at 100.1.1.1"}
//
// # Bridge to client: unsolicited events
//
// Distinguished from responses by carrying "ev" instead of "id".
//
//	{"ev":"hello","bridge":"eyelink","proto":1,"simulated":false,"caps":[]}
//	{"ev":"sample","t":1234567.0,"eye":"right","x":960.5,"y":540.25,"pa":1183.0}
//	{"ev":"fix_start","eye":"right","start":1234570.0,"sx":960.0,"sy":540.0}
//	{"ev":"fix_end","eye":"right","start":1234570.0,"end":1234810.0,"ax":961.2,"ay":539.8}
//	{"ev":"log","level":"warning","msg":"link data lost"}
//
// A sample with missing data (blink, lost track) carries "valid":false and
// OMITS x and y; the client marks any sample without both of them invalid. The
// EyeLink reports -32768 for missing coordinates, which is a plausible-looking
// number in the wrong hands, and a Tobii reports nan — which json.dumps writes
// as a bare NaN that Go's encoding/json rejects outright, so a bridge that
// forwarded it would drop the connection on the first blink.
//
// "pa" is pupil size in WHATEVER UNIT THE TRACKER USES: area in arbitrary units
// from an EyeLink, diameter in millimetres from a Tobii. See [Sample] — the
// bridge reports its unit at open, and it belongs in the data file.
//
// # Ordering and liveness
//
// The bridge sends "hello" first, before any response. The client uses it to
// verify the protocol version, to learn which optional commands the back end
// implements ("caps"), and to learn whether it is talking to real
// hardware or to a simulator — a distinction that must be visible in the data
// file, because a simulated run that is mistaken for a real one is worse than
// no run.
//
// Samples flow only between start_recording and stop_recording.

// protoVersion is the protocol version this client speaks. The bridge announces
// its own in the hello event; a mismatch is a hard error at Open rather than a
// puzzling failure three commands later.
const protoVersion = 1

// request is one command sent to the bridge.
type request struct {
	ID   int            `json:"id"`
	Cmd  string         `json:"cmd"`
	Args map[string]any `json:"args,omitempty"`
}

// response is the bridge's answer to one request.
type response struct {
	ID     int            `json:"id"`
	OK     bool           `json:"ok"`
	Error  string         `json:"error,omitempty"`
	Result map[string]any `json:"result,omitempty"`
}

// message is the union of everything the bridge can send. A line is a response
// when Ev is empty and an event otherwise; decoding into one struct avoids
// buffering the line and decoding it twice.
type message struct {
	// response fields
	ID     int            `json:"id"`
	OK     bool           `json:"ok"`
	Error  string         `json:"error,omitempty"`
	Result map[string]any `json:"result,omitempty"`

	// event discriminator
	Ev string `json:"ev,omitempty"`

	// hello
	Bridge    string `json:"bridge,omitempty"`
	Proto     int    `json:"proto,omitempty"`
	Simulated bool   `json:"simulated,omitempty"`
	// Caps lists the OPTIONAL commands the back end implements — currently
	// the stepwise-calibration group. A bridge older than this field sends
	// none at all, which is why the pointer matters: an absent list is "this
	// bridge cannot say", not "this bridge supports nothing".
	Caps *[]string `json:"caps,omitempty"`

	// sample
	T     float64  `json:"t,omitempty"`
	Eye   string   `json:"eye,omitempty"`
	X     *float64 `json:"x,omitempty"`
	Y     *float64 `json:"y,omitempty"`
	PA    float64  `json:"pa,omitempty"`
	Valid *bool    `json:"valid,omitempty"`

	// ocular events
	Start float64 `json:"start,omitempty"`
	End   float64 `json:"end,omitempty"`
	SX    float64 `json:"sx,omitempty"`
	SY    float64 `json:"sy,omitempty"`
	EX    float64 `json:"ex,omitempty"`
	EY    float64 `json:"ey,omitempty"`
	AX    float64 `json:"ax,omitempty"`
	AY    float64 `json:"ay,omitempty"`
	Ampl  float64 `json:"ampl,omitempty"`
	PVel  float64 `json:"pvel,omitempty"`

	// log
	Level string `json:"level,omitempty"`
	Msg   string `json:"msg,omitempty"`
}

// isEvent reports whether the message is an unsolicited event rather than a
// response to a request.
func (m *message) isEvent() bool { return m.Ev != "" }

// toSample converts a decoded sample event. now is the local timestamp to stamp
// it with.
func (m *message) toSample(now int64) Sample {
	s := Sample{
		TrackerMs: m.T,
		LocalNs:   now,
		Eye:       ParseEye(m.Eye),
		PupilArea: m.PA,
		Valid:     true,
	}
	if m.X != nil {
		s.X = *m.X
	}
	if m.Y != nil {
		s.Y = *m.Y
	}
	// Absent "valid" means valid: the common case should not need the field on
	// every one of a thousand samples per second. Missing coordinates are the
	// other way round — explicit, because silently passing -32768 through as a
	// gaze position is the failure this flag exists to prevent.
	if m.Valid != nil {
		s.Valid = *m.Valid
	}
	if m.X == nil || m.Y == nil {
		s.Valid = false
	}
	return s
}

// toEvent converts a decoded ocular event. now is the local timestamp.
func (m *message) toEvent(now int64) Event {
	return Event{
		Kind:         EventKind(m.Ev),
		Eye:          ParseEye(m.Eye),
		LocalNs:      now,
		StartMs:      m.Start,
		EndMs:        m.End,
		StartX:       m.SX,
		StartY:       m.SY,
		EndX:         m.EX,
		EndY:         m.EY,
		AvgX:         m.AX,
		AvgY:         m.AY,
		PupilArea:    m.PA,
		Amplitude:    m.Ampl,
		PeakVelocity: m.PVel,
	}
}

// isOcularEvent reports whether an event name is one of the parsed ocular
// events, as opposed to hello, sample or log.
func isOcularEvent(ev string) bool {
	switch EventKind(ev) {
	case FixationStart, FixationEnd, SaccadeStart, SaccadeEnd, BlinkStart, BlinkEnd:
		return true
	}
	return false
}
