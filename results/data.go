// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package results

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/chrplr/goxpyriment/apparatus"
)

// Default settings for OutputFile and DataFile.
const (
	OutputFileCommentChar = "#"          // Character used for comment lines in output files.
	OutputFileEOL         = "\n"         // Line ending written by WriteLine.
	DataFileDirectory     = "goxpy_data" // Default directory for data files when none is set.
	DataFileDelimiter     = ","          // Default CSV delimiter for DataFile.
)

// OutputFile represents a generic buffered text file.
// It is used as the backend for `DataFile` but can also be used for logs
// or any other line‑oriented output the experiment needs to produce.
type OutputFile struct {
	Filename    string
	Directory   string
	FullPath    string
	CommentChar string
	Buffer      []string
}

// Write adds content to the buffer.
func (o *OutputFile) Write(content string) {
	o.Buffer = append(o.Buffer, content)
}

// WriteLine adds content followed by EOL to the buffer.
func (o *OutputFile) WriteLine(content string) {
	o.Write(content + OutputFileEOL)
}

// WriteComment adds a comment line to the buffer.
func (o *OutputFile) WriteComment(comment string) {
	o.WriteLine(o.CommentChar + " " + comment)
}

// DataFile represents an experiment data file in CSV‑like format.
// It prepends subject ID to each row and supports quoted fields when they
// contain delimiters or quotes.
type DataFile struct {
	*OutputFile
	Delimiter     string
	SubjectID     int
	VariableNames []string
	StartTime     time.Time
}

// NewDataFile creates a new DataFile in the given directory (or in the
// default directory from DataFileDirectory, e.g. "$HOME/goxpy_data", if empty).
// The filename is derived from the experiment name, subject ID, and a timestamp.
func NewDataFile(directory string, subjectID int, expName string) (*DataFile, error) {
	if directory == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			directory = filepath.Join(home, DataFileDirectory)
		} else {
			directory = DataFileDirectory
		}
	}

	now := time.Now()
	filename := fmt.Sprintf("%s_sub-%03d_date-%s-%s.csv", expName, subjectID, now.Format("20060102"), now.Format("1504"))

	base, err := NewOutputFile(directory, filename)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	df := &DataFile{
		OutputFile:    base,
		Delimiter:     DataFileDelimiter,
		SubjectID:     subjectID,
		VariableNames: make([]string, 0),
		StartTime:     start,
	}

	df.WriteComment("--EXPERIMENT INFO")
	df.WriteComment(fmt.Sprintf("e mainfile: %s", expName))
	df.WriteComment(fmt.Sprintf("e cmdline: %s", strings.Join(os.Args, " ")))
	df.WriteComment(fmt.Sprintf("e start_time: %s", start.Format("20060102-150405")))
	if hostname, err := os.Hostname(); err == nil {
		df.WriteComment(fmt.Sprintf("e hostname: %s", hostname))
	}
	if u, err := user.Current(); err == nil {
		df.WriteComment(fmt.Sprintf("e username: %s", u.Username))
	}
	df.WriteComment(fmt.Sprintf("e os: %s/%s", runtime.GOOS, runtime.GOARCH))
	df.WriteComment(fmt.Sprintf("e framework: goxpyriment %s --- see http://chrplr.github.io/goxpyriment", Version))
	df.WriteComment("e framework_author: Christophe Pallier <christophe.pallier.org>")
	df.WriteComment("--SUBJECT INFO")
	df.WriteComment(fmt.Sprintf("s id: %d", subjectID))
	df.WriteComment("#")

	if err := df.Save(); err != nil {
		return nil, err
	}

	return df, nil
}

// Add appends a row of data to the data file.
// The subject ID is automatically prepended as the first column.
// Numeric and boolean fields are written as-is; all other fields are
// always quoted (with internal double-quotes doubled per RFC 4180).
func (df *DataFile) Add(data ...interface{}) {
	parts := make([]string, 0, len(data)+1)
	parts = append(parts, fmt.Sprint(df.SubjectID))

	for _, d := range data {
		s := fmt.Sprint(d)
		switch d.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64, bool:
			parts = append(parts, s)
		default:
			s = strings.ReplaceAll(s, "\"", "\"\"")
			parts = append(parts, fmt.Sprintf("\"%s\"", s))
		}
	}

	df.WriteLine(strings.Join(parts, df.Delimiter))
}

