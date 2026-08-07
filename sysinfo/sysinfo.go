// Package sysinfo collects and formats hardware and software information about
// the running system (machine type, OS, CPU, memory, GPU, audio).
// It is a library adaptation of the go-inxi CLI tool.
package sysinfo

import (
	"fmt"
	"strings"
)

// MachineInfo describes the physical device chassis.
type MachineInfo struct {
	DeviceType     string // e.g. "laptop", "desktop"
	SysVendor      string // e.g. "Dell Inc."
	ProductName    string
	ProductVersion string
}

// SystemInfo describes the operating system and runtime environment.
type SystemInfo struct {
	Hostname string
	Kernel   string
	Arch     string
	Uptime   string
	OS       string
	Shell    string
	Desktop  string
}

// CPUInfo describes the processor.
type CPUInfo struct {
	Model   string
	Cores   int     // physical cores
	Threads int     // logical (SMT) threads
	MHz     float64 // average current speed
	MinMHz  float64
	MaxMHz  float64
}

// MemInfo describes physical and swap memory in KiB.
type MemInfo struct {
	TotalKB     int64
	UsedKB      int64
	SwapTotalKB int64
	SwapUsedKB  int64
}

// GPUInfo describes one graphics card.
type GPUInfo struct {
	Model  string
	Driver string
}

// AudioCard describes one sound card.
type AudioCard struct {
	Name   string
	Driver string
}

// AudioInfo describes the audio subsystem.
type AudioInfo struct {
	Cards  []AudioCard
	Server string // e.g. "PipeWire", "PulseAudio", "CoreAudio"
	SrvVer string
	ALSA   string // ALSA driver version (Linux only)
}

// SysInfo is the top-level structure returned by Collect.
type SysInfo struct {
	Machine MachineInfo
	System  SystemInfo
	CPU     CPUInfo
	Memory  MemInfo
	GPUs    []GPUInfo
	Audio   AudioInfo
	// Sched is how the OS is scheduling this process. It belongs in a system
	// report because it changes timing results by more than the experiment code
	// does, and leaves no trace in the recorded data.
	Sched SchedulingInfo
}

// Collect gathers all system information and returns it as a SysInfo.
func Collect() SysInfo {
	return SysInfo{
		Machine: collectMachine(),
		System:  collectSystem(),
		CPU:     collectCPU(),
		Memory:  collectMemory(),
		GPUs:    collectGPUs(),
		Audio:   collectAudio(),
		Sched:   collectScheduling(),
	}
}

// String returns inxi-style formatted output for all collected subsystems.
func (s SysInfo) String() string {
	var b strings.Builder

	ws := func(label string, lines ...[]string) {
		const width = 11
		indent := strings.Repeat(" ", width+1)
		first := true
		for _, pairs := range lines {
			pairs = compact(pairs)
			if len(pairs) == 0 {
				continue
			}
			row := strings.Join(pairs, "  ")
			if first {
				fmt.Fprintf(&b, "%-*s %s\n", width, label+":", row)
				first = false
			} else {
				fmt.Fprintf(&b, "%s%s\n", indent, row)
			}
		}
	}

	// Machine
	m := s.Machine
	if m != (MachineInfo{}) {
		ws("Machine",
			[]string{
				kv("product", m.ProductName),
				kv("v", m.ProductVersion),
				kv("System", m.SysVendor),
				kv("Type", m.DeviceType),
			},
		)
	}

	// System
	sys := s.System
	if sys.Hostname != "" || sys.Kernel != "" || sys.OS != "" {
		ws("System",
			[]string{
				kv("Host", sys.Hostname),
				kv("Kernel", strings.TrimSpace(sys.Kernel+" "+sys.Arch)),
				kv("Uptime", sys.Uptime),
			},
			[]string{
				kv("OS", sys.OS),
				kv("Shell", sys.Shell),
				kv("Desktop", sys.Desktop),
			},
		)
	}

	// CPU
	cpu := s.CPU
	if cpu.Model != "" || cpu.Cores > 0 {
		topology := ""
		switch {
		case cpu.Cores > 0 && cpu.Threads > cpu.Cores:
			topology = fmt.Sprintf("%d cores / %d threads", cpu.Cores, cpu.Threads)
		case cpu.Cores > 0:
			topology = fmt.Sprintf("%d cores", cpu.Cores)
		}
		speed := ""
		if cpu.MHz > 0 {
			speed = fmt.Sprintf("%.0f MHz", cpu.MHz)
			if cpu.MinMHz > 0 && cpu.MaxMHz > 0 {
				speed += fmt.Sprintf(" (min: %.0f / max: %.0f)", cpu.MinMHz, cpu.MaxMHz)
			}
		}
		ws("CPU",
			[]string{
				kv("Model", cpu.Model),
				kv("Info", topology),
				kv("Speed", speed),
			},
		)
	}

	// Memory
	mem := s.Memory
	if mem.TotalKB > 0 {
		pct := float64(mem.UsedKB) / float64(mem.TotalKB) * 100
		ram := fmt.Sprintf("total: %s  used: %s (%.1f%%)",
			fmtBytes(mem.TotalKB*1024),
			fmtBytes(mem.UsedKB*1024),
			pct,
		)
		row := []string{kv("RAM", ram)}
		if mem.SwapTotalKB > 0 {
			swapPct := float64(mem.SwapUsedKB) / float64(mem.SwapTotalKB) * 100
			swap := fmt.Sprintf("total: %s  used: %s (%.1f%%)",
				fmtBytes(mem.SwapTotalKB*1024),
				fmtBytes(mem.SwapUsedKB*1024),
				swapPct,
			)
			row = append(row, kv("Swap", swap))
		}
		ws("Memory", row)
	}

	// Graphics
	if len(s.GPUs) > 0 {
		lines := make([][]string, len(s.GPUs))
		for i, c := range s.GPUs {
			lines[i] = []string{kv("Card", c.Model), kv("Driver", c.Driver)}
		}
		ws("Graphics", lines...)
	}

	// Audio
	audio := s.Audio
	if len(audio.Cards) > 0 || audio.Server != "" {
		var lines [][]string
		for _, c := range audio.Cards {
			lines = append(lines, []string{kv("Card", c.Name), kv("Driver", c.Driver)})
		}
		var serverParts []string
		if audio.Server != "" {
			serverParts = append(serverParts, kv("Server", audio.Server))
			if audio.SrvVer != "" {
				serverParts = append(serverParts, kv("v", audio.SrvVer))
			}
		}
		if audio.ALSA != "" {
			serverParts = append(serverParts, kv("ALSA", audio.ALSA))
		}
		if len(serverParts) > 0 {
			lines = append(lines, serverParts)
		}
		ws("Audio", lines...)
	}

	// Scheduling
	if line := s.Sched.String(); line != "" {
		ws("Sched", []string{line})
	}

	return b.String()
}

// Print writes String() to stdout.
func (s SysInfo) Print() {
	fmt.Print(s.String())
}
