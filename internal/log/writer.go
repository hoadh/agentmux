package log

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Writer persists agent events as JSONL files.
type Writer struct {
	dir   string
	files map[string]*os.File
	ts    string
	mu    sync.Mutex
}

// NewWriter creates a log writer for the given directory. Creates the directory if needed.
func NewWriter(dir string) (*Writer, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	return &Writer{
		dir:   dir,
		files: make(map[string]*os.File),
		ts:    time.Now().Format("20060102-150405"),
	}, nil
}

// Write serializes an event as a JSON line and appends to the agent's log file.
func (w *Writer) Write(agentName string, event interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	f, ok := w.files[agentName]
	if !ok {
		filename := fmt.Sprintf("%s-%s.jsonl", w.ts, agentName)
		path := filepath.Join(w.dir, filename)
		var err error
		f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		w.files[agentName] = f
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// Close flushes and closes all open log files.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var lastErr error
	for name, f := range w.files {
		if err := f.Close(); err != nil {
			lastErr = err
		}
		delete(w.files, name)
	}
	return lastErr
}
