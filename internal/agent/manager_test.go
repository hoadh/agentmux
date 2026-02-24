package agent

import (
	"testing"
	"time"

	"github.com/hoadh/agentmux/internal/config"
)

func TestNewManager(t *testing.T) {
	defaults := config.AgentDefaults{
		Model:    "sonnet",
		MaxTurns: 10,
	}

	m := NewManager(defaults)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if len(m.agents) != 0 {
		t.Errorf("agents count: got %d, want 0", len(m.agents))
	}
}

func TestRegister(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	cfg := config.AgentConfig{
		Prompt:   "Test prompt",
		WorkDir:  "/tmp",
		Model:    "sonnet",
		MaxTurns: 10,
	}

	m.Register("agent1", cfg)

	info := m.Get("agent1")
	if info == nil {
		t.Fatal("agent1 not registered")
	}

	if info.Name != "agent1" {
		t.Errorf("Name: got %q, want %q", info.Name, "agent1")
	}

	if info.State != StatePending {
		t.Errorf("State: got %v, want %v", info.State, StatePending)
	}

	if info.Config.Prompt != "Test prompt" {
		t.Errorf("Prompt: got %q, want %q", info.Config.Prompt, "Test prompt")
	}
}

func TestRegister_Multiple(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("agent1", config.AgentConfig{Prompt: "Prompt 1"})
	m.Register("agent2", config.AgentConfig{Prompt: "Prompt 2"})
	m.Register("agent3", config.AgentConfig{Prompt: "Prompt 3"})

	list := m.List()
	if len(list) != 3 {
		t.Fatalf("agents count: got %d, want 3", len(list))
	}
}

func TestList_Empty(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	list := m.List()
	if len(list) != 0 {
		t.Fatalf("agents count: got %d, want 0", len(list))
	}
}

func TestList_Alphabetical(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("zebra", config.AgentConfig{Prompt: "Z"})
	m.Register("apple", config.AgentConfig{Prompt: "A"})
	m.Register("mango", config.AgentConfig{Prompt: "M"})

	list := m.List()

	expected := []string{"apple", "mango", "zebra"}
	if len(list) != len(expected) {
		t.Fatalf("agents count: got %d, want %d", len(list), len(expected))
	}

	for i, agent := range list {
		if agent.Name != expected[i] {
			t.Errorf("agent[%d]: got %q, want %q", i, agent.Name, expected[i])
		}
	}
}

func TestGet(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	cfg := config.AgentConfig{Prompt: "Test"}
	m.Register("agent1", cfg)

	info := m.Get("agent1")
	if info == nil {
		t.Fatal("Get returned nil")
	}

	if info.Name != "agent1" {
		t.Errorf("Name: got %q, want %q", info.Name, "agent1")
	}
}

func TestGet_NotFound(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	info := m.Get("nonexistent")
	if info != nil {
		t.Errorf("Get should return nil for nonexistent agent, got %v", info)
	}
}

func TestSetState(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("agent1", config.AgentConfig{Prompt: "Test"})

	m.SetState("agent1", StateRunning)
	info := m.Get("agent1")
	if info.State != StateRunning {
		t.Errorf("State: got %v, want %v", info.State, StateRunning)
	}

	m.SetState("agent1", StateDone)
	info = m.Get("agent1")
	if info.State != StateDone {
		t.Errorf("State: got %v, want %v", info.State, StateDone)
	}
}

func TestSetState_NotFound(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	// Should not panic
	m.SetState("nonexistent", StateRunning)
}

func TestUpdateTokens(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("agent1", config.AgentConfig{Prompt: "Test"})

	m.UpdateTokens("agent1", 100, 50)
	info := m.Get("agent1")
	if info.Tokens.Input != 100 {
		t.Errorf("Input tokens: got %d, want 100", info.Tokens.Input)
	}
	if info.Tokens.Output != 50 {
		t.Errorf("Output tokens: got %d, want 50", info.Tokens.Output)
	}

	m.UpdateTokens("agent1", 50, 25)
	info = m.Get("agent1")
	if info.Tokens.Input != 150 {
		t.Errorf("Input tokens after update: got %d, want 150", info.Tokens.Input)
	}
	if info.Tokens.Output != 75 {
		t.Errorf("Output tokens after update: got %d, want 75", info.Tokens.Output)
	}
}

