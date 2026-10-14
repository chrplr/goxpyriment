// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package triggers

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Naming a TTL output device in one string, so a program can take it from a
// single command-line flag (-ttl, -device) and open it without a switch of its
// own. Before this existed every example carried a copy of that switch, and
// the copies had drifted: different device sets, different syntaxes, and one
// that quietly substituted a null device when auto-detection found nothing.

// DeviceSpecUsage documents the device-spec syntax, for flag help text.
const DeviceSpecUsage = `Device spec: KIND[:PORT][,key=value,...]

  dlpio8    [port]  DLP-IO8-G; auto-detected when the port is omitted or "auto"
  dlpio20   [port]  DLP-IO20; auto-detected when the port is omitted or "auto"
  megttlbox port    NeuroSpin MEG TTL box (Arduino Mega), e.g. megttlbox:/dev/ttyACM0
  mmbts     port    NEUROSPEC MMBT-S, e.g. mmbts:/dev/ttyACM0[,mode=p|s]
  parallel  [port]  parallel port; the first accessible one when omitted
  gpio      [chip]  Linux GPIO character device, [,pins=17+27+22+5+6+13+19+26]
  ft232h            Adafruit FT232H (Linux)
  labjackt4 host    LabJack T4 over Modbus TCP, e.g. labjackt4:192.168.1.100
  null              no hardware: writes go nowhere (for testing a program)

The value right after the colon is the device's port (its host for labjackt4,
its chip for gpio); it can also be written port=..., host=..., chip=....`

// DeviceKinds lists the accepted device kinds, in the order they are documented.
var DeviceKinds = []string{"dlpio8", "dlpio20", "megttlbox", "mmbts", "parallel", "gpio", "ft232h", "labjackt4", "null"}

// kindAliases maps the names some programs used before this parser existed.
var kindAliases = map[string]string{"parport": "parallel", "megttl": "megttlbox"}

// deviceParams is the set of parameter keys each kind accepts, beyond "pin",
// which every kind accepts (see DeviceSpec). The first key is the one the short
// form KIND:VALUE sets. An unlisted key is an error rather than a silent no-op:
// "pins=" on a dlpio8 is a typo, and ignoring it would drive a line the
// operator did not ask for.
var deviceParams = map[string][]string{
	"dlpio8":    {"port"},
	"dlpio20":   {"port"},
	"megttlbox": {"port"},
	"mmbts":     {"port", "mode"},
	"parallel":  {"port"},
	"gpio":      {"chip", "pins"},
	"ft232h":    {},
	"labjackt4": {"host"},
	"null":      {},
}

// DefaultGPIOPins are the BCM lines a gpio device drives when pins= is not
// given. They match the Timing-Tests default, so a command line moved from one
// program to another drives the same header pins.
var DefaultGPIOPins = [8]int{17, 27, 22, 5, 6, 13, 19, 26}

// DeviceSpec is a parsed device string: which kind of device, and how to reach
// it. It does not touch hardware; OpenDeviceSpec does.
//
// Params may contain "pin" (1-8, validated by ParseDeviceSpec). Opening ignores
// it — a device is opened whole — but programs that pulse a single line read
// it from here.
type DeviceSpec struct {
	Kind   string            // one of DeviceKinds
	Params map[string]string // validated keys only; never nil
	Raw    string            // the string as given, for messages and data files
}

