//go:build windows

package sysinfo

func collectGPUs() []GPUInfo {
	rows := wmicPath("Win32_VideoController", "Name", "DriverVersion")
	if len(rows) == 0 {
		return nil
	}
	cards := make([]GPUInfo, len(rows))
	for i, r := range rows {
		cards[i] = GPUInfo{
			Model:  r["Name"],
			Driver: r["DriverVersion"],
		}
	}
	return cards
}
