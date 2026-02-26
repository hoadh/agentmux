package headless

import (
	"bytes"
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hoadh/agentmux/internal/agent/backend"
	"github.com/hoadh/agentmux/internal/dag"
)

// mockManager implements AgentManager for tests.
type mockManager struct {
	stopAllCalled bool
}

func (m *mockManager) UpdateLastEvent(string, string) {}
func (m *mockManager) UpdateDuration(string)          {}
func (m *mockManager) UpdateTokens(string, int, int)  {}
func (m *mockManager) StopAll()                       { m.stopAllCalled = true }

// mockScheduler implements PipelineScheduler for tests.
type mockScheduler struct {
	events []tea.Msg
	ch     chan tea.Msg
}

func newMockScheduler(events ...tea.Msg) *mockScheduler {
	return &mockScheduler{events: events, ch: make(chan tea.Msg, len(events)+1)}
}

func (s *mockScheduler) Run(_ context.Context) {
	for _, e := range s.events {
		s.ch <- e
	}
}

func (s *mockScheduler) EventCh() <-chan tea.Msg {
	return s.ch
}

func TestRunner_SuccessfulPipeline(t *testing.T) {
	var buf bytes.Buffer
	mgr := &mockManager{}
	sched := newMockScheduler(
		dag.AgentStartedMsg{Name: "a"},
		backend.AssistantEvent{AgentName: "a", Text: "hello"},
		backend.AgentDoneEvent{AgentName: "a", ExitCode: 0},
		dag.PipelineDoneMsg{Success: true},
	)
	runner := &Runner{
		manager:   mgr,
		scheduler: sched,
		formatter: &TextFormatter{},
		output:    &buf,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	code := runner.Run(ctx, cancel)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(buf.String(), "STARTED") {
		t.Error("expected STARTED in output")
	}
	if !strings.Contains(buf.String(), "COMPLETED") {
		t.Error("expected COMPLETED in output")
	}
}

func TestRunner_FailedPipeline(t *testing.T) {
	var buf bytes.Buffer
	mgr := &mockManager{}
	sched := newMockScheduler(
		dag.AgentStartedMsg{Name: "a"},
		backend.AgentDoneEvent{AgentName: "a", ExitCode: 1},
		dag.PipelineDoneMsg{Success: false},
	)
	runner := &Runner{
		manager:   mgr,
		scheduler: sched,
		formatter: &TextFormatter{},
		output:    &buf,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	code := runner.Run(ctx, cancel)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "FAILED") {
		t.Error("expected FAILED in output")
	}
}

func TestRunner_ContextCancellation(t *testing.T) {
	mgr := &mockManager{}
	// Scheduler that sends nothing — runner will block until ctx cancelled
	sched := &mockScheduler{ch: make(chan tea.Msg, 1)}
	runner := &Runner{
		manager:   mgr,
		scheduler: sched,
		formatter: &TextFormatter{},
		output:    &bytes.Buffer{},
	}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan int, 1)
	go func() {
		done <- runner.Run(ctx, cancel)
	}()

	cancel()
	code := <-done
	if code != 1 {
		t.Errorf("expected exit code 1 on cancellation, got %d", code)
	}
	if !mgr.stopAllCalled {
		t.Error("expected StopAll to be called on cancellation")
	}
}

func TestRunner_NDJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	mgr := &mockManager{}
	sched := newMockScheduler(
		backend.AssistantEvent{AgentName: "a", Text: "hi"},
		dag.PipelineDoneMsg{Success: true},
	)
	runner := &Runner{
		manager:   mgr,
		scheduler: sched,
		formatter: &NDJSONFormatter{},
		output:    &buf,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	code := runner.Run(ctx, cancel)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 NDJSON lines, got %d", len(lines))
	}
	// Verify each line is valid JSON
	for i, line := range lines {
		parsed := parseJSON(t, line)
		if _, ok := parsed["ts"]; !ok {
			t.Errorf("line %d missing 'ts' field", i)
		}
		if _, ok := parsed["type"]; !ok {
			t.Errorf("line %d missing 'type' field", i)
		}
	}
}
