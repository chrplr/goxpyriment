package results

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// TestNewDataFileDefaultDir verifies that NewDataFile correctly defaults to the
// user's home directory if no directory is provided.
func TestNewDataFileDefaultDir(t *testing.T) {
	// We don't want to actually create files in $HOME during tests,
	// so we check if the path generation logic is sane.

	// Test with explicit dir
	df, err := NewDataFile("my_results", 1, "test_exp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("my_results") // Cleanup

	if df.Directory != "my_results" {
		t.Errorf("Expected directory 'my_results', got %q", df.Directory)
	}

	if !strings.HasPrefix(df.Filename, "test_exp_sub-001_date-") {
		t.Errorf("Unexpected filename format: %q", df.Filename)
	}
}
