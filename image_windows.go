//go:build windows

package goclip

import (
	"context"
	"errors"
	"fmt"
	"image"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	procRegisterClipboardFormat = user32.NewProc("RegisterClipboardFormatW")
	procGlobalSize              = kernel32.NewProc("GlobalSize")
)

func (d *WindowsDriver) ReadImage(ctx context.Context) (image.Image, error) {
	contents, err := d.ReadContents(ctx)
	if err != nil {
		return nil, err
	}
	if contents.Image == nil {
		return nil, ErrNoImage
	}
	return contents.Image, nil
}

func (d *WindowsDriver) ReadContents(ctx context.Context) (Contents, error) {
	data, dib, text, err := d.readContentsBytes(ctx)
	if err != nil {
		return Contents{}, err
	}
	if len(data) == 0 {
		return Contents{Text: text}, nil
	}
	var img image.Image
	if dib {
		img, err = decodeClipboardDIB(data)
	} else {
		img, err = decodeClipboardImage(data)
	}
	return Contents{Image: img, Text: text}, err
}

// Copy both representations under one clipboard lock; decode after releasing it.
func (d *WindowsDriver) readContentsBytes(ctx context.Context) (data []byte, dib bool, text string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	opened := uintptr(0)
	for attempt := 0; attempt < 5; attempt++ {
		if err = ctx.Err(); err != nil {
			return
		}
		opened, _, err = procOpenClipboard.Call(0)
		if opened != 0 {
			break
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			err = ctx.Err()
			return
		case <-timer.C:
		}
	}
	if opened == 0 {
		err = fmt.Errorf("opening clipboard: %w", err)
		return
	}
	defer procCloseClipboard.Call()
	name, _ := syscall.UTF16PtrFromString("PNG")
	pngFormat, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
	for _, format := range []uintptr{pngFormat, 17, 8} {
		if format == 0 {
			continue
		}
		handle, _, _ := procGetClipboardData.Call(format)
		if handle == 0 {
			continue
		}
		data, err = clipboardGlobalBytes(handle)
		if err != nil {
			return
		}
		dib = format != pngFormat
		break
	}
	handle, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if handle != 0 {
		var raw []byte
		raw, err = clipboardGlobalBytes(handle)
		if err != nil {
			return
		}
		if len(raw)%2 != 0 {
			err = errors.New("invalid clipboard UTF-16 text")
			return
		}
		units := make([]uint16, 0, len(raw)/2)
		for i := 0; i+1 < len(raw); i += 2 {
			value := uint16(raw[i]) | uint16(raw[i+1])<<8
			if value == 0 {
				break
			}
			units = append(units, value)
		}
		text = syscall.UTF16ToString(units)
	}
	err = nil
	return
}

func clipboardGlobalBytes(handle uintptr) ([]byte, error) {
	size, _, _ := procGlobalSize.Call(handle)
	if size == 0 || size > maxImageBytes {
		return nil, errors.New("invalid clipboard representation size")
	}
	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		return nil, fmt.Errorf("locking clipboard: %w", err)
	}
	defer procGlobalUnlock.Call(handle)
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(size))...), nil
}
