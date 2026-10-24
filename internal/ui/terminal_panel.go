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

// ptyPollEvery spaces interactive read polls so keystrokes queue behind at
// most one quick round-trip on the serial transport.
const ptyPollEvery = 200 * time.Millisecond

const maxTermLines = 500

type termOpenedResult struct {
	id  string
	err error
}

type termReadResult struct {
	id   string
	data []byte
	next uint64
	err  error
}

type termWriteResult struct {
	err error
}

func (m Model) termCall(request protocol.Message) (protocol.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return m.remote.Call(ctx, request)
}

func (m Model) termOpenCommand() tea.Cmd {
	rows, cols := m.Height-4, m.Width
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 40
	}
	payload, err := json.Marshal(protocol.PtyOpenRequest{Rows: rows, Cols: cols})
	if err != nil {
		return func() tea.Msg { return termOpenedResult{err: err} }
	}
	return func() tea.Msg {
		response, err := m.termCall(protocol.Message{Type: protocol.MessagePtyOpen, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return termOpenedResult{err: err}
		}
		if response.Type != protocol.MessagePtyOpened || response.WorkspaceID != m.workspace.ID {
			return termOpenedResult{err: fmt.Errorf("unexpected pty open response")}
		}
		var opened protocol.PtyOpened
		if err := json.Unmarshal(response.Payload, &opened); err != nil || opened.ID == "" {
			return termOpenedResult{err: fmt.Errorf("invalid pty open response")}
		}
		return termOpenedResult{id: opened.ID}
	}
}

func (m Model) termReadCommand() tea.Cmd {
	id, after := m.termPtyID, m.termPtyNext
	payload, err := json.Marshal(protocol.PtyTarget{ID: id, After: after})
	if err != nil {
		return func() tea.Msg { return termReadResult{id: id, err: err} }
	}
	return func() tea.Msg {
		response, err := m.termCall(protocol.Message{Type: protocol.MessagePtyRead, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return termReadResult{id: id, err: err}
		}
		if response.Type != protocol.MessagePtyOutput || response.WorkspaceID != m.workspace.ID {
			return termReadResult{id: id, err: fmt.Errorf("unexpected pty read response")}
		}
		var output protocol.PtyOutput
		if err := json.Unmarshal(response.Payload, &output); err != nil || output.ID != id {
			return termReadResult{id: id, err: fmt.Errorf("invalid pty read response")}
		}
		return termReadResult{id: id, data: output.Data, next: output.Next}
	}
}

func (m Model) termWriteCommand(data []byte) tea.Cmd {
	payload, err := json.Marshal(protocol.PtyTarget{ID: m.termPtyID, Data: data})
	if err != nil {
		return func() tea.Msg { return termWriteResult{err: err} }
	}
	return func() tea.Msg {
		response, err := m.termCall(protocol.Message{Type: protocol.MessagePtyWrite, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return termWriteResult{err: err}
		}
		if response.Type != protocol.MessagePtyOk || response.WorkspaceID != m.workspace.ID {
			return termWriteResult{err: fmt.Errorf("unexpected pty write response")}
		}
		return termWriteResult{}
	}
}

func (m Model) termResizeCommand() tea.Cmd {
	rows, cols := m.Height-4, m.Width
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 40
	}
	payload, err := json.Marshal(protocol.PtyTarget{ID: m.termPtyID, Rows: rows, Cols: cols})
	if err != nil {
		return nil
	}
	return func() tea.Msg {
		_, _ = m.termCall(protocol.Message{Type: protocol.MessagePtyResize, WorkspaceID: m.workspace.ID, Payload: payload})
		return nil
	}
}

func (m Model) termCloseCommand() tea.Cmd {
	payload, err := json.Marshal(protocol.PtyTarget{ID: m.termPtyID})
	if err != nil {
		return nil
	}
	return func() tea.Msg {
		_, _ = m.termCall(protocol.Message{Type: protocol.MessagePtyClose, WorkspaceID: m.workspace.ID, Payload: payload})
		return nil
	}
}

func (m Model) receiveTermOpened(result termOpenedResult) (tea.Model, tea.Cmd) {
	if result.err != nil || result.id == "" {
		m.termPtyID = ""
		m.termPtyOpen = false
		if result.err != nil {
			m.Status = result.err.Error()
		}
		return m, nil
	}
	if m.Panel != PanelTerminal {
		// Left before the open completed: release the orphan session.
		id := result.id
		payload, err := json.Marshal(protocol.PtyTarget{ID: id})
		if err != nil {
			return m, nil
		}
		return m, func() tea.Msg {
			_, _ = m.termCall(protocol.Message{Type: protocol.MessagePtyClose, WorkspaceID: m.workspace.ID, Payload: payload})
			return nil
		}
	}
	m.termPtyID = result.id
	m.termPtyOpen = true
	m.termPtyNext = 0
	m.Status = "terminal connected"
	return m, m.termReadCommand()
}

func (m Model) receiveTermRead(result termReadResult) (tea.Model, tea.Cmd) {
	if result.id != m.termPtyID {
		// Stale poll from a previous session: discard, keep the live one.
		return m, nil
	}
	if result.err != nil {
		m.termPtyOpen = false
		m.Status = result.err.Error()
		return m, nil
	}
	if len(result.data) > 0 {
		m.termPtyNext = result.next
		m.termOutput = appendTermBytes(m.termOutput, result.data)
		m.termScroll = m.termMaxScroll()
	}
	if m.Panel == PanelTerminal && m.termPtyID != "" && m.termPtyOpen {
		return m, m.termPollCommand()
	}
	return m, nil
}

func (m Model) termPollCommand() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(ptyPollEvery)
		return m.termReadCommand()()
	}
}

