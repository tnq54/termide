package ui

import (
	"fmt"
	"strings"
	"termide/internal/protocol"

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
	files           fileEditor
	dashboard       dashboardState
	diff            diffState
	termInput       string
	termOutput      []string
	termScroll      int
	termBusy        bool
}

func NewModel() Model {
	return Model{Panel: PanelDashboard, Status: "disconnected", Project: "no project", VPS: "no vps", files: newFileEditor()}
}
func (m Model) Init() tea.Cmd { return m.openRemoteWorkspace() }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch value := msg.(type) {
	case fileResult:
		return m.receiveFile(value)
	case dashboardResult:
		return m.receiveDashboard(value)
	case dashboardWaitResult:
		return m.receiveDashboardWait(value)
	case terminalResult:
		return m.receiveTerminal(value)
	case diffResult:
		return m.receiveDiff(value)
	case dashboardReplayResult:
		return m.receiveDashboardReplay(value)
	case remoteResult:
		m.saving = false
		if value.err != nil {
			m.Status = value.err.Error()
			return m, nil
		}
		if value.taskID == "" {
			m.ready = true
			m.Status = "connected"
			if m.remote != nil {
				if m.dashboard.snapshot != nil {
					m.dashboard.busy = true
					m.Status = "replaying task history"
					return m, m.dashboardReplayCommand()
				}
				return m, m.dashboardCommand(protocol.MessageListTasks, "", 0, 0)
			}
		} else {
			m.TaskID = value.taskID
			m.Status = "task queued"
			m.Panel = PanelAgents
		}
	case tea.WindowSizeMsg:
		m.Width, m.Height = value.Width, value.Height
		m.files.width = max(1, value.Width)
		m.files.height = max(1, value.Height-4)
		m.files.input.SetWidth(m.files.width)
		m.files.input.SetHeight(m.files.height)
		for i := range m.files.tabs {
			m.files.tabs[i].input.SetWidth(m.files.width)
			m.files.tabs[i].input.SetHeight(m.files.height)
		}
	case tea.KeyMsg:
		if value.Type == tea.KeyCtrlC {
			if m.files.busy && m.files.document != nil {
				m.Status = "Save in progress; wait for server confirmation"
				return m, nil
			}
			if m.files.anyDirty() {
				m.Panel = PanelEditor
				m.Status = "Unsaved changes: Ctrl+S save; Ctrl+G discard"
				return m, nil
			}
			return m, tea.Quit
		}
		if m.Panel == PanelEditor {
			return m.editorKey(value)
		}
		if m.Panel == PanelTerminal {
			return m.terminalKey(value)
		}
		if m.Panel == PanelDiff {
			return m.diffKey(value)
		}
		if m.Panel == PanelAgents && m.dashboard.snapshot != nil {
			return m.dashboardKey(value)
		}
		if m.Panel == PanelDashboard {
			return m.dashboardKey(value)
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
			if m.remote != nil && m.ready && m.files.document == nil && !m.files.busy {
				m.files.busy = true
				return m, m.fileCommand(protocol.MessageListFiles, protocol.FileRequest{Path: m.files.folder})
			}
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
	default:
		if m.Panel == PanelEditor && m.files.document != nil {
			var cmd tea.Cmd
			m.files.input, cmd = m.files.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}
func (m Model) View() string {
	if m.Panel == PanelEditor {
		return m.editorView()
	}
	if m.Panel == PanelTerminal {
		return m.terminalView()
	}
	if m.Panel == PanelDiff {
		return m.diffView()
	}
	if m.Panel == PanelDashboard || (m.Panel == PanelAgents && m.dashboard.snapshot != nil) {
		return m.dashboardView()
	}
	return fmt.Sprintf("TermIDE\nVPS: %s | Project: %s | Status: %s\nPanel: %s\nTask: %s\n[n]ew task [e]ditor [a]gents [t]erminal [f]iff [p]ermissions [q]uit\n", m.VPS, m.Project, m.Status, m.Panel, m.TaskDescription)
}
