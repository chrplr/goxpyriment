// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package control

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
)

// EventLogEntry represents a single high-level event in the experiment
// timeline (e.g., "trial started", "response recorded", etc.).
// This struct is intentionally minimal; users of the library can extend it
// or store additional data in the Details field.
type EventLogEntry struct {
	Timestamp string // RFC3339 timestamp of the event
	Event     string // short event label
	Details   string // optional human-readable description or JSON payload
}

// EventLog summarizes an experiment run, including system configuration and
// environment information that can be useful for debugging or reproducibility.
//
// Some fields (SubjectID, CSVHeader, Entries, StartTime, EndTime, Completed)
// are owned by the user of the library and are not automatically managed
// by the Experiment. System-related fields are populated by CollectEventLog.
type EventLog struct {
	SubjectID         string
	CSVHeader         []string
	Entries           []EventLogEntry
	StartTime         string
	EndTime           string
	Completed         bool
	SDLVersion        string
	Platform          string
	Hostname          string
	Username          string
	VideoDriver       string
	AudioDriver       string
	Renderer          string
	DisplayMode       string
	LogicalResolution string
	Font              string
	FontSize          int
	CommandLine       string
}

// SetSubjectID sets the subject identifier associated with this log.
func (l *EventLog) SetSubjectID(id string) {
	l.SubjectID = id
}

// SetCSVHeader sets the CSV header (column names) associated with the
// trial/response data this log describes.
func (l *EventLog) SetCSVHeader(header []string) {
	l.CSVHeader = append([]string(nil), header...)
}

// SetEntries replaces the current list of EventLogEntry with the provided
// slice. Callers can also manipulate Entries directly if they prefer.
func (l *EventLog) SetEntries(entries []EventLogEntry) {
	l.Entries = append([]EventLogEntry(nil), entries...)
}

// SetStartTime sets the StartTime field. Callers typically use an RFC3339
// string such as time.Now().Format(time.RFC3339).
func (l *EventLog) SetStartTime(ts string) {
	l.StartTime = ts
}

// SetEndTime sets the EndTime field. Callers typically use an RFC3339
// string such as time.Now().Format(time.RFC3339).
func (l *EventLog) SetEndTime(ts string) {
	l.EndTime = ts
}

// SetCompleted marks whether the experiment run associated with this log
// finished successfully (true) or not (false).
func (l *EventLog) SetCompleted(done bool) {
	l.Completed = done
}

// CollectEventLog gathers information about the current system, SDL setup,
// and Experiment display/audio configuration and returns an EventLog
// populated with those fields.
//
// The caller is responsible for filling in SubjectID, CSVHeader, Entries,
// StartTime, EndTime and Completed as appropriate for their experiment.
func (e *Experiment) CollectEventLog() EventLog {
	var log EventLog

	// SDL version
	v := sdl.GetVersion()
	log.SDLVersion = fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch())

	// Platform
	log.Platform = runtime.GOOS

	// Hostname and username
	if hn, err := os.Hostname(); err == nil {
		log.Hostname = hn
	}
	if u, err := user.Current(); err == nil {
		log.Username = u.Username
	}

	// SDL drivers
	log.VideoDriver = sdl.GetCurrentVideoDriver()
	log.AudioDriver = sdl.GetCurrentAudioDriver()

	// Renderer and display information
	if e.Screen != nil {
		if e.Screen.Renderer != nil {
			if name, err := e.Screen.Renderer.Name(); err == nil {
				log.Renderer = name
			}
			if w, h, err := e.Screen.Renderer.RenderOutputSize(); err == nil {
				log.DisplayMode = fmt.Sprintf("%dx%d", w, h)
			}
		}

		if e.Screen.LogicalSize != nil {
			log.LogicalResolution = fmt.Sprintf("%dx%d",
				int(e.Screen.LogicalSize.X), int(e.Screen.LogicalSize.Y))
		}

		// Attempt to infer font name/size from the experiment's default font
		if e.DefaultFont != nil {
			// We don't have a robust font "name" here, but we can at least
			// record that a default font is present.
			log.Font = "default"
			if sz, err := e.DefaultFont.Size(); err == nil {
				log.FontSize = int(sz)
			}
		}
	}

	// Command line
	log.CommandLine = strings.Join(os.Args, " ")

	// Provide a reasonable default StartTime if the user hasn't set one yet.
	if log.StartTime == "" {
		log.StartTime = time.Now().Format(time.RFC3339)
	}

	return log
}

// String returns a human-readable representation of the EventLog suitable
// for inclusion at the top of results files. Each "field: value" pair is
// placed on its own line, and lines are separated by "\n#":
//
//	#SubjectID: ...
//	#CSVHeader: ...
//	#SDLVersion: ...
//
// Slice fields such as CSVHeader are rendered in a compact form
// (comma‑separated for CSVHeader, and a simple count for Entries).
func (l EventLog) String() string {
	var lines []string

	lines = append(lines, fmt.Sprintf("SubjectID: %s", l.SubjectID))
	if len(l.CSVHeader) > 0 {
		lines = append(lines, fmt.Sprintf("CSVHeader: %s", strings.Join(l.CSVHeader, ",")))
	}
	lines = append(lines, fmt.Sprintf("Entries: %d", len(l.Entries)))
	if l.StartTime != "" {
		lines = append(lines, fmt.Sprintf("StartTime: %s", l.StartTime))
	}
	if l.EndTime != "" {
		lines = append(lines, fmt.Sprintf("EndTime: %s", l.EndTime))
	}
	lines = append(lines, fmt.Sprintf("Completed: %v", l.Completed))
	if l.SDLVersion != "" {
		lines = append(lines, fmt.Sprintf("SDLVersion: %s", l.SDLVersion))
	}
	if l.Platform != "" {
		lines = append(lines, fmt.Sprintf("Platform: %s", l.Platform))
	}
	if l.Hostname != "" {
		lines = append(lines, fmt.Sprintf("Hostname: %s", l.Hostname))
	}
	if l.Username != "" {
		lines = append(lines, fmt.Sprintf("Username: %s", l.Username))
	}
	if l.VideoDriver != "" {
		lines = append(lines, fmt.Sprintf("VideoDriver: %s", l.VideoDriver))
	}
	if l.AudioDriver != "" {
		lines = append(lines, fmt.Sprintf("AudioDriver: %s", l.AudioDriver))
	}
	if l.Renderer != "" {
		lines = append(lines, fmt.Sprintf("Renderer: %s", l.Renderer))
	}
	if l.DisplayMode != "" {
		lines = append(lines, fmt.Sprintf("DisplayMode: %s", l.DisplayMode))
	}
	if l.LogicalResolution != "" {
		lines = append(lines, fmt.Sprintf("LogicalResolution: %s", l.LogicalResolution))
	}
	if l.Font != "" {
		lines = append(lines, fmt.Sprintf("Font: %s", l.Font))
	}
	if l.FontSize != 0 {
		lines = append(lines, fmt.Sprintf("FontSize: %d", l.FontSize))
	}
	if l.CommandLine != "" {
		lines = append(lines, fmt.Sprintf("CommandLine: %s", l.CommandLine))
	}

	if len(lines) == 0 {
		return ""
	}

	return "#" + strings.Join(lines, "\n#")
}
