//go:build windows

package sysinfo

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// wmicGet runs `wmic <class> get <fields...> /value` and returns all instances.
// Each instance is a map[string]string; multiple instances are separated by blank lines.
func wmicGet(class string, fields ...string) []map[string]string {
	args := append([]string{class, "get"}, fields...)
	args = append(args, "/value")
	return parseWMICOutput(run("wmic", args...))
}

// wmicPath runs `wmic path <path> get <fields...> /value`.
func wmicPath(path string, fields ...string) []map[string]string {
	args := append([]string{"path", path, "get"}, fields...)
	args = append(args, "/value")
	return parseWMICOutput(run("wmic", args...))
}

// parseWMICOutput splits wmic /value output into one map per instance.
// Lines are "Key=Value\r\n"; blank lines separate instances.
func parseWMICOutput(out string) []map[string]string {
	var results []map[string]string
	cur := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(cur) > 0 {
				results = append(results, cur)
				cur = map[string]string{}
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		cur[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if len(cur) > 0 {
		results = append(results, cur)
	}
	return results
}

// first returns the first instance from a wmicGet/wmicPath result, or an empty map.
func first(rows []map[string]string) map[string]string {
	if len(rows) > 0 {
		return rows[0]
	}
	return map[string]string{}
}

// parseWMIDatetime parses the WMI datetime format "YYYYMMDDHHMMSS.ffffff±UUU"
// where UUU is the UTC offset in minutes.
func parseWMIDatetime(s string) (time.Time, error) {
	if len(s) < 14 {
		return time.Time{}, fmt.Errorf("wmi datetime too short: %q", s)
	}
	year, _ := strconv.Atoi(s[0:4])
	month, _ := strconv.Atoi(s[4:6])
	day, _ := strconv.Atoi(s[6:8])
	hour, _ := strconv.Atoi(s[8:10])
	min, _ := strconv.Atoi(s[10:12])
	sec, _ := strconv.Atoi(s[12:14])

	loc := time.Local
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		rest := s[dot+1:]
		for i, c := range rest {
			if c == '+' || c == '-' {
				if offsetMins, err := strconv.Atoi(rest[i+1:]); err == nil {
					sign := 1
					if c == '-' {
						sign = -1
					}
					loc = time.FixedZone("", sign*offsetMins*60)
				}
				break
			}
		}
	}
	return time.Date(year, time.Month(month), day, hour, min, sec, 0, loc), nil
}
