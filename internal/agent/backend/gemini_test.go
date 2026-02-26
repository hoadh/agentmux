package backend

import (
	"reflect"
	"testing"
)

func TestGeminiBuildArgs_WithModel(t *testing.T) {
	g := &Gemini{}
	args := g.BuildArgs("hello", "gemini-2.5-pro", []string{"Read"}, 10)

	// Tools and maxTurns are ignored for Gemini
	expected := []string{
		"hello",
		"--output-format", "stream-json",
		"--approval-mode", "auto_edit",
		"--model", "gemini-2.5-pro",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("BuildArgs:\n  got  %v\n  want %v", args, expected)
	}
}

func TestGeminiBuildArgs_NoModel(t *testing.T) {
	g := &Gemini{}
	args := g.BuildArgs("hello", "", nil, 0)

	expected := []string{
		"hello",
		"--output-format", "stream-json",
		"--approval-mode", "auto_edit",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("BuildArgs:\n  got  %v\n  want %v", args, expected)
	}
}

func TestGeminiBinary(t *testing.T) {
	g := &Gemini{}
	if g.Binary() != "gemini" {
		t.Errorf("Binary: got %q, want %q", g.Binary(), "gemini")
	}
}

func TestGeminiConvertMessage(t *testing.T) {
	g := &Gemini{}
	raw := map[string]any{
		"type":    "message",
		"role":    "assistant",
		"content": "Analyzing...",
	}
	msg := g.ConvertEvent("a1", raw)
	event, ok := msg.(AssistantEvent)
	if !ok {
		t.Fatalf("expected AssistantEvent, got %T", msg)
	}
	if event.Text != "Analyzing..." {
		t.Errorf("Text: got %q, want %q", event.Text, "Analyzing...")
	}
	if event.AgentName != "a1" {
		t.Errorf("AgentName: got %q, want %q", event.AgentName, "a1")
	}
}

func TestGeminiConvertMessage_UserSkipped(t *testing.T) {
	g := &Gemini{}
	raw := map[string]any{
		"type":    "message",
		"role":    "user",
		"content": "should be ignored",
	}
	msg := g.ConvertEvent("a1", raw)
	if msg != nil {
		t.Errorf("expected nil for user message, got %T", msg)
	}
}

func TestGeminiConvertToolUse(t *testing.T) {
	g := &Gemini{}
	raw := map[string]any{
		"type":      "tool_use",
		"tool_name": "read_file",
		"tool_id":   "t1",
		"parameters": map[string]any{
			"path": "main.go",
		},
	}
	msg := g.ConvertEvent("a1", raw)
	event, ok := msg.(ToolUseEvent)
	if !ok {
		t.Fatalf("expected ToolUseEvent, got %T", msg)
	}
	if event.ToolName != "read_file" {
		t.Errorf("ToolName: got %q, want %q", event.ToolName, "read_file")
	}
}

func TestGeminiConvertToolResult(t *testing.T) {
	g := &Gemini{}
	raw := map[string]any{
		"type":    "tool_result",
		"tool_id": "t1",
		"status":  "success",
		"output":  "package main...",
	}
	msg := g.ConvertEvent("a1", raw)
	event, ok := msg.(ToolResultEvent)
	if !ok {
		t.Fatalf("expected ToolResultEvent, got %T", msg)
	}
	if event.Content != "package main..." {
		t.Errorf("Content: got %q, want %q", event.Content, "package main...")
	}
}

func TestGeminiConvertResult(t *testing.T) {
	g := &Gemini{}
	raw := map[string]any{
		"type":           "result",
		"success":        true,
		"final_response": "Done.",
		"stats": map[string]any{
			"input_tokens":  float64(100),
			"output_tokens": float64(50),
		},
	}
	msg := g.ConvertEvent("a1", raw)
	event, ok := msg.(ResultEvent)
	if !ok {
		t.Fatalf("expected ResultEvent, got %T", msg)
	}
	if event.Result != "Done." {
		t.Errorf("Result: got %q, want %q", event.Result, "Done.")
	}
	if event.InputTokens != 100 {
		t.Errorf("InputTokens: got %d, want 100", event.InputTokens)
	}
	if event.OutputTokens != 50 {
		t.Errorf("OutputTokens: got %d, want 50", event.OutputTokens)
	}
}

func TestGeminiConvertInit_Skipped(t *testing.T) {
	g := &Gemini{}
	raw := map[string]any{
		"type":       "init",
		"session_id": "abc",
		"model":      "gemini-2.5-pro",
	}
	msg := g.ConvertEvent("a1", raw)
	if msg != nil {
		t.Errorf("expected nil for init event, got %T", msg)
	}
}

func TestGeminiConvertUnknown_Skipped(t *testing.T) {
	g := &Gemini{}
	raw := map[string]any{"type": "something_new"}
	msg := g.ConvertEvent("a1", raw)
	if msg != nil {
		t.Errorf("expected nil for unknown event, got %T", msg)
	}
}
