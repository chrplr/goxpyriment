// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.bug.st/serial"
)

// DLP-IO20 binary command set (FTDI virtual COM port).
//
// FTDI, not USB-CDC: it appears as ttyUSB rather than ttyACM, and so is subject
// to the ftdi_sio latency timer, which CDC devices are not. That distinction
// matters — see the note on read round-trips in [DLPIO20].
//
// Unlike the DLP-IO8, which takes single ASCII bytes, the DLP-IO20 uses
// multi-byte packets whose first byte is the packet length *including itself*:
//
//	Ping        : 02 27                    → 'Y' (0x59)
//	Digital I/O : 05 35 <ch> <dir> <val>   → 1 byte, only when dir = input
//
// Direction is carried by every digital command, so a channel is re-configured
// on each call — there is no persistent direction register to set up at open.
// Byte 4 (the output value) must be present even in input mode, where it is
// ignored.
//
// Only the digital-I/O subset of the protocol is implemented here. The device
// also offers 14 analog inputs, two latching relays, two 32-bit event counters
// and DS18B20+ temperature sensing; see the datasheet if those are ever needed:
// https://www.dlpdesign.com/usb/dlp-io20-ds-v11.pdf

const (
	io20BaudRate            = 115200
	io20DefaultPollInterval = 5 * time.Millisecond
	io20DefaultReadTimeout  = 200 * time.Millisecond

	io20CmdPing      byte = 0x27
	io20CmdDigitalIO byte = 0x35
	io20PingReply    byte = 'Y'

	io20DirOutput byte = 0x00
	io20DirInput  byte = 0x01
	io20ValLow    byte = 0x00
	io20ValHigh   byte = 0x01

	io20LinesPerGroup = 8
)

// IO20Channel identifies one of the DLP-IO20's 20 channels, by the code the
// device's digital-I/O command expects.
type IO20Channel byte

// DLP-IO20 channel codes, in datasheet order.
//
// AN0–AN13, RA4, RB6 and RB7 are the 17 general-purpose digital I/O lines.
// P5–P7 are relay-driver outputs (Darlington pairs, 5 V coil) — they can be
// driven by the same command but are *not* TTL lines and cannot be read back,
// so they are a poor choice for triggers.
//
// Note the datasheet's inversion: RB7 is code 0x12 and RB6 is code 0x13.
const (
	IO20_AN0  IO20Channel = 0x00
	IO20_AN1  IO20Channel = 0x01
	IO20_AN2  IO20Channel = 0x02
	IO20_AN3  IO20Channel = 0x03
	IO20_AN4  IO20Channel = 0x04
	IO20_AN5  IO20Channel = 0x05
	IO20_AN6  IO20Channel = 0x06
	IO20_AN7  IO20Channel = 0x07
	IO20_AN8  IO20Channel = 0x08
	IO20_AN9  IO20Channel = 0x09
	IO20_AN10 IO20Channel = 0x0A
	IO20_AN11 IO20Channel = 0x0B
	IO20_AN12 IO20Channel = 0x0C
	IO20_AN13 IO20Channel = 0x0D
	IO20_RA4  IO20Channel = 0x0E
	IO20_P5   IO20Channel = 0x0F
	IO20_P6   IO20Channel = 0x10
	IO20_P7   IO20Channel = 0x11
	IO20_RB7  IO20Channel = 0x12
	IO20_RB6  IO20Channel = 0x13
)

// io20MaxChannel is the highest valid channel code.
const io20MaxChannel = IO20_RB6

var io20ChannelNames = [...]string{
	"AN0", "AN1", "AN2", "AN3", "AN4", "AN5", "AN6", "AN7",
	"AN8", "AN9", "AN10", "AN11", "AN12", "AN13",
	"RA4", "P5", "P6", "P7", "RB7", "RB6",
}

// String returns the terminal-block name of the channel, e.g. "AN12".
func (c IO20Channel) String() string {
	if int(c) < len(io20ChannelNames) {
		return io20ChannelNames[c]
	}
	return fmt.Sprintf("IO20Channel(0x%02X)", byte(c))
}

