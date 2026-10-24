package server

import (
	"context"
	"encoding/json"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func TestSubscribeReplaysOnlyWorkspaceEventsAfterCursor(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"w", "other"} {
		if err := s.CreateWorkspace(ctx, domain.Workspace{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	for _, event := range []domain.EventEnvelope{
		{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte("one")},
		{WorkspaceID: "other", TaskID: "x", AgentID: "a", Type: domain.EventOutput, Payload: []byte("hidden")},
		{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte("two")},
	} {
		if err := s.AppendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(s)
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageSubscribe, WorkspaceID: "w", AfterSequence: 1})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageReplay || response.WorkspaceID != "w" {
		t.Fatal(response)
	}
	var events []domain.EventEnvelope
	if err := json.Unmarshal(response.Payload, &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || string(events[0].Payload) != "two" || events[0].Sequence <= 1 {
		t.Fatal(events)
	}
}

func TestSubscribeRejectsUnknownWorkspace(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := NewHandler(s).Handle(context.Background(), protocol.Message{Type: protocol.MessageSubscribe, WorkspaceID: "missing"}); err == nil {
		t.Fatal("unknown workspace accepted")
	}
}
