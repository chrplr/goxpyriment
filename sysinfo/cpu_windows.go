//go:build windows

package sysinfo

import (
	"strconv"
	"strings"
)

func collectCPU() CPUInfo {
	rows := wmicGet("cpu",
		"Name", "NumberOfCores", "NumberOfLogicalProcessors",
		"CurrentClockSpeed", "MaxClockSpeed",
	)
	if len(rows) == 0 {
		return CPUInfo{}
	}

	model := strings.Join(strings.Fields(rows[0]["Name"]), " ")
	var totalCores, totalThreads int
	var totalMHz, maxMHz float64

	for _, r := range rows {
		if n, err := strconv.Atoi(r["NumberOfCores"]); err == nil {
			totalCores += n
		}
		if n, err := strconv.Atoi(r["NumberOfLogicalProcessors"]); err == nil {
			totalThreads += n
		}
		if mhz, err := strconv.ParseFloat(r["CurrentClockSpeed"], 64); err == nil {
			totalMHz += mhz
		}
		if mhz, err := strconv.ParseFloat(r["MaxClockSpeed"], 64); err == nil && mhz > maxMHz {
			maxMHz = mhz
		}
	}

	avgMHz := 0.0
	if len(rows) > 0 {
		avgMHz = totalMHz / float64(len(rows))
	}

	return CPUInfo{
		Model:   model,
		Cores:   totalCores,
		Threads: totalThreads,
		MHz:     avgMHz,
		MaxMHz:  maxMHz,
	}
}
