package goclip

import (
	"testing"
)

func TestCLIDriver_Basics(t *testing.T) {
	driver := NewCLIDriver()
	if driver.Name() != "cli" {
		t.Errorf("Expected driver name 'cli', got %q", driver.Name())
	}

	// Available() depends on environment tools; calling it must not panic
	_ = driver.Available()
}

func TestCLIDriver_FallbackErrorWhenUnavailable(t *testing.T) {
	driver := NewCLIDriver()
	// If no display or tools are present in test environment, ReadText should return ErrUnavailable
	if !driver.Available() {
		_, err := driver.ReadText()
		if err != ErrUnavailable {
			t.Errorf("Expected ErrUnavailable on unavailable CLIDriver, got %v", err)
		}
		err = driver.WriteText("test")
		if err != ErrUnavailable {
			t.Errorf("Expected ErrUnavailable on unavailable CLIDriver, got %v", err)
		}
	}
}
func TestCLIDriver_Clear(t *testing.T) {
	driver := NewCLIDriver()
	_ = driver.Clear()
}

func TestCLIDriver_RunCommandHelper(t *testing.T) {
	ctx := t.Context()
	// Test echo command with input via runCommandWithInput helper
	err := runCommandWithInput(ctx, "cat", []string{}, "sample text")
	if err != nil {
		// On non-unix/cat environments this might fail, so we only check it doesn't crash
		_ = err
	}
}
