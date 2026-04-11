//go:build !windows

package sysinfo

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func collectCPU() CPUInfo {
	switch runtime.GOOS {
	case "linux":
		return linuxCPU()
	default:
		return bsdCPU()
	}
}

func linuxCPU() CPUInfo {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return CPUInfo{}
	}
	defer f.Close()

	var info CPUInfo
	physIDs := map[string]struct{}{}
	var totalMHz float64
	var mhzCount int

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)

		switch k {
		case "model name":
			if info.Model == "" {
				info.Model = strings.Join(strings.Fields(v), " ")
			}
		case "physical id":
			physIDs[v] = struct{}{}
		case "cpu cores":
			if info.Cores == 0 {
				info.Cores, _ = strconv.Atoi(v)
			}
		case "siblings":
			if info.Threads == 0 {
				info.Threads, _ = strconv.Atoi(v)
			}
		case "cpu MHz":
			if mhz, err := strconv.ParseFloat(v, 64); err == nil {
				totalMHz += mhz
				mhzCount++
			}
		}
	}

	// Scale for multi-socket systems.
	if sockets := len(physIDs); sockets > 1 {
		info.Cores *= sockets
		info.Threads *= sockets
	}
	if mhzCount > 0 {
		info.MHz = totalMHz / float64(mhzCount)
	}

	info.MinMHz = freqMHz("/sys/devices/system/cpu/cpu0/cpufreq/scaling_min_freq")
	info.MaxMHz = freqMHz("/sys/devices/system/cpu/cpu0/cpufreq/scaling_max_freq")
	if info.MHz == 0 {
		info.MHz = info.MaxMHz
	}
	return info
}

// freqMHz reads a cpufreq file that stores frequency in kHz.
func freqMHz(path string) float64 {
	if s := readFile(path); s != "" {
		if kHz, err := strconv.ParseFloat(s, 64); err == nil {
			return kHz / 1000
		}
	}
	return 0
}

func bsdCPU() CPUInfo {
	var info CPUInfo
	info.Model = run("sysctl", "-n", "hw.model")
	info.Threads, _ = strconv.Atoi(run("sysctl", "-n", "hw.ncpu"))
	if runtime.GOOS == "darwin" {
		info.Cores, _ = strconv.Atoi(run("sysctl", "-n", "machdep.cpu.core_count"))
		if fHz, err := strconv.ParseFloat(run("sysctl", "-n", "hw.cpufrequency"), 64); err == nil {
			info.MHz = fHz / 1e6
		}
	}
	if info.Cores == 0 {
		info.Cores = info.Threads
	}
	return info
}
