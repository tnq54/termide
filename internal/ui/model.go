package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Panel string

const (
	PanelDashboard Panel = "dashboard"
	PanelEditor    Panel = "editor"
	PanelAgents    Panel = "agents"
	PanelTerminal  Panel = "terminal"
	PanelDiff      Panel = "diff"
	PanelApprovals Panel = "approvals"
	PanelTask      Panel = "task"
)

type Model struct {
	Width, Height   int
	Panel           Panel
	Status          string
	TaskDescription string
	Project         string
	VPS             string
	TaskID          string
	remote          remoteCaller
	workspace       workspaceConfig
	ready           bool
	saving          bool
}

func NewModel() Model {
	return Model{Panel: PanelDashboard, Status: "disconnected", Project: "no project", VPS: "no vps"}
}
func (m Model) Init() tea.Cmd { return m.openRemoteWorkspace() }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch value := msg.(type) {
	case remoteResult:
		m.saving = false
		if value.err != nil {
			m.Status = value.err.Error()
			return m, nil
		}
		if value.taskID == "" {
			m.ready = true
			m.Status = "connected"
		} else {
			m.TaskID = value.taskID
			m.Status = "task queued"
			m.Panel = PanelAgents
		}
	case tea.WindowSizeMsg:
		m.Width, m.Height = value.Width, value.Height
	case tea.KeyMsg:
		if value.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.Panel == PanelTask {
			switch value.Type {
			case tea.KeyEsc:
				m.Panel = PanelDashboard
			case tea.KeyEnter:
				if strings.TrimSpace(m.TaskDescription) != "" {
					if m.remote != nil {
						if !m.ready || m.saving {
							return m, nil
						}
						m.saving = true
						m.Status = "saving task"
						return m, m.saveRemoteTask()
					}
					m.Panel = PanelAgents
				}
			case tea.KeyBackspace, tea.KeyCtrlH:
				runes := []rune(m.TaskDescription)
				if len(runes) > 0 {
					m.TaskDescription = string(runes[:len(runes)-1])
				}
			case tea.KeyRunes:
				m.TaskDescription += string(value.Runes)
			case tea.KeySpace:
				m.TaskDescription += " "
			}
			return m, nil
		}
		key := value.String()
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "d":
			m.Panel = PanelDashboard
		case "e":
			m.Panel = PanelEditor
		case "a":
			m.Panel = PanelAgents
		case "t":
			m.Panel = PanelTerminal
		case "f":
			m.Panel = PanelDiff
		case "p":
			m.Panel = PanelApprovals
		case "n":
			m.Panel = PanelTask
			m.TaskDescription = ""
		case "enter":
			if m.Panel == PanelTask && m.TaskDescription != "" {
				m.Panel = PanelAgents
			}
		case "backspace":
			if m.Panel == PanelTask && len(m.TaskDescription) > 0 {
				m.TaskDescription = m.TaskDescription[:len(m.TaskDescription)-1]
			}
		default:
			if m.Panel == PanelTask && value.Type == tea.KeyRunes {
				m.TaskDescription += string(value.Runes)
			}
		}
	}
	return m, nil
}
func (m Model) View() string {
	return fmt.Sprintf("TermIDE\nVPS: %s | Project: %s | Status: %s\nPanel: %s\nTask: %s\n[n]ew task [e]ditor [a]gents [t]erminal [f]iff [p]ermissions [q]uit\n", m.VPS, m.Project, m.Status, m.Panel, m.TaskDescription)
}
