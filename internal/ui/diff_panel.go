package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/protocol"
)

type diffState struct {
	taskID string
	lines  []string
	scroll int
	busy   bool
}

type diffResult struct {
	taskID string
	diffs  []protocol.TaskDiff
	err    error
}

func (m Model) diffCommand(taskID string) tea.Cmd {
	return func() tea.Msg {
		timeout := 15 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		response, err := m.remote.Call(ctx, protocol.Message{Type: protocol.MessageTaskDiff, WorkspaceID: m.workspace.ID, TaskID: taskID})
		if err != nil {
			return diffResult{taskID: taskID, err: err}
		}
		result := diffResult{taskID: taskID}
		if response.Type != protocol.MessageTaskDiffs || response.WorkspaceID != m.workspace.ID || response.TaskID != taskID {
			result.err = fmt.Errorf("unexpected task diff response")
			return result
		}
		result.err = json.Unmarshal(response.Payload, &result.diffs)
		return result
	}
}

func (m Model) receiveDiff(result diffResult) (tea.Model, tea.Cmd) {
	m.diff.busy = false
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	var lines []string
	for _, diff := range result.diffs {
		header := "agent " + safeLabel(diff.AgentID)
		if diff.Branch != "" {
			header += " " + safeLabel(diff.Branch)
		}
		lines = append(lines, header)
		for _, file := range diff.Files {
			lines = append(lines, "  "+safeLabel(file))
		}
		if diff.Error != "" {
			lines = append(lines, "  [diff error: "+safeLabel(diff.Error)+"]")
			continue
		}
		for _, line := range strings.Split(diff.Patch, "\n") {
			text := strings.TrimRight(safeLabel(line), " \t")
			if text != "" {
				lines = append(lines, text)
			}
		}
		if diff.Truncated {
			lines = append(lines, "[patch truncated]")
		}
	}
	m.diff.taskID = result.taskID
	m.diff.lines = lines
	m.diff.scroll = 0
	m.Panel = PanelDiff
	m.Status = "diff loaded"
	return m, nil
}

func (m Model) diffKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.diff = diffState{}
		if m.dashboard.snapshot != nil {
			m.Panel = PanelAgents
		} else {
			m.Panel = PanelDashboard
		}
		m.Status = "task history loaded"
		return m, nil
	case tea.KeyUp:
		m.diff.scroll = max(0, m.diff.scroll-1)
	case tea.KeyDown:
		m.diff.scroll = min(m.diffMaxScroll(), m.diff.scroll+1)
	case tea.KeyPgUp:
		m.diff.scroll = max(0, m.diff.scroll-m.diffRows())
	case tea.KeyPgDown:
		m.diff.scroll = min(m.diffMaxScroll(), m.diff.scroll+m.diffRows())
	}
	return m, nil
}

func (m Model) diffRows() int {
	if m.Height-3 < 1 {
		return 1
	}
	return m.Height - 3
}

func (m Model) diffMaxScroll() int {
	return max(0, len(m.diff.lines)-m.diffRows())
}

func (m Model) diffView() string {
	width, height := m.Width, m.Height
	if width < 1 {
		width = 40
	}
	if height < 1 {
		height = 18
	}
	lines := []string{"Diff: " + safeLabel(m.diff.taskID)}
	visible := m.diffRows()
	start := min(m.diff.scroll, max(0, len(m.diff.lines)-visible))
	for i := start; i < len(m.diff.lines) && i < start+visible; i++ {
		lines = append(lines, m.diff.lines[i])
	}
	lines = append(lines, "Esc back", safeLabel(m.Status))
	for i, line := range lines {
		lines[i] = truncateSafe(line, width)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}
