package goclip

import (
	"errors"
	"sync"
)

var (
	// ErrUnavailable is returned when a clipboard driver cannot be used in the current environment.
	ErrUnavailable = errors.New("clipboard driver is unavailable")
	// ErrEmpty is returned when the clipboard contains no text data.
	ErrEmpty = errors.New("clipboard is empty")
	// ErrUnsupportedFormat is returned when the requested clipboard format is not supported.
	ErrUnsupportedFormat = errors.New("unsupported clipboard format")
)

// Driver defines the interface that all platform clipboard backends must implement.
type Driver interface {
	// Name returns the unique human-readable identifier of the driver.
	Name() string
	// Available checks whether the driver can operate in the current environment.
	Available() bool
	// ReadText retrieves UTF-8 plain text from the clipboard.
	ReadText() (string, error)
	// WriteText stores UTF-8 plain text into the clipboard.
	WriteText(text string) error
	// Clear empties the clipboard content.
	Clear() error
}

var (
	registryMu sync.RWMutex
	drivers    = make(map[string]Driver)
	order      []string
)

// RegisterDriver registers a clipboard driver by name at default priority.
func RegisterDriver(driver Driver) {
	RegisterDriverPriority(driver, false)
}

// RegisterDriverPriority registers a driver, optionally prepending it for highest priority.
func RegisterDriverPriority(driver Driver, highPriority bool) {
	registryMu.Lock()
	defer registryMu.Unlock()
	name := driver.Name()
	if _, exists := drivers[name]; !exists {
		if highPriority {
			order = append([]string{name}, order...)
		} else {
			order = append(order, name)
		}
	}
	drivers[name] = driver
}

// GetDriver returns a registered driver by name.
func GetDriver(name string) (Driver, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	d, ok := drivers[name]
	return d, ok
}

// RegisteredDrivers returns a list of all registered driver names in priority order.
func RegisteredDrivers() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	res := make([]string, len(order))
	copy(res, order)
	return res
}
