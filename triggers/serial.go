// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package triggers

import (
	"bufio"
	"fmt"
	"strings"

	"go.bug.st/serial"
)

// SerialPort represents a connection to a generic serial device such as an
// Arduino, a response box, or any UART-based instrument.
type SerialPort struct {
	PortName string
	BaudRate int
	port     serial.Port
	reader   *bufio.Reader
}

// NewSerialPort creates a SerialPort. Call [SerialPort.Open] before use.
func NewSerialPort(name string, baud int) *SerialPort {
	return &SerialPort{PortName: name, BaudRate: baud}
}

// Open opens the serial port with the configured settings.
func (s *SerialPort) Open() error {
	mode := &serial.Mode{
		BaudRate: s.BaudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	p, err := serial.Open(s.PortName, mode)
	if err != nil {
		return err
	}
	s.port = p
	s.reader = bufio.NewReader(p)
	return nil
}

// Close closes the serial port.
func (s *SerialPort) Close() error {
	if s.port != nil {
		err := s.port.Close()
		s.port = nil
		s.reader = nil
		return err
	}
	return nil
}

// Send sends a single byte through the port.
func (s *SerialPort) Send(data byte) error {
	if s.port == nil {
		return fmt.Errorf("serial port not open")
	}
	_, err := s.port.Write([]byte{data})
	return err
}

// SendLine sends a string with optional CR and/or LF termination.
func (s *SerialPort) SendLine(data string, cr, lf bool) error {
	if s.port == nil {
		return fmt.Errorf("serial port not open")
	}
	out := data
	if cr {
		out += "\r"
	}
	if lf {
		out += "\n"
	}
	_, err := s.port.Write([]byte(out))
	return err
}

// Poll attempts to read one byte without blocking. Returns 0 if nothing is
// immediately available.
func (s *SerialPort) Poll() (byte, error) {
	if s.port == nil {
		return 0, fmt.Errorf("serial port not open")
	}
	buf := make([]byte, 1)
	n, err := s.port.Read(buf)
	if n > 0 {
		return buf[0], nil
	}
	return 0, err
}

// ReadLine reads bytes until a newline and returns the trimmed line.
func (s *SerialPort) ReadLine() (string, error) {
	if s.reader == nil {
		return "", fmt.Errorf("serial port not open")
	}
	line, err := s.reader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

// Clear flushes the input buffer.
func (s *SerialPort) Clear() error {
	if s.port == nil {
		return fmt.Errorf("serial port not open")
	}
	return s.port.ResetInputBuffer()
}

// AvailablePorts returns the serial port names visible to the OS
// (e.g. /dev/ttyUSB0, /dev/ttyACM0, COM3).
func AvailablePorts() ([]string, error) {
	return serial.GetPortsList()
}
