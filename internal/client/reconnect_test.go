package client

import (
	"context"
	"encoding/json"
	"net"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/server"
	"termide/internal/store"
	"testing"
)

func TestRemoteSubscribeReplaysFromCursor(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	s.CreateWorkspace(ctx, domain.Workspace{ID: "w"})
	for _, text := range []string{"one", "two", "three"} {
		s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte(text)})
	}
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() { defer right.Close(); done <- server.NewHandler(s).Serve(ctx, right, right) }()
	remote := NewRemote(left)
	defer func() { remote.Close(); <-done }()
	response, err := remote.Call(ctx, protocol.Message{Type: protocol.MessageSubscribe, WorkspaceID: "w", AfterSequence: 1})
	if err != nil {
		t.Fatal(err)
	}
	var events []domain.EventEnvelope
	if err := json.Unmarshal(response.Payload, &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Sequence != 2 || events[1].Sequence != 3 {
		t.Fatal(events)
	}
}