// ParseDeviceSpec parses KIND[:VALUE][,key=value,...] or KIND[:key=value,...].
// It validates the kind, the parameter names, the pin range and the
// parameters a kind requires; it does not touch hardware.
func ParseDeviceSpec(s string) (DeviceSpec, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return DeviceSpec{}, fmt.Errorf("empty device spec; %s", kindList())
	}

	kind, rest, _ := strings.Cut(raw, ":")
	kind = strings.ToLower(strings.TrimSpace(kind))
	if alias, ok := kindAliases[kind]; ok {
		kind = alias
	}
	allowed, known := deviceParams[kind]
	if !known {
		return DeviceSpec{}, fmt.Errorf("unknown device kind %q; %s", kind, kindList())
	}

	spec := DeviceSpec{Kind: kind, Params: map[string]string{}, Raw: raw}
	for i, field := range strings.Split(rest, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			// The short form: a bare value in first position is the kind's
			// primary parameter — dlpio8:/dev/ttyUSB0, labjackt4:10.0.0.2.
			if i != 0 || len(allowed) == 0 {
				return DeviceSpec{}, fmt.Errorf("device %q: %q is not key=value", raw, field)
			}
			key, value = allowed[0], field
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if _, dup := spec.Params[key]; dup {
			return DeviceSpec{}, fmt.Errorf("device %q: %s given twice", raw, key)
		}
		if key == "pin" {
			n, err := strconv.Atoi(value)
			if err != nil {
				return DeviceSpec{}, fmt.Errorf("device %q: pin=%q is not a number", raw, value)
			}
			if n < 1 || n > 8 {
				return DeviceSpec{}, fmt.Errorf("device %q: pin=%d is out of range; every device exposes 8 lines, numbered 1 to 8", raw, n)
			}
			spec.Params["pin"] = value
			continue
		}
		if !slices.Contains(allowed, key) {
			return DeviceSpec{}, fmt.Errorf("device %q: %s takes no %q parameter (accepts %s)",
				raw, kind, key, paramList(allowed))
		}
		// "auto" means what an omitted port means: probe for the device.
		if key == "port" && strings.EqualFold(value, "auto") {
			value = ""
		}
		spec.Params[key] = value
	}

	// Required parameters. Auto-detection exists for the DLP boxes and the
	// parallel port; the others have nothing to probe for — the MMBT-S never
	// replies on its serial line, so it cannot even be recognised once opened.
	switch kind {
	case "megttlbox":
		if spec.Params["port"] == "" {
			return DeviceSpec{}, fmt.Errorf("device %q: megttlbox needs port=, e.g. megttlbox:/dev/ttyACM0", raw)
		}
	case "mmbts":
		if spec.Params["port"] == "" {
			return DeviceSpec{}, fmt.Errorf("device %q: mmbts needs port=, e.g. mmbts:/dev/ttyACM0", raw)
		}
		if _, err := ParseMMBTSMode(spec.Params["mode"]); err != nil {
			return DeviceSpec{}, fmt.Errorf("device %q: %w", raw, err)
		}
	case "labjackt4":
		if spec.Params["host"] == "" {
			return DeviceSpec{}, fmt.Errorf("device %q: labjackt4 needs host=, e.g. labjackt4:192.168.1.100", raw)
		}
	case "gpio":
		if pins, ok := spec.Params["pins"]; ok {
			if _, err := ParseGPIOPins(pins); err != nil {
				return DeviceSpec{}, fmt.Errorf("device %q: pins=%q: %w", raw, pins, err)
			}
		}
	}
	return spec, nil
}

// ParseGPIOPins parses a '+'-joined list of exactly 8 GPIO line numbers.
// ',' separates parameters in a device spec, so it cannot separate the list.
// Duplicates are rejected: two identical entries make two lines one, which is
// invisible in a trace and reads as a wiring fault.
func ParseGPIOPins(s string) ([8]int, error) {
	parts := strings.Split(s, "+")
	if len(parts) != 8 {
		return [8]int{}, fmt.Errorf("expected 8 pin numbers joined with '+', got %d", len(parts))
	}
	var pins [8]int
	seen := make(map[int]int, 8)
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return [8]int{}, fmt.Errorf("pin %d: %w", i+1, err)
		}
		if n < 0 {
			return [8]int{}, fmt.Errorf("pin %d: %d is negative", i+1, n)
		}
		if first, dup := seen[n]; dup {
			return [8]int{}, fmt.Errorf("pin %d repeats line %d, already given as pin %d", i+1, n, first)
		}
		seen[n] = i + 1
		pins[i] = n
	}
	return pins, nil
}

// ParseMMBTSMode maps an mmbts mode= value onto the runtime mode set by the
// box's P/S switch. Empty means the factory setting, Pulse.
func ParseMMBTSMode(s string) (MMBTSMode, error) {
	switch strings.ToLower(s) {
	case "", "p", "pulse":
		return MMBTSPulseMode, nil
	case "s", "simple":
		return MMBTSSimpleMode, nil
	}
	return 0, fmt.Errorf("mmbts mode=%q: choose p (pulse, the factory setting) or s (simple)", s)
}

