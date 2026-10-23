package client

import (
	"context"
	"net"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/server"
	"termide/internal/store"
)

func TestRemoteCreatesTasksOverOneConnection(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() { defer right.Close(); done <- server.NewHandler(s).Serve(ctx, right, right) }()
	c := NewRemote(left)
	for _, id := range []string{"one", "two"} {
		response, err := c.Call(ctx, protocol.Message{Type: protocol.MessageCreateTask, WorkspaceID: "w", TaskID: id, Payload: []byte("task")})
		if err != nil {
			t.Fatal(err)
		}
		if response.Event == nil || response.Event.TaskID != id {
			t.Fatal(response)
		}
		if _, err := s.GetTask(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	c.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRemoteCancellationClosesBlockedConnection(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	c := NewRemote(left)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.Call(ctx, protocol.Message{Type: protocol.MessageHello}); err == nil {
		t.Fatal("cancelled call succeeded")
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation blocked")
	}
}

func TestRemoteRejectsMismatchedResponse(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	go func() {
		protocol.Read(right)
		protocol.Write(right, protocol.Message{Type: protocol.MessageHello, RequestID: "wrong"})
	}()
	c := NewRemote(left)
	defer c.Close()
	if _, err := c.Call(context.Background(), protocol.Message{Type: protocol.MessageHello}); err == nil {
		t.Fatal("mismatch accepted")
	}
}
