//go:build !windows

package sysinfo

import (
	"runtime"
	"strconv"
	"strings"
)

// chassisType maps SMBIOS chassis-type integers to human-readable strings.
var chassisType = map[int]string{
	2:  "unknown",
	3:  "desktop",
	4:  "desktop",
	5:  "pizza-box",
	6:  "desktop",
	7:  "desktop",
	8:  "portable",
	9:  "laptop",
	10: "laptop",
	11: "portable",
	12: "docking-station",
	13: "desktop",
	14: "notebook",
	15: "desktop",
	16: "laptop",
	17: "server",
	18: "expansion-chassis",
	19: "sub-chassis",
	20: "bus-expansion",
	21: "peripheral",
	22: "raid",
	23: "server",
	24: "desktop",
	25: "multimount-chassis",
	26: "compact-pci",
	27: "blade",
	28: "blade",
	29: "blade-enclosure",
	30: "tablet",
	31: "convertible",
	32: "detachable",
}

func collectMachine() MachineInfo {
	switch runtime.GOOS {
	case "linux":
		return linuxMachineInfo()
	case "darwin":
		return darwinMachineInfo()
	}
	return MachineInfo{}
}

func linuxMachineInfo() MachineInfo {
	dir := "/sys/class/dmi/id/"
	if readFile(dir+"sys_vendor") == "" {
		dir = "/sys/devices/virtual/dmi/id/"
		if readFile(dir+"sys_vendor") == "" {
			return MachineInfo{}
		}
	}

	device := ""
	if ct := readFile(dir + "chassis_type"); ct != "" {
		if n, err := strconv.Atoi(ct); err == nil {
			device = chassisType[n]
		}
	}

	return MachineInfo{
		DeviceType:     device,
		SysVendor:      readFile(dir + "sys_vendor"),
		ProductName:    readFile(dir + "product_name"),
		ProductVersion: readFile(dir + "product_version"),
	}
}

func darwinMachineInfo() MachineInfo {
	model := run("sysctl", "-n", "hw.model")
	if model == "" {
		return MachineInfo{}
	}
	lower := strings.ToLower(model)
	device := ""
	switch {
	case strings.Contains(lower, "macbook"):
		device = "laptop"
	case strings.Contains(lower, "macpro"), strings.Contains(lower, "macmini"),
		strings.Contains(lower, "imac"), strings.Contains(lower, "mac"):
		device = "desktop"
	}
	return MachineInfo{
		DeviceType:  device,
		SysVendor:   "Apple",
		ProductName: model,
	}
}
