package goclip

import (
	"sync"
)

var (
	defaultDriver Driver
	driverOnce    sync.Once
	driverMu      sync.RWMutex
)

func init() {
	// Register default cross-platform and fallback drivers
	RegisterDriver(NewWaylandDriver())
	RegisterDriver(NewX11Driver())
	RegisterDriver(NewCLIDriver())
	RegisterDriver(NewFileDriver(""))
}

// SelectBestDriver evaluates available drivers in priority order and returns the first usable one.
func SelectBestDriver() Driver {
	for _, name := range RegisteredDrivers() {
		if d, ok := GetDriver(name); ok && d.Available() {
			return d
		}
	}
	return NewFileDriver("")
}

// ActiveDriver returns the currently selected clipboard driver, initializing it if necessary.
func ActiveDriver() Driver {
	driverMu.RLock()
	d := defaultDriver
	driverMu.RUnlock()
	if d != nil {
		return d
	}

	driverOnce.Do(func() {
		d = SelectBestDriver()
		driverMu.Lock()
		defaultDriver = d
		driverMu.Unlock()
	})
	return d
}

// SetActiveDriver overrides the active clipboard driver. Passing nil resets to auto-detection.
func SetActiveDriver(d Driver) {
	driverMu.Lock()
	defer driverMu.Unlock()
	defaultDriver = d
}

// ReadText reads UTF-8 plain text from the system clipboard using the active driver,
// with automatic fallback to file-based local storage if the primary driver fails.
func ReadText() (string, error) {
	d := ActiveDriver()
	if d != nil && d.Available() {
		text, err := d.ReadText()
		if err == nil {
			return text, nil
		}
	}

	// Secondary fallback: FileDriver
	if fd, ok := GetDriver("file"); ok && fd.Available() {
		return fd.ReadText()
	}
	return "", ErrUnavailable
}

// WriteText writes UTF-8 plain text to the system clipboard using the active driver,
// with automatic fallback to file-based storage if necessary.
func WriteText(text string) error {
	d := ActiveDriver()
	if d != nil && d.Available() {
		err := d.WriteText(text)
		if err == nil {
			return nil
		}
	}

	// Secondary fallback: FileDriver
	if fd, ok := GetDriver("file"); ok && fd.Available() {
		return fd.WriteText(text)
	}
	return ErrUnavailable
}

// Clear empties the system clipboard.
func Clear() error {
	d := ActiveDriver()
	if d != nil && d.Available() {
		_ = d.Clear()
	}
	if fd, ok := GetDriver("file"); ok && fd.Available() {
		_ = fd.Clear()
	}
	return nil
}

// Get is a convenience wrapper returning the clipboard string or empty string on error.
func Get() string {
	s, _ := ReadText()
	return s
}

// Set is a convenience wrapper writing a string to the clipboard and ignoring errors.
func Set(text string) {
	_ = WriteText(text)
}
