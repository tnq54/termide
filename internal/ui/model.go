package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type Panel string
const (
	PanelDashboard Panel = "dashboard"
	PanelEditor Panel = "editor"
	PanelAgents Panel = "agents"
	PanelTerminal Panel = "terminal"
	PanelDiff Panel = "diff"
	PanelApprovals Panel = "approvals"
)

type Model struct { Width, Height int; Panel Panel; Status string }
func NewModel() Model { return Model{Panel: PanelDashboard, Status: "disconnected"} }
func (m Model) Init() tea.Cmd { return nil }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch value := msg.(type) {
	case tea.WindowSizeMsg: m.Width, m.Height = value.Width, value.Height
	case tea.KeyMsg:
		switch value.String() {
		case "q", "ctrl+c": return m, tea.Quit
		case "d": m.Panel = PanelDashboard
		case "e": m.Panel = PanelEditor
		case "a": m.Panel = PanelAgents
		case "t": m.Panel = PanelTerminal
		case "f": m.Panel = PanelDiff
		case "p": m.Panel = PanelApprovals
		}
	}
	return m, nil
}
func (m Model) View() string { return fmt.Sprintf("TermIDE [%s]  panel: %s\n[e]ditor [a]gents [t]erminal [f]iff [p]ermissions [q]uit\n", m.Status, m.Panel) }
