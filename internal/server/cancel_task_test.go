package server

import (
	"context"
	"encoding/json"
	"net"
	"termide/internal/agent"
	"termide/internal/client"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
	"testing"
	"time"
)

func TestCancelTaskThroughTransportScopesAndPersists(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	for _, id := range []string{"w", "other"} {
		if err := s.CreateWorkspace(ctx, domain.Workspace{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	runner := waitingRunner{make(chan struct{}), make(chan struct{})}
	h := NewHandlerWithRunner(s, runner)
	left, right := net.Pipe()
	remote := client.NewRemote(left)
	done := make(chan error, 1)
	go func() { defer right.Close(); done <- h.Serve(ctx, right, right) }()
	t.Cleanup(func() { remote.Close(); <-done })
	deadline, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	plan, _ := json.Marshal([]agent.Spec{{ID: "a", Role: "coder"}})
	if _, err := remote.Call(deadline, protocol.Message{Type: protocol.MessageStartTask, TaskID: "t", WorkspaceID: "w", Payload: plan}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-runner.entered:
	case <-deadline.Done():
		t.Fatal("runner did not start")
	}
	if _, err := remote.Call(deadline, protocol.Message{Type: protocol.MessageCancel, TaskID: "t", WorkspaceID: "other"}); err == nil {
		t.Fatal("foreign cancel accepted")
	}
	select {
	case <-runner.exited:
		t.Fatal("foreign cancel stopped runner")
	default:
	}
	response, err := remote.Call(deadline, protocol.Message{Type: protocol.MessageCancel, TaskID: "t", WorkspaceID: "w"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != "tasks.cancel_requested" || response.TaskID != "t" || response.WorkspaceID != "w" {
		t.Fatal(response)
	}
	select {
	case <-runner.exited:
	case <-deadline.Done():
		t.Fatal("runner did not cancel")
	}
	h.jobs.Wait()
	response, err = remote.Call(deadline, protocol.Message{Type: protocol.MessageTaskSnapshot, TaskID: "t", WorkspaceID: "w"})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot domain.TaskSnapshot
	if err := json.Unmarshal(response.Payload, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Task.Status != domain.TaskCancelled || len(snapshot.Agents) != 1 || snapshot.Agents[0].Status != domain.AgentCancelled {
		t.Fatal(snapshot)
	}
	if _, err := remote.Call(deadline, protocol.Message{Type: protocol.MessageCancel, TaskID: "t", WorkspaceID: "w"}); err == nil {
		t.Fatal("terminal task accepted cancellation")
	}
}

func TestCancelDoesNotChangeUnownedTask(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	s.CreateWorkspace(ctx, domain.Workspace{ID: "w"})
	s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning})
	h := NewHandler(s)
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageCancel, TaskID: "t", WorkspaceID: "w"}); err == nil {
		t.Fatal("unowned cancellation accepted")
	}
	task, err := s.GetTask(ctx, "t")
	if err != nil || task.Status != domain.TaskRunning {
		t.Fatal(task, err)
	}
}