func appendTermBytes(lines []string, data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	for _, line := range strings.Split(text, "\n") {
		if len(lines) == 0 {
			lines = append(lines, "")
		}
		lines[len(lines)-1] += line
		lines = append(lines, "")
	}
	lines = lines[:len(lines)-1]
	for len(lines) > maxTermLines {
		lines = lines[1:]
	}
	return lines
}

func (m Model) termMaxScroll() int {
	return max(0, len(m.termOutput)-m.termRows())
}

func (m Model) terminalView() string {
	width, height := m.Width, m.Height
	if width < 1 {
		width = 40
	}
	if height < 1 {
		height = 18
	}
	var lines []string
	if m.termPtyID == "" || !m.termPtyOpen {
		lines = append(lines, "(no shell: Esc back, t retry)")
	} else {
		visible := m.termRows()
		start := min(m.termScroll, max(0, len(m.termOutput)-visible))
		for i := start; i < len(m.termOutput) && i < start+visible; i++ {
			lines = append(lines, m.termOutput[i])
		}
	}
	lines = append(lines, "type to send | PgUp/PgDn scroll | Esc back", safeLabel(m.Status))
	for i, line := range lines {
		lines[i] = truncateSafe(line, width)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) termRows() int {
	if m.Height-4 < 1 {
		return 1
	}
	return m.Height - 4
}

func (m Model) terminalKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		cmd := m.termCloseCommand()
		m.termPtyID = ""
		m.termPtyOpen = false
		m.Panel = PanelDashboard
		m.Status = "terminal closed"
		return m, cmd
	case tea.KeyPgUp:
		m.termScroll = max(0, m.termScroll-m.termRows())
		return m, nil
	case tea.KeyPgDown:
		m.termScroll = min(m.termMaxScroll(), m.termScroll+m.termRows())
		return m, nil
	}
	if !m.termPtyOpen || m.termPtyID == "" || m.remote == nil || !m.ready {
		return m, nil
	}
	var data []byte
	switch key.Type {
	case tea.KeyRunes:
		data = []byte(string(key.Runes))
	case tea.KeySpace:
		data = []byte(" ")
	case tea.KeyEnter:
		data = []byte("\r")
	case tea.KeyBackspace, tea.KeyCtrlH:
		data = []byte{127}
	case tea.KeyLeft:
		data = []byte("\x1b[D")
	case tea.KeyRight:
		data = []byte("\x1b[C")
	case tea.KeyUp:
		data = []byte("\x1b[A")
	case tea.KeyDown:
		data = []byte("\x1b[B")
	case tea.KeyTab:
		data = []byte("\t")
	case tea.KeyCtrlC:
		data = []byte("\x03")
	case tea.KeyCtrlD:
		data = []byte("\x04")
	default:
		return m, nil
	}
	return m, m.termWriteCommand(data)
}
