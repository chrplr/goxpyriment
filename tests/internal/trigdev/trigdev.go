// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// Package trigdev names a TTL output device on a command line and opens it.
//
// # Why this exists
//
// Timing-Tests selects one trigger device with a family of flags — one per
// device kind (-port, -parallel-port, -gpio-chip, -gpio-pins, -labjack-host)
// plus -trigger-device to say which of them applies. That works when exactly
// one device is opened and stops working when two are: there is nowhere to put
// the second port, and no way to say which pin belongs to which device.
//
// So a device is named here by a single self-contained string, and the flag
// carrying it is repeatable:
//
//	-device parallel:pin=1
//	-device dlpio8:port=/dev/ttyUSB0,pin=1
//	-device gpio:pins=17+27+22+5+6+13+19+26,pin=1
//
// Parameters are comma-separated, which is why the GPIO pin list joins with
// '+' rather than ','.
//
// # Failing to open is fatal here
//
// Timing-Tests answers a device that will not open with a NullOutputTTLDevice
// and a warning, so the visual half of its measurement is still obtained. This
// package returns an error instead, and its callers exit. A silent Null device
// in a device-comparison run puts a flat trace on a scope channel and the loss
// is only discovered when the capture is read.
package trigdev

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/chrplr/goxpyriment/triggers"
)

// Usage documents the -device syntax. Programs print it from their flag help.
const Usage = `Device spec: KIND[:key=value,key=value,...]

  dlpio8    port=/dev/ttyUSB0   (empty or omitted → auto-detect)
  dlpio20   port=/dev/ttyUSB1   (empty or omitted → auto-detect)
  megttlbox port=/dev/ttyACM0   (required)
  mmbts     port=/dev/ttyACM0[,mode=p|s]  (port required; mode defaults to p,
             the box's factory setting. In mode p the width is 8 ms, fixed in
             firmware, whatever duration the program asks for)
  parallel  port=/dev/parport0  (empty or omitted → first accessible port)
  gpio      chip=/dev/gpiochip0 pins=17+27+22+5+6+13+19+26
  ft232h    (no parameters)
  labjackt4 host=192.168.1.100[:502]  (required)
  null      (no hardware at all: writes go nowhere. For checking this program
             itself — a run made of null devices measures software, not signals)

Every kind also takes pin=N, the output line to pulse, numbered 1-8 as on the
device's terminal block (default 1). On gpio, pin=N selects the Nth entry of
pins=, not a BCM number.`

// Kinds lists the accepted device kinds, in the order they are documented.
var Kinds = []string{"dlpio8", "dlpio20", "megttlbox", "mmbts", "parallel", "gpio", "ft232h", "labjackt4", "null"}

