package goclip

import (
	"os"
	"testing"
)

func TestWaylandDriver_Basics(t *testing.T) {
	driver := NewWaylandDriver()
	if driver.Name() != "wayland" {
		t.Errorf("Expected driver name 'wayland', got %q", driver.Name())
	}

	origWayland := os.Getenv("WAYLAND_DISPLAY")
	os.Setenv("WAYLAND_DISPLAY", "")
	defer os.Setenv("WAYLAND_DISPLAY", origWayland)

	driverWithoutWayland := NewWaylandDriver()
	if driverWithoutWayland.Available() {
		t.Error("WaylandDriver should not be available when WAYLAND_DISPLAY is empty")
	}
}
func TestWaylandDriver_Clear(t *testing.T) {
	driver := NewWaylandDriver()
	_ = driver.Clear()
}

func TestWaylandDriver_UnavailableOperations(t *testing.T) {
	origWayland := os.Getenv("WAYLAND_DISPLAY")
	os.Setenv("WAYLAND_DISPLAY", "")
	defer os.Setenv("WAYLAND_DISPLAY", origWayland)

	driver := NewWaylandDriver()
	if !driver.Available() {
		_, _ = driver.ReadText()
		_ = driver.WriteText("test")
	}
}
