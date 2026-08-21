package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unxed/goclip"
)

func TestCLI_HelpAndVersion(t *testing.T) {
	var out bytes.Buffer

	code := runCLI(nil, &out, []string{"-h"})
	if code != 0 {
		t.Errorf("Expected exit code 0 for -h, got %d", code)
	}
	if !strings.Contains(out.String(), "Usage: goclip") {
		t.Errorf("Help text missing. Got: %s", out.String())
	}

	out.Reset()
	code = runCLI(nil, &out, []string{"-version"})
	if code != 0 {
		t.Errorf("Expected exit code 0 for -version, got %d", code)
	}
	if !strings.Contains(out.String(), "goclip 1.0") {
		t.Errorf("Version text missing. Got: %s", out.String())
	}
}

func TestCLI_ReadWritePipeline(t *testing.T) {
	tmpDir := t.TempDir()
	fileDriver := goclip.NewFileDriver(filepath.Join(tmpDir, "test.data"))
	goclip.SetActiveDriver(fileDriver)

	inputData := "Line 1\nLine 2\nLine 3\n"
	inBuf := strings.NewReader(inputData)
	var outBuf bytes.Buffer

	// 1. Write to clipboard via stdin
	code := runCLI(inBuf, &outBuf, []string{"-i"})
	if code != 0 {
		t.Fatalf("runCLI in-mode failed with exit code %d", code)
	}

	// 2. Read from clipboard via -o
	outBuf.Reset()
	code = runCLI(nil, &outBuf, []string{"-o"})
	if code != 0 {
		t.Fatalf("runCLI out-mode failed with exit code %d", code)
	}
	if outBuf.String() != inputData {
		t.Errorf("Clipboard output mismatch. Got %q, want %q", outBuf.String(), inputData)
	}

	// 3. Read with -rmlastnl
	outBuf.Reset()
	code = runCLI(nil, &outBuf, []string{"-o", "-rmlastnl"})
	if code != 0 {
		t.Fatalf("runCLI -o -rmlastnl failed with exit code %d", code)
	}
	expectedTrimmed := "Line 1\nLine 2\nLine 3"
	if outBuf.String() != expectedTrimmed {
		t.Errorf("rmlastnl mismatch. Got %q, want %q", outBuf.String(), expectedTrimmed)
	}
}

func TestCLI_FilterOption(t *testing.T) {
	tmpDir := t.TempDir()
	fileDriver := goclip.NewFileDriver(filepath.Join(tmpDir, "filter.data"))
	goclip.SetActiveDriver(fileDriver)

	inputData := "Piped and filtered text"
	inBuf := strings.NewReader(inputData)
	var outBuf bytes.Buffer

	code := runCLI(inBuf, &outBuf, []string{"-i", "-f"})
	if code != 0 {
		t.Fatalf("runCLI with -filter failed with exit code %d", code)
	}
	if outBuf.String() != inputData {
		t.Errorf("Filter output mismatch: got %q, want %q", outBuf.String(), inputData)
	}
}

func TestCLI_ReadFromMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	fileDriver := goclip.NewFileDriver(filepath.Join(tmpDir, "files.data"))
	goclip.SetActiveDriver(fileDriver)

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	_ = os.WriteFile(file1, []byte("Part 1 | "), 0600)
	_ = os.WriteFile(file2, []byte("Part 2"), 0600)

	var outBuf bytes.Buffer
	code := runCLI(nil, &outBuf, []string{"-selection", "clipboard", file1, file2})
	if code != 0 {
		t.Fatalf("runCLI with multiple files failed with exit code %d", code)
	}

	outBuf.Reset()
	code = runCLI(nil, &outBuf, []string{"-o", "-sel", "c"})
	if code != 0 {
		t.Fatalf("runCLI -o failed with exit code %d", code)
	}
	if outBuf.String() != "Part 1 | Part 2" {
		t.Errorf("Multi-file content mismatch: got %q, want %q", outBuf.String(), "Part 1 | Part 2")
	}
}

func TestCLI_SelectionAbbreviations(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"c", "clipboard"},
		{"clip", "clipboard"},
		{"CLIPBOARD", "clipboard"},
		{"p", "primary"},
		{"prim", "primary"},
		{"s", "secondary"},
		{"sec", "secondary"},
		{"b", "buffer-cut"},
	}

	for _, tc := range cases {
		got := normalizeSelectionName(tc.input)
		if got != tc.want {
			t.Errorf("normalizeSelectionName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCLI_OptionErrors(t *testing.T) {
	var out bytes.Buffer

	// 1. Missing argument for -d
	code := runCLI(nil, &out, []string{"-d"})
	if code != 1 {
		t.Errorf("Expected exit code 1 for missing -d argument, got %d", code)
	}

	// 2. Missing argument for -t
	code = runCLI(nil, &out, []string{"-t"})
	if code != 1 {
		t.Errorf("Expected exit code 1 for missing -t argument, got %d", code)
	}

	// 3. Missing argument for -selection
	code = runCLI(nil, &out, []string{"-selection"})
	if code != 1 {
		t.Errorf("Expected exit code 1 for missing -selection argument, got %d", code)
	}

	// 4. Unrecognized option
	code = runCLI(nil, &out, []string{"-invalid-flag-123"})
	if code != 1 {
		t.Errorf("Expected exit code 1 for invalid flag, got %d", code)
	}
}

func TestCLI_CompatibilityFlags(t *testing.T) {
	tmpDir := t.TempDir()
	fileDriver := goclip.NewFileDriver(filepath.Join(tmpDir, "compat.data"))
	goclip.SetActiveDriver(fileDriver)

	var out bytes.Buffer
	inBuf := strings.NewReader("Compat Data")

	// Test -l 1 -silent -noutf8 -verbose -q
	code := runCLI(inBuf, &out, []string{"-l", "1", "-silent", "-noutf8", "-verbose", "-q", "-selection=clipboard"})
	if code != 0 {
		t.Fatalf("runCLI with compatibility flags failed: %d", code)
	}

	out.Reset()
	code = runCLI(nil, &out, []string{"-o", "-sel=c"})
	if code != 0 {
		t.Fatalf("runCLI -o with -sel=c failed: %d", code)
	}
	if out.String() != "Compat Data" {
		t.Errorf("Expected %q, got %q", "Compat Data", out.String())
	}
}

func TestCLI_NonExistentFile(t *testing.T) {
	var out bytes.Buffer
	code := runCLI(nil, &out, []string{"/path/to/definitely/non_existent_file_xyz123.txt"})
	if code != 1 {
		t.Errorf("Expected exit code 1 for non-existent file, got %d", code)
	}
}
func TestCLI_DebugFlag(t *testing.T) {
	tmpDir := t.TempDir()
	fileDriver := goclip.NewFileDriver(filepath.Join(tmpDir, "debug.data"))
	goclip.SetActiveDriver(fileDriver)

	var out bytes.Buffer
	inBuf := strings.NewReader("Debug Input Text")

	code := runCLI(inBuf, &out, []string{"--debug", "-i"})
	if code != 0 {
		t.Fatalf("runCLI with --debug failed: %d", code)
	}

	out.Reset()
	code = runCLI(nil, &out, []string{"--debug", "-o"})
	if code != 0 {
		t.Fatalf("runCLI -o with --debug failed: %d", code)
	}
	if out.String() != "Debug Input Text" {
		t.Errorf("Expected %q, got %q", "Debug Input Text", out.String())
	}
}
