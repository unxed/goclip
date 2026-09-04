//go:build android

package goclip

// X11Driver is a stub on Android (Termux): there is no X server, and the
// pure-Go xgb client is compiled out so terminal-only builds stay lean.
// The driver is registered as usual but never reports itself available,
// so SelectBestDriver falls through to the CLI and file drivers.
type X11Driver struct{}

func NewX11Driver() *X11Driver { return &X11Driver{} }

func (d *X11Driver) Name() string                { return "x11" }
func (d *X11Driver) Available() bool             { return false }
func (d *X11Driver) ReadText() (string, error)   { return "", ErrUnavailable }
func (d *X11Driver) WriteText(text string) error { return ErrUnavailable }
func (d *X11Driver) Clear() error                { return ErrUnavailable }
