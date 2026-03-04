package io

import (
	"fmt"
)

// SerialPort represents a connection to a serial device.
type SerialPort struct {
	PortName string
	BaudRate int
	// In a full implementation, we'd have an os.File or serial.Port here
}

func NewSerialPort(name string, baud int) *SerialPort {
	return &SerialPort{PortName: name, BaudRate: baud}
}

func (s *SerialPort) Send(data byte) error {
	fmt.Printf("Serial Send: %v to %s\n", data, s.PortName)
	return nil
}

// ParallelPort represents a connection to a parallel device (LPT).
type ParallelPort struct {
	Address uint16
}

func NewParallelPort(address uint16) *ParallelPort {
	return &ParallelPort{Address: address}
}

func (p *ParallelPort) SetData(data byte) error {
	fmt.Printf("Parallel Port SetData: %v at %x\n", data, p.Address)
	return nil
}
