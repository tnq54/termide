package ui

import (
	"context"
	"fmt"
	"net"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/client"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/server"
	"termide/internal/store"
)

func TestTUISavesTaskThroughRemoteServer(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() { defer right.Close(); done <- server.NewHandler(s).Serve(context.Background(), right, right) }()
	remote := client.NewRemote(left)
	defer func() { remote.Close(); <-done }()
	m := NewRemoteModel(remote, domain.Workspace{ID: "w", Root: t.TempDir(), Host: "test"})
	next, _ := m.Update(m.Init()())
	m = next.(Model)
	if m.Status != "connected" {
		t.Fatal(m.Status)
	}
	next, _ = m.Update(key("n"))
	m = next.(Model)
	next, _ = m.Update(key("sửa mã Python"))
	m = next.(Model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil || m.Status != "saving task" {
		t.Fatal("task was not sent")
	}
	_, duplicate := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("duplicate submission")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.Status != "task queued" || m.TaskID == "" {
		t.Fatal(m.Status)
	}
	task, err := s.GetTask(context.Background(), m.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Description != "sửa mã Python" || task.WorkspaceID != "w" {
		t.Fatal(task)
	}
}

type failingRemote struct{}

func (failingRemote) Call(context.Context, protocol.Message) (protocol.Message, error) {
	return protocol.Message{}, fmt.Errorf("SSH disconnected")
}

func TestRemoteFailureDoesNotPretendTaskWasSaved(t *testing.T) {
	m := NewRemoteModel(failingRemote{}, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelTask
	m.TaskDescription = "keep this description"
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.TaskID != "" || m.Panel != PanelTask || m.saving {
		t.Fatal("failure reported as success")
	}
	if m.TaskDescription != "keep this description" || m.Status != "SSH disconnected" {
		t.Fatal(m.Status)
	}
}
