package result

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWriter_Write(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := w.Write("scout", "Result text"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "scout.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "Result text" {
		t.Errorf("got %q, want %q", string(data), "Result text")
	}
}

func TestWriter_Overwrite(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	w.Write("scout", "First")
	w.Write("scout", "Second")
	data, _ := os.ReadFile(filepath.Join(dir, "scout.md"))
	if string(data) != "Second" {
		t.Errorf("expected overwrite, got %q", string(data))
	}
}

func TestWriter_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			w.Write(fmt.Sprintf("agent%d", n), fmt.Sprintf("result %d", n))
		}(i)
	}
	wg.Wait()
	for i := 0; i < 10; i++ {
		path := filepath.Join(dir, fmt.Sprintf("agent%d.md", i))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing result file for agent%d", i)
		}
	}
}
