//go:build !windows

package sysinfo

import (
	"bufio"
	"runtime"
	"strconv"
	"strings"
)

func collectAudio() AudioInfo {
	switch runtime.GOOS {
	case "linux":
		return linuxAudioInfo()
	case "darwin":
		return darwinAudioInfo()
	}
	return AudioInfo{}
}

// linuxAudioInfo reads /proc/asound/cards and /proc/asound/modules for card
// names and kernel modules, then detects the active sound server.
func linuxAudioInfo() AudioInfo {
	cards := parseProcAsoundCards()

	// Read driver modules from /proc/asound/modules (index → module).
	modules := map[int]string{}
	if f := readFile("/proc/asound/modules"); f != "" {
		sc := bufio.NewScanner(strings.NewReader(f))
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) >= 2 {
				if idx, err := strconv.Atoi(fields[0]); err == nil {
					modules[idx] = fields[1]
				}
			}
		}
	}
	for i := range cards {
		if mod, ok := modules[i]; ok {
			cards[i].Driver = mod
		}
	}

	srv, ver := detectSoundServer()

	alsa := ""
	if v := readFile("/proc/asound/version"); v != "" {
		// "Advanced Linux Sound Architecture Driver Version k6.17.0-19-generic"
		if fields := strings.Fields(v); len(fields) > 0 {
			alsa = strings.TrimRight(fields[len(fields)-1], ".")
		}
	}

	return AudioInfo{Cards: cards, Server: srv, SrvVer: ver, ALSA: alsa}
}

// parseProcAsoundCards parses /proc/asound/cards lines like:
//
//	0 [sofsoundwire   ]: sof-soundwire - sof-soundwire (DellInc.-Precision5490)
//	1 [StreamCam      ]: USB-Audio - Logitech StreamCam
func parseProcAsoundCards() []AudioCard {
	raw := readFile("/proc/asound/cards")
	if raw == "" {
		return nil
	}
	var cards []AudioCard
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		// Skip lines that start with a non-digit (continuation lines).
		if _, err := strconv.Atoi(fields[0]); err != nil {
			continue
		}
		// Description is after " - ": take everything after the last " - ".
		name := ""
		if idx := strings.LastIndex(line, " - "); idx >= 0 {
			name = strings.TrimSpace(line[idx+3:])
		}
		if name == "" {
			// Fallback: use the bracket identifier.
			if start := strings.Index(line, "["); start >= 0 {
				if end := strings.Index(line[start:], "]"); end >= 0 {
					name = strings.TrimSpace(line[start+1 : start+end])
				}
			}
		}
		if name != "" {
			cards = append(cards, AudioCard{Name: name})
		}
	}
	return cards
}

// detectSoundServer checks for PipeWire, PulseAudio, and JACK in that order.
func detectSoundServer() (name, version string) {
	if v := run("pipewire", "--version"); v != "" {
		// "pipewire\nCompiled with libpipewire 1.4.7\nLinked with libpipewire 1.4.7"
		ver := ""
		for _, line := range strings.Split(v, "\n") {
			if strings.Contains(line, "Compiled with libpipewire") {
				if fields := strings.Fields(line); len(fields) > 0 {
					ver = fields[len(fields)-1]
				}
				break
			}
		}
		return "PipeWire", ver
	}
	if v := run("pulseaudio", "--version"); v != "" {
		// "pulseaudio 15.99.1"
		fields := strings.Fields(v)
		ver := ""
		if len(fields) >= 2 {
			ver = fields[1]
		}
		return "PulseAudio", ver
	}
	if v := run("jackd", "--version"); v != "" {
		fields := strings.Fields(v)
		ver := ""
		for i, f := range fields {
			if f == "version" && i+1 < len(fields) {
				ver = fields[i+1]
				break
			}
		}
		return "JACK", ver
	}
	return "", ""
}

// darwinAudioInfo parses `system_profiler SPAudioDataType` for device names.
func darwinAudioInfo() AudioInfo {
	out := run("system_profiler", "SPAudioDataType")
	if out == "" {
		return AudioInfo{}
	}
	var cards []AudioCard
	sc := bufio.NewScanner(strings.NewReader(out))
	// Devices appear as indented section headers followed by key:value lines.
	// A device name line has trailing ":" and is indented with exactly 8 spaces.
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "Audio:" || trimmed == "Devices:" {
			continue
		}
		// Device name lines end with ":" and have no "  key:" sub-structure
		// pattern — they are the section header (8-space indent).
		if strings.HasSuffix(trimmed, ":") && strings.HasPrefix(line, "        ") &&
			!strings.HasPrefix(line, "          ") {
			name := strings.TrimSuffix(trimmed, ":")
			cards = append(cards, AudioCard{Name: name})
		}
	}
	return AudioInfo{Cards: cards, Server: "CoreAudio"}
}
