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

type terminalResult struct {
	command string
	result  protocol.CommandResult
	err     error
}

func (m Model) terminalCommand(command string) tea.Cmd {
	return func() tea.Msg {
		timeout := 45 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		payload, err := json.Marshal(protocol.CommandRequest{Command: command})
		if err != nil {
			return terminalResult{command: command, err: err}
		}
		response, err := m.remote.Call(ctx, protocol.Message{Type: protocol.MessageRunCommand, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return terminalResult{command: command, err: err}
		}
		result := terminalResult{command: command}
		if response.Type != protocol.MessageCommandResult || response.WorkspaceID != m.workspace.ID {
			result.err = fmt.Errorf("unexpected command response")
			return result
		}
		result.err = json.Unmarshal(response.Payload, &result.result)
		return result
	}
}

func (m Model) receiveTerminal(result terminalResult) (tea.Model, tea.Cmd) {
	m.termBusy = false
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	m.termOutput = append(m.termOutput, "$ "+result.command)
	for _, line := range strings.Split(result.result.Output, "\n") {
		text := strings.TrimRight(safeLabel(line), " \t")
		if text != "" {
			m.termOutput = append(m.termOutput, text)
		}
	}
	if result.result.Truncated {
		m.termOutput = append(m.termOutput, "[output truncated]")
	}
	if result.result.ExitCode != 0 {
		m.termOutput = append(m.termOutput, fmt.Sprintf("[exit %d]", result.result.ExitCode))
	}
	m.termScroll = m.termMaxScroll()
	m.Status = "command finished"
	return m, nil
}

func (m Model) termMaxScroll() int {
	visible := m.Height - 4
	if visible < 1 {
		visible = 1
	}
	return max(0, len(m.termOutput)-visible)
}

func (m Model) terminalKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.Panel = PanelDashboard
		return m, nil
	case tea.KeyEnter:
		if m.remote == nil || !m.ready || m.termBusy {
			return m, nil
		}
		command := strings.TrimSpace(m.termInput)
		if command == "" {
			return m, nil
		}
		m.termBusy = true
		m.Status = "running command"
		return m, m.terminalCommand(command)
	case tea.KeyBackspace, tea.KeyCtrlH:
		runes := []rune(m.termInput)
		if len(runes) > 0 {
			m.termInput = string(runes[:len(runes)-1])
		}
		return m, nil
	case tea.KeyUp:
		m.termScroll = max(0, m.termScroll-1)
		return m, nil
	case tea.KeyDown:
		m.termScroll = min(m.termMaxScroll(), m.termScroll+1)
		return m, nil
	case tea.KeyPgUp:
		m.termScroll = max(0, m.termScroll-m.termRows())
		return m, nil
	case tea.KeyPgDown:
		m.termScroll = min(m.termMaxScroll(), m.termScroll+m.termRows())
		return m, nil
	case tea.KeyRunes:
		m.termInput += string(key.Runes)
		return m, nil
	case tea.KeySpace:
		m.termInput += " "
		return m, nil
	}
	return m, nil
}

func (m Model) termRows() int {
	if m.Height-4 < 1 {
		return 1
	}
	return m.Height - 4
}

func (m Model) terminalView() string {
	width, height := m.Width, m.Height
	if width < 1 {
		width = 40
	}
	if height < 1 {
		height = 18
	}
	lines := []string{"$ " + safeLabel(m.termInput)}
	visible := m.termRows()
	start := min(m.termScroll, max(0, len(m.termOutput)-visible))
	for i := start; i < len(m.termOutput) && i < start+visible; i++ {
		lines = append(lines, m.termOutput[i])
	}
	lines = append(lines, "Enter run | Esc dashboard", safeLabel(m.Status))
	for i, line := range lines {
		lines[i] = truncateSafe(line, width)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}
