package server

import (
	"context"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func TestSubscribeRejectsReplayExceedingEventLimit(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxReplayEvents+1; i++ {
		if err := s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte("x")}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NewHandler(s).Handle(ctx, protocol.Message{Type: protocol.MessageSubscribe, WorkspaceID: "w"}); err == nil {
		t.Fatal("oversized replay accepted")
	}
}