// OpenedDevice is a device opened from a DeviceSpec.
type OpenedDevice struct {
	Device OutputTTLDevice
	Spec   DeviceSpec
	// Where is what was actually opened: the port found by auto-detection,
	// the parallel port chosen, the host or GPIO chip. Empty for ft232h and
	// null, which have no address.
	Where string
	// GPIOPins are the lines driven, for kind "gpio" only.
	GPIOPins [8]int
	// Note is a choice the opener made that the operator should know about,
	// e.g. which of several parallel ports it took. Empty when there is none.
	Note string
}

// Description is one line naming the device, for logs and data-file headers.
func (o OpenedDevice) Description() string {
	d := o.Spec.Kind
	if o.Where != "" {
		d += " " + o.Where
	}
	switch dev := o.Device.(type) {
	case *MEGTTLBox:
		d += fmt.Sprintf(" (firmware %s)", dev.Info())
	case *MMBTS:
		d += fmt.Sprintf(" (mode %s)", dev.Mode())
	}
	if o.Spec.Kind == "gpio" {
		d += fmt.Sprintf(" pins=%v", o.GPIOPins)
	}
	if o.Spec.Kind == "null" {
		d += " (NO HARDWARE)"
	}
	if o.Note != "" {
		d += "; " + o.Note
	}
	return d
}

// Open parses and opens a device spec in one call. An empty string means no
// device and returns (nil, "", nil); anything else either opens a device or
// fails. It never substitutes a null device for one that would not open: a
// recording that asked for triggers must not start without them.
func Open(s string) (OutputTTLDevice, string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, "", nil
	}
	spec, err := ParseDeviceSpec(s)
	if err != nil {
		return nil, "", err
	}
	o, err := OpenDeviceSpec(spec)
	if err != nil {
		return nil, "", err
	}
	return o.Device, o.Description(), nil
}

// OpenDeviceSpec opens the device a spec names. An error is final: callers
// should exit rather than substitute a NullOutputTTLDevice.
func OpenDeviceSpec(spec DeviceSpec) (OpenedDevice, error) {
	o := OpenedDevice{Spec: spec}
	var err error
	switch spec.Kind {
	case "dlpio8":
		o.Device, o.Where, err = openDLPIO8Spec(spec.Params["port"])
	case "dlpio20":
		o.Device, o.Where, err = openDLPIO20Spec(spec.Params["port"])
	case "megttlbox":
		o.Where = spec.Params["port"]
		var box *MEGTTLBox
		if box, err = NewMEGTTLBox(o.Where); err != nil {
			err = fmt.Errorf("MEG TTL box on %s: %w", o.Where, err)
		} else {
			o.Device = box
		}
	case "mmbts":
		o.Device, o.Where, err = openMMBTSSpec(spec)
	case "parallel":
		o.Device, o.Where, o.Note, err = openParallelSpec(spec.Params["port"])
	case "gpio":
		o.Device, o.Where, o.GPIOPins, err = openGPIOSpec(spec)
	case "ft232h":
		// NewFT232H rather than AutoDetectFT232H: the latter turns a failure
		// into a NullOutputTTLDevice and keeps the reason to itself, and the
		// reason is usually actionable.
		var dev *FT232HTrigger
		if dev, err = NewFT232H(); err != nil {
			err = fmt.Errorf("FT232H: %w "+
				"(Linux only; ftdi_sio must not hold the device and /dev/bus/usb/... must be "+
				"readable and writable: sudo rmmod ftdi_sio, then a udev rule or the plugdev group)", err)
		} else {
			o.Device = dev
		}
	case "labjackt4":
		o.Where = spec.Params["host"]
		// The output group is left at the driver's default, DIO4-DIO11: the
		// T4's DIO0-DIO3 are analog inputs and cannot be driven.
		var dev *LabJackT4
		if dev, err = NewLabJackT4(o.Where); err != nil {
			err = fmt.Errorf("LabJack T4 at %s: %w (the T4 must be reachable on Modbus TCP port 502)", o.Where, err)
		} else {
			o.Device = dev
		}
	case "null":
		o.Device = NullOutputTTLDevice{}
	default:
		err = fmt.Errorf("unknown device kind %q; %s", spec.Kind, kindList())
	}
	if err != nil {
		return OpenedDevice{}, err
	}
	return o, nil
}

