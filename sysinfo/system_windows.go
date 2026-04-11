//go:build windows

package sysinfo

import (
	"os"
	"path/filepath"
	"time"
)

func collectSystem() SystemInfo {
	host, _ := os.Hostname()
	info := first(wmicGet("OS", "Caption", "Version", "LastBootUpTime"))

	uptime := ""
	if boot, err := parseWMIDatetime(info["LastBootUpTime"]); err == nil {
		uptime = fmtDuration(time.Since(boot))
	}

	return SystemInfo{
		Hostname: host,
		OS:       info["Caption"],
		Kernel:   info["Version"],
		Uptime:   uptime,
		Shell:    currentShell(),
	}
}

func currentShell() string {
	// PowerShell sets PSModulePath; cmd.exe does not.
	if os.Getenv("PSModulePath") != "" {
		return "PowerShell"
	}
	if comspec := os.Getenv("COMSPEC"); comspec != "" {
		return filepath.Base(comspec)
	}
	return "cmd.exe"
}
