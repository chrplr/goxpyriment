//go:build windows

package sysinfo

func collectMachine() MachineInfo {
	sys := first(cimGet("Win32_ComputerSystem"))

	// PCSystemType: 1=Desktop, 2=Mobile/Laptop, 3=Workstation, 4=EnterpriseServer,
	// 5=SOHOServer, 6=AppliancePC, 7=PerformanceServer, 8=Slate
	pcTypes := map[string]string{
		"1": "desktop",
		"2": "laptop",
		"3": "workstation",
		"4": "enterprise-server",
		"5": "soho-server",
		"6": "appliance-pc",
		"7": "performance-server",
		"8": "slate",
	}

	return MachineInfo{
		DeviceType:  pcTypes[sys["PCSystemType"]],
		SysVendor:   sys["Manufacturer"],
		ProductName: sys["Model"],
	}
}
