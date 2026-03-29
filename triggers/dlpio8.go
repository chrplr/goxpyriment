// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package triggers

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.bug.st/serial"
)

// DLP-IO8 / DLP-IO8-G ASCII command set (USB-CDC / virtual COM port).
//
// The device communicates at 115200 baud over a USB-to-serial interface.
// All commands are single ASCII bytes:
//
//	Set HIGH  line 0–7 : '1'–'8'
//	Set LOW   line 0–7 : 'Q','W','E','R','T','Y','U','I'
//	Read      line 0–7 : 'A','S','D','F','G','H','J','K'
//	Ping              : '\'' → device responds with 'Q'
//	Binary read mode  : '\\' → subsequent reads return 0x00 or 0x01

const (
	dlpBaudRate            = 115200
	dlpDefaultPollInterval = 5 * time.Millisecond
)

// Internal command tables (1-indexed: index 1 = line 0, index 8 = line 7).
var (
	setHighCmd = [9]byte{0, '1', '2', '3', '4', '5', '6', '7', '8'}
	setLowCmd  = [9]byte{0, 'Q', 'W', 'E', 'R', 'T', 'Y', 'U', 'I'}
	readCmd    = [9]byte{0, 'A', 'S', 'D', 'F', 'G', 'H', 'J', 'K'}
)

// DLPIO8 controls a DLP-IO8 or DLP-IO8-G digital I/O device over USB-CDC
// serial. It implements both [OutputTTLDevice] and [InputTTLDevice].
// Construct with [NewDLPIO8] or [AutoDetectDLPIO8].
type DLPIO8 struct {
	port         serial.Port
	pollInterval time.Duration
}

// NewDLPIO8 opens the given serial port (e.g. "/dev/ttyUSB0"), pings the
// device, and enables binary-mode reads. Returns an error if the device does
// not respond to the ping.
func NewDLPIO8(device string) (*DLPIO8, error) {
	mode := &serial.Mode{
		BaudRate: dlpBaudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	p, err := serial.Open(device, mode)
	if err != nil {
		return nil, fmt.Errorf("dlpio8: open %s: %w", device, err)
	}
	p.SetReadTimeout(200 * time.Millisecond)

	d := &DLPIO8{port: p, pollInterval: dlpDefaultPollInterval}
	if ok, err := d.ping(); err != nil || !ok {
		p.Close()
		if err != nil {
			return nil, fmt.Errorf("dlpio8: ping %s: %w", device, err)
		}
		return nil, fmt.Errorf("dlpio8: no DLP-IO8 found on %s", device)
	}
	// Enable binary mode: subsequent reads return 0x00/0x01.
	if _, err := p.Write([]byte("\\")); err != nil {
		p.Close()
		return nil, fmt.Errorf("dlpio8: set binary mode on %s: %w", device, err)
	}
	return d, nil
}

// AutoDetectDLPIO8 scans all available serial ports for a DLP-IO8-G. On
// success it returns the device and the matched port name. If no device is
// found it returns a [NullOutputTTLDevice] and logs a warning; callers do not
// need to nil-check the returned [OutputTTLDevice].
func AutoDetectDLPIO8() (OutputTTLDevice, string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return NullOutputTTLDevice{}, "", fmt.Errorf("dlpio8: enumerate ports: %w", err)
	}
	for _, name := range ports {
		d, err := NewDLPIO8(name)
		if err == nil {
			return d, name, nil
		}
	}
	log.Println("dlpio8: no DLP-IO8-G found — trigger output disabled")
	return NullOutputTTLDevice{}, "", nil
}

// ping checks that the device responds to the ping command with 'Q'.
func (d *DLPIO8) ping() (bool, error) {
	d.port.ResetInputBuffer()
	if _, err := d.port.Write([]byte("'")); err != nil {
		return false, err
	}
	buf := make([]byte, 1)
	for i := 0; i < 3; i++ {
		n, err := d.port.Read(buf)
		if n == 1 {
			return buf[0] == 'Q', nil
		}
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

// SetHigh drives line HIGH. line is 0-indexed (0–7). Implements [OutputTTLDevice].
func (d *DLPIO8) SetHigh(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("dlpio8: line %d out of range (0–7)", line)
	}
	_, err := d.port.Write([]byte{setHighCmd[line+1]})
	return err
}

// SetLow drives line LOW. line is 0-indexed (0–7). Implements [OutputTTLDevice].
func (d *DLPIO8) SetLow(line int) error {
	if line < 0 || line > 7 {
		return fmt.Errorf("dlpio8: line %d out of range (0–7)", line)
	}
	_, err := d.port.Write([]byte{setLowCmd[line+1]})
	return err
}

// Send sets all 8 output lines simultaneously from a bitmask.
// Bit N drives line N. Implements [OutputTTLDevice].
func (d *DLPIO8) Send(mask byte) error {
	for line := 0; line < 8; line++ {
		var err error
		if mask&(1<<uint(line)) != 0 {
			err = d.SetHigh(line)
		} else {
			err = d.SetLow(line)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Pulse drives line HIGH for dur, then LOW. Implements [OutputTTLDevice].
func (d *DLPIO8) Pulse(line int, dur time.Duration) error {
	return defaultPulse(d, line, dur)
}

// AllLow sets all 8 output lines LOW. Implements [OutputTTLDevice].
func (d *DLPIO8) AllLow() error { return d.Send(0x00) }

// ReadLine returns the state (0 or 1) of a single input line (0-indexed).
// Implements [InputTTLDevice].
func (d *DLPIO8) ReadLine(line int) (byte, error) {
	if line < 0 || line > 7 {
		return 0, fmt.Errorf("dlpio8: line %d out of range (0–7)", line)
	}
	d.port.ResetInputBuffer()
	if _, err := d.port.Write([]byte{readCmd[line+1]}); err != nil {
		return 0, err
	}
	buf := make([]byte, 1)
	for i := 0; i < 3; i++ {
		n, err := d.port.Read(buf)
		if n == 1 {
			return buf[0] & 0x01, nil
		}
		if err != nil {
			return 0, err
		}
	}
	return 0, fmt.Errorf("dlpio8: ReadLine timeout on line %d", line)
}

// ReadAll returns the current state of all 8 input lines as a bitmask.
// Bit N reflects line N. Implements [InputTTLDevice].
func (d *DLPIO8) ReadAll() (byte, error) {
	var mask byte
	for line := 0; line < 8; line++ {
		v, err := d.ReadLine(line)
		if err != nil {
			return 0, err
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
func (d *DLPIO8) WaitForInput(ctx context.Context) (byte, time.Duration, error) {
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return 0, time.Since(start), err
		}
		mask, err := d.ReadAll()
		if err != nil {
			return 0, time.Since(start), err
		}
		if mask != 0 {
			return mask, time.Since(start), nil
		}
		time.Sleep(d.pollInterval)
	}
}

// DrainInputs polls until all input lines are inactive or ctx is cancelled.
// Implements [InputTTLDevice].
func (d *DLPIO8) DrainInputs(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		mask, err := d.ReadAll()
		if err != nil {
			return err
		}
		if mask == 0 {
			return nil
		}
		time.Sleep(d.pollInterval)
	}
}

// Close sets all lines LOW and closes the serial port.
func (d *DLPIO8) Close() error {
	_ = d.AllLow()
	return d.port.Close()
}
