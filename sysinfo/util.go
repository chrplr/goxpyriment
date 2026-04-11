package sysinfo

import (
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

// run executes a command and returns its trimmed stdout, or "" on error.
func run(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
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
