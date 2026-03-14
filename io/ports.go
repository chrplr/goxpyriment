// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package io

import (
	"bufio"
	"fmt"
	"runtime"
	"strings"

	"go.bug.st/serial"
)

// SerialPort represents a connection to a serial device.
type SerialPort struct {
	PortName string
	BaudRate int
	port     serial.Port
	reader   *bufio.Reader
}

// NewSerialPort creates a new SerialPort instance.
func NewSerialPort(name string, baud int) *SerialPort {
	return &SerialPort{
		PortName: name,
		BaudRate: baud,
	}
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

// Send sends a single byte of data through the serial port.
func (s *SerialPort) Send(data byte) error {
	if s.port == nil {
		return fmt.Errorf("serial port not open")
	}
	_, err := s.port.Write([]byte{data})
	return err
}

// SendLine sends a string followed by an optional carriage return and/or line feed.
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

// Poll polls the serial port for a single byte of data.
// Returns the byte read, or 0 if no data is available.
func (s *SerialPort) Poll() (byte, error) {
	if s.port == nil {
		return 0, fmt.Errorf("serial port not open")
	}
	
	// Check if there's data in the OS buffer
	// (Note: go.bug.st/serial doesn't have a direct "available" method in all cases,
	// but we can set a very short timeout or use non-blocking read if supported).
	
	// For now, we'll try a non-blocking read approach by checking if we can read 1 byte.
	// This might need refinement depending on the OS.
	buf := make([]byte, 1)
	n, err := s.port.Read(buf)
	if n > 0 {
		return buf[0], nil
	}
	return 0, err
}

// ReadInput reads all currently available input from the serial port.
func (s *SerialPort) ReadInput() ([]byte, error) {
	if s.port == nil {
		return nil, fmt.Errorf("serial port not open")
	}
	
	var result []byte
	buf := make([]byte, 128)
	for {
		n, err := s.port.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if n < len(buf) || err != nil {
			break
		}
	}
	return result, nil
}

// ReadLine reads a line from the serial port (until a newline character).
func (s *SerialPort) ReadLine() (string, error) {
	if s.reader == nil {
		return "", fmt.Errorf("serial port not open or reader not initialized")
	}
	line, err := s.reader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

// Clear clears the serial port input buffer.
func (s *SerialPort) Clear() error {
	if s.port == nil {
		return fmt.Errorf("serial port not open")
	}
	return s.port.ResetInputBuffer()
}

// GetAvailablePorts returns a list of available serial port names (e.g. /dev/ttyUSB0, COM3).
func GetAvailablePorts() ([]string, error) {
	return serial.GetPortsList()
}

// ParallelPort represents a connection to a parallel device (LPT).
// Note: Actual hardware access to parallel ports is highly platform-dependent.
// On Linux, this typically uses /dev/parportN.
// On Windows, it often requires a third-party driver like InpOut32.
type ParallelPort struct {
	Address uintptr // Base address (e.g., 0x378)
	Device  string  // Device path (e.g., "/dev/parport0" on Linux)
	
	// internal state for platform-specific handles
	handle interface{}
}

// NewParallelPort creates a new ParallelPort instance.
func NewParallelPort(address uintptr, device string) *ParallelPort {
	return &ParallelPort{
		Address: address,
		Device:  device,
	}
}

// Initialize attempts to open the parallel port device.
func (p *ParallelPort) Initialize() error {
	if runtime.GOOS == "linux" && p.Device != "" {
		// In a real implementation, we would open /dev/parportN and use ioctl
		// For now, we'll simulate the interface.
		fmt.Printf("Initializing Parallel Port on %s (stub)\n", p.Device)
		return nil
	}
	if p.Address != 0 {
		fmt.Printf("Initializing Parallel Port at 0x%x (stub)\n", p.Address)
		return nil
	}
	return fmt.Errorf("no parallel port device or address specified")
}

// Send sends data (8 bits) through the data lines of the parallel port.
func (p *ParallelPort) Send(data byte) error {
	// Stub implementation
	fmt.Printf("Parallel Port Send: %d (0x%x) to address 0x%x\n", data, data, p.Address)
	return nil
}

// Poll polls the parallel port status lines.
// Returns the state of the status lines (Acknowledge, Paper-Out, Selected).
// Following Expyriment's convention, this returns 3 bits of data.
func (p *ParallelPort) Poll() (byte, error) {
	// Stub implementation
	return 0, nil
}

// Clear clears the parallel port (sets data lines to 0).
func (p *ParallelPort) Clear() error {
	return p.Send(0)
}

// GetAvailableParallelPorts returns a list of potentially available parallel ports.
func GetAvailableParallelPorts() []string {
	if runtime.GOOS == "linux" {
		// Check for /dev/parport*
		// This is a simplified check
		return []string{"/dev/parport0", "/dev/parport1"}
	}
	if runtime.GOOS == "windows" {
		return []string{"LPT1", "LPT2"}
	}
	return nil
}

// IsParallelPort reports whether the given name looks like a parallel port (e.g. LPT1, /dev/parport0).
func IsParallelPort(name string) bool {
	n := strings.ToLower(name)
	return strings.HasPrefix(n, "lpt") || strings.HasPrefix(n, "/dev/parport")
}
