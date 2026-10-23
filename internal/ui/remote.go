package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"termide/internal/domain"
	"termide/internal/protocol"
)

type workspaceConfig = domain.Workspace
type remoteCaller interface {
	Call(context.Context, protocol.Message) (protocol.Message, error)
}
type remoteResult struct {
	taskID string
	err    error
}

func NewRemoteModel(remote remoteCaller, w domain.Workspace) Model {
	m := NewModel()
	m.remote = remote
	m.workspace = w
	m.Project = w.Root
	m.VPS = w.Host
	m.Status = "opening workspace"
	return m
}

func (m Model) openRemoteWorkspace() tea.Cmd {
	if m.remote == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		payload, err := json.Marshal(m.workspace)
		if err != nil {
			return remoteResult{err: err}
		}
		response, err := m.remote.Call(ctx, protocol.Message{Type: protocol.MessageOpenWorkspace, Payload: payload})
		if err == nil && (response.Type != protocol.MessageWorkspaceReady || response.WorkspaceID != m.workspace.ID) {
			err = fmt.Errorf("unexpected workspace response")
		}
		return remoteResult{err: err}
	}
}

func (m Model) saveRemoteTask() tea.Cmd {
	id := uuid.NewString()
	description := m.TaskDescription
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		response, err := m.remote.Call(ctx, protocol.Message{Type: protocol.MessageCreateTask, WorkspaceID: m.workspace.ID, TaskID: id, Payload: []byte(description)})
		if err == nil && (response.Event == nil || response.Event.TaskID != id) {
			err = fmt.Errorf("unexpected task response")
		}
		return remoteResult{taskID: id, err: err}
	}
}