func TestUpdateTokens_NotFound(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	// Should not panic
	m.UpdateTokens("nonexistent", 100, 50)
}

func TestUpdateDuration(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("agent1", config.AgentConfig{Prompt: "Test"})
	m.SetState("agent1", StateRunning)

	startTime := time.Now()
	info := m.Get("agent1")
	info.StartedAt = startTime

	time.Sleep(10 * time.Millisecond)

	m.UpdateDuration("agent1")
	info = m.Get("agent1")

	if info.Duration == 0 {
		t.Fatal("Duration should be greater than 0")
	}

	if info.Duration < 10*time.Millisecond {
		t.Errorf("Duration: got %v, want >= 10ms", info.Duration)
	}
}

func TestUpdateDuration_NotRunning(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("agent1", config.AgentConfig{Prompt: "Test"})
	m.SetState("agent1", StatePending)

	m.UpdateDuration("agent1")
	info := m.Get("agent1")

	if info.Duration != 0 {
		t.Errorf("Duration should remain 0 for non-running agent, got %v", info.Duration)
	}
}

func TestUpdateLastEvent(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("agent1", config.AgentConfig{Prompt: "Test"})

	m.UpdateLastEvent("agent1", "Assistant: Hello")
	info := m.Get("agent1")
	if info.LastEvent != "Assistant: Hello" {
		t.Errorf("LastEvent: got %q, want %q", info.LastEvent, "Assistant: Hello")
	}

	m.UpdateLastEvent("agent1", "Tool: Read")
	info = m.Get("agent1")
	if info.LastEvent != "Tool: Read" {
		t.Errorf("LastEvent: got %q, want %q", info.LastEvent, "Tool: Read")
	}
}

func TestUpdateLastEvent_NotFound(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	// Should not panic
	m.UpdateLastEvent("nonexistent", "Some event")
}

func TestStopAll_NoAgents(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	// Should not panic
	m.StopAll()
}

func TestStopAll_NoRunningAgents(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("agent1", config.AgentConfig{Prompt: "Test"})
	m.Register("agent2", config.AgentConfig{Prompt: "Test"})

	// Should not panic
	m.StopAll()
}

func TestAgentState_String(t *testing.T) {
	tests := []struct {
		state AgentState
		want  string
	}{
		{StatePending, "Pending"},
		{StateRunning, "Running"},
		{StateDone, "Done"},
		{StateFailed, "Failed"},
		{StateKilled, "Killed"},
		{StateBlocked, "Blocked"},
		{AgentState(999), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State.String(): got %q, want %q", got, tt.want)
		}
	}
}

func TestConcurrentRegisterAndList(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	// Register agents concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			name := "agent-" + string(rune(idx))
			m.Register(name, config.AgentConfig{Prompt: "Test"})
			done <- true
		}(i)
	}

	// Wait for all registrations
	for i := 0; i < 10; i++ {
		<-done
	}

	// List agents concurrently
	for i := 0; i < 10; i++ {
		go func() {
			list := m.List()
			if len(list) != 10 {
				t.Errorf("agents count: got %d, want 10", len(list))
			}
			done <- true
		}()
	}

	// Wait for all lists
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConcurrentStateUpdates(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("agent1", config.AgentConfig{Prompt: "Test"})

	// Update state concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			state := AgentState(idx % 3)
			m.SetState("agent1", state)
			done <- true
		}(i)
	}

	// Wait for all updates
	for i := 0; i < 10; i++ {
		<-done
	}

	info := m.Get("agent1")
	if info == nil {
		t.Fatal("agent1 should exist")
	}
}

func TestTokenUsageAccumulation(t *testing.T) {
	m := NewManager(config.AgentDefaults{})

	m.Register("agent1", config.AgentConfig{Prompt: "Test"})

	m.UpdateTokens("agent1", 100, 50)
	m.UpdateTokens("agent1", 100, 50)
	m.UpdateTokens("agent1", 100, 50)

	info := m.Get("agent1")
	if info.Tokens.Input != 300 {
		t.Errorf("Total input tokens: got %d, want 300", info.Tokens.Input)
	}
	if info.Tokens.Output != 150 {
		t.Errorf("Total output tokens: got %d, want 150", info.Tokens.Output)
	}
}
