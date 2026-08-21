//go:build darwin

package goclip

import (
	"runtime"
	"sync"
)

// DarwinDriver manages macOS NSPasteboard access via pbcopy/pbpaste or direct bridge.
type DarwinDriver struct {
	mu  sync.Mutex
	cli *CLIDriver
}

func NewDarwinDriver() *DarwinDriver {
	return &DarwinDriver{
		cli: NewCLIDriver(),
	}
}
func init() {
	RegisterDriverPriority(NewDarwinDriver(), true)
}

func (d *DarwinDriver) Name() string {
	return "darwin"
}

func (d *DarwinDriver) Available() bool {
	return runtime.GOOS == "darwin"
}

func (d *DarwinDriver) ReadText() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cli.ReadText()
}

func (d *DarwinDriver) WriteText(text string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cli.WriteText(text)
}

func (d *DarwinDriver) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cli.Clear()
}
