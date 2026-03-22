// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package triggers

import (
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
//	Set HIGH  pin 1–8 : '1'–'8'
//	Set LOW   pin 1–8 : 'Q','W','E','R','T','Y','U','I'
//	Read      pin 1–8 : 'A','S','D','F','G','H','J','K'
//	Ping              : '\'' → device responds with 'Q'
//	Binary read mode  : '\\' → subsequent reads return 0x00 or 0x01

const dlpBaudRate = 115200

var (
	setHighCmd = [9]byte{0, '1', '2', '3', '4', '5', '6', '7', '8'}
	setLowCmd  = [9]byte{0, 'Q', 'W', 'E', 'R', 'T', 'Y', 'U', 'I'}
	readCmd    = [9]byte{0, 'A', 'S', 'D', 'F', 'G', 'H', 'J', 'K'}
)

// DLPIO8 controls a DLP-IO8 or DLP-IO8-G digital I/O device over USB-CDC
// serial. Construct with [NewDLPIO8] or [AutoDetectDLPIO8].
type DLPIO8 struct {
	port serial.Port
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

	d := &DLPIO8{port: p}
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
// found it returns a [NullTrigger] and logs a warning; callers do not need
// to nil-check the returned Trigger.
func AutoDetectDLPIO8() (Trigger, string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return NullTrigger{}, "", fmt.Errorf("dlpio8: enumerate ports: %w", err)
	}
	for _, name := range ports {
		d, err := NewDLPIO8(name)
		if err == nil {
			return d, name, nil
		}
	}
	log.Println("dlpio8: no DLP-IO8-G found — trigger output disabled")
	return NullTrigger{}, "", nil
}

// ping checks that the device responds to the '`'`' command with 'Q'.
func (d *DLPIO8) ping() (bool, error) {
	d.port.ResetInputBuffer()
	if _, err := d.port.Write([]byte("'")); err != nil {
		return false, err
	}
	buf := make([]byte, 1)
	// Retry short reads up to 3 times (USB latency can split the response).
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

// SetHigh drives pin HIGH. pin is 1-indexed (1–8).
func (d *DLPIO8) SetHigh(pin int) error {
	if pin < 1 || pin > 8 {
		return fmt.Errorf("dlpio8: pin %d out of range (1–8)", pin)
	}
	_, err := d.port.Write([]byte{setHighCmd[pin]})
	return err
}

// SetLow drives pin LOW. pin is 1-indexed (1–8).
func (d *DLPIO8) SetLow(pin int) error {
	if pin < 1 || pin > 8 {
		return fmt.Errorf("dlpio8: pin %d out of range (1–8)", pin)
	}
	_, err := d.port.Write([]byte{setLowCmd[pin]})
	return err
}

// Send sets all 8 output lines simultaneously from a bitmask.
// Bit 0 = pin 1, bit 7 = pin 8.
func (d *DLPIO8) Send(value byte) error {
	for pin := 1; pin <= 8; pin++ {
		var err error
		if value&(1<<uint(pin-1)) != 0 {
			err = d.SetHigh(pin)
		} else {
			err = d.SetLow(pin)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Pulse drives pin HIGH for durationMs milliseconds, then LOW.
func (d *DLPIO8) Pulse(pin int, durationMs int) error {
	return defaultPulse(d, pin, durationMs)
}

// ReadPin returns the current logical state of a single pin (0 or 1).
func (d *DLPIO8) ReadPin(pin int) (byte, error) {
	if pin < 1 || pin > 8 {
		return 0, fmt.Errorf("dlpio8: pin %d out of range (1–8)", pin)
	}
	d.port.ResetInputBuffer()
	if _, err := d.port.Write([]byte{readCmd[pin]}); err != nil {
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
	return 0, fmt.Errorf("dlpio8: ReadPin timeout on pin %d", pin)
}

// ReadAll returns the state of all 8 pins as a slice (index 0 = pin 1).
func (d *DLPIO8) ReadAll() ([]byte, error) {
	states := make([]byte, 8)
	for i := 1; i <= 8; i++ {
		v, err := d.ReadPin(i)
		if err != nil {
			return nil, err
		}
		states[i-1] = v
	}
	return states, nil
}

// AllLow sets all 8 output lines LOW.
func (d *DLPIO8) AllLow() error { return d.Send(0x00) }

// Close sets all pins LOW and closes the serial port.
func (d *DLPIO8) Close() error {
	_ = d.AllLow()
	return d.port.Close()
}
