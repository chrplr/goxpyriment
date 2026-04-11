//go:build !windows

package sysinfo

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func collectSystem() SystemInfo {
	host, _ := os.Hostname()
	kernel, arch := kernelAndArch()
	return SystemInfo{
		Hostname: host,
		Kernel:   kernel,
		Arch:     arch,
		Uptime:   sysUptime(),
		OS:       distro(),
		Shell:    shellInfo(),
		Desktop:  desktop(),
	}
}

func kernelAndArch() (kernel, arch string) {
	// Linux: /proc/version  →  "Linux version 5.15.0 (...)"
	if content := readFile("/proc/version"); content != "" {
		fields := strings.Fields(content)
		if len(fields) >= 3 {
			kernel = fields[2]
		}
	}
	if kernel == "" {
		kernel = run("uname", "-r")
	}
	arch = run("uname", "-m")
	if arch == "" {
		arch = runtime.GOARCH
	}
	return
}

func sysUptime() string {
	if runtime.GOOS == "linux" {
		if content := readFile("/proc/uptime"); content != "" {
			fields := strings.Fields(content)
			if len(fields) > 0 {
				if secs, err := strconv.ParseFloat(fields[0], 64); err == nil {
					return fmtDuration(time.Duration(secs) * time.Second)
				}
			}
		}
	}
	// BSD / macOS: sysctl kern.boottime  →  "{ sec = 1700000000, usec = 0 } ..."
	out := run("sysctl", "-n", "kern.boottime")
	for _, part := range strings.Fields(out) {
		part = strings.TrimRight(part, ",")
		if after, ok := strings.CutPrefix(part, "sec="); ok {
			if sec, err := strconv.ParseInt(after, 10, 64); err == nil {
				return fmtDuration(time.Since(time.Unix(sec, 0)))
			}
		}
	}
	return ""
}

func distro() string {
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if after, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
				return strings.Trim(after, `"`)
			}
		}
	}
	if runtime.GOOS == "darwin" {
		name := run("sw_vers", "-productName")
		ver := run("sw_vers", "-productVersion")
		return strings.TrimSpace(name + " " + ver)
	}
	return ""
}

func shellInfo() string {
	s := os.Getenv("SHELL")
	if s == "" {
		return ""
	}
	name := filepath.Base(s)
	ver := shellVersion(name)
	if ver != "" {
		return name + " " + ver
	}
	return name
}

func shellVersion(name string) string {
	switch name {
	case "bash":
		// "GNU bash, version 5.1.16(1)-release ..."
		out := run("bash", "--version")
		for i, f := range strings.Fields(out) {
			if f == "version" {
				fields := strings.Fields(out)
				if i+1 < len(fields) {
					v := fields[i+1]
					if idx := strings.IndexByte(v, '('); idx >= 0 {
						v = v[:idx]
					}
					return v
				}
			}
		}
	case "zsh":
		// "zsh 5.9 (...)"
		out := run("zsh", "--version")
		if fields := strings.Fields(out); len(fields) >= 2 {
			return fields[1]
		}
	case "fish":
		// "fish, version 3.6.4"
		out := run("fish", "--version")
		if fields := strings.Fields(out); len(fields) >= 3 {
			return fields[len(fields)-1]
		}
	}
	return ""
}

func desktop() string {
	for _, env := range []string{"XDG_CURRENT_DESKTOP", "DESKTOP_SESSION"} {
		if val := os.Getenv(env); val != "" {
			return val
		}
	}
	return ""
}