// Default line groups: AN0–AN7 drive triggers, AN8–AN13 + RB6/RB7 read responses.
var (
	io20DefaultOutputs = [io20LinesPerGroup]IO20Channel{
		IO20_AN0, IO20_AN1, IO20_AN2, IO20_AN3, IO20_AN4, IO20_AN5, IO20_AN6, IO20_AN7,
	}
	io20DefaultInputs = [io20LinesPerGroup]IO20Channel{
		IO20_AN8, IO20_AN9, IO20_AN10, IO20_AN11, IO20_AN12, IO20_AN13, IO20_RB6, IO20_RB7,
	}
)

// DLPIO20 controls a DLP-IO20 20-channel data-acquisition module over USB-CDC
// serial, using its digital I/O as TTL trigger outputs and response inputs.
// It implements both [OutputTTLDevice] and [InputTTLDevice].
// Construct with [NewDLPIO20] or [AutoDetectDLPIO20].
//
// The TTL interfaces are 8-bit, while the device has 17 usable digital
// channels. Interface lines 0–7 therefore address a *window* of 8 channels —
// AN0–AN7 for output and AN8–AN13 + RB6/RB7 for input by default, remappable
// with [WithIO20OutputChannels] and [WithIO20InputChannels]. Every channel,
// in or out of the window, is reachable through [DLPIO20.SetChannelHigh],
// [DLPIO20.SetChannelLow] and [DLPIO20.ReadChannel].
//
// Timing: each channel takes its own 5-byte packet, so [DLPIO20.Send] costs 8
// packets (~3.5 ms of wire time at 115200 baud) plus USB latency. For
// sub-millisecond trigger onsets, prefer [DLPIO20.SetHigh] on a single line —
// or a parallel port / LabJack. On Linux the FTDI latency timer (16 ms by
// default) dominates read round-trips; lower it with
//
//	echo 1 | sudo tee /sys/bus/usb-serial/devices/ttyUSBn/latency_timer
//
// Measured on a DLP-IO8 on the same driver: the round trip for a read tracks
// the timer exactly, 15.98 ms at the default of 16 and 1.01 ms at 1, so a
// polling loop runs at 63 Hz or 995 Hz depending on nothing but that setting.
type DLPIO20 struct {
	port         serial.Port
	outputs      [io20LinesPerGroup]IO20Channel
	inputs       [io20LinesPerGroup]IO20Channel
	pollInterval time.Duration
	readTimeout  time.Duration
}

// DLPIO20Option configures a [DLPIO20] at construction time.
type DLPIO20Option func(*dlpio20Config)

// dlpio20Config holds option values until [NewDLPIO20] can validate them.
type dlpio20Config struct {
	outputs      []IO20Channel
	inputs       []IO20Channel
	pollInterval time.Duration
	readTimeout  time.Duration
}

// WithIO20OutputChannels maps interface output lines 0–7 onto 8 specific
// channels. Default: AN0–AN7.
func WithIO20OutputChannels(chans ...IO20Channel) DLPIO20Option {
	return func(c *dlpio20Config) { c.outputs = append([]IO20Channel(nil), chans...) }
}

// WithIO20InputChannels maps interface input lines 0–7 onto 8 specific
// channels. Default: AN8–AN13, RB6, RB7.
func WithIO20InputChannels(chans ...IO20Channel) DLPIO20Option {
	return func(c *dlpio20Config) { c.inputs = append([]IO20Channel(nil), chans...) }
}

// WithIO20PollInterval sets the polling interval used by
// [DLPIO20.WaitForInput] and [DLPIO20.DrainInputs]. Default: 5 ms.
func WithIO20PollInterval(d time.Duration) DLPIO20Option {
	return func(c *dlpio20Config) { c.pollInterval = d }
}

// WithIO20ReadTimeout sets the serial read timeout. Default: 200 ms.
func WithIO20ReadTimeout(d time.Duration) DLPIO20Option {
	return func(c *dlpio20Config) { c.readTimeout = d }
}

