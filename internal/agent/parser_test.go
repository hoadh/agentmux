package agent

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseStream_AssistantEvent(t *testing.T) {
	ndjson := `{"type":"assistant","message":{"content":{"text":"Hello, world!"}}}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	msg := <-ch
	event, ok := msg.(AssistantEvent)
	if !ok {
		t.Fatalf("expected AssistantEvent, got %T", msg)
	}

	if event.AgentName != "agent1" {
		t.Errorf("AgentName: got %q, want %q", event.AgentName, "agent1")
	}

	if event.Text != "Hello, world!" {
		t.Errorf("Text: got %q, want %q", event.Text, "Hello, world!")
	}
}

func TestParseStream_AssistantEvent_ArrayContent(t *testing.T) {
	ndjson := `{"type":"assistant","message":{"content":[{"text":"Hello, world!"}]}}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	msg := <-ch
	event, ok := msg.(AssistantEvent)
	if !ok {
		t.Fatalf("expected AssistantEvent, got %T", msg)
	}

	if event.Text != "Hello, world!" {
		t.Errorf("Text: got %q, want %q", event.Text, "Hello, world!")
	}
}

func TestParseStream_ToolUseEvent(t *testing.T) {
	ndjson := `{"type":"tool_use","tool":{"name":"Read","input":{"file_path":"/tmp/file.txt"}}}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	msg := <-ch
	event, ok := msg.(ToolUseEvent)
	if !ok {
		t.Fatalf("expected ToolUseEvent, got %T", msg)
	}

	if event.AgentName != "agent1" {
		t.Errorf("AgentName: got %q, want %q", event.AgentName, "agent1")
	}

	if event.ToolName != "Read" {
		t.Errorf("ToolName: got %q, want %q", event.ToolName, "Read")
	}

	if !strings.Contains(event.Input, "file_path") {
		t.Errorf("Input should contain file_path, got %q", event.Input)
	}
}

func TestParseStream_ToolResultEvent(t *testing.T) {
	ndjson := `{"type":"tool_result","content":"File contents here"}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	msg := <-ch
	event, ok := msg.(ToolResultEvent)
	if !ok {
		t.Fatalf("expected ToolResultEvent, got %T", msg)
	}

	if event.AgentName != "agent1" {
		t.Errorf("AgentName: got %q, want %q", event.AgentName, "agent1")
	}

	if event.Content != "File contents here" {
		t.Errorf("Content: got %q, want %q", event.Content, "File contents here")
	}
}

