//go:build windows

package sysinfo

func collectAudio() AudioInfo {
	devices := wmicPath("Win32_SoundDevice", "Name", "Manufacturer", "DriverVersion")
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
