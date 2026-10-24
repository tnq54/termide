package server

import (
	"context"
	"testing"

	"termide/internal/agent"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

type handlerRunner struct{}

func (handlerRunner) Run(_ context.Context, spec agent.Spec, emit func(agent.Event)) error {
	emit(agent.Event{AgentID: spec.ID, Kind: "completed"})
	return nil
}

func TestHandlerCreatesTaskFromProtocolMessage(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w1", Name: "demo", Host: "local", Root: "/tmp"}); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s)
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageCreateTask, WorkspaceID: "w1", TaskID: "t1", Payload: []byte("build feature")})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageEvent || response.Event == nil {
		t.Fatalf("response = %#v", response)
	}
	if _, err := s.GetTask(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerStartsTaskAgents(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w1", Name: "demo", Host: "local", Root: "/tmp"}); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s)
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageCreateTask, WorkspaceID: "w1", TaskID: "t1", Payload: []byte("build")}); err != nil {
		t.Fatal(err)
	}
	response, err := h.HandleWithAgents(ctx, protocol.Message{Type: protocol.MessageStartTask, WorkspaceID: "w1", TaskID: "t1"}, []agent.Spec{{ID: "a1", Role: "coder"}}, handlerRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageEvent {
		t.Fatalf("response = %#v", response)
	}
	got, err := s.GetTask(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.TaskCompleted {
		t.Fatalf("task status = %q", got.Status)
	}
}
