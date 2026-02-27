package result

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Writer saves agent results as markdown files.
type Writer struct {
	dir string
	mu  sync.Mutex
}

// NewWriter creates a result writer for the given directory.
func NewWriter(dir string) (*Writer, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create results dir: %w", err)
	}
	return &Writer{dir: dir}, nil
}

// Write saves result text to {dir}/{agentName}.md. Overwrites existing.
func (w *Writer) Write(agentName, result string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	path := filepath.Join(w.dir, agentName+".md")
	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		return fmt.Errorf("write result for %q: %w", agentName, err)
	}
	return nil
}
