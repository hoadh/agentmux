package headless

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hoadh/agentmux/internal/agent/backend"
	"github.com/hoadh/agentmux/internal/dag"
)

func TestTextFormatter_AssistantEvent(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(backend.AssistantEvent{AgentName: "researcher", Text: "hello world"})
	assertContains(t, result, "[researcher]")
	assertContains(t, result, "hello world")
}

func TestTextFormatter_ToolUseEvent(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(backend.ToolUseEvent{AgentName: "coder", ToolName: "bash", Input: "ls"})
	assertContains(t, result, "[coder]")
	assertContains(t, result, "Tool: bash")
	assertContains(t, result, "ls")
}

func TestTextFormatter_ToolResultEvent(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(backend.ToolResultEvent{AgentName: "coder", Content: "file.go"})
	assertContains(t, result, "[coder]")
	assertContains(t, result, "file.go")
}

func TestTextFormatter_ResultEvent(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(backend.ResultEvent{AgentName: "a", InputTokens: 100, OutputTokens: 50})
	assertContains(t, result, "[a]")
	assertContains(t, result, "100 in")
	assertContains(t, result, "50 out")
}

func TestTextFormatter_AgentDoneEvent_Success(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(backend.AgentDoneEvent{AgentName: "a", ExitCode: 0})
	assertContains(t, result, "DONE")
}

func TestTextFormatter_AgentDoneEvent_Failure(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(backend.AgentDoneEvent{AgentName: "a", ExitCode: 1})
	assertContains(t, result, "FAILED")
	assertContains(t, result, "exit 1")
}

func TestTextFormatter_ErrorEvent(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(backend.ErrorEvent{AgentName: "a", Err: fmt.Errorf("boom")})
	assertContains(t, result, "ERROR")
	assertContains(t, result, "boom")
}

func TestTextFormatter_AgentStartedMsg(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(dag.AgentStartedMsg{Name: "planner"})
	assertContains(t, result, "[planner]")
	assertContains(t, result, "STARTED")
}

func TestTextFormatter_AgentBlockedMsg(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(dag.AgentBlockedMsg{Name: "deploy", Reason: "build failed"})
	assertContains(t, result, "[deploy]")
	assertContains(t, result, "BLOCKED")
	assertContains(t, result, "build failed")
}

func TestTextFormatter_PipelineDoneMsg_Success(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(dag.PipelineDoneMsg{Success: true})
	assertContains(t, result, "COMPLETED")
}

func TestTextFormatter_PipelineDoneMsg_Failure(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent(dag.PipelineDoneMsg{Success: false})
	assertContains(t, result, "FAILED")
}

func TestTextFormatter_UnknownEvent(t *testing.T) {
	f := &TextFormatter{}
	result := f.FormatEvent("unknown event")
	if result != "" {
		t.Errorf("expected empty string for unknown event, got %q", result)
	}
}

// --- NDJSON Formatter ---

func TestNDJSONFormatter_AssistantEvent(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent(backend.AssistantEvent{AgentName: "researcher", Text: "hello"})
	parsed := parseJSON(t, result)
	assertField(t, parsed, "agent", "researcher")
	assertField(t, parsed, "type", "assistant")
}

func TestNDJSONFormatter_ToolUseEvent(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent(backend.ToolUseEvent{AgentName: "coder", ToolName: "bash", Input: "ls"})
	parsed := parseJSON(t, result)
	assertField(t, parsed, "agent", "coder")
	assertField(t, parsed, "type", "tool_use")
}

func TestNDJSONFormatter_ToolResultEvent(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent(backend.ToolResultEvent{AgentName: "coder", Content: "output"})
	parsed := parseJSON(t, result)
	assertField(t, parsed, "agent", "coder")
	assertField(t, parsed, "type", "tool_result")
}

func TestNDJSONFormatter_ResultEvent(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent(backend.ResultEvent{AgentName: "a", InputTokens: 10, OutputTokens: 5, SessionID: "s1"})
	parsed := parseJSON(t, result)
	assertField(t, parsed, "agent", "a")
	assertField(t, parsed, "type", "result")
}

func TestNDJSONFormatter_AgentDoneEvent(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent(backend.AgentDoneEvent{AgentName: "a", ExitCode: 0})
	parsed := parseJSON(t, result)
	assertField(t, parsed, "type", "agent_done")
}

func TestNDJSONFormatter_ErrorEvent(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent(backend.ErrorEvent{AgentName: "a", Err: fmt.Errorf("fail")})
	parsed := parseJSON(t, result)
	assertField(t, parsed, "type", "error")
}

func TestNDJSONFormatter_AgentStartedMsg(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent(dag.AgentStartedMsg{Name: "planner"})
	parsed := parseJSON(t, result)
	assertField(t, parsed, "agent", "planner")
	assertField(t, parsed, "type", "agent_started")
}

func TestNDJSONFormatter_AgentBlockedMsg(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent(dag.AgentBlockedMsg{Name: "deploy", Reason: "dep failed"})
	parsed := parseJSON(t, result)
	assertField(t, parsed, "type", "agent_blocked")
}

func TestNDJSONFormatter_PipelineDoneMsg(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent(dag.PipelineDoneMsg{Success: true})
	parsed := parseJSON(t, result)
	assertField(t, parsed, "agent", "pipeline")
	assertField(t, parsed, "type", "pipeline_done")
}

func TestNDJSONFormatter_UnknownEvent(t *testing.T) {
	f := &NDJSONFormatter{}
	result := f.FormatEvent("unknown")
	if result != "" {
		t.Errorf("expected empty string for unknown event, got %q", result)
	}
}

// --- helpers ---

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

func parseJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("invalid JSON %q: %v", s, err)
	}
	return m
}

func assertField(t *testing.T, m map[string]any, key, expected string) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("missing field %q in %v", key, m)
		return
	}
	if fmt.Sprint(val) != expected {
		t.Errorf("field %q: expected %q, got %v", key, expected, val)
	}
}
