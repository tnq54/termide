package server

import (
	"context"
	"encoding/json"
	"termide/internal/agent"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
	"testing"
)

func TestStartTaskRejectsTaskFromAnotherWorkspace(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.CreateWorkspace(ctx, domain.Workspace{ID: "w1"})
	s.CreateWorkspace(ctx, domain.Workspace{ID: "w2"})
	s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w2", Status: domain.TaskQueued})
	h := NewHandlerWithRunner(s, dashboardRunner{})
	payload, _ := json.Marshal([]agent.Spec{{ID: "a", Role: "coder"}})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageStartTask, WorkspaceID: "w1", TaskID: "t", Payload: payload}); err == nil {
		t.Fatal("accepted cross-workspace task")
	}
	task, _ := s.GetTask(ctx, "t")
	if task.Status != domain.TaskQueued {
		t.Fatalf("task mutated: %q", task.Status)
	}
}

func TestDashboardCanStartQueuedTaskAndRefreshStatus(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: t.TempDir()})
	s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Description: "run", Status: domain.TaskQueued})
	h := NewHandlerWithRunner(s, dashboardRunner{})
	payload, _ := json.Marshal([]agent.Spec{{ID: "a", Role: "coder", Command: "test"}})
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageStartTask, WorkspaceID: "w", TaskID: "t", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageEvent {
		t.Fatalf("response type = %q", response.Type)
	}
	h.jobs.Wait()
	task, err := s.GetTask(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskCompleted {
		t.Fatalf("status = %q", task.Status)
	}
}

type dashboardRunner struct{}

func (dashboardRunner) Run(_ context.Context, spec agent.Spec, emit func(agent.Event)) error {
	emit(agent.Event{AgentID: spec.ID, Output: "done"})
	return nil
}

func TestDashboardProtocolScopesTasksAndReturnsSnapshot(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"w", "other"} {
		s.CreateWorkspace(ctx, domain.Workspace{ID: id})
	}
	s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Description: "real task", Status: domain.TaskQueued})
	h := NewHandler(s)
	page, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageListTasks, WorkspaceID: "w"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded domain.TaskPage
	if err := json.Unmarshal(page.Payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if page.Type != protocol.MessageTaskPage || len(decoded.Tasks) != 1 || decoded.Tasks[0].ID != "t" {
		t.Fatal(page)
	}
	detail, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageTaskSnapshot, WorkspaceID: "w", TaskID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot domain.TaskSnapshot
	json.Unmarshal(detail.Payload, &snapshot)
	if snapshot.Task.Description != "real task" {
		t.Fatal(snapshot)
	}
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageTaskSnapshot, WorkspaceID: "other", TaskID: "t"}); err == nil {
		t.Fatal("cross-workspace detail")
	}
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageListTasks, WorkspaceID: "missing"}); err == nil {
		t.Fatal("missing workspace accepted")
	}
}
