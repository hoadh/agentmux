package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hoadh/agentmux/internal/agent"
)

// SidebarModel manages the agent list sidebar.
type SidebarModel struct {
	agents   []*agent.AgentInfo
	cursor   int
	width    int
	height   int
	focused  bool
}

// NewSidebar creates a sidebar with the given agents.
func NewSidebar(agents []*agent.AgentInfo) SidebarModel {
	return SidebarModel{
		agents: agents,
		cursor: 0,
	}
}

// Update handles input for the sidebar.
func (s SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	if !s.focused {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if s.cursor < len(s.agents)-1 {
				s.cursor++
			}
		case "k", "up":
			if s.cursor > 0 {
				s.cursor--
			}
		case "g":
			s.cursor = 0
		case "G":
			if len(s.agents) > 0 {
				s.cursor = len(s.agents) - 1
			}
		}
	}

	return s, nil
}

// View renders the sidebar.
func (s SidebarModel) View() string {
	if len(s.agents) == 0 {
		return lipgloss.NewStyle().
			Foreground(MutedColor).
			Padding(1, 1).
			Render("No agents")
	}

	var b strings.Builder
	availHeight := s.height - 2 // padding

	for i, info := range s.agents {
		if i >= availHeight {
			break
		}

		icon := StyledIcon(info.State)
		name := info.Name

		// Duration for running agents
		dur := ""
		if info.State == agent.StateRunning && info.Duration > 0 {
			dur = fmt.Sprintf(" %s", formatDuration(info.Duration))
		} else if info.State == agent.StateDone && info.Duration > 0 {
			dur = fmt.Sprintf(" %s", formatDuration(info.Duration))
		}

		// Arrow prefix for focused agent, space for others
		prefix := " "
		if i == s.cursor {
			prefix = "▸"
		}

		// Right-align duration: "▸ ● name       1m23s"
		usableW := s.width - 5 // prefix (1) + space (1) + icon (1) + space (1) + right margin (1)
		nameRunes := []rune(name)
		durRunes := []rune(dur)
		nameW := len(nameRunes)
		durW := len(durRunes)

		if usableW > 0 && nameW+durW > usableW {
			maxName := usableW - durW
			if maxName < 1 {
				maxName = 1
			}
			nameRunes = nameRunes[:maxName]
			nameW = maxName
		}

		gap := usableW - nameW - durW
		if gap < 0 {
			gap = 0
		}

		line := fmt.Sprintf("%s %s %s%s%s", prefix, icon, string(nameRunes), strings.Repeat(" ", gap), dur)

		if i == s.cursor {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}

		b.WriteString(line)
		if i < len(s.agents)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// SelectedAgent returns the name of the currently selected agent.
func (s SidebarModel) SelectedAgent() string {
	if s.cursor >= 0 && s.cursor < len(s.agents) {
		return s.agents[s.cursor].Name
	}
	return ""
}

// SetSize updates the sidebar dimensions.
func (s *SidebarModel) SetSize(w, h int) {
	s.width = w
	s.height = h
}

// SetFocused sets the focus state.
func (s *SidebarModel) SetFocused(focused bool) {
	s.focused = focused
}

// Refresh updates the agent list.
func (s *SidebarModel) Refresh(agents []*agent.AgentInfo) {
	s.agents = agents
	if s.cursor >= len(agents) && len(agents) > 0 {
		s.cursor = len(agents) - 1
	}
}
