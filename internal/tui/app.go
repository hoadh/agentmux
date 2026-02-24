package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hoadh/agentmux/internal/agent"
	"github.com/hoadh/agentmux/internal/config"
	"github.com/hoadh/agentmux/internal/dag"
	agentlog "github.com/hoadh/agentmux/internal/log"
)

type focus int

const (
	focusSidebar focus = iota
	focusDetail
	focusSpawn
)

// AppModel is the root Bubbletea model.
type AppModel struct {
	sidebar      SidebarModel
	detail       DetailModel
	statusbar    StatusBarModel
	spawn        SpawnModel
	focus        focus
	manager      *agent.Manager
	scheduler    *dag.Scheduler
	graph        *dag.Graph
	ctx          context.Context
	cancel       context.CancelFunc
	logWriter    *agentlog.Writer
	width        int
	height       int
	ready        bool
	confirmKill  string // agent name pending kill confirmation
	nextAgentID  int
}

// NewApp creates the root TUI model.
func NewApp(mgr *agent.Manager, sched *dag.Scheduler, graph *dag.Graph, ctx context.Context, cancel context.CancelFunc) AppModel {
	agents := mgr.List()
	sidebar := NewSidebar(agents)
	sidebar.SetFocused(true)

	logWriter, _ := agentlog.NewWriter()

	return AppModel{
		sidebar:   sidebar,
		detail:    NewDetail(),
		statusbar: NewStatusBar(),
		spawn:     NewSpawnDialog(),
		focus:     focusSidebar,
		manager:   mgr,
		scheduler: sched,
		graph:     graph,
		ctx:       ctx,
		cancel:    cancel,
		logWriter: logWriter,
	}
}

// Init starts the scheduler and arms event listeners.
func (m AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{}

	// Start scheduler in background
	if m.scheduler != nil {
		go m.scheduler.Run(m.ctx)
		cmds = append(cmds, agent.WaitForEvent(m.scheduler.EventCh()))
	}

	// Arm event listeners for any already-started agents
	for _, info := range m.manager.List() {
		if info.Process != nil {
			cmds = append(cmds, agent.WaitForEvent(info.Process.EventCh))
		}
	}

	// Select first agent if available
	if name := m.sidebar.SelectedAgent(); name != "" {
		m.detail.SetAgent(name)
	}

	return tea.Batch(cmds...)
}

