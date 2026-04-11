//go:build !windows

package sysinfo

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func collectMemory() MemInfo {
	switch runtime.GOOS {
	case "linux":
		return linuxMem()
	default:
		return bsdMem()
	}
}

func linuxMem() MemInfo {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemInfo{}
	}
	defer f.Close()

	m := map[string]int64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		// Values are "NNN kB"
		fields := strings.Fields(strings.TrimSpace(v))
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		m[strings.TrimSpace(k)] = n
	}

	avail := m["MemAvailable"]
	if avail == 0 {
		// Older kernels lack MemAvailable.
		avail = m["MemFree"] + m["Buffers"] + m["Cached"]
	}
	return MemInfo{
		TotalKB:     m["MemTotal"],
		UsedKB:      m["MemTotal"] - avail,
		SwapTotalKB: m["SwapTotal"],
		SwapUsedKB:  m["SwapTotal"] - m["SwapFree"],
	}
}

func bsdMem() MemInfo {
	var info MemInfo
	// Total physical memory in bytes.
	if s := run("sysctl", "-n", "hw.physmem"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			info.TotalKB = n / 1024
		}
	}
	if runtime.GOOS == "darwin" {
		info.UsedKB = info.TotalKB - macOSFreeKB()
	}
	return info
}

func macOSFreeKB() int64 {
	out := run("vm_stat")
	pageSize := int64(4096)
	var free int64
	for _, line := range strings.Split(out, "\n") {
		// "Mach Virtual Memory Statistics: (page size of 16384 bytes)"
		if strings.Contains(line, "page size of") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "of" && i+1 < len(fields) {
					pageSize, _ = strconv.ParseInt(fields[i+1], 10, 64)
				}
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimRight(strings.TrimSpace(parts[1]), ".")
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "Pages free", "Pages inactive", "Pages speculative":
			free += n
		}
	}
	return free * pageSize / 1024
}
