package server

import (
	"context"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func TestDurableHandlerUsesSupervisorAfterSessionContextEnds(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Description: "task", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	runner := &durableRunner{started: make(chan struct{}), done: make(chan struct{})}
	sup := NewSupervisor(s, runner, "owner")
	h := NewDurableHandler(s, sup)
	reqCtx, cancel := context.WithCancel(ctx)
	plan := []byte(`[{"id":"a","role":"coder"}]`)
	response, err := h.Handle(reqCtx, protocol.Message{Type: protocol.MessageStartTask, WorkspaceID: "w", TaskID: "t", Payload: plan})
	if err != nil || response.Type != protocol.MessageEvent {
		t.Fatal(response, err)
	}
	cancel()
	close(runner.done)
	if err := sup.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTask(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.TaskCompleted {
		t.Fatal(got)
	}
}
