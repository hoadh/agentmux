package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SpawnConfirmMsg is sent when the user confirms spawning a new agent.
type SpawnConfirmMsg struct {
	Prompt string
	Model  string
}

// SpawnCancelMsg is sent when the user cancels the spawn dialog.
type SpawnCancelMsg struct{}

// SpawnModel manages the spawn dialog overlay.
type SpawnModel struct {
	promptInput textinput.Model
	modelInput  textinput.Model
	focusIndex  int
	active      bool
	width       int
}

// NewSpawnDialog creates a new spawn dialog.
func NewSpawnDialog() SpawnModel {
	pi := textinput.New()
	pi.Placeholder = "Enter prompt for the agent..."
	pi.CharLimit = 500
	pi.Width = 50

	mi := textinput.New()
	mi.Placeholder = "Model (default: sonnet)"
	mi.CharLimit = 30
	mi.Width = 50

	return SpawnModel{
		promptInput: pi,
		modelInput:  mi,
	}
}

// Update handles input for the spawn dialog.
func (s SpawnModel) Update(msg tea.Msg) (SpawnModel, tea.Cmd) {
	if !s.active {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			prompt := s.promptInput.Value()
			if prompt == "" {
				return s, nil
			}
			model := s.modelInput.Value()
			return s, func() tea.Msg {
				return SpawnConfirmMsg{Prompt: prompt, Model: model}
			}
		case "esc":
			s.active = false
			return s, func() tea.Msg { return SpawnCancelMsg{} }
		case "tab":
			if s.focusIndex == 0 {
				s.focusIndex = 1
				s.promptInput.Blur()
				return s, s.modelInput.Focus()
			}
			s.focusIndex = 0
			s.modelInput.Blur()
			return s, s.promptInput.Focus()
		}
	}

	var cmd tea.Cmd
	if s.focusIndex == 0 {
		s.promptInput, cmd = s.promptInput.Update(msg)
	} else {
		s.modelInput, cmd = s.modelInput.Update(msg)
	}
	return s, cmd
}

// View renders the spawn dialog overlay.
func (s SpawnModel) View() string {
	if !s.active {
		return ""
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render("Spawn New Agent")

	content := title + "\n\n" +
		"Prompt:\n" + s.promptInput.View() + "\n\n" +
		"Model:\n" + s.modelInput.View() + "\n\n" +
		lipgloss.NewStyle().Foreground(MutedColor).Render("Enter:confirm  Tab:next field  Esc:cancel")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentColor).
		Padding(1, 2).
		Width(60).
		Render(content)

	return box
}

// SetActive shows or hides the dialog.
func (s *SpawnModel) SetActive(active bool) {
	s.active = active
}

// Reset clears the dialog fields.
func (s *SpawnModel) Reset() {
	s.promptInput.SetValue("")
	s.modelInput.SetValue("")
	s.focusIndex = 0
	s.active = false
	s.promptInput.Blur()
	s.modelInput.Blur()
}
