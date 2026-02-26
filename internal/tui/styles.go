package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/hoadh/agentmux/internal/agent"
)

var (
	SidebarMinWidth = 20
	SidebarMaxWidth = 50
	// Padding beyond agent name: prefix (1) + " ● " (3) + max duration " XXmXXs" (8) + right margin (2)
	SidebarPadding = 14

	// Colors
	AccentColor  = lipgloss.Color("62")
	SuccessColor = lipgloss.Color("42")
	ErrorColor   = lipgloss.Color("196")
	WarningColor = lipgloss.Color("214")
	MutedColor   = lipgloss.Color("241")
	PendingColor = lipgloss.Color("248")

	// Panel styles (width set dynamically via .Width() at render time)
	SidebarStyle = lipgloss.NewStyle().
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(MutedColor)

	SidebarFocusedStyle = lipgloss.NewStyle().
				BorderRight(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(AccentColor)

	DetailStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(MutedColor)

	DetailFocusedStyle = lipgloss.NewStyle().
				BorderLeft(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(AccentColor)

	DetailHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Background(AccentColor).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor).
			Padding(0, 1)

	// Status icons mapped by agent state
	StatusIcons = map[agent.AgentState]string{
		agent.StateRunning: "●",
		agent.StatePending: "○",
		agent.StateBlocked: "◌",
		agent.StateDone:    "✓",
		agent.StateFailed:  "✗",
		agent.StateKilled:  "✗",
	}

	// Status colors mapped by agent state
	StatusColors = map[agent.AgentState]lipgloss.Color{
		agent.StateRunning: AccentColor,
		agent.StatePending: PendingColor,
		agent.StateBlocked: WarningColor,
		agent.StateDone:    SuccessColor,
		agent.StateFailed:  ErrorColor,
		agent.StateKilled:  ErrorColor,
	}
)

// StyledIcon returns a colored status icon for the given state.
func StyledIcon(state agent.AgentState) string {
	icon := StatusIcons[state]
	color := StatusColors[state]
	return lipgloss.NewStyle().Foreground(color).Render(icon)
}
