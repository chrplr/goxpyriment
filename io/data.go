// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package io

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Default settings for OutputFile and DataFile.
const (
	OutputFileCommentChar = "#"           // Character used for comment lines in output files.
	OutputFileEOL         = "\n"          // Line ending written by WriteLine.
	DataFileDirectory     = "xpd_results" // Default directory for data files when none is set.
	DataFileDelimiter     = ","          // Default CSV delimiter for DataFile.
)

// OutputFile represents a generic buffered text file on disk.
// It is used as the backend for `DataFile` but can also be used for logs
// or any other line‑oriented output the experiment needs to produce.
type OutputFile struct {
	Filename    string
	Directory   string
	FullPath    string
	CommentChar string
	Buffer      []string
}

// NewOutputFile creates a new OutputFile in the given directory with the
// provided filename. The directory is created if it does not exist and the
// file is truncated if it already exists.
func NewOutputFile(directory, filename string) (*OutputFile, error) {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		if err := os.MkdirAll(directory, 0755); err != nil {
			return nil, err
		}
	}

	fullPath := filepath.Join(directory, filename)
	
	// Create/truncate file
	f, err := os.Create(fullPath)
	if err != nil {
		return nil, err
	}
	f.Close()

	return &OutputFile{
		Filename:    filename,
		Directory:   directory,
		FullPath:    fullPath,
		CommentChar: OutputFileCommentChar,
		Buffer:      make([]string, 0),
	}, nil
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
	o.WriteLine(o.CommentChar + comment)
}

// Save flushes the buffer to disk.
func (o *OutputFile) Save() error {
	if len(o.Buffer) == 0 {
		return nil
	}

	f, err := os.OpenFile(o.FullPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, line := range o.Buffer {
		if _, err := writer.WriteString(line); err != nil {
			return err
		}
	}
	o.Buffer = make([]string, 0)
	return writer.Flush()
}

// DataFile represents an experiment data file in CSV‑like format.
// It prepends subject ID to each row and supports quoted fields when they
// contain delimiters or quotes.
type DataFile struct {
	*OutputFile
	Delimiter     string
	SubjectID     int
	VariableNames []string
}

// NewDataFile creates a new DataFile in the given directory (or in the
// default directory from DataFileDirectory, e.g. "xpd_results", if empty).
// The filename is derived from the experiment name, subject ID, and a timestamp.
func NewDataFile(directory string, subjectID int, expName string) (*DataFile, error) {
	if directory == "" {
		directory = DataFileDirectory
	}
	
	timestamp := time.Now().Format("200601021504")
	filename := fmt.Sprintf("%s_%03d_%s.xpd", expName, subjectID, timestamp)
	
	base, err := NewOutputFile(directory, filename)
	if err != nil {
		return nil, err
	}

	df := &DataFile{
		OutputFile:    base,
		Delimiter:     DataFileDelimiter,
		SubjectID:     subjectID,
		VariableNames: make([]string, 0),
	}

	df.WriteComment("--EXPERIMENT INFO")
	df.WriteComment(fmt.Sprintf("e mainfile: %s", expName))
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
// Fields containing the delimiter or quotes are properly escaped.
func (df *DataFile) Add(data []interface{}) {
	parts := make([]string, 0, len(data)+1)
	parts = append(parts, fmt.Sprint(df.SubjectID))
	
	for _, d := range data {
		s := fmt.Sprint(d)
		if strings.Contains(s, df.Delimiter) || strings.Contains(s, "\"") {
			s = strings.ReplaceAll(s, "\"", "\"\"")
			s = fmt.Sprintf("\"%s\"", s)
		}
		parts = append(parts, s)
	}
	
	df.WriteLine(strings.Join(parts, df.Delimiter))
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
