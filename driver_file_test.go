package goclip

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFileDriver_BasicReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "clip.data")

	driver := NewFileDriver(filePath)
	if driver.Name() != "file" {
		t.Errorf("Expected driver name 'file', got %q", driver.Name())
	}
	if !driver.Available() {
		t.Error("File driver should always be available")
	}

	// 1. Initial read should return empty string without error
	text, err := driver.ReadText()
	if err != nil {
		t.Fatalf("Unexpected error on empty file read: %v", err)
	}
	if text != "" {
		t.Errorf("Expected empty string, got %q", text)
	}

	// 2. Write multi-line UTF-8 text
	expected := "Тестовый текст\nВторая строка с эмодзи 🚀🎉\nFinal Line."
	if err := driver.WriteText(expected); err != nil {
		t.Fatalf("WriteText failed: %v", err)
	}

	// 3. Read back and verify
	readBack, err := driver.ReadText()
	if err != nil {
		t.Fatalf("ReadText failed: %v", err)
	}
	if readBack != expected {
		t.Errorf("Content mismatch.\nGot:\n%s\nWant:\n%s", readBack, expected)
	}

	// 4. Overwrite content
	updated := "Updated clipboard snippet"
	if err := driver.WriteText(updated); err != nil {
		t.Fatalf("WriteText overwrite failed: %v", err)
	}
	readUpdated, err := driver.ReadText()
	if err != nil {
		t.Fatalf("ReadText after overwrite failed: %v", err)
	}
	if readUpdated != updated {
		t.Errorf("Updated content mismatch: got %q, want %q", readUpdated, updated)
	}

	// 5. Clear
	if err := driver.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	afterClear, err := driver.ReadText()
	if err != nil {
		t.Fatalf("ReadText after clear failed: %v", err)
	}
	if afterClear != "" {
		t.Errorf("Expected empty string after clear, got %q", afterClear)
	}
}

func TestFileDriver_Truncation(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "clip.data")
	driver := NewFileDriver(filePath)

	oversized := strings.Repeat("A", defaultMaxClipboardFileSize+5000)
	if err := driver.WriteText(oversized); err != nil {
		t.Fatalf("WriteText with oversized data failed: %v", err)
	}

	result, err := driver.ReadText()
	if err != nil {
		t.Fatalf("ReadText failed: %v", err)
	}
	if len(result) != defaultMaxClipboardFileSize {
		t.Errorf("Expected truncated size %d, got %d", defaultMaxClipboardFileSize, len(result))
	}
}

func TestFileDriver_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "clip_concurrent.data")
	driver := NewFileDriver(filePath)

	var wg sync.WaitGroup
	const workers = 10
	const iterations = 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				content := strings.Repeat("X", (id+1)*10)
				_ = driver.WriteText(content)
				_, _ = driver.ReadText()
			}
		}(i)
	}
	wg.Wait()

	final, err := driver.ReadText()
	if err != nil {
		t.Fatalf("Final ReadText failed: %v", err)
	}
	if len(final) == 0 {
		t.Error("Expected non-empty final clipboard after concurrent writes")
	}
}
func TestFileDriver_DefaultPath(t *testing.T) {
	driver := NewFileDriver("")
	path := driver.Path()
	if path == "" {
		t.Fatal("NewFileDriver(\"\") returned empty path")
	}
	if !strings.HasSuffix(path, "clipboard.data") {
		t.Errorf("Expected path ending in 'clipboard.data', got %q", path)
	}
}

func TestFileDriver_NestedDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "sub1", "sub2", "sub3", "clip.data")
	driver := NewFileDriver(nestedPath)

	err := driver.WriteText("Nested Directory Content")
	if err != nil {
		t.Fatalf("WriteText in deeply nested path failed: %v", err)
	}

	read, err := driver.ReadText()
	if err != nil {
		t.Fatalf("ReadText failed: %v", err)
	}
	if read != "Nested Directory Content" {
		t.Errorf("ReadText mismatch: got %q", read)
	}
}