// WriteSystemInfo appends SDL, renderer, and audio runtime properties as
// comment lines under a --SYSTEM INFO section. Called automatically by
// Experiment.Initialize() so every data file carries a complete record of the
// software and hardware configuration used during the session.
func (df *DataFile) WriteSystemInfo(info apparatus.SystemInfo) {
	df.WriteComment("--SYSTEM INFO")
	df.WriteComment(fmt.Sprintf("sys sdl_version: %s", info.SDLVersion))
	df.WriteComment(fmt.Sprintf("sys video_driver: %s", info.VideoDriver))
	df.WriteComment(fmt.Sprintf("sys renderer: %s", info.RendererName))
	df.WriteComment(fmt.Sprintf("sys physical_resolution: %dx%d px", info.PhysicalW, info.PhysicalH))
	df.WriteComment(fmt.Sprintf("sys logical_resolution: %dx%d px", info.LogicalW, info.LogicalH))
	df.WriteComment(fmt.Sprintf("sys fullscreen: %v", info.Fullscreen))
	df.WriteComment(fmt.Sprintf("sys vsync: %d", info.VSync))
	df.WriteComment(fmt.Sprintf("sys audio_driver: %s", info.AudioDriver))
	df.WriteComment(fmt.Sprintf("sys audio_format: %s", info.AudioFormat))
	df.WriteComment(fmt.Sprintf("sys audio_sample_rate_hz: %d", info.AudioFreq))
	df.WriteComment(fmt.Sprintf("sys audio_channels: %d", info.AudioChannels))
	df.WriteComment(fmt.Sprintf("sys audio_buffer_frames: %d", info.AudioFrames))
}

// WriteDisplayInfo appends display properties as comment lines in the metadata
// header so that the physical display configuration is preserved alongside the
// trial data for later analysis.
func (df *DataFile) WriteDisplayInfo(info apparatus.DisplayInfo) {
	df.WriteComment("--DISPLAY INFO")
	df.WriteComment(fmt.Sprintf("d id: %d", info.ID))
	df.WriteComment(fmt.Sprintf("d name: %s", info.Name))
	df.WriteComment(fmt.Sprintf("d native_resolution: %dx%d", info.NativeW, info.NativeH))
	df.WriteComment(fmt.Sprintf("d pixel_density: %.2f", info.PixelDensity))
	df.WriteComment(fmt.Sprintf("d content_scale: %.2f", info.ContentScale))
	df.WriteComment(fmt.Sprintf("d refresh_rate_hz: %.4f", info.RefreshRate))
	df.WriteComment(fmt.Sprintf("d bits_per_pixel: %d", info.BitsPerPixel))
	df.WriteComment(fmt.Sprintf("d bits_per_channel: %d", info.BitsPerChannel))
	df.WriteComment(fmt.Sprintf("d pixel_format: %s", info.PixelFormat))
	df.WriteComment(fmt.Sprintf("d bounds: %d,%d %dx%d", info.BoundsX, info.BoundsY, info.BoundsW, info.BoundsH))
}

// WriteParticipantInfo appends collected participant/session fields as comment
// lines under a --PARTICIPANT INFO section. Keys are written in sorted order so
// that the header is deterministic regardless of map iteration order.
// This is called automatically by Experiment.Initialize() when exp.Info is set.
func (df *DataFile) WriteParticipantInfo(info map[string]string) {
	if len(info) == 0 {
		return
	}
	keys := make([]string, 0, len(info))
	for k := range info {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	df.WriteComment("--PARTICIPANT INFO")
	for _, k := range keys {
		df.WriteComment(fmt.Sprintf("p %s: %s", k, info[k]))
	}
}

// WriteEndTime appends end-time and duration lines to the EXPERIMENT INFO
// section. It should be called once, just before the final Save.
func (df *DataFile) WriteEndTime() {
	end := time.Now()
	d := end.Sub(df.StartTime)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	df.WriteComment(fmt.Sprintf("e end_time: %s", end.Format("20060102-150405.000")))
	df.WriteComment(fmt.Sprintf("e duration: %02d:%02d:%02d.%03d", h, m, s, ms))
}

// AddVariableNames appends variable names and writes a header comment.
// This should typically be called once near the start of an experiment to
// document the column structure of subsequent calls to Add.
func (df *DataFile) AddVariableNames(names []string) {
	df.VariableNames = append(df.VariableNames, names...)
	// In Expyriment, this usually re-writes the header.
	header := "subject_id"
	if len(df.VariableNames) > 0 {
		header += df.Delimiter + strings.Join(df.VariableNames, df.Delimiter)
	}
	// Write the header as a plain CSV line (no leading comment character)
	// so that spreadsheet programs can automatically detect column names.
	df.WriteLine(header)
}
