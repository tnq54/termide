package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"
	"termide/internal/domain"
	"termide/internal/protocol"
)

type dashboardState struct {
	tasks        []domain.Task
	selected     int
	before       int64
	snapshot     *domain.TaskSnapshot
	logs         []string
	busy         bool
	loadingMore  bool
	waiting      bool
	detailOffset int
}

type dashboardResult struct {
	kind     protocol.MessageType
	page     domain.TaskPage
	snapshot domain.TaskSnapshot
	err      error
	before   int64
}

func (m Model) dashboardCommand(kind protocol.MessageType, taskID string, before int64, after uint64) tea.Cmd {
	return func() tea.Msg {
		timeout := 15 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		var payload []byte
		var err error
		switch kind {
		case protocol.MessageListTasks:
			payload, err = json.Marshal(protocol.TaskPageRequest{Before: before})
		case protocol.MessageStartTask:
			if m.dashboard.snapshot == nil || m.dashboard.snapshot.Task.ID != taskID {
				return dashboardResult{kind: kind, err: fmt.Errorf("task detail is not loaded")}
			}
			payload, err = json.Marshal([]map[string]string{{"id": uuid.NewString(), "role": "coder", "command": m.dashboard.snapshot.Task.Description}})
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
		result := dashboardResult{kind: kind, before: before}
		if response.WorkspaceID != m.workspace.ID {
			result.err = fmt.Errorf("dashboard workspace mismatch")
			return result
		}
		if kind == protocol.MessageListTasks {
			if response.Type != protocol.MessageTaskPage {
				result.err = fmt.Errorf("unexpected task page response")
			} else {
				result.err = json.Unmarshal(response.Payload, &result.page)
				if result.err == nil {
					seen := map[string]bool{}
					for _, task := range result.page.Tasks {
						if task.ID == "" || seen[task.ID] || task.WorkspaceID != m.workspace.ID {
							result.err = fmt.Errorf("invalid task page identity")
							break
						}
						seen[task.ID] = true
					}
					if result.page.Next < 0 || (before > 0 && result.page.Next >= before) || (result.page.Next != 0 && len(result.page.Tasks) == 0) {
						result.err = fmt.Errorf("invalid task page cursor")
					}
				}
			}
		} else if kind == protocol.MessageCancel {
			if response.Type != protocol.MessageTaskCancelRequested || response.TaskID != taskID {
				result.err = fmt.Errorf("unexpected task cancel response")
			}
		} else if kind == protocol.MessageApprove {
			if response.Type != protocol.MessageEvent || response.TaskID != taskID || response.Event == nil || response.Event.TaskID != taskID || response.Event.WorkspaceID != m.workspace.ID || response.Event.Type != domain.EventApproval {
				result.err = fmt.Errorf("unexpected task approve response")
			} else {
				result.err = response.Event.Validate()
			}
		} else if kind == protocol.MessageStartTask {
			if response.Type != protocol.MessageEvent || response.TaskID != taskID || response.Event == nil || response.Event.TaskID != taskID || response.Event.WorkspaceID != m.workspace.ID || response.Event.Type != domain.EventTaskUpdated {
				result.err = fmt.Errorf("unexpected task start response")
			} else {
				result.err = response.Event.Validate()
			}
		} else {
			if response.Type != protocol.MessageTaskDetail || response.TaskID != taskID {
				result.err = fmt.Errorf("unexpected task detail response")
			} else {
				result.err = json.Unmarshal(response.Payload, &result.snapshot)
				if result.err == nil {
					result.err = validateDashboardSnapshot(result.snapshot, m.workspace.ID, taskID, after)
				}
			}
		}
		return result
	}
}

func (m Model) dashboardApproveCommand(approved bool) tea.Cmd {
	return func() tea.Msg {
		if m.dashboard.snapshot == nil || len(m.dashboard.snapshot.Approvals) == 0 {
			return dashboardResult{kind: protocol.MessageApprove, err: fmt.Errorf("no pending approval")}
		}
		approval := m.dashboard.snapshot.Approvals[0]
		payload, err := json.Marshal(protocol.ApproveDecision{ID: approval.ID, Approved: approved})
		if err != nil {
			return dashboardResult{kind: protocol.MessageApprove, err: err}
		}
		timeout := 15 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		response, err := m.remote.Call(ctx, protocol.Message{
			Type: protocol.MessageApprove, WorkspaceID: m.workspace.ID,
			TaskID: m.dashboard.snapshot.Task.ID, Payload: payload,
		})
		if err != nil {
			return dashboardResult{kind: protocol.MessageApprove, err: err}
		}
		result := dashboardResult{kind: protocol.MessageApprove}
		if response.WorkspaceID != m.workspace.ID {
			result.err = fmt.Errorf("dashboard workspace mismatch")
			return result
		}
		if response.Type != protocol.MessageEvent || response.TaskID != m.dashboard.snapshot.Task.ID || response.Event == nil || response.Event.TaskID != m.dashboard.snapshot.Task.ID || response.Event.WorkspaceID != m.workspace.ID || response.Event.Type != domain.EventApproval {
			result.err = fmt.Errorf("unexpected task approve response")
		} else {
			result.err = response.Event.Validate()
		}
		return result
	}
}

func validateDashboardSnapshot(s domain.TaskSnapshot, workspaceID, taskID string, after uint64) error {
	if s.Task.ID != taskID || s.Task.WorkspaceID != workspaceID {
		return fmt.Errorf("snapshot task identity mismatch")
	}
	for _, a := range s.Agents {
		if a.ID == "" || a.TaskID != taskID {
			return fmt.Errorf("snapshot agent identity mismatch")
		}
	}
	for _, approval := range s.Approvals {
		if approval.ID == "" || approval.TaskID != taskID || approval.Status != domain.ApprovalPending {
			return fmt.Errorf("snapshot approval identity mismatch")
		}
		if err := approval.Validate(); err != nil {
			return err
		}
	}
	sequence := after
	for _, e := range s.Events {
		if e.WorkspaceID != workspaceID || e.TaskID != taskID || e.Sequence <= sequence {
			return fmt.Errorf("snapshot event identity or sequence mismatch")
		}
		if err := e.Validate(); err != nil {
			return err
		}
		sequence = e.Sequence
	}
	if s.NextSequence != sequence || (s.More && len(s.Events) == 0) {
		return fmt.Errorf("snapshot cursor does not match events")
	}
	return nil
}

func (m Model) receiveDashboard(result dashboardResult) (tea.Model, tea.Cmd) {
	m.dashboard.busy = false
	m.dashboard.loadingMore = false
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	switch result.kind {
	case protocol.MessageStartTask, protocol.MessageCancel, protocol.MessageApprove:
		m.Status = "loading task result"
		if result.kind == protocol.MessageCancel {
			m.Status = "cancellation requested; loading task status"
		}
		if result.kind == protocol.MessageApprove {
			m.Status = "decision sent; loading task status"
		}
		if m.dashboard.snapshot != nil {
			m.dashboard.busy = true
			return m, m.dashboardCommand(protocol.MessageTaskSnapshot, m.dashboard.snapshot.Task.ID, 0, m.dashboard.snapshot.NextSequence)
		}
	case protocol.MessageListTasks:
		if result.before == 0 {
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
		for i := range m.dashboard.tasks {
			if m.dashboard.tasks[i].ID == result.snapshot.Task.ID {
				m.dashboard.tasks[i] = result.snapshot.Task
			}
		}
		m.dashboard.logs = append(m.dashboard.logs, snapshotLogLines(result.snapshot.Events)...)
		m.Status = "task detail loaded"
		if cmd := m.waitCommand(); cmd != nil {
			m.dashboard.waiting = true
			return m, cmd
		}
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
			logs = append(logs, fmt.Sprintf("[%s] %s", safeLabel(event.AgentID), text))
		}
	}
	return logs
}

func (m Model) dashboardKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	needsRemote := key.String() == "r" || key.String() == "l" || key.String() == "s" || key.String() == "c" || key.Type == tea.KeyEnter
	if needsRemote && (m.remote == nil || !m.ready) {
		m.Status = "remote workspace is not connected"
		return m, nil
	}
	if m.dashboard.busy {
		return m, nil
	}
	if m.dashboard.snapshot != nil {
		switch key.Type {
		case tea.KeyUp:
			m.dashboard.detailOffset = max(0, m.dashboard.detailOffset-1)
		case tea.KeyDown:
			m.dashboard.detailOffset = min(m.detailMaxOffset(), m.dashboard.detailOffset+1)
		case tea.KeyPgUp:
			m.dashboard.detailOffset = max(0, m.dashboard.detailOffset-m.dashboardRows())
		case tea.KeyPgDown:
			m.dashboard.detailOffset = min(m.detailMaxOffset(), m.dashboard.detailOffset+m.dashboardRows())
		case tea.KeyHome:
			m.dashboard.detailOffset = 0
		case tea.KeyEnd:
			m.dashboard.detailOffset = m.detailMaxOffset()
		case tea.KeyEsc, tea.KeyBackspace:
			m.dashboard.snapshot = nil
			m.dashboard.logs = nil
			m.Panel = PanelDashboard
			m.Status = "task history loaded"
			return m, nil
		case tea.KeyRunes:
			if string(key.Runes) == "a" && len(m.dashboard.snapshot.Approvals) > 0 {
				m.dashboard.busy = true
				m.Status = "approving command"
				return m, m.dashboardApproveCommand(true)
			}
			if string(key.Runes) == "d" && len(m.dashboard.snapshot.Approvals) > 0 {
				m.dashboard.busy = true
				m.Status = "denying command"
				return m, m.dashboardApproveCommand(false)
			}
			if string(key.Runes) == "c" && cancellableTask(m.dashboard.snapshot.Task.Status) {
				m.dashboard.busy = true
				m.Status = "requesting task cancellation"
				return m, m.dashboardCommand(protocol.MessageCancel, m.dashboard.snapshot.Task.ID, 0, 0)
			}
			if string(key.Runes) == "s" && m.dashboard.snapshot.Task.Status == domain.TaskQueued {
				m.dashboard.busy = true
				m.Status = "starting task"
				if m.remote == nil || !m.ready {
					m.Status = "remote workspace is not connected"
					return m, nil
				}
				return m, m.dashboardCommand(protocol.MessageStartTask, m.dashboard.snapshot.Task.ID, 0, 0)
			}
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
		m.dashboard.detailOffset = 0
		m.Status = "loading task detail"
		return m, m.dashboardCommand(protocol.MessageTaskSnapshot, task.ID, 0, 0)
	case tea.KeyRunes:
		switch string(key.Runes) {
		case "r":
			m.dashboard.busy = true
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
	var body []string
	help := "Enter details | r refresh | l more"
	if m.dashboard.snapshot != nil {
		s := m.dashboard.snapshot
		body = append(body, "Task: "+safeLabel(s.Task.Description), "Status: "+safeLabel(string(s.Task.Status)), "Agents:")
		for _, agent := range s.Agents {
			body = append(body, "- "+safeLabel(agent.Role)+" / "+safeLabel(string(agent.Status)))
		}
		if len(s.Approvals) > 0 {
			body = append(body, "Approvals:")
			for _, approval := range s.Approvals {
				body = append(body, "! "+safeLabel(approval.ID)+" "+safeLabel(approval.AgentID)+": "+safeLabel(approval.Command))
			}
		}
		body = append(body, "Logs:")
		body = append(body, m.dashboard.logs...)
		start := min(m.dashboard.detailOffset, max(0, len(body)-m.dashboardRows()))
		body = body[start:min(len(body), start+m.dashboardRows())]
		help = "c cancel | r refresh | Esc back"
		if len(s.Approvals) > 0 {
			help = "a approve | d deny | c cancel | r refresh | Esc back"
		}
	} else {
		if len(m.dashboard.tasks) == 0 {
			body = append(body, "(no tasks)")
		}
		start := max(0, m.dashboard.selected-m.dashboardRows()+1)
		for i := start; i < len(m.dashboard.tasks) && i < start+m.dashboardRows(); i++ {
			task := m.dashboard.tasks[i]
			prefix := "  "
			if i == m.dashboard.selected {
				prefix = "> "
			}
			body = append(body, prefix+safeLabel(task.Description)+" ["+safeLabel(string(task.Status))+"]")
		}
	}
	if height == 1 {
		lines = []string{safeLabel(m.Status)}
	} else if height == 2 {
		lines = append(lines, safeLabel(m.Status))
	} else {
		lines = append(lines, body[:min(len(body), m.dashboardRows())]...)
		lines = append(lines, help, safeLabel(m.Status))
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
	return ansi.Truncate(safeLabel(value), width, "")
}

func (m Model) dashboardRows() int {
	h := m.Height
	if h < 1 {
		h = 18
	}
	return max(0, h-3)
}

func cancellableTask(status domain.TaskStatus) bool {
	return status == domain.TaskRunning || status == domain.TaskPlanning || status == domain.TaskWaitingApproval || status == domain.TaskBlocked
}
func (m Model) detailMaxOffset() int {
	if m.dashboard.snapshot == nil {
		return 0
	}
	extra := 0
	if len(m.dashboard.snapshot.Approvals) > 0 {
		extra = 1 + len(m.dashboard.snapshot.Approvals)
	}
	return max(0, 4+len(m.dashboard.snapshot.Agents)+extra+len(m.dashboard.logs)-m.dashboardRows())
}
