package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const banner = `
     ╱|、
    (˚ˎ。7       g h - z e n
    |、˜\
    じしˍ,)ノ
`

type model struct {
	width  int
	height int
}

func New() tea.Model {
	return model{}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).Render(banner)
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  press q to quit")
	return title + "\n" + hint + "\n"
}
