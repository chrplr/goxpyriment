// Package sysinfo collects and formats hardware and software information about
// the running system (machine type, OS, CPU, memory, GPU, audio).
// It is a library adaptation of the go-inxi CLI tool.
package sysinfo

import (
	"fmt"
	"strings"
	"sync"
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

// hostOnce guards the cached host snapshot returned by Host.
var (
	hostOnce  sync.Once
	hostCache SysInfo
)

// Host returns the static machine and OS facts, collected at most once per
// process.
//
// It is cached and it is meant to be called EARLY -- before a program raises
// itself to a real-time scheduling policy. Collect shells out to lspci and
// friends, and a fork inherits the caller's policy, so collecting after the
// elevation runs a PCI enumeration at SCHED_FIFO. Priming this cache first
// keeps that fork at ordinary priority and leaves the later, timing-sensitive
// call free of any fork at all.
//
// The Sched field of the returned value is whatever was in force at the first
// call, which is usually not what the experiment ran under. Read scheduling
// with Scheduling() instead, which is live.
func Host() SysInfo {
	hostOnce.Do(func() { hostCache = Collect() })
	return hostCache
}

// PrimeHost starts collecting the host snapshot in the background and returns
// immediately. A later Host() blocks until it is ready.
//
// Two reasons to call this first thing in a program's start-up, both measured:
//
//   - It is not free. The first lspci after boot took 2.07 s on a Precision
//     5490 (0.04 s warm), and that is time a participant spends looking at
//     nothing. Overlapped with SDL initialisation, window creation and font
//     loading -- which have to happen anyway -- it disappears.
//   - The fork must not inherit a real-time policy. Linux applies
//     sched_setscheduler(0, ...) to the calling THREAD, so a collection started
//     before the main thread elevates itself stays at ordinary priority even if
//     it is still running afterwards.
func PrimeHost() {
	go Host()
}

// Fields returns the host facts worth recording in a data file, as ordered
// key/value pairs with empty values already dropped.
//
// It is deliberately narrower than String(). Two rules decide what is here:
//
//   - Only what a stimulus program cannot report about itself. SDL already
//     names the display mode, the audio device it opened and the renderer it
//     got, and those are recorded beside these; repeating them from a second
//     source only creates two numbers that can disagree.
//   - Only what changes results. The compositor, the sound server version, the
//     kernel and the set of GPUs present are all conditions under which a
//     measurement was taken -- on this framework's own hardware the display
//     stack moved onset precision by an order of magnitude. Uptime and the
//     login shell are not, and are left to String().
func (s SysInfo) Fields() [][2]string {
	var f [][2]string
	add := func(k, v string) {
		if v != "" {
			f = append(f, [2]string{k, v})
		}
	}

	m := s.Machine
	add("machine", strings.TrimSpace(strings.Join(compact([]string{
		m.SysVendor, m.ProductName, m.ProductVersion,
	}), " ")))
	add("machine_type", m.DeviceType)

	add("os", s.System.OS)
	add("kernel", strings.TrimSpace(s.System.Kernel+" "+s.System.Arch))
	// The single largest effect measured with this framework was the display
	// stack. SDL reports "wayland"; which compositor it was is only here.
	add("desktop", s.System.Desktop)

	cpu := s.CPU
	add("cpu", cpu.Model)
	if cpu.Cores > 0 {
		topology := fmt.Sprintf("%d cores", cpu.Cores)
		if cpu.Threads > cpu.Cores {
			topology = fmt.Sprintf("%d cores / %d threads", cpu.Cores, cpu.Threads)
		}
		add("cpu_topology", topology)
	}
	// Current clock against maximum: a throttled machine times differently, and
	// nothing else in the file would show it.
	if cpu.MHz > 0 && cpu.MaxMHz > 0 {
		add("cpu_mhz", fmt.Sprintf("%.0f (max %.0f)", cpu.MHz, cpu.MaxMHz))
	}

	if s.Memory.TotalKB > 0 {
		ram := fmtBytes(s.Memory.TotalKB * 1024)
		if s.Memory.UsedKB > 0 {
			ram += fmt.Sprintf(" (%.0f%% used at start)",
				float64(s.Memory.UsedKB)/float64(s.Memory.TotalKB)*100)
		}
		add("ram", ram)
	}

	// Every card, not just the one SDL rendered on: a laptop with a second,
	// switchable GPU can be offloading, and gl_renderer names only the winner.
	for i, g := range s.GPUs {
		key := "gpu"
		if len(s.GPUs) > 1 {
			key = fmt.Sprintf("gpu%d", i)
		}
		add(key, strings.TrimSpace(g.Model+kvSuffix(" [driver ", g.Driver, "]")))
	}

	for i, c := range s.Audio.Cards {
		key := "audio_card"
		if len(s.Audio.Cards) > 1 {
			key = fmt.Sprintf("audio_card%d", i)
		}
		add(key, strings.TrimSpace(c.Name+kvSuffix(" [driver ", c.Driver, "]")))
	}
	// The sound server and its version, not merely SDL's backend name: a server
	// that silently grows the buffer mid-run does so by version.
	add("audio_server", strings.TrimSpace(s.Audio.Server+kvSuffix(" ", s.Audio.SrvVer, "")))
	add("alsa", s.Audio.ALSA)

	return f
}

// kvSuffix returns prefix+val+suffix, or "" when val is empty.
func kvSuffix(prefix, val, suffix string) string {
	if val == "" {
		return ""
	}
	return prefix + val + suffix
}
