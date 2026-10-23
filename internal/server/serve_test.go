package server

import (
	"bytes"
	"context"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
	"testing"
)

func TestServeProcessesMultipleRequests(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	var input, output bytes.Buffer
	for _, id := range []string{"one", "two"} {
		if err := protocol.Write(&input, protocol.Message{Type: protocol.MessageCreateTask, RequestID: id, TaskID: id, WorkspaceID: "w", Payload: []byte("task")}); err != nil {
			t.Fatal(err)
		}
	}
	if err := NewHandler(s).Serve(ctx, &input, &output); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two"} {
		response, err := protocol.Read(&output)
		if err != nil {
			t.Fatal(err)
		}
		if response.RequestID != id || response.Type != protocol.MessageEvent {
			t.Fatal(response)
		}
		if _, err := s.GetTask(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
}
