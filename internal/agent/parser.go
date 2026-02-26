package agent

import (
	"bufio"
	"encoding/json"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// EventConverter converts a raw NDJSON object to a typed event.
type EventConverter func(agentName string, raw map[string]any) tea.Msg

// ParseStream reads NDJSON line-by-line from r and sends typed events to ch.
func ParseStream(agentName string, r io.Reader, ch chan<- tea.Msg, convert EventConverter) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		msg := convert(agentName, raw)
		if msg != nil {
			ch <- msg
		}
	}
}
