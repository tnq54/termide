package server

import (
	"context"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func TestHandlerCreatesTaskFromProtocolMessage(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil { t.Fatal(err) }
	defer s.Close()
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w1", Name: "demo", Host: "local", Root: "/tmp"}); err != nil { t.Fatal(err) }
	h := NewHandler(s)
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageCreateTask, WorkspaceID: "w1", TaskID: "t1", Payload: []byte("build feature")})
	if err != nil { t.Fatal(err) }
	if response.Type != protocol.MessageEvent || response.Event == nil { t.Fatalf("response = %#v", response) }
	if _, err := s.GetTask(ctx, "t1"); err != nil { t.Fatal(err) }
}
