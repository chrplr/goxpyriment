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
	"strconv"
	"strings"

	"github.com/chrplr/goxpyriment/triggers"
)

// Parsing and opening live in package triggers (triggers.ParseDeviceSpec,
// triggers.OpenDeviceSpec), shared with the experiments' -ttl flag. This
// package adds what a line-level measurement needs on top: the pin to pulse,
// a column label, and the wiring notes printed for the operator.

// Usage documents the -device syntax. Programs print it from their flag help.
const Usage = triggers.DeviceSpecUsage + `

Every kind also takes pin=N, the output line to pulse, numbered 1-8 as on the
device's terminal block (default 1). On gpio, pin=N selects the Nth entry of
pins=, not a BCM number. In mmbts mode p the width is 8 ms, fixed in firmware,
whatever duration the program asks for.`

// Kinds lists the accepted device kinds, in the order they are documented.
var Kinds = triggers.DeviceKinds

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

func (s Spec) deviceSpec() triggers.DeviceSpec {
	return triggers.DeviceSpec{Kind: s.Kind, Params: s.Params, Raw: s.Raw}
}

// ParseSpec parses one device string (see Usage). It validates the kind, the
// parameter names, and the pin range; it does not touch hardware.
func ParseSpec(s string) (Spec, error) {
	ds, err := triggers.ParseDeviceSpec(s)
	if err != nil {
		return Spec{}, err
	}
	spec := Spec{Kind: ds.Kind, Params: ds.Params, Pin: 1, Raw: ds.Raw}
	if p, ok := ds.Params["pin"]; ok {
		// Already range-checked by ParseDeviceSpec.
		spec.Pin, _ = strconv.Atoi(p)
	}
	return spec, nil
}