// allowedParams is the set of parameter keys each kind accepts, beyond "pin"
// which every kind accepts. An unlisted key is an error rather than a silent
// no-op: "pins=" on a dlpio8 is a typo, and ignoring it would drive a line the
// operator did not ask for while the probe sits on the one they did.
var allowedParams = map[string][]string{
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

// defaultGPIOPins matches the Timing-Tests default, so a command line moved
// from one program to the other drives the same header pins.
var defaultGPIOPins = [8]int{17, 27, 22, 5, 6, 13, 19, 26}

// Spec is one parsed -device value: what to open and how.
type Spec struct {
	Kind   string            // one of Kinds
	Params map[string]string // validated keys only; never nil
	Pin    int               // 1-8, as printed on the device
	Raw    string            // the string as given, for messages and the data file
}

// Line is the 0-indexed line number the triggers API takes. Spec.Pin is the
// 1-indexed number the hardware is labelled with.
func (s Spec) Line() int { return s.Pin - 1 }

// ParseSpec parses one KIND[:key=value,...] string. It validates the kind, the
// parameter names, and the pin range; it does not touch hardware.
func ParseSpec(s string) (Spec, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Spec{}, fmt.Errorf("empty device spec; %s", kindList())
	}

	kind, rest, _ := strings.Cut(raw, ":")
	kind = strings.ToLower(strings.TrimSpace(kind))
	allowed, known := allowedParams[kind]
	if !known {
		return Spec{}, fmt.Errorf("unknown device kind %q; %s", kind, kindList())
	}

	spec := Spec{Kind: kind, Params: map[string]string{}, Pin: 1, Raw: raw}
	for _, field := range strings.Split(rest, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			return Spec{}, fmt.Errorf("device %q: %q is not key=value", raw, field)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		// pin is stored in Params as well as in its own field, so one check
		// covers every key.
		if _, dup := spec.Params[key]; dup {
			return Spec{}, fmt.Errorf("device %q: %s given twice", raw, key)
		}
		if key == "pin" {
			n, err := strconv.Atoi(value)
			if err != nil {
				return Spec{}, fmt.Errorf("device %q: pin=%q is not a number", raw, value)
			}
			if n < 1 || n > 8 {
				return Spec{}, fmt.Errorf("device %q: pin=%d is out of range; every device exposes 8 lines, numbered 1 to 8", raw, n)
			}
			spec.Pin = n
			spec.Params["pin"] = value
			continue
		}
		if !slices.Contains(allowed, key) {
			return Spec{}, fmt.Errorf("device %q: %s takes no %q parameter (accepts %s)",
				raw, kind, key, paramList(allowed))
		}
		spec.Params[key] = value
	}

	// Required parameters. Auto-detection exists for the DLP boxes and the
	// parallel port; the others have nothing to probe for — the MMBT-S never
	// replies on its serial line, so it cannot even be recognised once opened.
	switch kind {
	case "megttlbox":
		if spec.Params["port"] == "" {
			return Spec{}, fmt.Errorf("device %q: megttlbox needs port=, e.g. megttlbox:port=/dev/ttyACM0", raw)
		}
	case "mmbts":
		if spec.Params["port"] == "" {
			return Spec{}, fmt.Errorf("device %q: mmbts needs port=, e.g. mmbts:port=/dev/ttyACM0", raw)
		}
		if _, err := parseMMBTSMode(spec.Params["mode"]); err != nil {
			return Spec{}, fmt.Errorf("device %q: %w", raw, err)
		}
	case "labjackt4":
		if spec.Params["host"] == "" {
			return Spec{}, fmt.Errorf("device %q: labjackt4 needs host=, e.g. labjackt4:host=192.168.1.100", raw)
		}
	case "gpio":
		if pins, ok := spec.Params["pins"]; ok {
			if _, err := ParsePins(pins); err != nil {
				return Spec{}, fmt.Errorf("device %q: pins=%q: %w", raw, pins, err)
			}
		}
	}
	return spec, nil
}

