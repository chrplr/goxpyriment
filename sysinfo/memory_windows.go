//go:build windows

package sysinfo

import (
	"strconv"
)

func collectMemory() MemInfo {
	osInfo := first(wmicGet("OS", "TotalVisibleMemorySize", "FreePhysicalMemory"))
	// Values are in KiB.
	totalKB, _ := strconv.ParseInt(osInfo["TotalVisibleMemorySize"], 10, 64)
	freeKB, _ := strconv.ParseInt(osInfo["FreePhysicalMemory"], 10, 64)
	if totalKB == 0 {
		return MemInfo{}
	}
	usedKB := totalKB - freeKB

	var swapTotalKB, swapUsedKB int64
	// Page file (Windows equivalent of swap). AllocatedBaseSize and CurrentUsage are in MiB.
	pf := first(wmicPath("Win32_PageFileUsage", "AllocatedBaseSize", "CurrentUsage"))
	if pfTotal, err := strconv.ParseInt(pf["AllocatedBaseSize"], 10, 64); err == nil && pfTotal > 0 {
		pfUsed, _ := strconv.ParseInt(pf["CurrentUsage"], 10, 64)
		swapTotalKB = pfTotal << 10 // MiB → KiB
		swapUsedKB = pfUsed << 10
	}

	return MemInfo{
		TotalKB:     totalKB,
		UsedKB:      usedKB,
		SwapTotalKB: swapTotalKB,
		SwapUsedKB:  swapUsedKB,
	}
}