func TestParseStream_ResultEvent(t *testing.T) {
	ndjson := `{"type":"result","result":"Task completed","session_id":"sess-123","usage":{"input_tokens":100,"output_tokens":50}}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	msg := <-ch
	event, ok := msg.(ResultEvent)
	if !ok {
		t.Fatalf("expected ResultEvent, got %T", msg)
	}

	if event.AgentName != "agent1" {
		t.Errorf("AgentName: got %q, want %q", event.AgentName, "agent1")
	}

	if event.Result != "Task completed" {
		t.Errorf("Result: got %q, want %q", event.Result, "Task completed")
	}

	if event.SessionID != "sess-123" {
		t.Errorf("SessionID: got %q, want %q", event.SessionID, "sess-123")
	}

	if event.InputTokens != 100 {
		t.Errorf("InputTokens: got %d, want 100", event.InputTokens)
	}

	if event.OutputTokens != 50 {
		t.Errorf("OutputTokens: got %d, want 50", event.OutputTokens)
	}
}

func TestParseStream_UnknownType(t *testing.T) {
	ndjson := `{"type":"unknown","data":"something"}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	select {
	case msg := <-ch:
		if msg != nil {
			t.Fatalf("expected no message for unknown type, got %T", msg)
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: no message sent
	}
}

func TestParseStream_MalformedJSON(t *testing.T) {
	ndjson := `{"type":"assistant","message":invalid json`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	select {
	case <-ch:
		t.Fatal("expected no message on malformed JSON, but got one")
	case <-time.After(100 * time.Millisecond):
		// Expected: malformed line is skipped
	}
}

func TestParseStream_MultipleEvents(t *testing.T) {
	ndjson := `{"type":"assistant","message":{"content":{"text":"Hello"}}}
{"type":"tool_use","tool":{"name":"Bash","input":{"command":"ls"}}}
{"type":"tool_result","content":"file1.txt"}
{"type":"result","result":"Done","session_id":"s1","usage":{"input_tokens":10,"output_tokens":5}}`

	ch := make(chan tea.Msg, 4)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	events := make([]tea.Msg, 0, 4)
	for i := 0; i < 4; i++ {
		select {
		case msg := <-ch:
			if msg != nil {
				events = append(events, msg)
			}
		case <-time.After(500 * time.Millisecond):
			break
		}
	}

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	_, ok1 := events[0].(AssistantEvent)
	_, ok2 := events[1].(ToolUseEvent)
	_, ok3 := events[2].(ToolResultEvent)
	_, ok4 := events[3].(ResultEvent)

	if !ok1 || !ok2 || !ok3 || !ok4 {
		t.Fatal("event types do not match expected sequence")
	}
}

func TestParseStream_EmptyInput(t *testing.T) {
	ndjson := ""

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	select {
	case msg := <-ch:
		if msg != nil {
			t.Fatalf("expected no message on empty input, got %T", msg)
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: EOF on empty input
	}
}

func TestParseStream_ToolUseInputTruncation(t *testing.T) {
	// Create a long input JSON
	longInput := strings.Repeat("x", 500)
	ndjson := `{"type":"tool_use","tool":{"name":"Read","input":{"data":"` + longInput + `"}}}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	msg := <-ch
	event, ok := msg.(ToolUseEvent)
	if !ok {
		t.Fatalf("expected ToolUseEvent, got %T", msg)
	}

	// The truncate function adds "... (N more)" which extends the length
	if len(event.Input) <= 120 {
		t.Errorf("Input should be longer than 120 (includes truncation marker), got %d", len(event.Input))
	}

	if !strings.Contains(event.Input, "...") {
		t.Errorf("Truncated input should contain ..., got %q", event.Input)
	}
}

func TestParseStream_ToolResultTruncation(t *testing.T) {
	// Create a long result
	longContent := strings.Repeat("x", 500)
	ndjson := `{"type":"tool_result","content":"` + longContent + `"}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	msg := <-ch
	event, ok := msg.(ToolResultEvent)
	if !ok {
		t.Fatalf("expected ToolResultEvent, got %T", msg)
	}

	// truncate(s, 200) returns s[:200] + "... (N more)" suffix
	if len(event.Content) > 230 {
		t.Errorf("Content should be truncated (200 + suffix), got %d", len(event.Content))
	}

	if !strings.Contains(event.Content, "...") {
		t.Errorf("Truncated content should contain ..., got %q", event.Content)
	}
}

func TestParseStream_NoType(t *testing.T) {
	ndjson := `{"message":{"content":{"text":"Hello"}}}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	select {
	case msg := <-ch:
		if msg != nil {
			t.Fatalf("expected no message when type is missing, got %T", msg)
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: no message
	}
}

func TestParseStream_PartialData(t *testing.T) {
	// Missing optional fields
	ndjson := `{"type":"assistant"}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	msg := <-ch
	event, ok := msg.(AssistantEvent)
	if !ok {
		t.Fatalf("expected AssistantEvent, got %T", msg)
	}

	if event.Text != "" {
		t.Errorf("Text should be empty, got %q", event.Text)
	}
}

func TestParseStream_ResultMissingUsage(t *testing.T) {
	ndjson := `{"type":"result","result":"Done","session_id":"s1"}`

	ch := make(chan tea.Msg, 1)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	msg := <-ch
	event, ok := msg.(ResultEvent)
	if !ok {
		t.Fatalf("expected ResultEvent, got %T", msg)
	}

	if event.InputTokens != 0 {
		t.Errorf("InputTokens: got %d, want 0", event.InputTokens)
	}

	if event.OutputTokens != 0 {
		t.Errorf("OutputTokens: got %d, want 0", event.OutputTokens)
	}
}

func TestParseStream_MixedValidAndInvalid(t *testing.T) {
	ndjson := `{"type":"assistant","message":{"content":{"text":"Hello"}}}
{"invalid json here
{"type":"tool_result","content":"Result"}`

	ch := make(chan tea.Msg, 2)
	go ParseStream("agent1", strings.NewReader(ndjson), ch)

	events := make([]tea.Msg, 0, 2)
	for i := 0; i < 2; i++ {
		select {
		case msg := <-ch:
			if msg != nil {
				events = append(events, msg)
			}
		case <-time.After(100 * time.Millisecond):
			break
		}
	}

	// Should have 2 valid events
	if len(events) != 2 {
		t.Fatalf("expected 2 events (malformed lines skipped), got %d", len(events))
	}

	_, ok1 := events[0].(AssistantEvent)
	_, ok2 := events[1].(ToolResultEvent)

	if !ok1 || !ok2 {
		t.Fatal("event types do not match expected sequence")
	}
}