// ParsePins parses a '+'-joined list of exactly 8 GPIO pin numbers.
func ParsePins(s string) ([8]int, error) { return triggers.ParseGPIOPins(s) }

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
	od, err := triggers.OpenDeviceSpec(spec.deviceSpec())
	if err != nil {
		return Opened{}, err
	}
	o := Opened{Spec: spec, Device: od.Device, Line: spec.Line()}
	line, pin, where := spec.Line(), spec.Pin, od.Where
	switch spec.Kind {
	case "dlpio8":
		o.Label = fmt.Sprintf("dlpio8:%d", pin)
		o.Desc = fmt.Sprintf("dlpio8 port=%s pin=%d", where, pin)
		o.Notes = []string{
			fmt.Sprintf("probe terminal %d on the DLP-IO8-G block (%s); ground is GND on the same block", pin, where),
			"5 V logic; every write crosses a USB-CDC serial link",
		}
	case "dlpio20":
		o.Label = fmt.Sprintf("dlpio20:AN%d", line)
		o.Desc = fmt.Sprintf("dlpio20 port=%s pin=%d channel=AN%d", where, pin, line)
		o.Notes = []string{
			fmt.Sprintf("probe AN%d on the DLP-IO20 (%s); ground is GND on the same block", line, where),
			"5 V logic; every write crosses a USB-CDC serial link",
			"note: only single-line writes are used here — the device's 8-line Send is 8 packets, ~3.5 ms",
		}
	case "megttlbox":
		// D30 is line 0 and the pins ascend from there on the Mega's header;
		// print the pin the probe clips onto, not only the line index.
		box := od.Device.(*triggers.MEGTTLBox)
		d := megOutputPin(line)
		o.Label = fmt.Sprintf("megttlbox:D%d", d)
		o.Desc = fmt.Sprintf("megttlbox port=%s pin=%d arduino=D%d firmware=%q", where, pin, d, box.Info().String())
		o.Notes = []string{
			fmt.Sprintf("probe D%d on the Mega (%s); ground is any GND pin on the same header", d, where),
			"5 V logic; the command crosses USB, the edge is driven by the firmware",
			fmt.Sprintf("firmware: %s", box.Info()),
		}
	case "mmbts":
		// Bit N of the byte drives D-Sub 25 pin N+2. The line index is what
		// the API takes; the D-Sub pin is what the probe clips onto, and
		// confusing the two is a silent miswiring.
		box := od.Device.(*triggers.MMBTS)
		mode := box.Mode()
		o.Label = fmt.Sprintf("mmbts:D%d", line+2)
		o.Desc = fmt.Sprintf("mmbts port=%s pin=%d dsub25=%d mode=%s", where, pin, line+2, mode)
		o.Notes = []string{
			fmt.Sprintf("probe D-Sub 25 pin %d (= bit %d) on the MMBT-S (%s); ground is any of pins 20-25", line+2, pin, where),
			"5 V logic; every write crosses a USB-CDC serial link at 9600 baud",
			fmt.Sprintf("runtime mode %s — CHECK THE P/S SWITCH on the box: the driver cannot read it", mode),
		}
		if mode == triggers.MMBTSPulseMode {
			o.Notes = append(o.Notes,
				fmt.Sprintf("in pulse mode the firmware fixes the width at %v and ignores the requested duration; "+
					"codes closer together than that are delayed", box.PulseWidth()))
		}
	case "parallel":
		// D0-D7 are DB25 pins 2-9, so the connector pin is the line index + 2.
		o.Label = fmt.Sprintf("parallel:D%d", line)
		o.Desc = fmt.Sprintf("parallel device=%s pin=%d line=D%d db25=%d", where, pin, line, line+2)
		o.Notes = []string{
			fmt.Sprintf("probe DB25 pin %d (= D%d) on %s; ground is any of DB25 pins 18-25", line+2, line, where),
			"5 V logic; the write is a local ioctl, no bus in between",
		}
		if od.Note != "" {
			o.Notes = append(o.Notes, od.Note)
		}
	case "gpio":
		// Print the pin actually driven, not its index: pin=1 on the defaults
		// is BCM 17, a number nowhere in the command line and the one to probe.
		bcm := od.GPIOPins[line]
		o.Label = fmt.Sprintf("gpio:%d", bcm)
		o.Desc = fmt.Sprintf("gpio chip=%s pin=%d line=%d pins=%v", where, pin, bcm, od.GPIOPins)
		o.Notes = []string{
			fmt.Sprintf("probe chip line %d (BCM %d on a Raspberry Pi) on %s; ground is any GND pin", bcm, bcm, where),
			"3.3 V logic, NOT 5 V — check that the instrument's threshold is below 3.3 V",
			"the write is a local ioctl, no bus in between",
		}
	case "ft232h":
		o.Label = fmt.Sprintf("ft232h:AD%d", line)
		o.Desc = fmt.Sprintf("ft232h pin=%d line=AD%d", pin, line)
		o.Notes = []string{
			fmt.Sprintf("probe AD%d on the FT232H board; ground is any GND pad", line),
			"3.3 V logic, NOT 5 V — check that the instrument's threshold is below 3.3 V",
			"every write crosses USB (MPSSE over usbfs)",
			"if reads or writes are sluggish, lower the FTDI latency timer",
		}
	case "labjackt4":
		dio := t4OutputBase + line
		term := T4TerminalName(dio)
		o.Label = fmt.Sprintf("labjackt4:%s", term)
		o.Desc = fmt.Sprintf("labjackt4 host=%s pin=%d dio=%d terminal=%s", where, pin, dio, term)
		o.Notes = []string{
			fmt.Sprintf("probe %s (DIO%d) on the T4 at %s; ground is any GND terminal", term, dio, where),
			"3.3 V logic, NOT 5 V — check that the instrument's threshold is below 3.3 V",
			"every write crosses the network — expect more latency and jitter than a local port",
		}
	case "null":
		// Opens nothing, so the program around this package — the schedule,
		// the data file, the statistics — can be exercised without hardware.
		o.Label = fmt.Sprintf("null:%d", pin)
		o.Desc = fmt.Sprintf("null pin=%d (NO HARDWARE)", pin)
		o.Notes = []string{
			"NO HARDWARE: writes go nowhere and nothing will appear on an instrument",
			"only the software path is exercised — do not report these numbers as a measurement",
		}
	}
	return o, nil
}

// megOutputPin maps a 0-indexed output line to the Arduino Mega pin the box
// wires it to: line 0 is D30, line 7 is D37.
func megOutputPin(line int) int { return 30 + line }

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