// ParsePins parses a '+'-joined list of exactly 8 GPIO pin numbers.
//
// Duplicates are rejected: two identical entries would make two pin= values
// drive the same line, which is invisible in a trace and reads as a wiring
// fault. (Timing-Tests rejects them in its comma-separated -gpio-pins for the
// same reason.)
func ParsePins(s string) ([8]int, error) {
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

// Flags collects repeated -device values. It implements flag.Value:
//
//	var devs trigdev.Flags
//	flag.Var(&devs, "device", "TTL output device; repeat for several.\n"+trigdev.Usage)
type Flags []Spec

func (f *Flags) String() string {
	raws := make([]string, 0, len(*f))
	for _, s := range *f {
		raws = append(raws, s.Raw)
	}
	return strings.Join(raws, " ")
}

// Set parses and appends one -device value, rejecting a spec identical to one
// already given. Two specs of the same kind with different pins are legal and
// useful — that measures the skew between two lines of one device.
func (f *Flags) Set(value string) error {
	spec, err := ParseSpec(value)
	if err != nil {
		return err
	}
	for _, prev := range *f {
		if sameSpec(prev, spec) {
			return fmt.Errorf("device %q given twice; two lines of one device need different pin= values", spec.Raw)
		}
	}
	*f = append(*f, spec)
	return nil
}

func sameSpec(a, b Spec) bool {
	if a.Kind != b.Kind || a.Pin != b.Pin || len(a.Params) != len(b.Params) {
		return false
	}
	for k, v := range a.Params {
		if b.Params[k] != v {
			return false
		}
	}
	return true
}

// Opened is a device that is open and ready to pulse.
type Opened struct {
	Spec   Spec
	Device triggers.OutputTTLDevice
	Line   int      // 0-indexed line to drive, = Spec.Line()
	Label  string   // short, for table columns: "parallel:D0"
	Desc   string   // one line for the data-file header
	Notes  []string // what the operator needs: probe point, logic level, ground
}

// Close closes the underlying device, first driving every line LOW.
func (o Opened) Close() error {
	if o.Device == nil {
		return nil
	}
	return o.Device.Close()
}

// Open opens the device a Spec names.
//
// An error here is final: the caller should exit rather than substitute a
// NullOutputTTLDevice (see the package comment).
func Open(spec Spec) (Opened, error) {
	switch spec.Kind {
	case "dlpio8":
		return openDLPIO8(spec)
	case "dlpio20":
		return openDLPIO20(spec)
	case "megttlbox":
		return openMEGTTLBox(spec)
	case "mmbts":
		return openMMBTS(spec)
	case "parallel":
		return openParallel(spec)
	case "gpio":
		return openGPIO(spec)
	case "ft232h":
		return openFT232H(spec)
	case "labjackt4":
		return openLabJackT4(spec)
	case "null":
		return openNull(spec)
	}
	return Opened{}, fmt.Errorf("unknown device kind %q; %s", spec.Kind, kindList())
}

func openDLPIO8(spec Spec) (Opened, error) {
	port := spec.Params["port"]
	var dev triggers.OutputTTLDevice
	if port != "" {
		d, err := triggers.NewDLPIO8(port)
		if err != nil {
			return Opened{}, fmt.Errorf("DLP-IO8 on %s: %w", port, err)
		}
		dev = d
	} else {
		d, found, err := triggers.AutoDetectDLPIO8()
		if err != nil {
			return Opened{}, fmt.Errorf("DLP-IO8 auto-detect: %w", err)
		}
		if found == "" {
			return Opened{}, fmt.Errorf("no DLP-IO8-G found; pass dlpio8:port=/dev/ttyUSBn to name one")
		}
		dev, port = d, found
	}
	return Opened{
		Spec: spec, Device: dev, Line: spec.Line(),
		Label: fmt.Sprintf("dlpio8:%d", spec.Pin),
		Desc:  fmt.Sprintf("dlpio8 port=%s pin=%d", port, spec.Pin),
		Notes: []string{
			fmt.Sprintf("probe terminal %d on the DLP-IO8-G block (%s); ground is GND on the same block", spec.Pin, port),
			"5 V logic; every write crosses a USB-CDC serial link",
		},
	}, nil
}

func openDLPIO20(spec Spec) (Opened, error) {
	port := spec.Params["port"]
	var dev triggers.OutputTTLDevice
	if port != "" {
		d, err := triggers.NewDLPIO20(port)
		if err != nil {
			return Opened{}, fmt.Errorf("DLP-IO20 on %s: %w", port, err)
		}
		dev = d
	} else {
		d, found, err := triggers.AutoDetectDLPIO20()
		if err != nil {
			return Opened{}, fmt.Errorf("DLP-IO20 auto-detect: %w", err)
		}
		if found == "" {
			return Opened{}, fmt.Errorf("no DLP-IO20 found; pass dlpio20:port=/dev/ttyUSBn to name one")
		}
		dev, port = d, found
	}
	return Opened{
		Spec: spec, Device: dev, Line: spec.Line(),
		Label: fmt.Sprintf("dlpio20:AN%d", spec.Line()),
		Desc:  fmt.Sprintf("dlpio20 port=%s pin=%d channel=AN%d", port, spec.Pin, spec.Line()),
		Notes: []string{
			fmt.Sprintf("probe AN%d on the DLP-IO20 (%s); ground is GND on the same block", spec.Line(), port),
			"5 V logic; every write crosses a USB-CDC serial link",
			"note: only single-line writes are used here — the device's 8-line Send is 8 packets, ~3.5 ms",
		},
	}, nil
}

func openMEGTTLBox(spec Spec) (Opened, error) {
	port := spec.Params["port"]
	box, err := triggers.NewMEGTTLBox(port)
	if err != nil {
		return Opened{}, fmt.Errorf("MEG TTL box on %s: %w", port, err)
	}
	// D30 is line 0 and the pins descend from there on the Mega's header; print
	// the pin the probe clips onto, not only the line index.
	pin := megOutputPin(spec.Line())
	notes := []string{
		fmt.Sprintf("probe D%d on the Mega (%s); ground is any GND pin on the same header", pin, port),
		"5 V logic; the command crosses USB, the edge is driven by the firmware",
		fmt.Sprintf("firmware: %s", box.Info()),
	}
	return Opened{
		Spec: spec, Device: box, Line: spec.Line(),
		Label: fmt.Sprintf("megttlbox:D%d", pin),
		Desc:  fmt.Sprintf("megttlbox port=%s pin=%d arduino=D%d firmware=%q", port, spec.Pin, pin, box.Info().String()),
		Notes: notes,
	}, nil
}

// megOutputPin maps a 0-indexed output line to the Arduino Mega pin the box
// wires it to: line 0 is D30, line 7 is D37.
func megOutputPin(line int) int { return 30 + line }

// parseMMBTSMode maps the mode= parameter onto the runtime mode set by the
// box's P/S switch. Empty means the factory setting, Pulse.
func parseMMBTSMode(s string) (triggers.MMBTSMode, error) {
	switch strings.ToLower(s) {
	case "", "p", "pulse":
		return triggers.MMBTSPulseMode, nil
	case "s", "simple":
		return triggers.MMBTSSimpleMode, nil
	}
	return 0, fmt.Errorf("mmbts mode=%q: choose p (pulse, the factory setting) or s (simple)", s)
}

func openMMBTS(spec Spec) (Opened, error) {
	port := spec.Params["port"]
	mode, err := parseMMBTSMode(spec.Params["mode"])
	if err != nil {
		return Opened{}, err
	}
	box, err := triggers.NewMMBTS(port, triggers.WithMMBTSMode(mode))
	if err != nil {
		return Opened{}, fmt.Errorf("MMBT-S on %s: %w "+
			"(rw access to the port is needed: sudo usermod -aG dialout $USER, then log in again)", port, err)
	}
	// Bit N of the byte drives D-Sub 25 pin N+2, so the connector pin is the
	// line index + 2. The line index is what the API takes; the D-Sub pin is
	// what the probe clips onto, and confusing the two is a silent miswiring.
	line := spec.Line()
	notes := []string{
		fmt.Sprintf("probe D-Sub 25 pin %d (= bit %d) on the MMBT-S (%s); ground is any of pins 20-25", line+2, spec.Pin, port),
		"5 V logic; every write crosses a USB-CDC serial link at 9600 baud",
		fmt.Sprintf("runtime mode %s — CHECK THE P/S SWITCH on the box: the driver cannot read it", mode),
	}
	if mode == triggers.MMBTSPulseMode {
		notes = append(notes,
			fmt.Sprintf("in pulse mode the firmware fixes the width at %v and ignores the requested duration; "+
				"codes closer together than that are delayed", box.PulseWidth()))
	}
	return Opened{
		Spec: spec, Device: box, Line: line,
		Label: fmt.Sprintf("mmbts:D%d", line+2),
		Desc:  fmt.Sprintf("mmbts port=%s pin=%d dsub25=%d mode=%s", port, spec.Pin, line+2, mode),
		Notes: notes,
	}, nil
}

func openParallel(spec Spec) (Opened, error) {
	device := spec.Params["port"]
	if device == "" {
		ports := triggers.AvailableParallelPorts()
		if len(ports) == 0 {
			return Opened{}, fmt.Errorf("no accessible parallel port found " +
				"(needs Linux with ppdev loaded and rw access: sudo modprobe ppdev; " +
				"sudo usermod -aG lp $USER, then log in again. If dmesg says " +
				"'lp0: using parport0', also unload the lp printer module: sudo rmmod lp)")
		}
		// Report the choice rather than making it silently: a machine with two
		// LPT ports would otherwise fire whichever enumerated first.
		device = ports[0]
		if len(ports) > 1 {
			return openParallelAt(spec, device, fmt.Sprintf("%d ports accessible %v — using the first; pass parallel:port=… to choose", len(ports), ports))
		}
	}
	return openParallelAt(spec, device, "")
}

func openParallelAt(spec Spec, device, extraNote string) (Opened, error) {
	p := triggers.NewParallelPort(device)
	if err := p.Open(); err != nil {
		return Opened{}, fmt.Errorf("parallel port %s: %w", device, err)
	}
	// D0-D7 are DB25 pins 2-9, so the connector pin is the line index + 2. The
	// data-line number is what the API takes; the DB25 pin is what the probe
	// clips onto, and confusing the two is a silent miswiring.
	line := spec.Line()
	notes := []string{
		fmt.Sprintf("probe DB25 pin %d (= D%d) on %s; ground is any of DB25 pins 18-25", line+2, line, device),
		"5 V logic; the write is a local ioctl, no bus in between",
	}
	if extraNote != "" {
		notes = append(notes, extraNote)
	}
	return Opened{
		Spec: spec, Device: p, Line: line,
		Label: fmt.Sprintf("parallel:D%d", line),
		Desc:  fmt.Sprintf("parallel device=%s pin=%d line=D%d db25=%d", device, spec.Pin, line, line+2),
		Notes: notes,
	}, nil
}

func openGPIO(spec Spec) (Opened, error) {
	chip := spec.Params["chip"]
	if chip == "" {
		chip = "/dev/gpiochip0"
	}
	pins := defaultGPIOPins
	if list, ok := spec.Params["pins"]; ok {
		parsed, err := ParsePins(list)
		if err != nil {
			return Opened{}, fmt.Errorf("gpio pins=%q: %w", list, err)
		}
		pins = parsed
	}
	dev, err := triggers.NewLinuxGPIOTrigger(
		triggers.WithGPIOChip(chip),
		triggers.WithGPIOOutputPins(pins),
	)
	if err != nil {
		return Opened{}, fmt.Errorf("GPIO on %s: %w "+
			"(needs Linux, kernel >= 5.10, and rw access to the chip: "+
			"sudo usermod -aG gpio $USER, then log in again)", chip, err)
	}
	// Print the pin actually driven, not its index: pin=1 on the defaults is
	// BCM 17, a number nowhere in the command line and the one to probe.
	bcm := pins[spec.Line()]
	return Opened{
		Spec: spec, Device: dev, Line: spec.Line(),
		Label: fmt.Sprintf("gpio:%d", bcm),
		Desc:  fmt.Sprintf("gpio chip=%s pin=%d line=%d pins=%v", chip, spec.Pin, bcm, pins),
		Notes: []string{
			fmt.Sprintf("probe chip line %d (BCM %d on a Raspberry Pi) on %s; ground is any GND pin", bcm, bcm, chip),
			"3.3 V logic, NOT 5 V — check that the instrument's threshold is below 3.3 V",
			"the write is a local ioctl, no bus in between",
		},
	}, nil
}

func openFT232H(spec Spec) (Opened, error) {
	// NewFT232H rather than AutoDetectFT232H: the latter turns a failure into a
	// NullOutputTTLDevice and keeps the reason to itself, and the reason here is
	// usually actionable (ftdi_sio holding the interface, or no rw access).
	dev, err := triggers.NewFT232H()
	if err != nil {
		return Opened{}, fmt.Errorf("FT232H: %w "+
			"(Linux only; ftdi_sio must not hold the device and /dev/bus/usb/... must be "+
			"readable and writable: sudo rmmod ftdi_sio, then a udev rule or the plugdev group)", err)
	}
	line := spec.Line()
	return Opened{
		Spec: spec, Device: dev, Line: line,
		Label: fmt.Sprintf("ft232h:AD%d", line),
		Desc:  fmt.Sprintf("ft232h pin=%d line=AD%d", spec.Pin, line),
		Notes: []string{
			fmt.Sprintf("probe AD%d on the FT232H board; ground is any GND pad", line),
			"3.3 V logic, NOT 5 V — check that the instrument's threshold is below 3.3 V",
			"every write crosses USB (MPSSE over usbfs)",
			"if reads or writes are sluggish, lower the FTDI latency timer",
		},
	}, nil
}

func openLabJackT4(spec Spec) (Opened, error) {
	host := spec.Params["host"]
	// The output group is left at the driver's default, DIO4-DIO11: the T4's
	// DIO0-DIO3 are the analog inputs AIN0-AIN3 and cannot be driven.
	dev, err := triggers.NewLabJackT4(host)
	if err != nil {
		return Opened{}, fmt.Errorf("LabJack T4 at %s: %w "+
			"(the T4 must be reachable on Modbus TCP port 502; check with "+
			"go run ./tests/test_labjackt4 -host %s -hold)", host, err, host)
	}
	dio := t4OutputBase + spec.Line()
	term := T4TerminalName(dio)
	return Opened{
		Spec: spec, Device: dev, Line: spec.Line(),
		Label: fmt.Sprintf("labjackt4:%s", term),
		Desc:  fmt.Sprintf("labjackt4 host=%s pin=%d dio=%d terminal=%s", host, spec.Pin, dio, term),
		Notes: []string{
			fmt.Sprintf("probe %s (DIO%d) on the T4 at %s; ground is any GND terminal", term, dio, host),
			"3.3 V logic, NOT 5 V — check that the instrument's threshold is below 3.3 V",
			"every write crosses the network — expect more latency and jitter than a local port",
		},
	}, nil
}

// openNull opens nothing. It exists so the program around this package — the
// schedule, the data file, the statistics — can be exercised on a machine with
// no trigger hardware. IsNull reports it so a report can say plainly that no
// signal was produced.
func openNull(spec Spec) (Opened, error) {
	return Opened{
		Spec: spec, Device: triggers.NullOutputTTLDevice{}, Line: spec.Line(),
		Label: fmt.Sprintf("null:%d", spec.Pin),
		Desc:  fmt.Sprintf("null pin=%d (NO HARDWARE)", spec.Pin),
		Notes: []string{
			"NO HARDWARE: writes go nowhere and nothing will appear on an instrument",
			"only the software path is exercised — do not report these numbers as a measurement",
		},
	}, nil
}

// IsNull reports whether this device drives nothing.
func (o Opened) IsNull() bool { return o.Spec.Kind == "null" }

// t4OutputBase is the DIO number of the T4 output line 0 — the driver's default
// (triggers.WithT4OutputBase is left unset), repeated here only so the printed
// pin can be resolved to a screw terminal.
const t4OutputBase = 4

// T4TerminalName maps a T4 DIO number to the label printed on the hardware.
// The DIO numbering is contiguous but the terminals are not: DIO4 is FIO4 on
// the screw block while DIO8 is EIO0 on the DB15, so the number in the API is
// not the number to look for on the device.
func T4TerminalName(dio int) string {
	switch {
	case dio >= 0 && dio <= 3:
		return fmt.Sprintf("AIN%d (analog only)", dio)
	case dio <= 7:
		return fmt.Sprintf("FIO%d", dio)
	case dio <= 15:
		return fmt.Sprintf("EIO%d", dio-8)
	case dio <= 19:
		return fmt.Sprintf("CIO%d", dio-16)
	}
	return fmt.Sprintf("DIO%d", dio)
}

func kindList() string { return "choose " + strings.Join(Kinds, ", ") }

func paramList(allowed []string) string {
	all := append([]string{"pin"}, allowed...)
	sort.Strings(all)
	return strings.Join(all, ", ")
}
