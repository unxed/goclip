package goclip

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// CLIDriver communicates with external system clipboard CLI tools:
// - Wayland: wl-copy / wl-paste
// - X11: xclip, xsel
// - macOS: pbcopy / pbpaste
// - Android (Termux with the Termux:API add-on): termux-clipboard-get / termux-clipboard-set
type CLIDriver struct{}

const cliTimeout = 2 * time.Second

func NewCLIDriver() *CLIDriver {
	return &CLIDriver{}
}

func (d *CLIDriver) Name() string {
	return "cli"
}

func (d *CLIDriver) Available() bool {
	if runtime.GOOS == "darwin" {
		_, err := exec.LookPath("pbcopy")
		return err == nil
	}
	if runtime.GOOS == "android" {
		_, err := exec.LookPath("termux-clipboard-get")
		return err == nil
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return true
		}
	}
	if os.Getenv("DISPLAY") != "" {
		if _, err := exec.LookPath("xclip"); err == nil {
			return true
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			return true
		}
	}
	return false
}

func (d *CLIDriver) ReadText() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()

	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("pbpaste"); err == nil {
			cmd := exec.CommandContext(ctx, "pbpaste")
			out, err := cmd.Output()
			if err != nil {
				return "", err
			}
			return string(out), nil
		}
	}

	if runtime.GOOS == "android" {
		if _, err := exec.LookPath("termux-clipboard-get"); err == nil {
			cmd := exec.CommandContext(ctx, "termux-clipboard-get")
			out, err := cmd.Output()
			if err != nil {
				return "", err
			}
			return string(out), nil
		}
	}

	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-paste"); err == nil {
			cmd := exec.CommandContext(ctx, "wl-paste", "--no-newline")
			out, err := cmd.Output()
			if err == nil {
				return string(out), nil
			}
		}
	}

	if os.Getenv("DISPLAY") != "" {
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-out")
			out, err := cmd.Output()
			if err == nil {
				return string(out), nil
			}
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			cmd := exec.CommandContext(ctx, "xsel", "--clipboard", "--output")
			out, err := cmd.Output()
			if err == nil {
				return string(out), nil
			}
		}
	}

	return "", ErrUnavailable
}

func (d *CLIDriver) WriteText(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()

	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("pbcopy"); err == nil {
			cmd := exec.CommandContext(ctx, "pbcopy")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
	}

	if runtime.GOOS == "android" {
		if _, err := exec.LookPath("termux-clipboard-set"); err == nil {
			cmd := exec.CommandContext(ctx, "termux-clipboard-set")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
	}

	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.CommandContext(ctx, "wl-copy")
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
	}

	if os.Getenv("DISPLAY") != "" {
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-in")
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			cmd := exec.CommandContext(ctx, "xsel", "--clipboard", "--input")
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
	}

	return ErrUnavailable
}

func (d *CLIDriver) Clear() error {
	return d.WriteText("")
}

// Helper to run command safely with input
func runCommandWithInput(ctx context.Context, name string, args []string, input string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader([]byte(input))
	return cmd.Run()
}
