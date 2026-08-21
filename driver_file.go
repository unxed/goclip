package goclip

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// FileDriver implements a persistent, process-shared fallback clipboard backed
// by a local file in the user's config directory (similar to far2l / FSClipboardBackend).
type FileDriver struct {
	mu       sync.RWMutex
	filePath string
}

const (
	defaultMaxClipboardFileSize = 10 * 1024 * 1024 // 10 MB limit
)

// NewFileDriver creates a file-backed clipboard driver storing data at the specified path.
// If customPath is empty, it uses standard OS config/cache directories.
func NewFileDriver(customPath string) *FileDriver {
	if customPath == "" {
		customPath = defaultClipboardFilePath()
	}
	return &FileDriver{filePath: customPath}
}

func defaultClipboardFilePath() string {
	var baseDir string
	if runtime.GOOS == "windows" {
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			baseDir = os.Getenv("USERPROFILE")
		}
	} else {
		baseDir = os.Getenv("XDG_DATA_HOME")
		if baseDir == "" {
			home, _ := os.UserHomeDir()
			if home != "" {
				baseDir = filepath.Join(home, ".local", "share")
			}
		}
	}
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	return filepath.Join(baseDir, "goclip", "clipboard.data")
}

func (d *FileDriver) Name() string {
	return "file"
}

func (d *FileDriver) Available() bool {
	return true
}

func (d *FileDriver) ReadText() (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	data, err := os.ReadFile(d.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func (d *FileDriver) WriteText(text string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(text) > defaultMaxClipboardFileSize {
		text = text[:defaultMaxClipboardFileSize]
	}

	dir := filepath.Dir(d.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	tempPath := filepath.Join(dir, ".clip-"+hex.EncodeToString(rnd[:])+".tmp")

	if err := os.WriteFile(tempPath, []byte(text), 0600); err != nil {
		return err
	}

	// Atomic rename to destination
	if err := os.Rename(tempPath, d.filePath); err != nil {
		_ = os.Remove(tempPath)
		// Fallback for Windows if destination exists
		if runtime.GOOS == "windows" {
			_ = os.Remove(d.filePath)
			if err2 := os.Rename(tempPath, d.filePath); err2 != nil {
				return err2
			}
			return nil
		}
		return err
	}
	return nil
}

func (d *FileDriver) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	err := os.Remove(d.filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (d *FileDriver) Path() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.filePath
}