// Update handles all messages.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.resizeAll()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	// Agent events
	case agent.AssistantEvent:
		m.detail.AppendLine(msg.AgentName, msg.Text)
		m.manager.UpdateLastEvent(msg.AgentName, "writing...")
		m.manager.UpdateDuration(msg.AgentName)
		m.logEvent(msg.AgentName, msg)
		m.refreshSidebar()
		cmds = append(cmds, m.rearmScheduler())
		return m, tea.Batch(cmds...)

	case agent.ToolUseEvent:
		line := fmt.Sprintf("→ Tool: %s %s", msg.ToolName, msg.Input)
		m.detail.AppendLine(msg.AgentName, line)
		m.manager.UpdateLastEvent(msg.AgentName, fmt.Sprintf("Tool: %s", msg.ToolName))
		m.manager.UpdateDuration(msg.AgentName)
		m.logEvent(msg.AgentName, msg)
		m.refreshSidebar()
		cmds = append(cmds, m.rearmScheduler())
		return m, tea.Batch(cmds...)

	case agent.ToolResultEvent:
		line := fmt.Sprintf("  ← %s", msg.Content)
		m.detail.AppendLine(msg.AgentName, line)
		m.logEvent(msg.AgentName, msg)
		cmds = append(cmds, m.rearmScheduler())
		return m, tea.Batch(cmds...)

	case agent.ResultEvent:
		line := fmt.Sprintf("✓ Done (%d in / %d out tokens)", msg.InputTokens, msg.OutputTokens)
		m.detail.AppendLine(msg.AgentName, line)
		m.manager.UpdateTokens(msg.AgentName, msg.InputTokens, msg.OutputTokens)
		m.logEvent(msg.AgentName, msg)
		m.refreshSidebar()
		m.refreshStats()
		cmds = append(cmds, m.rearmScheduler())
		return m, tea.Batch(cmds...)

	case agent.AgentDoneEvent:
		m.manager.UpdateDuration(msg.AgentName)
		if msg.ExitCode != 0 {
			m.detail.AppendLine(msg.AgentName, fmt.Sprintf("✗ Exited with code %d", msg.ExitCode))
		}
		m.logEvent(msg.AgentName, msg)
		m.refreshSidebar()
		m.refreshStats()
		cmds = append(cmds, m.rearmScheduler())
		return m, tea.Batch(cmds...)

	case agent.ErrorEvent:
		m.detail.AppendLine(msg.AgentName, fmt.Sprintf("✗ Error: %s", msg.Err))
		m.logEvent(msg.AgentName, msg)
		cmds = append(cmds, m.rearmScheduler())
		return m, tea.Batch(cmds...)

	// Scheduler events
	case dag.AgentStartedMsg:
		info := m.manager.Get(msg.Name)
		if info != nil && info.Process != nil {
			cmds = append(cmds, agent.WaitForEvent(info.Process.EventCh))
		}
		m.refreshSidebar()
		m.refreshStats()
		cmds = append(cmds, m.rearmScheduler())
		return m, tea.Batch(cmds...)

	case dag.AgentBlockedMsg:
		m.detail.AppendLine(msg.Name, fmt.Sprintf("◌ Blocked: %s", msg.Reason))
		m.refreshSidebar()
		m.refreshStats()
		cmds = append(cmds, m.rearmScheduler())
		return m, tea.Batch(cmds...)

	case dag.PipelineDoneMsg:
		status := "completed"
		if !msg.Success {
			status = "completed with failures"
		}
		m.statusbar.SetMode(fmt.Sprintf("Pipeline %s", status))
		m.refreshStats()
		return m, nil

	case SpawnConfirmMsg:
		return m.handleSpawnConfirm(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Spawn dialog captures all input when active
	if m.focus == focusSpawn {
		var cmd tea.Cmd
		m.spawn, cmd = m.spawn.Update(msg)
		return m, cmd
	}

	// Confirm-kill mode
	if m.confirmKill != "" {
		switch msg.String() {
		case "y":
			m.manager.Stop(m.confirmKill)
			m.confirmKill = ""
			m.statusbar.SetMode("normal")
			m.refreshSidebar()
			m.refreshStats()
		case "n", "esc":
			m.confirmKill = ""
			m.statusbar.SetMode("normal")
		}
		return m, nil
	}

	// Global keys
	switch msg.String() {
	case "tab":
		if m.focus == focusSidebar {
			m.focus = focusDetail
			m.sidebar.SetFocused(false)
		} else {
			m.focus = focusSidebar
			m.sidebar.SetFocused(true)
		}
		return m, nil

	case "n":
		m.spawn.SetActive(true)
		m.focus = focusSpawn
		return m, m.spawn.promptInput.Focus()

	case "k":
		name := m.sidebar.SelectedAgent()
		if name != "" {
			info := m.manager.Get(name)
			if info != nil && info.State == agent.StateRunning {
				m.confirmKill = name
				m.statusbar.SetMode("confirm-kill")
			}
		}
		return m, nil

	case "r":
		name := m.sidebar.SelectedAgent()
		if name != "" {
			ch, err := m.manager.Restart(name)
			if err == nil {
				m.refreshSidebar()
				return m, agent.WaitForEvent(ch)
			}
		}
		return m, nil

	case "l":
		m.detail.ToggleLogView()
		return m, nil

	case "p":
		m.detail.ShowPipelineView(m.renderPipelineView())
		return m, nil

	case "q", "ctrl+c":
		if m.logWriter != nil {
			m.logWriter.Close()
		}
		m.cancel()
		m.manager.StopAll()
		return m, tea.Quit
	}

	// Delegate to focused panel
	switch m.focus {
	case focusSidebar:
		var cmd tea.Cmd
		prev := m.sidebar.SelectedAgent()
		m.sidebar, cmd = m.sidebar.Update(msg)
		if curr := m.sidebar.SelectedAgent(); curr != prev {
			m.detail.SetAgent(curr)
			if info := m.manager.Get(curr); info != nil {
				m.detail.SetHeader(info)
			}
		}
		return m, cmd
	case focusDetail:
		// Clear pipeline view on any key
		m.detail.ClearPipelineView()
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *AppModel) handleSpawnConfirm(msg SpawnConfirmMsg) (tea.Model, tea.Cmd) {
	name := fmt.Sprintf("agent-%d", m.nextAgentID)
	m.nextAgentID++

	cfg := config.AgentConfig{
		Prompt: msg.Prompt,
		Model:  msg.Model,
	}
	m.manager.Register(name, cfg)
	ch, err := m.manager.Start(name)

	m.spawn.Reset()
	m.focus = focusSidebar
	m.sidebar.SetFocused(true)
	m.refreshSidebar()
	m.refreshStats()

	if err == nil {
		return m, agent.WaitForEvent(ch)
	}
	return m, nil
}

// View renders the full TUI.
func (m AppModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Header
	header := HeaderStyle.Width(m.width).Render("agentmux")

	// Sidebar
	sidebarStyle := SidebarStyle
	if m.focus == focusSidebar {
		sidebarStyle = SidebarFocusedStyle
	}
	sidebar := sidebarStyle.Height(m.height - 3).Render(m.sidebar.View())

	// Detail
	detailWidth := m.width - SidebarWidth - 2
	if detailWidth < 10 {
		detailWidth = 10
	}
	detail := DetailStyle.Width(detailWidth).Height(m.height - 3).Render(m.detail.View())

	// Body
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, detail)

	// Statusbar
	statusbar := m.statusbar.View()

	// Spawn overlay
	if m.focus == focusSpawn {
		return lipgloss.JoinVertical(lipgloss.Left, header, body, statusbar) + "\n" + m.spawn.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusbar)
}

