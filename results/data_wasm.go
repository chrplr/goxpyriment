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
//
// When the page was served by JATOS, the two files are uploaded to the server
// instead (see finalizeToJatos), and the zip download becomes the fallback for
// an upload that fails.
func (df *DataFile) Finalize() error {
	csv := strings.Join(df.OutputFile.Buffer, "")
	info := strings.Join(df.InfoFile.Buffer, "")
	if df.UploadsToJatos() {
		err := df.finalizeToJatos(csv, info)
		df.OutputFile.Buffer = make([]string, 0)
		df.InfoFile.Buffer = make([]string, 0)
		df.sent = 0
		if err == nil {
			return nil
		}
		log.Printf("results: sending the data to JATOS failed (%v); downloading it instead", err)
		if dlErr := df.downloadZip(csv, info); dlErr != nil {
			return fmt.Errorf("results.DataFile.Finalize: %w; fallback download: %v", err, dlErr)
		}
		return fmt.Errorf("results.DataFile.Finalize: %w (data downloaded as %s instead)", err, df.ZipFilename())
	}
	df.OutputFile.Buffer = make([]string, 0)
	df.InfoFile.Buffer = make([]string, 0)
	return df.downloadZip(csv, info)
}

// downloadZip packs the CSV and info contents into one archive and hands it to
// the participant.
func (df *DataFile) downloadZip(csv, info string) error {
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
			return fmt.Errorf("results.DataFile.downloadZip: creating %q in zip: %w", member.name, err)
		}
		if _, err := w.Write([]byte(member.content)); err != nil {
			return fmt.Errorf("results.DataFile.downloadZip: writing %q to zip: %w", member.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("results.DataFile.downloadZip: closing zip: %w", err)
	}

	name := strings.TrimSuffix(df.OutputFile.Filename, ".csv") + ".zip"
	log.Printf("Saving experiment results to %s (%s, %s)...", name, df.OutputFile.Filename, df.InfoFile.Filename)
	return downloadBytes(name, buf.Bytes(), "application/zip")
}

// ZipFilename reports the name of the single archive Finalize downloads. It
// lets callers log what the participant actually receives instead of the two
// paths the desktop build writes. Under JATOS it is only the fallback's name.
func (df *DataFile) ZipFilename() string {
	return strings.TrimSuffix(df.OutputFile.Filename, ".csv") + ".zip"
}

// UploadsToJatos reports whether Finalize sends the data to a JATOS server
// rather than downloading it: true when the page was served by JATOS.
func (df *DataFile) UploadsToJatos() bool { return JatosAvailable() }

// saveRemote appends the CSV rows written since the previous Save to the
// study run's JATOS result data. It returns without waiting: the request is
// queued (see jatos_wasm.go) and Finalize waits for it.
//
// JATOS result data is plain text that each append extends, so once the
// session is over it holds the whole CSV — and if the participant closes the
// tab midway, it holds every block saved so far, which a download at the end
// never could. The info file is not appended: it would interleave with the
// rows, and it is uploaded whole by Finalize.
func (df *DataFile) saveRemote() error {
	if !JatosAvailable() || df.sent >= len(df.OutputFile.Buffer) {
		return nil
	}
	chunk := strings.Join(df.OutputFile.Buffer[df.sent:], "")
	df.sent = len(df.OutputFile.Buffer)
	jatosEnqueue("appendResultData", chunk)
	return nil
}

// finalizeToJatos completes the result data with any rows not yet appended,
// uploads the CSV and the info file as result files under their desktop names
// (with the characters JATOS refuses replaced, see JatosFilename), and waits for
// all of it to reach the server.
//
// Both representations are kept on purpose. Result data shows up in the JATOS
// GUI and survives an abandoned session, but is capped per component
// (jatos.resultData.maxSize, 5 MB by default). Result files carry the exact
// files a desktop run writes, under a much larger cap
// (jatos.resultUploads.maxFileSize, 30 MB).
func (df *DataFile) finalizeToJatos(csv, info string) error {
	if err := df.saveRemote(); err != nil {
		return err
	}
	if csv != "" {
		jatosEnqueue("uploadResultFile", csv, JatosFilename(df.OutputFile.Filename))
	}
	if info != "" {
		jatosEnqueue("uploadResultFile", info, JatosFilename(df.InfoFile.Filename))
	}
	log.Printf("Sending experiment results to JATOS (%s, %s)...",
		JatosFilename(df.OutputFile.Filename), JatosFilename(df.InfoFile.Filename))
	return jatosAwait()
}
