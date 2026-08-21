package goclip

import (
	"os"
	"testing"
)

func TestX11Driver_Basics(t *testing.T) {
	driver := NewX11Driver()
	if driver.Name() != "x11" {
		t.Errorf("Expected driver name 'x11', got %q", driver.Name())
	}

	// Without DISPLAY set, driver must report unavailable
	origDisplay := os.Getenv("DISPLAY")
	os.Setenv("DISPLAY", "")
	defer os.Setenv("DISPLAY", origDisplay)

	driverWithoutDisplay := NewX11Driver()
	if driverWithoutDisplay.Available() {
		t.Error("X11Driver should not be available when DISPLAY is empty")
	}
}
func TestX11Driver_Clear(t *testing.T) {
	driver := NewX11Driver()
	// Clear without active connection should not panic
	_ = driver.Clear()
}

func TestX11Driver_UnavailableOperations(t *testing.T) {
	origDisplay := os.Getenv("DISPLAY")
	os.Setenv("DISPLAY", "")
	defer os.Setenv("DISPLAY", origDisplay)

	driver := NewX11Driver()
	if !driver.Available() {
		// Reading/Writing without connection must fail gracefully
		_, _ = driver.ReadText()
		_ = driver.WriteText("test")
	}
}
