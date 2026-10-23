package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

type dashboardState struct {
	tasks       []domain.Task
	selected    int
	before      int64
	snapshot    *domain.TaskSnapshot
	logs        []string
	busy        bool
	loadingMore bool
}

type dashboardResult struct {
	kind     protocol.MessageType
	page     domain.TaskPage
	snapshot domain.TaskSnapshot
	err      error
}

func (m Model) dashboardCommand(kind protocol.MessageType, taskID string, before int64, after uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var payload []byte
		var err error
		switch kind {
		case protocol.MessageListTasks:
			payload, err = json.Marshal(protocol.TaskPageRequest{Before: before})
		}
		if err != nil {
			return dashboardResult{kind: kind, err: err}
		}
		response, err := m.remote.Call(ctx, protocol.Message{
			Type: kind, WorkspaceID: m.workspace.ID, TaskID: taskID,
			AfterSequence: after, Payload: payload,
		})
		if err != nil {
			return dashboardResult{kind: kind, err: err}
		}
		result := dashboardResult{kind: kind}
		if kind == protocol.MessageListTasks {
			if response.Type != protocol.MessageTaskPage {
				result.err = fmt.Errorf("unexpected task page response")
			} else {
				result.err = json.Unmarshal(response.Payload, &result.page)
			}
		} else {
			if response.Type != protocol.MessageTaskDetail || response.TaskID != taskID {
				result.err = fmt.Errorf("unexpected task detail response")
			} else {
				result.err = json.Unmarshal(response.Payload, &result.snapshot)
			}
		}
		return result
	}
}

func (m Model) receiveDashboard(result dashboardResult) (tea.Model, tea.Cmd) {
	m.dashboard.busy = false
	m.dashboard.loadingMore = false
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	switch result.kind {
	case protocol.MessageListTasks:
		if m.dashboard.before == 0 {
			m.dashboard.tasks = append([]domain.Task(nil), result.page.Tasks...)
		} else {
			m.dashboard.tasks = append(m.dashboard.tasks, result.page.Tasks...)
		}
		m.dashboard.before = result.page.Next
		if len(m.dashboard.tasks) == 0 {
			m.dashboard.selected = 0
		} else if m.dashboard.selected >= len(m.dashboard.tasks) {
			m.dashboard.selected = len(m.dashboard.tasks) - 1
		}
		m.Status = "task history loaded"
	case protocol.MessageTaskSnapshot:
		m.dashboard.snapshot = &result.snapshot
		m.dashboard.logs = append(m.dashboard.logs, snapshotLogLines(result.snapshot.Events)...)
		m.Status = "task detail loaded"
	}
	return m, nil
}

func snapshotLogLines(events []domain.EventEnvelope) []string {
	logs := make([]string, 0, len(events))
	for _, event := range events {
		text := string(event.Payload)
		var output struct {
			Output string `json:"Output"`
		}
		if json.Unmarshal(event.Payload, &output) == nil && output.Output != "" {
			text = output.Output
		}
		text = safeLabel(text)
		text = strings.TrimSpace(text)
		if text != "" {
			logs = append(logs, fmt.Sprintf("[%s] %s", event.AgentID, text))
		}
	}
	return logs
}

func (m Model) dashboardKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.dashboard.busy {
		return m, nil
	}
	if m.dashboard.snapshot != nil {
		switch key.Type {
		case tea.KeyEsc, tea.KeyBackspace:
			m.dashboard.snapshot = nil
			m.dashboard.logs = nil
			m.Panel = PanelDashboard
			m.Status = "task history loaded"
			return m, nil
		case tea.KeyRunes:
			if string(key.Runes) == "r" {
				m.dashboard.busy = true
				m.Status = "refreshing task detail"
				return m, m.dashboardCommand(protocol.MessageTaskSnapshot, m.dashboard.snapshot.Task.ID, 0, m.dashboard.snapshot.NextSequence)
			}
		}
		return m, nil
	}
	switch key.Type {
	case tea.KeyUp:
		if m.dashboard.selected > 0 {
			m.dashboard.selected--
		}
	case tea.KeyDown:
		if m.dashboard.selected+1 < len(m.dashboard.tasks) {
			m.dashboard.selected++
		}
	case tea.KeyEnter:
		if len(m.dashboard.tasks) == 0 {
			return m, nil
		}
		task := m.dashboard.tasks[m.dashboard.selected]
		m.dashboard.busy = true
		m.dashboard.logs = nil
		m.Panel = PanelAgents
		m.Status = "loading task detail"
		return m, m.dashboardCommand(protocol.MessageTaskSnapshot, task.ID, 0, 0)
	case tea.KeyRunes:
		switch string(key.Runes) {
		case "r":
			m.dashboard.busy = true
			m.dashboard.before = 0
			m.Status = "refreshing task history"
			return m, m.dashboardCommand(protocol.MessageListTasks, "", 0, 0)
		case "l":
			if m.dashboard.before != 0 {
				m.dashboard.busy = true
				m.dashboard.loadingMore = true
				return m, m.dashboardCommand(protocol.MessageListTasks, "", m.dashboard.before, 0)
			}
		case "e":
			m.Panel = PanelEditor
			if m.remote != nil && m.ready && m.files.document == nil && !m.files.busy {
				m.files.busy = true
				return m, m.fileCommand(protocol.MessageListFiles, protocol.FileRequest{Path: m.files.folder})
			}
		case "n":
			m.Panel = PanelTask
			m.TaskDescription = ""
		case "a":
			m.Panel = PanelAgents
		}
	case tea.KeyEsc:
		m.Panel = PanelDashboard
	}
	return m, nil
}

func (m Model) dashboardView() string {
	width, height := m.Width, m.Height
	if width < 1 {
		width = 40
	}
	if height < 1 {
		height = 18
	}
	lines := []string{"Task dashboard"}
	if m.dashboard.snapshot != nil {
		s := m.dashboard.snapshot
		lines = append(lines, "Task: "+safeLabel(s.Task.Description), "Status: "+safeLabel(string(s.Task.Status)), "Agents:")
		for _, agent := range s.Agents {
			lines = append(lines, "- "+safeLabel(agent.Role)+" / "+safeLabel(string(agent.Status)))
		}
		lines = append(lines, "Logs:")
		lines = append(lines, m.dashboard.logs...)
		lines = append(lines, "r refresh | Esc back")
	} else {
		if len(m.dashboard.tasks) == 0 {
			lines = append(lines, "(no tasks)")
		}
		for i, task := range m.dashboard.tasks {
			prefix := "  "
			if i == m.dashboard.selected {
				prefix = "> "
			}
			lines = append(lines, prefix+safeLabel(task.Description)+" ["+safeLabel(string(task.Status))+"]")
		}
		lines = append(lines, "↑/↓ select | Enter details | r refresh | l more | e editor")
	}
	lines = append(lines, safeLabel(m.Status))
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = truncateSafe(line, width)
	}
	return strings.Join(lines, "\n")
}

func truncateSafe(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return string([]rune(value)[:min(len([]rune(value)), width)])
}

func formatCursor(value int64) string { return strconv.FormatInt(value, 10) }
