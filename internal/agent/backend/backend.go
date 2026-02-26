package backend

import (
	"encoding/json"
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// Backend abstracts a CLI agent backend (e.g., Claude, Gemini).
type Backend interface {
	Name() string
	Binary() string
	BuildArgs(prompt, model string, allowedTools []string, maxTurns int) []string
	ConvertEvent(agentName string, raw map[string]any) tea.Msg
}

var registry = map[string]Backend{}

// Register adds a backend to the registry. Called from init().
func Register(name string, b Backend) {
	registry[name] = b
}

// Get returns a backend by name. Empty string defaults to "claude".
func Get(name string) (Backend, error) {
	if name == "" {
		name = "claude"
	}
	b, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q (available: %v)", name, List())
	}
	return b, nil
}

// List returns registered backend names sorted alphabetically.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Event types sent to the TUI via tea.Msg.

// AssistantEvent represents text output from the assistant.
type AssistantEvent struct {
	AgentName string
	Text      string
}

// ToolUseEvent represents a tool invocation by the agent.
type ToolUseEvent struct {
	AgentName string
	ToolName  string
	Input     string
}

// ToolResultEvent represents the result of a tool invocation.
type ToolResultEvent struct {
	AgentName string
	Content   string
}

// ResultEvent represents the final result of an agent run.
type ResultEvent struct {
	AgentName    string
	Result       string
	SessionID    string
	InputTokens  int
	OutputTokens int
}

// ErrorEvent represents a parsing or process error.
type ErrorEvent struct {
	AgentName string
	Err       error
}

// AgentDoneEvent is sent when the agent process exits.
type AgentDoneEvent struct {
	AgentName string
	ExitCode  int
}

// Shared utilities used by backend implementations.

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("... (%d more)", len(s)-maxLen)
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}