// NewDLPIO20 opens the given serial port (e.g. "/dev/ttyUSB0"), pings the
// device to confirm a DLP-IO20 is present, and drives every output line LOW.
// Returns an error if the device does not answer the ping with 'Y'.
func NewDLPIO20(device string, opts ...DLPIO20Option) (*DLPIO20, error) {
	cfg := dlpio20Config{
		pollInterval: io20DefaultPollInterval,
		readTimeout:  io20DefaultReadTimeout,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	d := &DLPIO20{
		outputs:      io20DefaultOutputs,
		inputs:       io20DefaultInputs,
		pollInterval: cfg.pollInterval,
		readTimeout:  cfg.readTimeout,
	}
	if cfg.outputs != nil {
		g, err := io20Group("output", cfg.outputs)
		if err != nil {
			return nil, err
		}
		d.outputs = g
	}
	if cfg.inputs != nil {
		g, err := io20Group("input", cfg.inputs)
		if err != nil {
			return nil, err
		}
		d.inputs = g
	}
	if err := io20CheckOverlap(d.outputs, d.inputs); err != nil {
		return nil, err
	}

	mode := &serial.Mode{
		BaudRate: io20BaudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	p, err := serial.Open(device, mode)
	if err != nil {
		return nil, fmt.Errorf("dlpio20: open %s: %w", device, err)
	}
	// Unchecked, a failure here leaves reads blocking forever, so an absent
	// device hangs the experiment instead of returning an error.
	if err := p.SetReadTimeout(d.readTimeout); err != nil {
		p.Close()
		return nil, fmt.Errorf("dlpio20: set read timeout on %s: %w", device, err)
	}
	d.port = p

	if ok, err := d.ping(); err != nil || !ok {
		p.Close()
		if err != nil {
			return nil, fmt.Errorf("dlpio20: ping %s: %w", device, err)
		}
		return nil, fmt.Errorf("dlpio20: no DLP-IO20 found on %s", device)
	}
	if err := d.AllLow(); err != nil {
		p.Close()
		return nil, fmt.Errorf("dlpio20: init outputs on %s: %w", device, err)
	}
	return d, nil
}

// io20Group validates an 8-channel line group supplied through an option.
func io20Group(name string, chans []IO20Channel) ([io20LinesPerGroup]IO20Channel, error) {
	var g [io20LinesPerGroup]IO20Channel
	if len(chans) != io20LinesPerGroup {
		return g, fmt.Errorf("dlpio20: %s group needs exactly %d channels, got %d",
			name, io20LinesPerGroup, len(chans))
	}
	seen := map[IO20Channel]bool{}
	for i, c := range chans {
		if c > io20MaxChannel {
			return g, fmt.Errorf("dlpio20: %s line %d: invalid channel 0x%02X", name, i, byte(c))
		}
		if seen[c] {
			return g, fmt.Errorf("dlpio20: %s group lists %s twice", name, c)
		}
		seen[c] = true
		g[i] = c
	}
	return g, nil
}

// io20CheckOverlap rejects a channel used as both an output and an input.
func io20CheckOverlap(outputs, inputs [io20LinesPerGroup]IO20Channel) error {
	in := map[IO20Channel]bool{}
	for _, c := range inputs {
		in[c] = true
	}
	for i, c := range outputs {
		if in[c] {
			return fmt.Errorf("dlpio20: channel %s is mapped as both output line %d and an input", c, i)
		}
	}
	return nil
}

// AutoDetectDLPIO20 looks for a DLP-IO20 and returns it with the port name it
// was found on. If none is found it returns a [NullOutputTTLDevice] and logs a
// warning; callers do not need to nil-check the returned [OutputTTLDevice].
//
// A DLP-IO8 on another port is not mistaken for a DLP-IO20: it answers the
// ping with 'Q' rather than 'Y'.
//
// It does not probe every serial port. Opening one is not free — a USB-CDC port
// resets the device behind it, so a blind sweep reboots any Arduino on the
// machine, and an instrument with its own command grammar can be left
// mid-stream by unsolicited probe bytes. See [dlpPortCandidates] for how the
// search is narrowed on each platform.
func AutoDetectDLPIO20(opts ...DLPIO20Option) (OutputTTLDevice, string, error) {
	ports, err := dlpPortCandidates()
	if err != nil {
		return NullOutputTTLDevice{}, "", fmt.Errorf("dlpio20: enumerate ports: %w", err)
	}
	for _, name := range ports {
		d, err := NewDLPIO20(name, opts...)
		if err == nil {
			return d, name, nil
		}
	}
	log.Println("dlpio20: no DLP-IO20 found — trigger output disabled")
	return NullOutputTTLDevice{}, "", nil
}

// writePacket sends a command packet, prepending the length byte the device
// expects (the length counts itself).
func (d *DLPIO20) writePacket(payload ...byte) error {
	pkt := append([]byte{byte(len(payload) + 1)}, payload...)
	_, err := d.port.Write(pkt)
	return err
}

// readByte reads a single response byte, retrying across read timeouts.
func (d *DLPIO20) readByte() (byte, error) {
	buf := make([]byte, 1)
	for i := 0; i < 3; i++ {
		n, err := d.port.Read(buf)
		if n == 1 {
			return buf[0], nil
		}
		if err != nil {
			return 0, err
		}
	}
	return 0, fmt.Errorf("timeout waiting for response")
}

// ping checks that the device answers the ping command with 'Y'.
//
// It retries: a previous probe that wrote a malformed packet can leave the
// device waiting for the rest of it, in which case the first ping is swallowed
// as padding and only a later one is parsed.
func (d *DLPIO20) ping() (bool, error) {
	for attempt := 0; attempt < 3; attempt++ {
		d.port.ResetInputBuffer()
		if err := d.writePacket(io20CmdPing); err != nil {
			return false, fmt.Errorf("dlpio20.ping: %w", err)
		}
		// A read timeout or a wrong byte both just mean "not a DLP-IO20 (yet)";
		// only a write failure is worth reporting as an error.
		if b, err := d.readByte(); err == nil && b == io20PingReply {
			return true, nil
		}
	}
	return false, nil
}

// --- Channel-level access (all 20 channels, beyond the 8-line windows) ---

// SetChannel drives any channel HIGH or LOW, whether or not it is part of the
// output window.
func (d *DLPIO20) SetChannel(ch IO20Channel, high bool) error {
	if ch > io20MaxChannel {
		return fmt.Errorf("dlpio20: invalid channel 0x%02X", byte(ch))
	}
	val := io20ValLow
	if high {
		val = io20ValHigh
	}
	if err := d.writePacket(io20CmdDigitalIO, byte(ch), io20DirOutput, val); err != nil {
		return fmt.Errorf("dlpio20: set %s: %w", ch, err)
	}
	return nil
}

// SetChannelHigh drives any channel HIGH.
func (d *DLPIO20) SetChannelHigh(ch IO20Channel) error { return d.SetChannel(ch, true) }

// SetChannelLow drives any channel LOW.
func (d *DLPIO20) SetChannelLow(ch IO20Channel) error { return d.SetChannel(ch, false) }

// ReadChannel switches any channel to input mode and returns its state (0 or 1).
//
// The relay-driver outputs P5–P7 cannot be read back and are rejected.
func (d *DLPIO20) ReadChannel(ch IO20Channel) (byte, error) {
	if ch > io20MaxChannel {
		return 0, fmt.Errorf("dlpio20: invalid channel 0x%02X", byte(ch))
	}
	if ch >= IO20_P5 && ch <= IO20_P7 {
		return 0, fmt.Errorf("dlpio20: %s is a relay-driver output and cannot be read", ch)
	}
	d.port.ResetInputBuffer()
	if err := d.writePacket(io20CmdDigitalIO, byte(ch), io20DirInput, io20ValLow); err != nil {
		return 0, fmt.Errorf("dlpio20: read %s: %w", ch, err)
	}
	b, err := d.readByte()
	if err != nil {
		return 0, fmt.Errorf("dlpio20: read %s: %w", ch, err)
	}
	return b & 0x01, nil
}

// OutputChannels returns the 8 channels currently mapped to output lines 0–7.
func (d *DLPIO20) OutputChannels() [io20LinesPerGroup]IO20Channel { return d.outputs }

// InputChannels returns the 8 channels currently mapped to input lines 0–7.
func (d *DLPIO20) InputChannels() [io20LinesPerGroup]IO20Channel { return d.inputs }

// --- OutputTTLDevice ---

// SetHigh drives output line 0–7 HIGH. Implements [OutputTTLDevice].
func (d *DLPIO20) SetHigh(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("dlpio20: line %d out of range (0–7)", line)
	}
	return d.SetChannel(d.outputs[line], true)
}

// SetLow drives output line 0–7 LOW. Implements [OutputTTLDevice].
func (d *DLPIO20) SetLow(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("dlpio20: line %d out of range (0–7)", line)
	}
	return d.SetChannel(d.outputs[line], false)
}

