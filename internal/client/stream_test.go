package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/server"
	"termide/internal/store"
)

func TestSubscribeStreamReceivesReplayThenPush(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	go func() {
		request, err := protocol.Read(right)
		if err != nil {
			return
		}
		if request.Type != protocol.MessageSubscribeStream || request.WorkspaceID != "w" || request.AfterSequence != 1 {
			panic(request)
		}
		events, _ := json.Marshal([]domain.EventEnvelope{{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 2, Type: domain.EventOutput}})
		_ = protocol.Write(right, protocol.Message{Type: protocol.MessageReplay, WorkspaceID: "w", RequestID: request.RequestID, Payload: events})
		push := domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 3, Type: domain.EventOutput}
		_ = protocol.Write(right, protocol.Message{Type: protocol.MessageEvent, WorkspaceID: "w", RequestID: request.RequestID, TaskID: "t", Event: &push})
	}()
	var got []domain.EventEnvelope
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := SubscribeStream(ctx, left, "w", 1, func(event domain.EventEnvelope) error {
		got = append(got, event)
		if len(got) == 2 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 || got[0].Sequence != 2 || got[1].Sequence != 3 {
		t.Fatalf("got = %#v", got)
	}
}

func TestSubscribeStreamRejectsForeignReplay(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	go func() {
		request, err := protocol.Read(right)
		if err != nil {
			return
		}
		events, _ := json.Marshal([]domain.EventEnvelope{{WorkspaceID: "other", TaskID: "t", AgentID: "a", Sequence: 2, Type: domain.EventOutput}})
		_ = protocol.Write(right, protocol.Message{Type: protocol.MessageReplay, WorkspaceID: "other", RequestID: request.RequestID, Payload: events})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := SubscribeStream(ctx, left, "w", 1, func(domain.EventEnvelope) error { return nil }); err == nil {
		t.Fatal("foreign replay accepted")
	}
}

func TestSubscribeStreamEndToEndOverServeControl(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte("old")}); err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	defer left.Close()
	done := make(chan error, 1)
	go func() {
		defer right.Close()
		done <- server.NewSupervisor(s, nil, "owner").Handler().Serve(ctx, right, right)
	}()
	defer func() { <-done }()
	var mu sync.Mutex
	var got []domain.EventEnvelope
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(got)
	}
	reads := make(chan error, 1)
	go func() {
		reads <- SubscribeStream(ctx, left, "w", 0, func(event domain.EventEnvelope) error {
			mu.Lock()
			defer mu.Unlock()
			got = append(got, event)
			return nil
		})
	}()
	deadline := time.Now().Add(3 * time.Second)
	for count() < 1 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if err := s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte("pushed")}); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for count() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	left.Close()
	select {
	case err := <-reads:
		if err == nil {
			t.Fatal("expected close error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not end on close")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || string(got[0].Payload) != "old" || string(got[1].Payload) != "pushed" {
		t.Fatalf("got = %#v", got)
	}
}

func TestStreamNextSkipsDuplicatePush(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	go func() {
		request, err := protocol.Read(right)
		if err != nil {
			return
		}
		if err := protocol.Write(right, protocol.Message{Type: protocol.MessageReplay, WorkspaceID: "w", RequestID: request.RequestID, Payload: []byte("[]")}); err != nil {
			return
		}
		dup := domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 2, Type: domain.EventOutput}
		_ = protocol.Write(right, protocol.Message{Type: protocol.MessageEvent, WorkspaceID: "w", RequestID: request.RequestID, TaskID: "t", Event: &dup})
		_ = protocol.Write(right, protocol.Message{Type: protocol.MessageEvent, WorkspaceID: "w", RequestID: request.RequestID, TaskID: "t", Event: &dup})
		next := domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 3, Type: domain.EventOutput}
		_ = protocol.Write(right, protocol.Message{Type: protocol.MessageEvent, WorkspaceID: "w", RequestID: request.RequestID, TaskID: "t", Event: &next})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stream, _, err := OpenStream(ctx, left, "w", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	event, err := stream.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if event.Sequence != 2 {
		t.Fatalf("event = %#v", event)
	}
	event, err = stream.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if event.Sequence != 3 {
		t.Fatalf("duplicate was not skipped: %#v", event)
	}
}

func TestSubscribeStreamSurfacesServerError(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	go func() {
		_, _ = protocol.Read(right)
		_ = protocol.Write(right, protocol.Message{Type: protocol.MessageError, Payload: []byte("nope")})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := SubscribeStream(ctx, left, "w", 0, func(domain.EventEnvelope) error { return nil }); err == nil {
		t.Fatal("server error swallowed")
	}
}
