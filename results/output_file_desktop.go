//go:build !js

// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package results

import (
	"bufio"
	"os"
	"path/filepath"
)

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