// Send sets all 8 output lines from a bitmask. Bit N drives line N.
//
// The device has no write-all command, so this issues 8 packets of 5 bytes in
// line order — 40 bytes, about 3.5 ms of wire time at 115200 — and the lines
// change across that whole interval rather than together.
//
// Against a system sampling at 1 kHz, 3.5 ms is three and a half sample
// periods, so a code change is not merely at risk of being caught
// mid-transition: it is CERTAIN to be, across several consecutive samples. If a
// multi-bit code has to be unambiguous, have the receiving system latch on a
// separate strobe raised once the code has settled, or use one line per event
// type — a single packet, no skew, and eight distinguishable events.
//
// For comparison, the DLP-IO8's single-byte commands settle in ~610 µs, which
// is measured (86.2 µs per byte, n=99); the 3.5 ms here is arithmetic from the
// packet size and has not been measured on hardware.
// Implements [OutputTTLDevice].
func (d *DLPIO20) Send(mask byte) error {
	for line := 0; line < io20LinesPerGroup; line++ {
		if err := d.SetChannel(d.outputs[line], mask&(1<<uint(line)) != 0); err != nil {
			return fmt.Errorf("dlpio20.Send: %w", err)
		}
	}
	return nil
}

// Pulse drives line HIGH for dur, then LOW. Implements [OutputTTLDevice].
//
// The device has no pulse timer, so the width is the interval between two host
// writes and inherits host scheduling in full. Measured on the closely related
// DLP-IO8 (n=50 per width): within ~20 µs of the request on an idle host, but a
// median error of +1.85 ms with 4.75 ms of spread under CPU load, returning to
// ~0.1 ms of spread when the process runs at real-time priority. See
// docs/SettingPriorityUnderLinux.md.
func (d *DLPIO20) Pulse(line int, dur time.Duration) error {
	return defaultPulse(d, line, dur)
}

