package goclip

import (
	"path/filepath"
	"strings"
	"testing"
)

type mockCustomDriver struct {
	name      string
	available bool
	text      string
}

func (m *mockCustomDriver) Name() string    { return m.name }
func (m *mockCustomDriver) Available() bool { return m.available }
func (m *mockCustomDriver) ReadText() (string, error) {
	if !m.available {
		return "", ErrUnavailable
	}
	return m.text, nil
}
func (m *mockCustomDriver) WriteText(text string) error {
	if !m.available {
		return ErrUnavailable
	}
	m.text = text
	return nil
}
func (m *mockCustomDriver) Clear() error {
	m.text = ""
	return nil
}

func TestGoclip_ActiveDriverAndOverride(t *testing.T) {
	orig := ActiveDriver()
	defer SetActiveDriver(orig)

	mock := &mockCustomDriver{name: "mock_primary", available: true}
	SetActiveDriver(mock)

	if err := WriteText("Hello Mock"); err != nil {
		t.Fatalf("WriteText failed: %v", err)
	}
	if mock.text != "Hello Mock" {
		t.Errorf("Mock did not receive written text: %q", mock.text)
	}

	got, err := ReadText()
	if err != nil {
		t.Fatalf("ReadText failed: %v", err)
	}
	if got != "Hello Mock" {
		t.Errorf("ReadText mismatch: got %q, want %q", got, "Hello Mock")
	}

	// Convenience methods
	Set("Convenient text")
	if Get() != "Convenient text" {
		t.Errorf("Get() mismatch: got %q, want %q", Get(), "Convenient text")
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	if Get() != "" {
		t.Errorf("Get() after Clear returned %q, want empty", Get())
	}
}

func TestGoclip_FallbackToFileWhenPrimaryFails(t *testing.T) {
	orig := ActiveDriver()
	defer SetActiveDriver(orig)

	tmpDir := t.TempDir()
	fileDriver := NewFileDriver(filepath.Join(tmpDir, "fallback.data"))
	RegisterDriver(fileDriver)

	// Set failing mock driver as primary
	failingMock := &mockCustomDriver{name: "failing_mock", available: false}
	SetActiveDriver(failingMock)

	// Write should fall back to fileDriver
	if err := WriteText("Fallback Success"); err != nil {
		t.Fatalf("WriteText fallback failed: %v", err)
	}

	fileContent, err := fileDriver.ReadText()
	if err != nil {
		t.Fatalf("File driver read failed: %v", err)
	}
	if fileContent != "Fallback Success" {
		t.Errorf("Fallback content mismatch: got %q, want %q", fileContent, "Fallback Success")
	}

	read, err := ReadText()
	if err != nil {
		t.Fatalf("ReadText fallback failed: %v", err)
	}
	if read != "Fallback Success" {
		t.Errorf("ReadText fallback mismatch: got %q, want %q", read, "Fallback Success")
	}
}

func TestGoclip_RegisteredDrivers(t *testing.T) {
	driversList := RegisteredDrivers()
	if len(driversList) == 0 {
		t.Fatal("Expected registered drivers list to be non-empty")
	}

	foundFile := false
	for _, name := range driversList {
		if name == "file" {
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Error("Default 'file' driver not found in registered drivers list")
	}
}
func TestGoclip_SelectBestDriver(t *testing.T) {
	driver := SelectBestDriver()
	if driver == nil {
		t.Fatal("SelectBestDriver returned nil, expected a valid driver fallback")
	}
	if !driver.Available() {
		t.Errorf("SelectBestDriver returned unavailable driver: %s", driver.Name())
	}
}

func TestGoclip_RegisterDriverPriority(t *testing.T) {
	orig := ActiveDriver()
	defer SetActiveDriver(orig)

	topMock := &mockCustomDriver{name: "high_priority_driver", available: true, text: "top priority"}
	RegisterDriverPriority(topMock, true)

	registered := RegisteredDrivers()
	if len(registered) == 0 || registered[0] != "high_priority_driver" {
		t.Errorf("RegisterDriverPriority did not prepend driver to order list: %v", registered)
	}

	best := SelectBestDriver()
	if best.Name() != "high_priority_driver" {
		t.Errorf("SelectBestDriver did not pick prepended high priority driver: got %s", best.Name())
	}
}

func TestGoclip_UnicodeAndEmojiHandling(t *testing.T) {
	orig := ActiveDriver()
	defer SetActiveDriver(orig)

	tmpDir := t.TempDir()
	fileDriver := NewFileDriver(filepath.Join(tmpDir, "unicode.data"))
	SetActiveDriver(fileDriver)

	payload := "Привет, мир! 🚀 🌟 🦀 — 日本語, 한국어, العربية, \u202Ereversed\u202C."
	Set(payload)
	got := Get()
	if got != payload {
		t.Errorf("Unicode payload corrupted.\nGot:  %q\nWant: %q", got, payload)
	}
}

func TestGoclip_LargeTextPayload(t *testing.T) {
	orig := ActiveDriver()
	defer SetActiveDriver(orig)

	tmpDir := t.TempDir()
	fileDriver := NewFileDriver(filepath.Join(tmpDir, "large.data"))
	SetActiveDriver(fileDriver)

	largeText := strings.Repeat("A multi-line chunk with numbers 0123456789\n", 5000)
	if err := WriteText(largeText); err != nil {
		t.Fatalf("WriteText large payload failed: %v", err)
	}

	readText, err := ReadText()
	if err != nil {
		t.Fatalf("ReadText large payload failed: %v", err)
	}
	if readText != largeText {
		t.Errorf("Large payload mismatch: len(got)=%d, len(want)=%d", len(readText), len(largeText))
	}
}

func TestGoclip_AllDriversFailing(t *testing.T) {
	orig := ActiveDriver()
	defer SetActiveDriver(orig)

	// Set custom driver pointing to impossible file path
	failingFile := NewFileDriver("/proc/non_existent_folder_xyz/clip.data")
	RegisterDriver(failingFile)
	failingMock := &mockCustomDriver{name: "failing_primary", available: false}
	SetActiveDriver(failingMock)

	err := WriteText("Test Fail")
	if err == nil {
		t.Error("Expected error when all drivers fail, got nil")
	}
}
