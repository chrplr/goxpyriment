package sysinfo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// kv returns "key: value", or "" when value is empty.
func kv(key, val string) string {
	if val == "" {
		return ""
	}
	return key + ": " + val
}

// compact removes empty strings from a slice without allocating when unneeded.
func compact(s []string) []string {
	out := s[:0:len(s)]
	for _, v := range s {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// readFile returns the trimmed content of a file, or "" on any error.
func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// probeTimeout bounds every external command this package runs.
//
// These probes (lspci, sysctl, system_profiler, wmic) are cheap and local, and
// on a healthy machine finish in milliseconds. The timeout is not there for
// them: it is there because Collect now runs at the start of every experiment,
// so a probe that blocks forever -- a wedged PCI enumeration, an unresponsive
// WMI service -- would hang the session in front of a participant rather than
// in front of whoever typed the command. A missing field in the data file is a
// far better outcome than an experiment that never starts, so the deadline is
// short and the failure is silent, like every other failure here.
const probeTimeout = 2 * time.Second

// run executes a command and returns its trimmed stdout, or "" on error or
// timeout.
func run(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func fmtDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	return strings.Join(parts, " ")
}

func fmtBytes(b int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
		GiB = 1024 * MiB
		TiB = 1024 * GiB
	)
	switch {
	case b >= TiB:
		return fmt.Sprintf("%.2f TiB", float64(b)/TiB)
	case b >= GiB:
		return fmt.Sprintf("%.2f GiB", float64(b)/GiB)
	case b >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(b)/MiB)
	default:
		return fmt.Sprintf("%d KiB", b/KiB)
	}
}
