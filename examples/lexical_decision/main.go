// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	_ "embed"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// defaultStimuli is the built-in stimulus list, used when no CSV file is
// passed on the command line.
//
//go:embed lexical_decision_stimuli.csv
var defaultStimuli string

const (
	WordResponseKey    = control.K_F
	NonWordResponseKey = control.K_J
	MaxResponseDelay   = 2000
)

// Hardcoded items for the practice block (4 words + 4 pseudowords).
var (
	practiceWords       = []string{"TABLE", "GREEN", "HORSE", "LIGHT"}
	practicePseudowords = []string{"FLORP", "TREVE", "GROMP", "SPLON"}
)

type lexicalTrial struct {
	item     string
	category string
	nchar    string
	stim     *stimuli.TextLine
}

// newTrial builds a lexicalTrial, creating its text stimulus and deriving the
// character count from the item length.
func newTrial(item, category string) lexicalTrial {
	stim := stimuli.NewTextLine(item, 0, 0, control.DefaultTextColor)
	return lexicalTrial{
		item:     item,
		category: category,
		nchar:    fmt.Sprintf("%d", len(item)),
		stim:     stim,
	}
}

// loadTrials reads the stimulus list and builds the trial sequence. When
// stimFile is empty the embedded default list is used. Each non-header row is
// expected to hold "item,category,nchar"; malformed rows are skipped with a log
// message.
func loadTrials(stimFile string) ([]lexicalTrial, error) {
	var reader *csv.Reader
	if stimFile != "" {
		file, err := os.Open(stimFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open stimuli file: %w", err)
		}
		defer file.Close()
		reader = csv.NewReader(file)
	} else {
		reader = csv.NewReader(strings.NewReader(defaultStimuli))
	}

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read stimuli: %w", err)
	}

	// Assume first line is header: item,category,nchar
	var trials []lexicalTrial
	for i, record := range records {
		if i == 0 {
			continue // skip header
		}
		if len(record) < 3 {
			log.Printf("skipping malformed CSV line %d: %#v", i+1, record)
			continue
		}
		item := record[0]
		category := record[1]
		nchar := record[2]
		stim := stimuli.NewTextLine(item, 0, 0, control.DefaultTextColor)
		trials = append(trials, lexicalTrial{item: item, category: category, nchar: nchar, stim: stim})
	}
	return trials, nil
}

// sessionStats accumulates per-category performance during the main block and
// produces the end-of-session summary.
type sessionStats struct {
	wordTrials, wordHits     int
	pseudoTrials, pseudoHits int
	wordCorrectRTs           []int64 // RTs of correct word responses
	pseudoCorrectRTs         []int64 // RTs of correct pseudoword responses
}

// record tallies one trial's outcome by category.
func (s *sessionStats) record(category string, correct bool, rt int64) {
	if isWordCategory(category) {
		s.wordTrials++
		if correct {
			s.wordHits++
			s.wordCorrectRTs = append(s.wordCorrectRTs, rt)
		}
	} else {
		s.pseudoTrials++
		if correct {
			s.pseudoHits++
			s.pseudoCorrectRTs = append(s.pseudoCorrectRTs, rt)
		}
	}
}

// summary renders the end-of-session report (median RT and hit rate per category).
func (s *sessionStats) summary() string {
	return formatSummary(
		s.wordTrials, s.wordHits, median(s.wordCorrectRTs),
		s.pseudoTrials, s.pseudoHits, median(s.pseudoCorrectRTs),
	)
}

// isWordCategory reports whether a category label denotes a real word.
// It is robust to the different conventions used in the stimuli files
// ("word"/"pseudo" or "W"/"P") by looking at the first letter.
func isWordCategory(category string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(category)), "w")
}

// correctKeyFor returns the response key that counts as correct for a category.
func correctKeyFor(category string) control.Keycode {
	if isWordCategory(category) {
		return WordResponseKey
	}
	return NonWordResponseKey
}

// median returns the median of a slice of reaction times (in ms).
// Returns -1 when the slice is empty.
func median(values []int64) float64 {
	n := len(values)
	if n == 0 {
		return -1
	}
	sorted := make([]int64, n)
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if n%2 == 1 {
		return float64(sorted[n/2])
	}
	return float64(sorted[n/2-1]+sorted[n/2]) / 2.0
}

