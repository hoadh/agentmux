package agent

import (
	"fmt"
	"sort"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hoadh/agentmux/internal/agent/backend"
	"github.com/hoadh/agentmux/internal/config"
)

// AgentState represents the lifecycle state of an agent.
type AgentState int

const (
	StatePending AgentState = iota
	StateRunning
	StateDone
	StateFailed
	StateKilled
	StateBlocked
)

// String returns a human-readable label for the state.
func (s AgentState) String() string {
	switch s {
	case StatePending:
		return "Pending"
	case StateRunning:
		return "Running"
	case StateDone:
		return "Done"
	case StateFailed:
		return "Failed"
	case StateKilled:
		return "Killed"
	case StateBlocked:
		return "Blocked"
	default:
		return "Unknown"
	}
}

// TokenUsage tracks input/output token counts.
type TokenUsage struct {
	Input  int
	Output int
}

// AgentInfo holds runtime state for a registered agent.
type AgentInfo struct {
	Name      string
	State     AgentState
	Config    config.AgentConfig
	Process   *Process
	StartedAt time.Time
	Duration  time.Duration
	Tokens    TokenUsage
	LastEvent string
}

// Manager controls the lifecycle of all agents.
type Manager struct {
	agents   map[string]*AgentInfo
	order    []string // preserves config-defined agent order
	defaults config.AgentDefaults
	mu       sync.RWMutex
}

// NewManager creates a manager with the given defaults.
func NewManager(defaults config.AgentDefaults) *Manager {
	return &Manager{
		agents:   make(map[string]*AgentInfo),
		defaults: defaults,
	}
}

// SetOrder sets the display order for agents (from config).
func (m *Manager) SetOrder(order []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.order = order
}

// Register adds an agent to the manager in Pending state.
func (m *Manager) Register(name string, cfg config.AgentConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[name] = &AgentInfo{
		Name:   name,
		State:  StatePending,
		Config: cfg,
	}
}

// Start launches a registered agent. Returns its event channel.
func (m *Manager) Start(name string) (chan tea.Msg, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not registered", name)
	}
	if info.State == StateRunning {
		return nil, fmt.Errorf("agent %q already running", name)
	}

	b, err := backend.Get(info.Config.Backend)
	if err != nil {
		info.State = StateFailed
		return nil, fmt.Errorf("agent %q: %w", name, err)
	}

	proc := NewProcess(name, info.Config, m.defaults)
	if err := proc.Start(info.Config, m.defaults, b); err != nil {
		info.State = StateFailed
		return nil, fmt.Errorf("start agent %q: %w", name, err)
	}

	info.Process = proc
	info.State = StateRunning
	info.StartedAt = time.Now()
	info.Duration = 0

	return proc.EventCh, nil
}

// Stop gracefully shuts down a running agent.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.agents[name]
	if !ok {
		return fmt.Errorf("agent %q not registered", name)
	}
	if info.State != StateRunning {
		return fmt.Errorf("agent %q not running (state: %s)", name, info.State)
	}

	if info.Process != nil {
		if err := info.Process.Shutdown(); err != nil {
			return err
		}
	}

	info.State = StateKilled
	info.Duration = time.Since(info.StartedAt)
	return nil
}

// Restart stops and re-starts an agent.
func (m *Manager) Restart(name string) (chan tea.Msg, error) {
	m.mu.RLock()
	info, ok := m.agents[name]
	isRunning := ok && info.State == StateRunning
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("agent %q not registered", name)
	}

	if isRunning {
		if err := m.Stop(name); err != nil {
			return nil, err
		}
	}

	return m.Start(name)
}

// List returns all agents in config-defined order (copies for safe concurrent access).
// Agents not in the order list (e.g. dynamically spawned) are appended alphabetically.
func (m *Manager) List() []*AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[string]bool, len(m.agents))
	result := make([]*AgentInfo, 0, len(m.agents))

	// First: agents in config-defined order
	for _, name := range m.order {
		if info, ok := m.agents[name]; ok {
			cp := *info
			result = append(result, &cp)
			seen[name] = true
		}
	}

	// Then: any remaining agents (dynamically spawned) sorted alphabetically
	extras := make([]string, 0)
	for name := range m.agents {
		if !seen[name] {
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)
	for _, name := range extras {
		info := m.agents[name]
		cp := *info
		result = append(result, &cp)
	}

	return result
}

// Get returns a copy of info for a specific agent.
func (m *Manager) Get(name string) *AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.agents[name]
	if !ok {
		return nil
	}
	cp := *info
	return &cp
}

// SetState updates agent state (used by scheduler).
func (m *Manager) SetState(name string, state AgentState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if info, ok := m.agents[name]; ok {
		info.State = state
	}
}

// UpdateTokens records token usage for an agent.
func (m *Manager) UpdateTokens(name string, input, output int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if info, ok := m.agents[name]; ok {
		info.Tokens.Input += input
		info.Tokens.Output += output
	}
}

// UpdateDuration refreshes the duration for running agents.
func (m *Manager) UpdateDuration(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if info, ok := m.agents[name]; ok && info.State == StateRunning {
		info.Duration = time.Since(info.StartedAt)
	}
}

// UpdateLastEvent sets the last event summary for sidebar display.
func (m *Manager) UpdateLastEvent(name string, summary string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if info, ok := m.agents[name]; ok {
		info.LastEvent = summary
	}
}

// StopAll gracefully shuts down all running agents.
func (m *Manager) StopAll() {
	m.mu.RLock()
	running := make([]string, 0)
	for name, info := range m.agents {
		if info.State == StateRunning {
			running = append(running, name)
		}
	}
	m.mu.RUnlock()

	for _, name := range running {
		m.Stop(name)
	}
}

// BatchEvents wraps multiple events into a single tea.Msg for efficient processing.
type BatchEvents []tea.Msg

// WaitForEvent blocks for the first event, then drains any buffered events.
// Returns a BatchEvents msg containing 1+ events for the TUI to process at once.
func WaitForEvent(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		// Block for first event
		msg, ok := <-ch
		if !ok {
			return nil
		}
		batch := BatchEvents{msg}
		// Drain any buffered events (non-blocking)
		for {
			select {
			case m, ok := <-ch:
				if !ok {
					return batch
				}
				batch = append(batch, m)
				if len(batch) >= 50 {
					return batch
				}
			default:
				return batch
			}
		}
	}
}
