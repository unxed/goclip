//go:build !android

package goclip

import (
	"context"
	"errors"
	"github.com/jezek/xgb/xproto"
	"image"
	"time"
)

func (d *X11Driver) ReadImage(ctx context.Context) (image.Image, error) {
	if err := d.ensure(); err != nil {
		return nil, err
	}
	for _, mime := range []string{"image/png", "image/jpeg", "image/bmp"} {
		reply, err := xproto.InternAtom(d.conn, true, uint16(len(mime)), mime).Reply()
		if err != nil {
			return nil, err
		}
		if reply == nil || reply.Atom == 0 {
			continue
		}
		data, err := d.readImageTarget(ctx, reply.Atom)
		if errors.Is(err, ErrNoImage) {
			continue
		}
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

func (d *X11Driver) readImageTarget(ctx context.Context, target xproto.Atom) ([]byte, error) {
	d.mu.Lock()
	if d.owning {
		d.mu.Unlock()
		return nil, ErrNoImage
	}
	if d.reading {
		d.mu.Unlock()
		return nil, errors.New("goclip: another paste is already in flight")
	}
	done := make(chan string, 1)
	d.reading, d.readDone, d.readBuf, d.readIncr = true, done, nil, false
	xproto.ConvertSelection(d.conn, d.win, d.clipboard, target, d.goclipProp, xproto.TimeCurrentTime)
	d.mu.Unlock()
	timer := time.NewTimer(x11ReadTimeout)
	defer timer.Stop()
	select {
	case data := <-done:
		if data == "" {
			return nil, ErrNoImage
		}
		return []byte(data), nil
	case <-ctx.Done():
		d.mu.Lock()
		d.reading, d.readDone = false, nil
		d.mu.Unlock()
		return nil, ctx.Err()
	case <-timer.C:
		d.mu.Lock()
		d.reading, d.readDone = false, nil
		d.mu.Unlock()
		return nil, errors.New("goclip: clipboard image owner did not answer")
	}
}
