//go:build !windows

package sysinfo

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func collectGPUs() []GPUInfo {
	// lspci is available on Linux and most BSDs (pciutils).
	if cards := lspciGPUs(); len(cards) > 0 {
		return cards
	}
	// BSD alternative.
	if runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" || runtime.GOOS == "netbsd" {
		if cards := pciconfGPUs(); len(cards) > 0 {
			return cards
		}
	}
	// macOS.
	if runtime.GOOS == "darwin" {
		if cards := macOSGPUs(); len(cards) > 0 {
			return cards
		}
	}
	// Linux sysfs fallback (no pciutils installed).
	return sysfsGPUs()
}

// lspciGPUs parses `lspci -k` which includes kernel driver information.
func lspciGPUs() []GPUInfo {
	out := run("lspci", "-k")
	if out == "" {
		return nil
	}

	var cards []GPUInfo
	var cur *GPUInfo

	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 {
			continue
		}
		// Non-indented lines are device headers.
		if line[0] != ' ' && line[0] != '\t' {
			if cur != nil {
				cards = append(cards, *cur)
				cur = nil
			}
			if isGPUClass(line) {
				// "00:02.0 VGA compatible controller: Intel Corporation UHD Graphics 630 (rev 02)"
				name := line
				if idx := strings.Index(line, ": "); idx >= 0 {
					name = strings.TrimSpace(line[idx+2:])
					// Strip "(rev XX)" suffix.
					if ridx := strings.LastIndex(name, " (rev "); ridx >= 0 {
						name = strings.TrimSpace(name[:ridx])
					}
				}
				cur = &GPUInfo{Model: name}
			}
			continue
		}
		// Indented lines: look for kernel driver.
		if cur != nil && strings.Contains(line, "Kernel driver in use:") {
			if _, v, ok := strings.Cut(line, ":"); ok {
				cur.Driver = strings.TrimSpace(v)
			}
		}
	}
	if cur != nil {
		cards = append(cards, *cur)
	}
	return cards
}

func isGPUClass(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "vga") ||
		strings.Contains(lower, "display controller") ||
		strings.Contains(lower, "3d controller") ||
		strings.Contains(lower, "graphics")
}

// pciconfGPUs parses FreeBSD `pciconf -lv` output.
func pciconfGPUs() []GPUInfo {
	out := run("pciconf", "-lv")
	if out == "" {
		return nil
	}
	var cards []GPUInfo
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if !isGPUClass(line) {
			continue
		}
		// Following lines: "    vendor     = 'NVIDIA Corporation'"
		//                  "    device     = 'GeForce RTX 3080'"
		var vendor, device string
		for j := i + 1; j < len(lines) && j < i+8; j++ {
			k, v, ok := strings.Cut(lines[j], "=")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			v = strings.Trim(strings.TrimSpace(v), "'")
			switch k {
			case "vendor":
				vendor = v
			case "device":
				device = v
			}
		}
		model := strings.TrimSpace(vendor + " " + device)
		if model == "" {
			model = "Unknown GPU"
		}
		cards = append(cards, GPUInfo{Model: model})
	}
	return cards
}

// macOSGPUs parses `system_profiler SPDisplaysDataType`.
func macOSGPUs() []GPUInfo {
	out := run("system_profiler", "SPDisplaysDataType")
	if out == "" {
		return nil
	}
	var cards []GPUInfo
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if _, v, ok := strings.Cut(line, "Chipset Model:"); ok {
			cards = append(cards, GPUInfo{Model: strings.TrimSpace(v)})
		}
	}
	return cards
}

// sysfsGPUs reads /sys/class/drm/cardN when lspci is unavailable.
func sysfsGPUs() []GPUInfo {
	matches, err := filepath.Glob("/sys/class/drm/card[0-9]")
	if err != nil || len(matches) == 0 {
		return nil
	}
	var cards []GPUInfo
	for _, m := range matches {
		driver := ""
		if link, err := os.Readlink(filepath.Join(m, "device/driver")); err == nil {
			driver = filepath.Base(link)
		}
		vendor := readFile(filepath.Join(m, "device/vendor"))
		device := readFile(filepath.Join(m, "device/device"))
		model := strings.TrimSpace(vendorName(vendor) + " " + device)
		if model == "" || model == " " {
			if driver != "" {
				model = "GPU (" + driver + ")"
			} else {
				model = "Unknown GPU"
			}
		}
		cards = append(cards, GPUInfo{Model: model, Driver: driver})
	}
	return cards
}

func vendorName(id string) string {
	switch strings.ToLower(id) {
	case "0x10de":
		return "NVIDIA"
	case "0x1002":
		return "AMD"
	case "0x8086":
		return "Intel"
	default:
		return id
	}
}
