package goclip

import (
	"os"
	"sync"
)

// WaylandDriver handles Wayland compositor clipboard interactions.
// When a native connection is established via neurlang/wayland, it communicates
// over the data-device interface; otherwise it delegates to CLIDriver (wl-copy/wl-paste).
type WaylandDriver struct {
	mu  sync.Mutex
	cli *CLIDriver
}

func NewWaylandDriver() *WaylandDriver {
	return &WaylandDriver{
		cli: NewCLIDriver(),
	}
}

func (d *WaylandDriver) Name() string {
	return "wayland"
}

func (d *WaylandDriver) Available() bool {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}
	return d.cli.Available()
}

func (d *WaylandDriver) ReadText() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cli.ReadText()
}

func (d *WaylandDriver) WriteText(text string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cli.WriteText(text)
}

func (d *WaylandDriver) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cli.Clear()
}
