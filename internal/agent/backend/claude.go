package backend

import (
	"encoding/json"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

func init() { Register("claude", &Claude{}) }

// Claude implements the Backend interface for Claude CLI.
type Claude struct{}

func (c *Claude) Name() string   { return "claude" }
func (c *Claude) Binary() string { return "claude" }

func (c *Claude) BuildArgs(prompt, model string, allowedTools []string, maxTurns int) []string {
	args := []string{"-p", prompt, "--output-format", "stream-json", "--verbose"}

	if model != "" {
		args = append(args, "--model", model)
	}
	for _, t := range allowedTools {
		args = append(args, "--allowedTools", t)
	}
	if maxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(maxTurns))
	}
	return args
}

// ConvertEvent maps a raw Claude NDJSON object to a typed event.
func (c *Claude) ConvertEvent(agentName string, raw map[string]any) tea.Msg {
	typ, _ := raw["type"].(string)
	switch typ {
	case "assistant":
		return c.convertAssistant(agentName, raw)
	case "result":
		return c.convertResult(agentName, raw)
	default:
		return nil
	}
}

func (c *Claude) convertAssistant(agentName string, raw map[string]any) tea.Msg {
	msg, ok := raw["message"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := msg["content"].([]any)
	if !ok || len(content) == 0 {
		return nil
	}

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
			ct := ""
			if content, ok := block["content"].(string); ok {
				ct = truncate(content, 200)
			}
			return ToolResultEvent{AgentName: agentName, Content: ct}
		}
	}
	return nil
}

func (c *Claude) convertResult(agentName string, raw map[string]any) tea.Msg {
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

