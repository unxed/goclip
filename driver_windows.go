//go:build windows

package goclip

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

type WindowsDriver struct {
	mu sync.Mutex
}

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")

	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalFree   = kernel32.NewProc("GlobalFree")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

func NewWindowsDriver() *WindowsDriver {
	return &WindowsDriver{}
}
func init() {
	RegisterDriverPriority(NewWindowsDriver(), true)
}

func (d *WindowsDriver) Name() string {
	return "windows"
}

func (d *WindowsDriver) Available() bool {
	return runtime.GOOS == "windows"
}

func (d *WindowsDriver) ReadText() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := procOpenClipboard.Find(); err != nil {
		return "", err
	}
	r, _, err := procOpenClipboard.Call(0)
	if r == 0 {
		return "", err
	}
	defer procCloseClipboard.Call()

	hMem, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if hMem == 0 {
		return "", ErrEmpty
	}

	ptr, _, err := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return "", err
	}
	defer procGlobalUnlock.Call(hMem)

	var u16Slice []uint16
	for i := 0; ; i++ {
		val := *(*uint16)(unsafe.Pointer(ptr + uintptr(i)*2))
		if val == 0 {
			break
		}
		u16Slice = append(u16Slice, val)
	}

	return syscall.UTF16ToString(u16Slice), nil
}

func (d *WindowsDriver) WriteText(text string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := procOpenClipboard.Find(); err != nil {
		return err
	}
	r, _, err := procOpenClipboard.Call(0)
	if r == 0 {
		return err
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	u16, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}

	size := uintptr(len(u16) * 2)
	hMem, _, err := procGlobalAlloc.Call(gmemMoveable, size)
	if hMem == 0 {
		return err
	}

	ptr, _, err := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return err
	}

	src := unsafe.Pointer(&u16[0])
	for i := uintptr(0); i < size; i++ {
		*(*byte)(unsafe.Pointer(ptr + i)) = *(*byte)(unsafe.Pointer(uintptr(src) + i))
	}
	procGlobalUnlock.Call(hMem)

	rSet, _, err := procSetClipboardData.Call(cfUnicodeText, hMem)
	if rSet == 0 {
		procGlobalFree.Call(hMem)
		return err
	}
	return nil
}

func (d *WindowsDriver) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := procOpenClipboard.Find(); err != nil {
		return err
	}
	r, _, err := procOpenClipboard.Call(0)
	if r == 0 {
		return err
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()
	return nil
}