// AllLow drives all 8 output lines LOW. Implements [OutputTTLDevice].
func (d *DLPIO20) AllLow() error { return d.Send(0x00) }

// Close drives all output lines LOW and closes the serial port.
func (d *DLPIO20) Close() error {
	_ = d.AllLow()
	return d.port.Close()
}

// --- InputTTLDevice ---

// ReadLine returns the state (0 or 1) of input line 0–7.
// Implements [InputTTLDevice].
func (d *DLPIO20) ReadLine(line int) (byte, error) {
	if line < 0 || line > 7 {
		return 0, fmt.Errorf("dlpio20: line %d out of range (0–7)", line)
	}
	v, err := d.ReadChannel(d.inputs[line])
	if err != nil {
		return 0, fmt.Errorf("dlpio20.ReadLine: %w", err)
	}
	return v, nil
}

// ReadAll returns the state of all 8 input lines as a bitmask. Bit N reflects
// line N. Each channel needs its own round trip, so this is 8 of them.
// Implements [InputTTLDevice].
func (d *DLPIO20) ReadAll() (byte, error) {
	var mask byte
	for line := 0; line < io20LinesPerGroup; line++ {
		v, err := d.ReadLine(line)
		if err != nil {
			return 0, fmt.Errorf("dlpio20.ReadAll: %w", err)
		}
		if v != 0 {
			mask |= 1 << uint(line)
		}
	}
	return mask, nil
}

// WaitForInput blocks until any input line becomes active or ctx is cancelled.
// Returns the active-line bitmask and the elapsed reaction time.
// Implements [InputTTLDevice].
func (d *DLPIO20) WaitForInput(ctx context.Context) (byte, time.Duration, error) {
	return pollWaitForInput(ctx, "dlpio20", d.ReadAll, d.pollInterval)
}

// DrainInputs polls until all input lines are inactive or ctx is cancelled.
// Implements [InputTTLDevice].
func (d *DLPIO20) DrainInputs(ctx context.Context) error {
	return pollDrainInputs(ctx, "dlpio20", d.ReadAll, d.pollInterval)
}
