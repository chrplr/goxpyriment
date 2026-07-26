// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

// gen-design generates the two RSVP design CSVs for the Hebart et al. (2023)
// replication: one for MEG (image 500 ms + fixation 1000±200 ms ⇒ SOA
// 1500±200 ms, jittered) and one for fMRI (image 500 ms + fixation 4 s ⇒ SOA
// 4.5 s, fixed). A few images are randomly marked as "catch" (oddball) trials,
// which the presentation program pixelates at runtime.
//
// Each CSV has the columns expected by the presentation program:
//
//	onset,duration,trial_type,file_path     (onset/duration in seconds)
//
// Usage (from the repo root):
//
//	go run ./examples/RSVP-Images/cmd/gen-design \
//	    -images examples/RSVP-Images/images -out examples/RSVP-Images
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
)

const (
	imageDurationSec = 0.5 // image-on time, both variants
	megSOAMeanSec    = 1.5 // MEG mean stimulus-onset asynchrony
	megSOAJitterSec  = 0.2 // MEG SOA jitter (uniform ±)
	fmriSOASec       = 4.5 // fMRI fixed stimulus-onset asynchrony
)

func main() {
	imagesDir := flag.String("images", "./images", "directory containing the stimulus images")
	outDir := flag.String("out", ".", "directory to write design_meg.csv and design_fmri.csv")
	ncatch := flag.Int("ncatch", 3, "number of images marked as catch (oddball) trials")
	seed := flag.Uint64("seed", 0, "random seed (0 = use a time-based seed)")
	flag.Parse()

	// Collect every image in the images directory.
	var files []string
	for _, pattern := range []string{"*.jpg", "*.jpeg", "*.png", "*.bmp"} {
		matches, err := filepath.Glob(filepath.Join(*imagesDir, pattern))
		if err != nil {
			log.Fatalf("globbing %s: %v", pattern, err)
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		log.Fatalf("no images found in %s (expected .jpg/.jpeg/.png/.bmp)", *imagesDir)
	}
	if *ncatch < 1 || *ncatch >= len(files) {
		log.Fatalf("ncatch must be between 1 and %d, got %d", len(files)-1, *ncatch)
	}

	var rng *rand.Rand
	if *seed == 0 {
		rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	} else {
		rng = rand.New(rand.NewPCG(*seed, *seed))
	}

	// Present the images in a fresh random order.
	rng.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })

	// Randomly choose which (shuffled) positions are catch trials.
	trialType := make([]string, len(files))
	for i := range trialType {
		trialType[i] = "exp"
	}
	for _, idx := range rng.Perm(len(files))[:*ncatch] {
		trialType[idx] = "catch"
	}

	// MEG onsets: jittered SOA accumulated; fMRI onsets: fixed SOA.
	megOnsets := make([]float64, len(files))
	fmriOnsets := make([]float64, len(files))
	for i := range files {
		if i > 0 {
			soa := megSOAMeanSec + (rng.Float64()*2-1)*megSOAJitterSec
			megOnsets[i] = megOnsets[i-1] + soa
			fmriOnsets[i] = fmriOnsets[i-1] + fmriSOASec
		}
	}

	megPath := filepath.Join(*outDir, "design_meg.csv")
	fmriPath := filepath.Join(*outDir, "design_fmri.csv")
	if err := writeDesign(megPath, files, trialType, megOnsets); err != nil {
		log.Fatalf("writing %s: %v", megPath, err)
	}
	if err := writeDesign(fmriPath, files, trialType, fmriOnsets); err != nil {
		log.Fatalf("writing %s: %v", fmriPath, err)
	}

	fmt.Printf("wrote %s and %s (%d trials, %d catch)\n", megPath, fmriPath, len(files), *ncatch)
}

// writeDesign emits a design CSV with columns onset,duration,trial_type,file_path.
func writeDesign(path string, files, trialType []string, onsets []float64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"onset", "duration", "trial_type", "file_path"}); err != nil {
		return err
	}
	for i, file := range files {
		rec := []string{
			strconv.FormatFloat(onsets[i], 'f', 3, 64),
			strconv.FormatFloat(imageDurationSec, 'f', 3, 64),
			trialType[i],
			file,
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
