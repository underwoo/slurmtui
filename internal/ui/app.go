package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(1, 2)
)

type model struct {
	width    int
	height   int
	showHelp bool
}

func New() tea.Model { return model{} }

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
		}
	}
	return m, nil
}

func (m model) View() string {
	body := "Waiting for Slurm queue data."
	if m.showHelp {
		body = "q / ctrl+c  quit\n?            toggle help"
	}
	content := titleStyle.Render("Slurm TUI") + "\n\n" + body
	if m.width > 0 {
		content = borderStyle.Width(max(1, m.width-6)).Render(content)
	}
	status := mutedStyle.Render("Status: waiting for Slurm backend")
	footer := mutedStyle.Render("q quit  •  ? help")
	return fmt.Sprintf("%s\n%s\n%s", content, status, footer)
}