// formatSummary builds the end-of-session report shown on screen and printed
// to the console: median RT for correct responses and hit rate, per category.
func formatSummary(wordTrials, wordHits int, wordMedRT float64, pseudoTrials, pseudoHits int, pseudoMedRT float64) string {
	hitRate := func(hits, total int) float64 {
		if total == 0 {
			return 0
		}
		return 100.0 * float64(hits) / float64(total)
	}
	medStr := func(m float64) string {
		if m < 0 {
			return "n/a (no correct responses)"
		}
		return fmt.Sprintf("%.0f ms", m)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Results\n\n")
	fmt.Fprintf(&b, "Words:        hit rate %.1f%% (%d/%d), median RT %s\n",
		hitRate(wordHits, wordTrials), wordHits, wordTrials, medStr(wordMedRT))
	fmt.Fprintf(&b, "Pseudowords:  hit rate %.1f%% (%d/%d), median RT %s\n",
		hitRate(pseudoHits, pseudoTrials), pseudoHits, pseudoTrials, medStr(pseudoMedRT))
	return b.String()
}

func main() {
	flag.Parse()

	// 1. Optional stimuli CSV file from the command line. When omitted, the
	//    embedded default list (lexical_decision_stimuli.csv) is used.
	stimFile := ""
	if args := flag.Args(); len(args) >= 1 {
		stimFile = args[0]
	}

	// 2. Collect participant info via a GUI dialog before launching.
	fields := []control.InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		control.FullscreenField,
	}
	info, err := control.GetParticipantInfo("Lexical Decision", fields)
	if err != nil {
		log.Fatalf("setup cancelled: %v", err)
	}
	subjectID, _ := strconv.Atoi(info["subject_id"])

	// 3. Build the experiment from the collected info.
	fullscreen := info["fullscreen"] == "true"
	width, height := 0, 0
	if !fullscreen {
		width, height = 1024, 768
	}
	exp := control.NewExperiment("Lexical Decision", width, height, fullscreen, control.Black, control.White, 32)
	exp.SubjectID = subjectID
	exp.Info = info
	if err := exp.Initialize(); err != nil {
		log.Fatalf("failed to initialize experiment: %v", err)
	}
	defer exp.End()

	// 4. Load stimuli — from the given CSV file, or the embedded default list.
	trials, err := loadTrials(stimFile)
	if err != nil {
		exp.Fatal("%v", err)
	}

	// Prepare event log header and write it as comments in the data file
	evLog := exp.CollectEventLog()
	evLog.SetSubjectID(fmt.Sprintf("%d", exp.SubjectID))
	evLog.SetCSVHeader([]string{"item", "category", "nchar", "key", "rt"})
	exp.Data.WriteComment("--EVENT LOG")
	exp.Data.WriteComment(evLog.String())
	exp.Data.WriteComment("--TRIAL DATA")

	exp.AddDataVariableNames([]string{"item", "category", "nchar", "key", "rt"})

	// 5. Shuffle trials
	design.ShuffleList(trials)

	// Build the practice block from the hardcoded items (4 words + 4 pseudowords).
	var practiceTrials []lexicalTrial
	for _, w := range practiceWords {
		practiceTrials = append(practiceTrials, newTrial(w, "word"))
	}
	for _, p := range practicePseudowords {
		practiceTrials = append(practiceTrials, newTrial(p, "pseudo"))
	}
	design.ShuffleList(practiceTrials)

	instrText := "When you'll see a stimulus, your task is to decide, as quickly as possible, whether it is a word or not.\n\nif it is a word, press 'F'\n\nif it is a non-word, press 'J'\n\nPress the SPACE bar to start."

	// Accumulator for the end-of-session summary.
	var stats sessionStats

	// 7. Run the experiment logic
	err = exp.Run(func() error {
		// Feedback dot (center, filled): green when correct, red otherwise.
		const feedbackRadius = 8
		greenDot := stimuli.NewCircle(feedbackRadius, control.Green)
		redDot := stimuli.NewCircle(feedbackRadius, control.Red)
		showFeedback := func(correct bool) {
			if correct {
				exp.ShowTimed(greenDot, 800)
			} else {
				exp.ShowTimed(redDot, 800)
			}
		}

		// Instructions
		exp.ShowInstructions(instrText)

		// Practice block (with feedback, not recorded in the data file).
		exp.ShowInstructions("First, a short practice block.\n\nAfter each response a dot appears: green if you were correct, red if you were incorrect or too slow.\n\nPress the SPACE bar to begin the practice.")
		for i, t := range practiceTrials {
			// Counter ("X/N"), then a 500 ms gap before the item.
			counter := stimuli.NewTextLine(fmt.Sprintf("%d/%d", i+1, len(practiceTrials)), 0, 0, control.DefaultTextColor)
			exp.ShowTimed(counter, 500)
			exp.Blank(500)
			key, _, _ := exp.ShowAndGetRT(t.stim, []control.Keycode{WordResponseKey, NonWordResponseKey}, MaxResponseDelay)
			showFeedback(key == correctKeyFor(t.category))
		}
		exp.ShowInstructions("End of practice.\n\nThe real experiment will now begin.\n\nTry to respond as quickly and accurately as possible.\n\nPress the SPACE bar to start.")

		// Loop through trials
		for i, t := range trials {
			// Trial counter ("X/N"), then a 500 ms gap before the item.
			counter := stimuli.NewTextLine(fmt.Sprintf("%d/%d", i+1, len(trials)), 0, 0, control.DefaultTextColor)
			exp.ShowTimed(counter, 500)
			exp.Blank(500)

			// Stimulus + response
			key, rt, _ := exp.ShowAndGetRT(t.stim, []control.Keycode{WordResponseKey, NonWordResponseKey}, MaxResponseDelay)

			correct := key == correctKeyFor(t.category)
			stats.record(t.category, correct, rt)

			exp.Data.Add(t.item, t.category, t.nchar, key, rt)
			fmt.Printf("Trial: Item=%s, Cat=%s, Nchar=%s, Key=%d, RT=%d ms\n", t.item, t.category, t.nchar, key, rt)

			// Feedback: green (correct) / red (incorrect) dot.
			showFeedback(correct)
		}

		// Compute and display the end-of-session summary.
		summary := stats.summary()
		fmt.Print(summary)
		exp.ShowEndMessage(summary +
			fmt.Sprintf("\nResults saved in %s\n", exp.Data.FullPath) +
			"\nPress any key to exit.")

		return control.EndLoop
	})

	if err != nil && !control.IsEndLoop(err) {
		exp.Fatal("experiment error: %v", err)
	}
}
