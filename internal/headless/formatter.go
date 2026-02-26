package headless

import (
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hoadh/agentmux/internal/agent/backend"
	"github.com/hoadh/agentmux/internal/dag"
)

// Formatter converts pipeline events to output strings.
type Formatter interface {
	FormatEvent(msg tea.Msg) string
}

// TextFormatter produces human-readable timestamped lines.
type TextFormatter struct{}

func (f *TextFormatter) FormatEvent(msg tea.Msg) string {
	ts := time.Now().Format("15:04:05")
	switch evt := msg.(type) {
	case backend.AssistantEvent:
		return fmt.Sprintf("[%s] [%s] %s", ts, evt.AgentName, evt.Text)
	case backend.ToolUseEvent:
		return fmt.Sprintf("[%s] [%s] -> Tool: %s %s", ts, evt.AgentName, evt.ToolName, evt.Input)
	case backend.ToolResultEvent:
		return fmt.Sprintf("[%s] [%s]    <- %s", ts, evt.AgentName, evt.Content)
	case backend.ResultEvent:
		return fmt.Sprintf("[%s] [%s] Done (%d in / %d out tokens)",
			ts, evt.AgentName, evt.InputTokens, evt.OutputTokens)
	case backend.AgentDoneEvent:
		if evt.ExitCode != 0 {
			return fmt.Sprintf("[%s] [%s] FAILED (exit %d)", ts, evt.AgentName, evt.ExitCode)
		}
		return fmt.Sprintf("[%s] [%s] DONE", ts, evt.AgentName)
	case backend.ErrorEvent:
		return fmt.Sprintf("[%s] [%s] ERROR: %s", ts, evt.AgentName, evt.Err)
	case dag.AgentStartedMsg:
		return fmt.Sprintf("[%s] [%s] STARTED", ts, evt.Name)
	case dag.AgentBlockedMsg:
		return fmt.Sprintf("[%s] [%s] BLOCKED: %s", ts, evt.Name, evt.Reason)
	case dag.PipelineDoneMsg:
		if evt.Success {
			return fmt.Sprintf("[%s] [pipeline] COMPLETED", ts)
		}
		return fmt.Sprintf("[%s] [pipeline] FAILED", ts)
	}
	return ""
}

// NDJSONFormatter produces one JSON object per line.
type NDJSONFormatter struct{}

type jsonEvent struct {
	Timestamp string `json:"ts"`
	Agent     string `json:"agent"`
	Type      string `json:"type"`
	Data      any    `json:"data,omitempty"`
}

func (f *NDJSONFormatter) FormatEvent(msg tea.Msg) string {
	evt := jsonEvent{Timestamp: time.Now().Format(time.RFC3339)}
	switch m := msg.(type) {
	case backend.AssistantEvent:
		evt.Agent = m.AgentName
		evt.Type = "assistant"
		evt.Data = map[string]string{"text": m.Text}
	case backend.ToolUseEvent:
		evt.Agent = m.AgentName
		evt.Type = "tool_use"
		evt.Data = map[string]string{"tool": m.ToolName, "input": m.Input}
	case backend.ToolResultEvent:
		evt.Agent = m.AgentName
		evt.Type = "tool_result"
		evt.Data = map[string]string{"content": m.Content}
	case backend.ResultEvent:
		evt.Agent = m.AgentName
		evt.Type = "result"
		evt.Data = map[string]any{
			"input_tokens":  m.InputTokens,
			"output_tokens": m.OutputTokens,
			"session_id":    m.SessionID,
		}
	case backend.AgentDoneEvent:
		evt.Agent = m.AgentName
		evt.Type = "agent_done"
		evt.Data = map[string]int{"exit_code": m.ExitCode}
	case backend.ErrorEvent:
		evt.Agent = m.AgentName
		evt.Type = "error"
		evt.Data = map[string]string{"error": m.Err.Error()}
	case dag.AgentStartedMsg:
		evt.Agent = m.Name
		evt.Type = "agent_started"
	case dag.AgentBlockedMsg:
		evt.Agent = m.Name
		evt.Type = "agent_blocked"
		evt.Data = map[string]string{"reason": m.Reason}
	case dag.PipelineDoneMsg:
		evt.Agent = "pipeline"
		evt.Type = "pipeline_done"
		evt.Data = map[string]bool{"success": m.Success}
	default:
		return ""
	}
	b, err := json.Marshal(evt)
	if err != nil {
		return ""
	}
	return string(b)
}