func openDLPIO8Spec(port string) (OutputTTLDevice, string, error) {
	if port != "" {
		d, err := NewDLPIO8(port)
		if err != nil {
			return nil, "", fmt.Errorf("DLP-IO8 on %s: %w", port, err)
		}
		return d, port, nil
	}
	d, found, err := AutoDetectDLPIO8()
	if err != nil {
		return nil, "", fmt.Errorf("DLP-IO8 auto-detect: %w", err)
	}
	if found == "" {
		return nil, "", fmt.Errorf("no DLP-IO8-G found on any serial port; name one with dlpio8:/dev/ttyUSBn")
	}
	return d, found, nil
}

func openDLPIO20Spec(port string) (OutputTTLDevice, string, error) {
	if port != "" {
		d, err := NewDLPIO20(port)
		if err != nil {
			return nil, "", fmt.Errorf("DLP-IO20 on %s: %w", port, err)
		}
		return d, port, nil
	}
	d, found, err := AutoDetectDLPIO20()
	if err != nil {
		return nil, "", fmt.Errorf("DLP-IO20 auto-detect: %w", err)
	}
	if found == "" {
		return nil, "", fmt.Errorf("no DLP-IO20 found on any serial port; name one with dlpio20:/dev/ttyUSBn")
	}
	return d, found, nil
}

func openMMBTSSpec(spec DeviceSpec) (OutputTTLDevice, string, error) {
	port := spec.Params["port"]
	mode, err := ParseMMBTSMode(spec.Params["mode"])
	if err != nil {
		return nil, "", err
	}
	box, err := NewMMBTS(port, WithMMBTSMode(mode))
	if err != nil {
		return nil, "", fmt.Errorf("MMBT-S on %s: %w "+
			"(rw access to the port is needed: sudo usermod -aG dialout $USER, then log in again)", port, err)
	}
	return box, port, nil
}

func openParallelSpec(device string) (OutputTTLDevice, string, string, error) {
	note := ""
	if device == "" {
		ports := AvailableParallelPorts()
		if len(ports) == 0 {
			return nil, "", "", fmt.Errorf("no accessible parallel port found " +
				"(needs Linux with ppdev loaded and rw access: sudo modprobe ppdev; " +
				"sudo usermod -aG lp $USER, then log in again. If dmesg says " +
				"'lp0: using parport0', also unload the lp printer module: sudo rmmod lp)")
		}
		// A machine with two LPT ports must not fire whichever enumerated
		// first without saying so.
		device = ports[0]
		if len(ports) > 1 {
			note = fmt.Sprintf("%d parallel ports accessible %v, using the first; name one with parallel:/dev/parportN", len(ports), ports)
		}
	}
	p := NewParallelPort(device)
	if err := p.Open(); err != nil {
		return nil, "", "", fmt.Errorf("parallel port %s: %w", device, err)
	}
	return p, device, note, nil
}

func openGPIOSpec(spec DeviceSpec) (OutputTTLDevice, string, [8]int, error) {
	chip := spec.Params["chip"]
	if chip == "" {
		chip = "/dev/gpiochip0"
	}
	pins := DefaultGPIOPins
	if list, ok := spec.Params["pins"]; ok {
		parsed, err := ParseGPIOPins(list)
		if err != nil {
			return nil, "", pins, fmt.Errorf("gpio pins=%q: %w", list, err)
		}
		pins = parsed
	}
	dev, err := NewLinuxGPIOTrigger(WithGPIOChip(chip), WithGPIOOutputPins(pins))
	if err != nil {
		return nil, "", pins, fmt.Errorf("GPIO on %s: %w "+
			"(needs Linux, kernel >= 5.10, and rw access to the chip: "+
			"sudo usermod -aG gpio $USER, then log in again)", chip, err)
	}
	return dev, chip, pins, nil
}

func kindList() string { return "choose " + strings.Join(DeviceKinds, ", ") }

func paramList(allowed []string) string {
	all := append([]string{"pin"}, allowed...)
	sort.Strings(all)
	return strings.Join(all, ", ")
}
