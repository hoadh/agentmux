package backend

import (
	"reflect"
	"strings"
	"testing"
)

func TestClaudeBuildArgs_Full(t *testing.T) {
	c := &Claude{}
	args := c.BuildArgs("hello", "opus", []string{"Read", "Write"}, 10)

	expected := []string{
		"-p", "hello",
		"--output-format", "stream-json", "--verbose",
		"--model", "opus",
		"--allowedTools", "Read",
		"--allowedTools", "Write",
		"--max-turns", "10",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("BuildArgs:\n  got  %v\n  want %v", args, expected)
	}
}

func TestClaudeBuildArgs_Minimal(t *testing.T) {
	c := &Claude{}
	args := c.BuildArgs("hello", "", nil, 0)

	expected := []string{"-p", "hello", "--output-format", "stream-json", "--verbose"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("BuildArgs:\n  got  %v\n  want %v", args, expected)
	}
}

func TestClaudeBinary(t *testing.T) {
	c := &Claude{}
	if c.Binary() != "claude" {
		t.Errorf("Binary: got %q, want %q", c.Binary(), "claude")
	}
}

func TestClaudeConvertAssistantText(t *testing.T) {
	c := &Claude{}
	raw := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": "Hello"},
			},
		},
	}
	msg := c.ConvertEvent("a1", raw)
	event, ok := msg.(AssistantEvent)
	if !ok {
		t.Fatalf("expected AssistantEvent, got %T", msg)
	}
	if event.Text != "Hello" {
		t.Errorf("Text: got %q, want %q", event.Text, "Hello")
	}
}

func TestClaudeConvertAssistantToolUse(t *testing.T) {
	c := &Claude{}
	raw := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"name":  "Read",
					"input": map[string]any{"path": "file.go"},
				},
			},
		},
	}
	msg := c.ConvertEvent("a1", raw)
	event, ok := msg.(ToolUseEvent)
	if !ok {
		t.Fatalf("expected ToolUseEvent, got %T", msg)
	}
	if event.ToolName != "Read" {
		t.Errorf("ToolName: got %q, want %q", event.ToolName, "Read")
	}
	if !strings.Contains(event.Input, "path") {
		t.Errorf("Input should contain path, got %q", event.Input)
	}
}

func TestClaudeConvertResult(t *testing.T) {
	c := &Claude{}
	raw := map[string]any{
		"type":       "result",
		"result":     "Done",
		"session_id": "s1",
		"usage": map[string]any{
			"input_tokens":  float64(100),
			"output_tokens": float64(50),
		},
	}
	msg := c.ConvertEvent("a1", raw)
	event, ok := msg.(ResultEvent)
	if !ok {
		t.Fatalf("expected ResultEvent, got %T", msg)
	}
	if event.Result != "Done" {
		t.Errorf("Result: got %q, want %q", event.Result, "Done")
	}
	if event.InputTokens != 100 {
		t.Errorf("InputTokens: got %d, want 100", event.InputTokens)
	}
	if event.OutputTokens != 50 {
		t.Errorf("OutputTokens: got %d, want 50", event.OutputTokens)
	}
}

func TestClaudeConvertUnknownType(t *testing.T) {
	c := &Claude{}
	raw := map[string]any{"type": "system"}
	msg := c.ConvertEvent("a1", raw)
	if msg != nil {
		t.Errorf("expected nil for system type, got %T", msg)
	}
}

func TestClaudeConvertEmptyContent(t *testing.T) {
	c := &Claude{}
	raw := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{},
		},
	}
	msg := c.ConvertEvent("a1", raw)
	if msg != nil {
		t.Errorf("expected nil for empty content, got %T", msg)
	}
}
