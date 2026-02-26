package backend

import (
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"
)

func init() { Register("gemini", &Gemini{}) }

// Gemini implements the Backend interface for Gemini CLI.
type Gemini struct{}

func (g *Gemini) Name() string   { return "gemini" }
func (g *Gemini) Binary() string { return "gemini" }

func (g *Gemini) BuildArgs(prompt, model string, _ []string, _ int) []string {
	args := []string{prompt, "--output-format", "stream-json", "--approval-mode", "auto_edit"}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
}

// ConvertEvent maps a raw Gemini NDJSON object to a typed event.
// Gemini uses flat event types unlike Claude's nested content blocks.
func (g *Gemini) ConvertEvent(agentName string, raw map[string]any) tea.Msg {
	typ, _ := raw["type"].(string)
	switch typ {
	case "message":
		return g.convertMessage(agentName, raw)
	case "tool_use":
		return g.convertToolUse(agentName, raw)
	case "tool_result":
		return g.convertToolResult(agentName, raw)
	case "result":
		return g.convertResult(agentName, raw)
	default:
		// init, error, unknown — skip
		return nil
	}
}

func (g *Gemini) convertMessage(agentName string, raw map[string]any) tea.Msg {
	role, _ := raw["role"].(string)
	if role != "assistant" {
		return nil
	}
	content, _ := raw["content"].(string)
	if content == "" {
		return nil
	}
	return AssistantEvent{AgentName: agentName, Text: content}
}

func (g *Gemini) convertToolUse(agentName string, raw map[string]any) tea.Msg {
	toolName, _ := raw["tool_name"].(string)
	input := ""
	if params, ok := raw["parameters"].(map[string]any); ok {
		b, _ := json.Marshal(params)
		input = truncate(string(b), 120)
	}
	return ToolUseEvent{AgentName: agentName, ToolName: toolName, Input: input}
}

func (g *Gemini) convertToolResult(agentName string, raw map[string]any) tea.Msg {
	output, _ := raw["output"].(string)
	return ToolResultEvent{AgentName: agentName, Content: truncate(output, 200)}
}

func (g *Gemini) convertResult(agentName string, raw map[string]any) tea.Msg {
	result, _ := raw["final_response"].(string)
	var inputTokens, outputTokens int
	if stats, ok := raw["stats"].(map[string]any); ok {
		inputTokens = intFromAny(stats["input_tokens"])
		outputTokens = intFromAny(stats["output_tokens"])
	}
	return ResultEvent{
		AgentName:    agentName,
		Result:       result,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}
}
