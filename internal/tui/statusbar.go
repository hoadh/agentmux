package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// PipelineStats holds aggregate pipeline statistics.
type PipelineStats struct {
	Running        int
	Pending        int
	Done           int
	Failed         int
	Blocked        int
	Total          int
	TotalTokensIn  int
	TotalTokensOut int
}

// StatusBarModel manages the bottom status bar.
type StatusBarModel struct {
	width      int
	stats      PipelineStats
	mode       string
	focusPanel string // "sidebar", "detail", or "spawn"
}

// NewStatusBar creates a new status bar.
func NewStatusBar() StatusBarModel {
	return StatusBarModel{mode: "normal"}
}

// View renders the status bar.
func (s StatusBarModel) View() string {
	var left, right string

	switch s.mode {
	case "confirm-kill":
		left = " y:confirm  n:cancel  Kill agent?"
	case "spawn":
		left = " Enter:confirm  Esc:cancel  Spawning new agent..."
	default:
		focusHint := lipgloss.NewStyle().Bold(true).Render("[" + s.focusPanel + "]")
		left = " " + focusHint + "  [n]ew [K]ill [r]estart [l]ogs [p]ipe  tab:focus  q:quit"
	}

	if s.stats.Total > 0 {
		right = fmt.Sprintf(" %d/%d done ", s.stats.Done, s.stats.Total)
		if s.stats.Running > 0 {
			right = fmt.Sprintf(" %d running  %s", s.stats.Running, right)
		}
		if s.stats.Failed > 0 {
			right = fmt.Sprintf(" %d failed  %s", s.stats.Failed, right)
		}
	}

	// Fill remaining width with spaces
	gap := s.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	fill := ""
	for i := 0; i < gap; i++ {
		fill += " "
	}

	return StatusBarStyle.Width(s.width).Render(left + fill + right)
}

// SetWidth updates the status bar width.
func (s *StatusBarModel) SetWidth(w int) {
	s.width = w
}

// SetStats updates the pipeline statistics.
func (s *StatusBarModel) SetStats(stats PipelineStats) {
	s.stats = stats
}

// SetMode switches the display mode.
func (s *StatusBarModel) SetMode(mode string) {
	s.mode = mode
}

// SetFocusPanel updates the active panel indicator.
func (s *StatusBarModel) SetFocusPanel(panel string) {
	s.focusPanel = panel
}
