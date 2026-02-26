package dag

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hoadh/agentmux/internal/agent"
	"github.com/hoadh/agentmux/internal/agent/backend"
)

// Scheduler events sent to the TUI.

// AgentStartedMsg signals that an agent was started by the scheduler.
type AgentStartedMsg struct{ Name string }

// AgentBlockedMsg signals that an agent was blocked due to a failed dependency.
type AgentBlockedMsg struct {
	Name   string
	Reason string
}

// PipelineDoneMsg signals that all agents have reached terminal states.
type PipelineDoneMsg struct{ Success bool }

// Scheduler executes agents according to the DAG dependency graph.
type Scheduler struct {
	graph   *Graph
	manager *agent.Manager
	pending map[string]int          // remaining dep count (mutable)
	status  map[string]agent.AgentState
	eventCh chan tea.Msg
	mu      sync.Mutex
}

// NewScheduler creates a scheduler for the given graph and manager.
func NewScheduler(g *Graph, mgr *agent.Manager) *Scheduler {
	return &Scheduler{
		graph:   g,
		manager: mgr,
		pending: make(map[string]int),
		status:  make(map[string]agent.AgentState),
		eventCh: make(chan tea.Msg, 1024),
	}
}

// EventCh returns the channel of scheduler events for the TUI.
func (s *Scheduler) EventCh() <-chan tea.Msg {
	return s.eventCh
}

// Run starts the pipeline. Blocks until all agents reach terminal state.
// Call in a goroutine.
func (s *Scheduler) Run(ctx context.Context) {
	nodes := s.graph.Nodes()
	if len(nodes) == 0 {
		s.eventCh <- PipelineDoneMsg{Success: true}
		return
	}

	// Init pending counts from graph in-degrees
	for _, name := range nodes {
		s.pending[name] = s.graph.InDegree(name)
		s.status[name] = agent.StatePending
	}

	// Collect all agent event channels
	agentEvents := make(chan tea.Msg, 256)

	// Start all roots
	roots := s.graph.Roots()
	for _, name := range roots {
		s.startAgent(name, agentEvents)
	}

	// Event loop
	for {
		select {
		case <-ctx.Done():
			s.manager.StopAll()
			return
		case msg := <-agentEvents:
			s.handleEvent(msg, agentEvents)
			// Forward to TUI
			s.eventCh <- msg

			if s.allTerminal() {
				s.eventCh <- PipelineDoneMsg{Success: s.allSuccess()}
				return
			}
		}
	}
}

func (s *Scheduler) startAgent(name string, agentEvents chan tea.Msg) {
	ch, err := s.manager.Start(name)
	if err != nil {
		s.mu.Lock()
		s.status[name] = agent.StateFailed
		s.mu.Unlock()
		s.eventCh <- backend.ErrorEvent{AgentName: name, Err: err}
		// Block downstream outside lock
		s.mu.Lock()
		blocked := s.collectBlocked(name)
		s.mu.Unlock()
		for _, b := range blocked {
			s.manager.SetState(b.Name, agent.StateBlocked)
			s.eventCh <- b
		}
		return
	}

	s.mu.Lock()
	s.status[name] = agent.StateRunning
	s.mu.Unlock()
	s.eventCh <- AgentStartedMsg{Name: name}

	// Forward agent events to the central channel
	go func() {
		for msg := range ch {
			agentEvents <- msg
		}
	}()
}

func (s *Scheduler) handleEvent(msg tea.Msg, agentEvents chan tea.Msg) {
	done, ok := msg.(backend.AgentDoneEvent)
	if !ok {
		return
	}

	s.mu.Lock()

	var toStart []string
	var blocked []AgentBlockedMsg

	if done.ExitCode == 0 {
		s.status[done.AgentName] = agent.StateDone

		// Check dependents
		for _, dep := range s.graph.Dependents(done.AgentName) {
			s.pending[dep]--
			if s.pending[dep] <= 0 && s.status[dep] == agent.StatePending {
				toStart = append(toStart, dep)
			}
		}
	} else {
		s.status[done.AgentName] = agent.StateFailed
		blocked = s.collectBlocked(done.AgentName)
	}
	s.mu.Unlock()

	// Update manager state outside lock
	if done.ExitCode == 0 {
		s.manager.SetState(done.AgentName, agent.StateDone)
		for _, name := range toStart {
			go s.startAgent(name, agentEvents)
		}
	} else {
		s.manager.SetState(done.AgentName, agent.StateFailed)
		for _, b := range blocked {
			s.manager.SetState(b.Name, agent.StateBlocked)
			s.eventCh <- b
		}
	}
}

// collectBlocked gathers all downstream agents to block (must hold s.mu).
func (s *Scheduler) collectBlocked(name string) []AgentBlockedMsg {
	var result []AgentBlockedMsg
	for _, dep := range s.graph.Dependents(name) {
		if s.status[dep] == agent.StatePending {
			s.status[dep] = agent.StateBlocked
			result = append(result, AgentBlockedMsg{Name: dep, Reason: name + " failed"})
			result = append(result, s.collectBlocked(dep)...)
		}
	}
	return result
}

func (s *Scheduler) allTerminal() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, state := range s.status {
		if state == agent.StatePending || state == agent.StateRunning {
			return false
		}
	}
	return true
}

func (s *Scheduler) allSuccess() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, state := range s.status {
		if state != agent.StateDone {
			return false
		}
	}
	return true
}

// Status returns a map of agent name → state for pipeline view.
func (s *Scheduler) Status() map[string]agent.AgentState {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string]agent.AgentState, len(s.status))
	for k, v := range s.status {
		result[k] = v
	}
	return result
}
