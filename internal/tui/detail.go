package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hoadh/agentmux/internal/agent"
)

const maxLines = 10000

// DetailModel manages the agent output detail panel.
type DetailModel struct {
	viewport   viewport.Model
	agentName  string
	lines      map[string][]string // per-agent output buffers
	autoScroll bool
	width      int
	height     int
	ready      bool
	showRaw    bool
	pipeView   string // non-empty = show pipeline view
}

// NewDetail creates a new detail panel.
func NewDetail() DetailModel {
	return DetailModel{
		lines:      make(map[string][]string),
		autoScroll: true,
	}
}

// Update handles input for the detail panel.
func (d DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "G", "end":
			d.autoScroll = true
			d.viewport.GotoBottom()
			return d, nil
		}

		// If user scrolls up, disable auto-scroll
		prevOffset := d.viewport.YOffset
		d.viewport, cmd = d.viewport.Update(msg)
		if d.viewport.YOffset < prevOffset {
			d.autoScroll = false
		}
		return d, cmd
	}

	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

// View renders the detail panel.
func (d DetailModel) View() string {
	if d.pipeView != "" {
		return d.pipeView
	}

	header := d.renderHeader()
	sep := lipgloss.NewStyle().Foreground(MutedColor).Render(strings.Repeat("─", d.width))

	return fmt.Sprintf("%s\n%s\n%s", header, sep, d.viewport.View())
}

func (d DetailModel) renderHeader() string {
	if d.agentName == "" {
		return DetailHeaderStyle.Render("No agent selected")
	}
	return DetailHeaderStyle.Render(d.agentName)
}

// SetSize updates the detail panel dimensions.
func (d *DetailModel) SetSize(w, h int) {
	d.width = w
	headerHeight := 3 // header + separator + padding
	vpHeight := h - headerHeight
	if vpHeight < 1 {
		vpHeight = 1
	}

	if !d.ready {
		d.viewport = viewport.New(w, vpHeight)
		d.ready = true
	} else {
		d.viewport.Width = w
		d.viewport.Height = vpHeight
	}
}

// SetAgent switches to showing output for the given agent.
func (d *DetailModel) SetAgent(name string) {
	d.agentName = name
	d.pipeView = ""
	d.refreshContent()
}

// AppendLine adds a line of output for a specific agent.
func (d *DetailModel) AppendLine(agentName, line string) {
	lines := d.lines[agentName]
	lines = append(lines, line)

	// Ring buffer: trim oldest 20% if over limit
	if len(lines) > maxLines {
		trim := maxLines / 5
		lines = lines[trim:]
	}
	d.lines[agentName] = lines

	// Update viewport if showing this agent
	if agentName == d.agentName {
		d.refreshContent()
	}
}

func (d *DetailModel) refreshContent() {
	lines := d.lines[d.agentName]
	content := strings.Join(lines, "\n")
	d.viewport.SetContent(content)

	if d.autoScroll {
		d.viewport.GotoBottom()
	}
}

// SetHeader updates the header with agent info.
func (d *DetailModel) SetHeader(info *agent.AgentInfo) {
	if info == nil {
		return
	}

	icon := StyledIcon(info.State)
	dur := ""
	if info.Duration > 0 {
		dur = formatDuration(info.Duration)
	}

	tokens := ""
	if info.Tokens.Input > 0 || info.Tokens.Output > 0 {
		tokens = fmt.Sprintf("  |  Tokens: %s in / %s out",
			formatTokens(info.Tokens.Input), formatTokens(info.Tokens.Output))
	}

	model := ""
	if info.Config.Model != "" {
		model = fmt.Sprintf("  |  Model: %s", info.Config.Model)
	}

	d.agentName = fmt.Sprintf("%s %s %s  %s%s%s",
		info.Name, icon, info.State, dur, model, tokens)
}

// ToggleLogView switches between formatted and raw output.
func (d *DetailModel) ToggleLogView() {
	d.showRaw = !d.showRaw
}

// ShowPipelineView sets the pipeline view content.
func (d *DetailModel) ShowPipelineView(content string) {
	d.pipeView = content
}

// ClearPipelineView returns to normal agent view.
func (d *DetailModel) ClearPipelineView() {
	d.pipeView = ""
}

func formatDuration(d interface{ Seconds() float64 }) string {
	secs := d.Seconds()
	if secs < 60 {
		return fmt.Sprintf("%.0fs", secs)
	}
	mins := int(secs) / 60
	remaining := int(secs) % 60
	return fmt.Sprintf("%dm%02ds", mins, remaining)
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
