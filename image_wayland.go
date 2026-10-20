package goclip

import (
	"context"
	"image"
	"os/exec"
	"strings"
)

func (d *WaylandDriver) ReadImage(ctx context.Context) (image.Image, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := exec.LookPath("wl-paste"); err != nil {
		return nil, ErrUnsupportedFormat
	}
	types, err := imageCommand(ctx, "wl-paste", "--list-types")
	if err != nil {
		return nil, err
	}
	for _, mime := range []string{"image/png", "image/jpeg", "image/bmp"} {
		found := false
		for _, offered := range strings.Fields(string(types)) {
			if offered == mime {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		data, err := imageCommand(ctx, "wl-paste", "--no-newline", "--type", mime)
		if err != nil {
			return nil, err
		}
		if mime == "image/bmp" {
			return decodeClipboardBMP(data)
		}
		return decodeClipboardImage(data)
	}
	return nil, ErrNoImage
}

func (d *WaylandDriver) ReadImageText(ctx context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	types, err := imageCommand(ctx, "wl-paste", "--list-types")
	if err != nil {
		return "", err
	}
	for _, mime := range []string{"text/plain;charset=utf-8", "text/plain", "UTF8_STRING"} {
		for _, offered := range strings.Fields(string(types)) {
			if offered != mime {
				continue
			}
			data, err := imageCommand(ctx, "wl-paste", "--no-newline", "--type", mime)
			return string(data), err
		}
	}
	return "", nil
}