func (m *AppModel) resizeAll() {
	sidebarH := m.height - 3
	m.sidebar.SetSize(SidebarWidth, sidebarH)

	detailW := m.width - SidebarWidth - 2
	if detailW < 10 {
		detailW = 10
	}
	m.detail.SetSize(detailW, sidebarH)

	m.statusbar.SetWidth(m.width)
	m.spawn.width = m.width
}

func (m *AppModel) refreshSidebar() {
	m.sidebar.Refresh(m.manager.List())
}

func (m *AppModel) refreshStats() {
	agents := m.manager.List()
	stats := PipelineStats{Total: len(agents)}
	for _, a := range agents {
		switch a.State {
		case agent.StateRunning:
			stats.Running++
		case agent.StatePending:
			stats.Pending++
		case agent.StateDone:
			stats.Done++
		case agent.StateFailed:
			stats.Failed++
		case agent.StateBlocked:
			stats.Blocked++
		}
		stats.TotalTokensIn += a.Tokens.Input
		stats.TotalTokensOut += a.Tokens.Output
	}
	m.statusbar.SetStats(stats)
}

func (m *AppModel) rearmScheduler() tea.Cmd {
	if m.scheduler != nil {
		return agent.WaitForEvent(m.scheduler.EventCh())
	}
	return nil
}

func (m *AppModel) logEvent(agentName string, evt interface{}) {
	if m.logWriter != nil {
		m.logWriter.Write(agentName, evt)
	}
}

func (m *AppModel) renderPipelineView() string {
	if m.graph == nil {
		return "No pipeline"
	}

	var b strings.Builder
	b.WriteString("Pipeline Status\n")
	b.WriteString("═══════════════\n\n")

	status := map[string]agent.AgentState{}
	if m.scheduler != nil {
		status = m.scheduler.Status()
	}

	// Render roots and their dependents recursively
	rendered := map[string]bool{}
	roots := m.graph.Roots()
	for _, root := range roots {
		m.renderNode(&b, root, status, rendered, 0)
	}

	return b.String()
}

func (m *AppModel) renderNode(b *strings.Builder, name string, status map[string]agent.AgentState, rendered map[string]bool, depth int) {
	if rendered[name] {
		return
	}
	rendered[name] = true

	indent := strings.Repeat("  ", depth)
	prefix := ""
	if depth > 0 {
		prefix = "└─► "
	}

	state := agent.StatePending
	if s, ok := status[name]; ok {
		state = s
	}
	icon := StyledIcon(state)

	info := m.manager.Get(name)
	dur := ""
	if info != nil && info.Duration > 0 {
		dur = fmt.Sprintf("  (%s)", formatDuration(info.Duration))
	}

	fmt.Fprintf(b, "%s%s%s %s  %s%s\n", indent, prefix, icon, name, state, dur)

	deps := m.graph.Dependents(name)
	for _, dep := range deps {
		m.renderNode(b, dep, status, rendered, depth+1)
	}
}
