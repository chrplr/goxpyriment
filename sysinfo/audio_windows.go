//go:build windows

package sysinfo

func collectAudio() AudioInfo {
	devices := cimGet("Win32_SoundDevice")
	if len(devices) == 0 {
		return AudioInfo{}
	}
	cards := make([]AudioCard, len(devices))
	for i, d := range devices {
		name := d["Name"]
		if mfr := d["Manufacturer"]; mfr != "" && mfr != name {
			name = mfr + " " + name
		}
		cards[i] = AudioCard{Name: name, Driver: d["DriverVersion"]}
	}
	return AudioInfo{Cards: cards}
}
