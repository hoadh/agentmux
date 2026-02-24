package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// Event types sent to the TUI via tea.Msg.

// AssistantEvent represents text output from the claude assistant.
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

// ParseStream reads NDJSON line-by-line from r and sends typed events to ch.
func ParseStream(agentName string, r io.Reader, ch chan<- tea.Msg) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			// Malformed line: skip
			continue
		}
		msg := convertEvent(agentName, raw)
		if msg != nil {
			ch <- msg
		}
	}
}

// convertEvent maps a raw JSON object to a typed event.
func convertEvent(agentName string, raw map[string]any) tea.Msg {
	typ, _ := raw["type"].(string)
	switch typ {
	case "assistant":
		return convertAssistant(agentName, raw)
	case "tool_use":
		return convertToolUse(agentName, raw)
	case "tool_result":
		return convertToolResult(agentName, raw)
	case "result":
		return convertResult(agentName, raw)
	default:
		return nil
	}
}

func convertAssistant(agentName string, raw map[string]any) tea.Msg {
	text := extractNestedString(raw, "message", "content", "text")
	if text == "" {
		// Try alternate structure: message.content is array
		if msg, ok := raw["message"].(map[string]any); ok {
			if content, ok := msg["content"].([]any); ok && len(content) > 0 {
				if block, ok := content[0].(map[string]any); ok {
					text, _ = block["text"].(string)
				}
			}
		}
	}
	return AssistantEvent{AgentName: agentName, Text: text}
}

func convertToolUse(agentName string, raw map[string]any) tea.Msg {
	toolName := ""
	input := ""
	if tool, ok := raw["tool"].(map[string]any); ok {
		toolName, _ = tool["name"].(string)
		if inp, ok := tool["input"].(map[string]any); ok {
			b, _ := json.Marshal(inp)
			input = truncate(string(b), 120)
		}
	}
	return ToolUseEvent{AgentName: agentName, ToolName: toolName, Input: input}
}

func convertToolResult(agentName string, raw map[string]any) tea.Msg {
	content := ""
	if c, ok := raw["content"].(string); ok {
		content = truncate(c, 200)
	}
	return ToolResultEvent{AgentName: agentName, Content: content}
}

func convertResult(agentName string, raw map[string]any) tea.Msg {
	result, _ := raw["result"].(string)
	sessionID, _ := raw["session_id"].(string)
	var inputTokens, outputTokens int
	if usage, ok := raw["usage"].(map[string]any); ok {
		inputTokens = intFromAny(usage["input_tokens"])
		outputTokens = intFromAny(usage["output_tokens"])
	}
	return ResultEvent{
		AgentName:    agentName,
		Result:       result,
		SessionID:    sessionID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}
}

func extractNestedString(m map[string]any, keys ...string) string {
	var current any = m
	for _, k := range keys {
		if mp, ok := current.(map[string]any); ok {
			current = mp[k]
		} else {
			return ""
		}
	}
	s, _ := current.(string)
	return s
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("... (%d more)", len(s)-maxLen)
}
