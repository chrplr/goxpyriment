//go:build js

// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

package results

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"
)

// Finalize packs the CSV and the companion info file into a single .zip and
// triggers one browser download.
//
// It must stay one download. Firing two in a row loses the second whenever the
// browser is set to ask where to save each file: the two downloads are
// serialized into two modal Save-As dialogs, only the first is shown, and the
// one still queued is cancelled as soon as the page goes away — which is
// exactly what a participant does when the session ends. Chrome records the
// casualty as state=CANCELLED, interrupt_reason=USER_CANCELED, 0 bytes
// received and no chosen filename. Measured 2026-08-29 against Chrome's own
// downloads table; every browser session since July had silently lost its
// results file this way while keeping the metadata.
//
// The two members keep the names and the byte-for-byte content they have on
// desktop, so unzipping a browser session yields the same pair of files a
// native run writes.
func (df *DataFile) Finalize() error {
	csv := strings.Join(df.OutputFile.Buffer, "")
	info := strings.Join(df.InfoFile.Buffer, "")
	df.OutputFile.Buffer = make([]string, 0)
	df.InfoFile.Buffer = make([]string, 0)
	if csv == "" && info == "" {
		return nil
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	now := time.Now()
	for _, member := range []struct{ name, content string }{
		{df.InfoFile.Filename, info},
		{df.OutputFile.Filename, csv},
	} {
		if member.content == "" {
			continue
		}
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:     member.name,
			Method:   zip.Deflate,
			Modified: now,
		})
		if err != nil {
			return fmt.Errorf("results.DataFile.Finalize: creating %q in zip: %w", member.name, err)
		}
		if _, err := w.Write([]byte(member.content)); err != nil {
			return fmt.Errorf("results.DataFile.Finalize: writing %q to zip: %w", member.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("results.DataFile.Finalize: closing zip: %w", err)
	}

	name := strings.TrimSuffix(df.OutputFile.Filename, ".csv") + ".zip"
	log.Printf("Saving experiment results to %s (%s, %s)...", name, df.OutputFile.Filename, df.InfoFile.Filename)
	return downloadBytes(name, buf.Bytes(), "application/zip")
}

// ZipFilename reports the name of the single archive Finalize downloads. It
// lets callers log what the participant actually receives instead of the two
// paths the desktop build writes.
func (df *DataFile) ZipFilename() string {
	return strings.TrimSuffix(df.OutputFile.Filename, ".csv") + ".zip"
}
