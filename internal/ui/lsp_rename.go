package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/protocol"
)

type renameResult struct {
	files []protocol.LspRenameFile
	err   error
}

func (m Model) renameCommand(path string, line, character int, newName string) tea.Cmd {
	payload, err := json.Marshal(protocol.LspRenameRequest{Path: path, Line: line, Character: character, NewName: newName})
	if err != nil {
		return func() tea.Msg { return renameResult{err: err} }
	}
	return func() tea.Msg {
		timeout := 15 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		response, err := m.remote.Call(ctx, protocol.Message{Type: protocol.MessageLspRename, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return renameResult{err: err}
		}
		if response.Type != protocol.MessageLspRenameResult || response.WorkspaceID != m.workspace.ID {
			return renameResult{err: fmt.Errorf("unexpected rename response")}
		}
		var result protocol.LspRenameResult
		if err := json.Unmarshal(response.Payload, &result); err != nil {
			return renameResult{err: err}
		}
		return renameResult{files: result.Files}
	}
}

func (m Model) receiveRename(result renameResult) (tea.Model, tea.Cmd) {
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
		m.Status = fmt.Sprintf("renamed in %d file(s)", len(result.files))
		return m, nil
	}
	if m.files.dirty() {
		m.Status = fmt.Sprintf("renamed on disk in %d file(s); buffer has unsaved edits", len(result.files))
		return m, nil
	}
	m.Status = fmt.Sprintf("renamed in %d file(s)", len(result.files))
	m.renameNotice = m.Status
	return m, m.fileCommand(protocol.MessageReadFile, protocol.FileRequest{Path: m.files.document.Path})
}

// renamePrompt returns the status line shown while collecting a new name.
func renamePrompt(buf string) string {
	if buf == "" {
		return "rename to: _"
	}
	return "rename to: " + buf
}
