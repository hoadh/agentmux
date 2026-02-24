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
// Real claude stream-json format has: system, assistant, result, rate_limit_event
// Tool use/result are embedded in assistant message.content array blocks.
func convertEvent(agentName string, raw map[string]any) tea.Msg {
	typ, _ := raw["type"].(string)
	switch typ {
	case "assistant":
		return convertAssistant(agentName, raw)
	case "result":
		return convertResult(agentName, raw)
	default:
		// system, rate_limit_event, etc. — skip
		return nil
	}
}

// convertAssistant handles assistant events where message.content is an array.
// Content blocks can be: {type:"text", text:"..."} or {type:"tool_use", name:"...", input:{...}}
// Returns multiple events via the first meaningful one found; tool_use blocks get separate events.
func convertAssistant(agentName string, raw map[string]any) tea.Msg {
	msg, ok := raw["message"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := msg["content"].([]any)
	if !ok || len(content) == 0 {
		return nil
	}

	// Process content blocks — return first meaningful event
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := block["type"].(string)
		switch blockType {
		case "text":
			text, _ := block["text"].(string)
			if text != "" {
				return AssistantEvent{AgentName: agentName, Text: text}
			}
		case "tool_use":
			toolName, _ := block["name"].(string)
			input := ""
			if inp, ok := block["input"].(map[string]any); ok {
				b, _ := json.Marshal(inp)
				input = truncate(string(b), 120)
			}
			return ToolUseEvent{AgentName: agentName, ToolName: toolName, Input: input}
		case "tool_result":
			c := ""
			if content, ok := block["content"].(string); ok {
				c = truncate(content, 200)
			}
			return ToolResultEvent{AgentName: agentName, Content: c}
		}
	}
	return nil
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
