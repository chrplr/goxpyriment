// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package io

import (
	"fmt"
	"strings"

	"go.bug.st/serial"
)

// DLPIO8 represents a connection to a DLP-IO8-G device.
type DLPIO8 struct {
	port serial.Port
}

// NewDLPIO8 creates a new connection to a DLP-IO8-G device.
func NewDLPIO8(device string, baudrate int) (*DLPIO8, error) {
	mode := &serial.Mode{
		BaudRate: baudrate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(device, mode)
	if err != nil {
		return nil, fmt.Errorf("error while trying to open %s at %d bps: %w", device, baudrate, err)
	}

	// Ping to check the device
	buff := make([]byte, 1)
	_, err = port.Write([]byte("'"))
	if err != nil {
		port.Close()
		return nil, err
	}
	
	n, err := port.Read(buff)
	if err != nil || n != 1 || buff[0] != 'Q' {
		port.Close()
		return nil, fmt.Errorf("device not responding correctly to ping (check connection and baudrate)")
	}

	// Set BINARY mode for return values (sending '\')
	_, err = port.Write([]byte("\\"))
	if err != nil {
		port.Close()
		return nil, fmt.Errorf("problem setting binary mode on device %s: %w", device, err)
	}

	return &DLPIO8{port: port}, nil
}

// Close closes the connection to the device.
func (d *DLPIO8) Close() error {
	if d.port != nil {
		return d.port.Close()
	}
	return nil
}

// Ping checks if the device is still responding.
func (d *DLPIO8) Ping() (bool, error) {
	buff := make([]byte, 1)
	_, err := d.port.Write([]byte("'"))
	if err != nil {
		return false, err
	}
	n, err := d.port.Read(buff)
	if err != nil {
		return false, err
	}
	if n != 1 {
		return false, fmt.Errorf("no char returned")
	}
	return buff[0] == 'Q', nil
}

// Read returns the states (0 or 1) of all 8 lines.
func (d *DLPIO8) Read() ([]byte, error) {
	cmds := []byte("ASDFGHJK")
	buff := make([]byte, 8)

	d.port.ResetOutputBuffer()
	d.port.ResetInputBuffer()

	_, err := d.port.Write(cmds)
	if err != nil {
		return nil, err
	}

	n, err := d.port.Read(buff)
	if err != nil {
		return nil, err
	}

	return buff[:n], nil
}

// Set sets the specified lines (e.g., "123") to HIGH (1).
func (d *DLPIO8) Set(lines string) error {
	d.port.ResetOutputBuffer()
	_, err := d.port.Write([]byte(lines))
	return err
}

// Unset sets the specified lines (e.g., "123") to LOW (0).
func (d *DLPIO8) Unset(lines string) error {
	cmd := strings.ReplaceAll(lines, "1", "Q")
	cmd = strings.ReplaceAll(cmd, "2", "W")
	cmd = strings.ReplaceAll(cmd, "3", "E")
	cmd = strings.ReplaceAll(cmd, "4", "R")
	cmd = strings.ReplaceAll(cmd, "5", "T")
	cmd = strings.ReplaceAll(cmd, "6", "Y")
	cmd = strings.ReplaceAll(cmd, "7", "U")
	cmd = strings.ReplaceAll(cmd, "8", "I")

	d.port.ResetOutputBuffer()
	_, err := d.port.Write([]byte(cmd))
	return err
}

// SetAllLow sets all lines to LOW.
func (d *DLPIO8) SetAllLow() error {
	return d.Unset("12345678")
}
