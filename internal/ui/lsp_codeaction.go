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

type codeActionListResult struct {
	actions []protocol.LspCodeActionItem
	err     error
}

type codeActionAppliedResult struct {
	files []protocol.LspRenameFile
	err   error
}

func (m Model) codeActionCommand(path string, startLine, startChar, endLine, endChar int) tea.Cmd {
	return m.codeActionCommandFiltered(path, startLine, startChar, endLine, endChar, nil)
}

// refactorActionCommand requests refactor actions for the line under the
// cursor, reused for the Ctrl+T "extract / refactor here" shortcut.
func (m Model) refactorActionCommand(path string) tea.Cmd {
	line, col := m.files.input.CursorLineCol()
	_ = col
	endChar := 0
	if text := m.files.input.Value(); text != "" {
		lines := strings.Split(text, "\n")
		if line >= 0 && line < len(lines) {
			endChar = len([]rune(lines[line]))
		}
	}
	return m.codeActionCommandFiltered(path, line, 0, line, endChar, []string{"refactor"})
}

// sourceActionCommand requests file-level source actions over the whole
// buffer, reused for the Ctrl+F "fix file" shortcut.
func (m Model) sourceActionCommand(path string) tea.Cmd {
	endLine := 0
	endChar := 0
	if text := m.files.input.Value(); text != "" {
		lines := strings.Split(text, "\n")
		endLine = len(lines) - 1
		if endLine >= 0 && endLine < len(lines) {
			endChar = len([]rune(lines[endLine]))
		}
	}
	return m.codeActionCommandFiltered(path, 0, 0, endLine, endChar, []string{"source"})
}

func (m Model) codeActionCommandFiltered(path string, startLine, startChar, endLine, endChar int, only []string) tea.Cmd {
	payload, err := json.Marshal(protocol.LspCodeActionRequest{Path: path, StartLine: startLine, StartChar: startChar, EndLine: endLine, EndChar: endChar, Only: only})
	if err != nil {
		return func() tea.Msg { return codeActionListResult{err: err} }
	}
	return func() tea.Msg {
		timeout := 15 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		response, err := m.remote.Call(ctx, protocol.Message{Type: protocol.MessageLspCodeAction, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return codeActionListResult{err: err}
		}
		if response.Type != protocol.MessageLspCodeActionList || response.WorkspaceID != m.workspace.ID {
			return codeActionListResult{err: fmt.Errorf("unexpected code action response")}
		}
		var result protocol.LspCodeActionList
		if err := json.Unmarshal(response.Payload, &result); err != nil {
			return codeActionListResult{err: err}
		}
		return codeActionListResult{actions: result.Actions}
	}
}

func (m Model) receiveCodeActionList(result codeActionListResult) (tea.Model, tea.Cmd) {
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	if len(result.actions) == 0 {
		m.Status = "no code actions"
		return m, nil
	}
	m.codeActions = result.actions
	m.codeActionSel = 0
	m.Status = "code action: Up/Down navigate, Enter applies"
	return m, nil
}

func (m Model) codeActionApplyCommand(id string) tea.Cmd {
	payload, err := json.Marshal(protocol.LspCodeActionApplyRequest{ID: id})
	if err != nil {
		return func() tea.Msg { return codeActionAppliedResult{err: err} }
	}
	return func() tea.Msg {
		timeout := 15 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		response, err := m.remote.Call(ctx, protocol.Message{Type: protocol.MessageLspCodeActionApply, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return codeActionAppliedResult{err: err}
		}
		if response.Type != protocol.MessageLspCodeActionApplied || response.WorkspaceID != m.workspace.ID {
			return codeActionAppliedResult{err: fmt.Errorf("unexpected code action response")}
		}
		var result protocol.LspRenameResult
		if err := json.Unmarshal(response.Payload, &result); err != nil {
			return codeActionAppliedResult{err: err}
		}
		return codeActionAppliedResult{files: result.Files}
	}
}

func (m Model) receiveCodeActionApplied(result codeActionAppliedResult) (tea.Model, tea.Cmd) {
	m.codeActions = nil
	m.codeActionSel = 0
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	affected := false
	for _, file := range result.files {
		if m.files.document != nil && file.Path == m.files.document.Path {
			affected = true
			break
		}
	}
	if !affected {
		m.Status = fmt.Sprintf("code action applied in %d file(s)", len(result.files))
		return m, nil
	}
	if m.files.dirty() {
		m.Status = fmt.Sprintf("code action applied on disk in %d file(s); buffer has unsaved edits", len(result.files))
		return m, nil
	}
	m.Status = fmt.Sprintf("code action applied in %d file(s)", len(result.files))
	m.reloadNotice = m.Status
	return m, m.fileCommand(protocol.MessageReadFile, protocol.FileRequest{Path: m.files.document.Path})
}

func (m Model) renderCodeActions(lines []string, width int) []string {
	if len(m.codeActions) == 0 || len(lines) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines)+len(m.codeActions)+2)
	out = append(out, lines[0])
	out = append(out, "--- code action (Esc) ---")
	shown := m.codeActions
	if len(shown) > 8 {
		shown = shown[:8]
	}
	for i, action := range shown {
		marker := "  "
		if i == m.codeActionSel {
			marker = "> "
		}
		text := strings.TrimSpace(safeLabel(action.Title))
		if detail := strings.TrimSpace(safeLabel(action.Kind)); detail != "" {
			text += " [" + detail + "]"
		}
		if width > 2 && len(text) > width-2 {
			text = text[:width-2]
		}
		out = append(out, marker+text)
	}
	return append(out, lines[1:]...)
}
