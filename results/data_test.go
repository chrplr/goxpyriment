package results

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrplr/goxpyriment/sysinfo"
)

// TestDataFileFormatting verifies that CSV rows are correctly formatted and escaped.
func TestDataFileFormatting(t *testing.T) {
	// Create a mock DataFile (using a temp dir for path logic)
	tmpDir, err := os.MkdirTemp("", "goxpy_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	df := &DataFile{
		OutputFile: &OutputFile{
			Buffer: make([]string, 0),
		},
		Delimiter: ",",
		SubjectID: 42,
	}

	// 1. Simple data — numbers bare, strings always quoted
	df.Add(1, "test", 3.14)
	expected := `42,1,"test",3.14`
	if !strings.Contains(df.Buffer[0], expected) {
		t.Errorf("Expected row to contain %q, got %q", expected, df.Buffer[0])
	}

	// 2. Data with delimiter (needs escaping)
	df.Add("hello, world")
	expectedEscaped := "42,\"hello, world\""
	if !strings.Contains(df.Buffer[1], expectedEscaped) {
		t.Errorf("Expected escaped row to contain %q, got %q", expectedEscaped, df.Buffer[1])
	}

	// 3. Data with quotes (needs double quotes)
	df.Add("He said \"Hello\"")
	expectedQuotes := "42,\"He said \"\"Hello\"\"\""
	if !strings.Contains(df.Buffer[2], expectedQuotes) {
		t.Errorf("Expected double-quoted row to contain %q, got %q", expectedQuotes, df.Buffer[2])
	}
}

// TestOutputBuffer verifies the buffering and clearing logic of OutputFile.
func TestOutputBuffer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "goxpy_buffer_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	of, err := NewOutputFile(tmpDir, "test.txt")
	if err != nil {
		t.Fatal(err)
	}

	of.WriteLine("Line 1")
	of.WriteLine("Line 2")

	if len(of.Buffer) != 2 {
		t.Errorf("Expected buffer size 2, got %d", len(of.Buffer))
	}

	if err := of.Save(); err != nil {
		t.Fatal(err)
	}

	if len(of.Buffer) != 0 {
		t.Error("Buffer was not cleared after Save")
	}

	// Verify file content
	content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Line 1") || !strings.Contains(string(content), "Line 2") {
		t.Error("File content does not match buffered lines")
	}
}

// TestNewDataFileDefaultDir verifies that NewDataFile correctly creates both the
// CSV file and the companion info file, with the expected filename patterns.
func TestNewDataFileDefaultDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "goxpy_newfile_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	df, err := NewDataFile(tmpDir, 1, "test_exp")
	if err != nil {
		t.Fatal(err)
	}

	// Directory and CSV filename format.
	if df.Directory != tmpDir {
		t.Errorf("Expected directory %q, got %q", tmpDir, df.Directory)
	}
	if !strings.HasPrefix(df.Filename, "test_exp_sub-001_date-") {
		t.Errorf("Unexpected CSV filename format: %q", df.Filename)
	}
	if !strings.HasSuffix(df.Filename, ".csv") {
		t.Errorf("CSV filename should end with .csv, got %q", df.Filename)
	}

	// Info file has the same basename with -info.txt suffix.
	expectedInfoSuffix := "-info.txt"
	if !strings.HasSuffix(df.InfoFile.Filename, expectedInfoSuffix) {
		t.Errorf("Info filename should end with %q, got %q", expectedInfoSuffix, df.InfoFile.Filename)
	}
	csvBase := strings.TrimSuffix(df.Filename, ".csv")
	infoBase := strings.TrimSuffix(df.InfoFile.Filename, "-info.txt")
	if csvBase != infoBase {
		t.Errorf("CSV base %q and info base %q should be equal", csvBase, infoBase)
	}

	// Flush and check: CSV must contain no comment lines; info file must have them.
	if err := df.Save(); err != nil {
		t.Fatal(err)
	}

	csvContent, err := os.ReadFile(df.FullPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(csvContent), "#") {
		t.Errorf("CSV file should not contain comment lines, got:\n%s", csvContent)
	}

	infoContent, err := os.ReadFile(df.InfoFile.FullPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(infoContent), "# ") {
		t.Errorf("Info file should contain comment lines, got:\n%s", infoContent)
	}
	if !strings.Contains(string(infoContent), "EXPERIMENT INFO") {
		t.Errorf("Info file should contain EXPERIMENT INFO section, got:\n%s", infoContent)
	}
}

func TestWriteHostInfoSection(t *testing.T) {
	dir := t.TempDir()
	df, err := NewDataFile(dir, 999, "hostinfo")
	if err != nil {
		t.Fatalf("NewDataFile: %v", err)
	}
	df.WriteHostInfo(sysinfo.Host())
	if err := df.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*-info.txt"))
	if len(matches) != 1 {
		t.Fatalf("expected one info file, got %v", matches)
	}
	b, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, "# --HOST INFO") {
		t.Errorf("info file has no --HOST INFO section:\n%s", got)
	}
	// Host facts must be prefixed "host", never "sys": the prefix is what tells
	// a reader whether a line came from the operating system or from SDL, and
	// the two can legitimately disagree about the same hardware.
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "# host ") && !strings.Contains(line, ": ") {
			t.Errorf("malformed host line %q, want \"# host key: value\"", line)
		}
	}
	if strings.Contains(got, "# sys machine:") || strings.Contains(got, "# sys kernel:") {
		t.Errorf("host facts were written under the sys prefix:\n%s", got)
	}
}

func TestWriteHostInfoSkipsEmptySection(t *testing.T) {
	// On a system where every probe failed, the header must not be written on
	// its own: a section with no lines under it reads as a truncated file.
	dir := t.TempDir()
	df, err := NewDataFile(dir, 999, "empty")
	if err != nil {
		t.Fatalf("NewDataFile: %v", err)
	}
	df.WriteHostInfo(sysinfo.SysInfo{})
	if err := df.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*-info.txt"))
	b, _ := os.ReadFile(matches[0])
	if strings.Contains(string(b), "HOST INFO") {
		t.Errorf("empty SysInfo still wrote a --HOST INFO header:\n%s", b)
	}
}
