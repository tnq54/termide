package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func subscribeWaitFixture(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.CreateWorkspace(context.Background(), domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSubscribeWaitReturnsImmediatelyWhenEventsExist(t *testing.T) {
	s := subscribeWaitFixture(t)
	ctx := context.Background()
	if err := s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	response, err := NewHandler(s).Handle(ctx, protocol.Message{Type: protocol.MessageSubscribe, WorkspaceID: "w", WaitMs: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("waited despite existing events: %v", elapsed)
	}
	var events []domain.EventEnvelope
	if err := json.Unmarshal(response.Payload, &events); err != nil || len(events) != 1 {
		t.Fatal(response, err)
	}
}

func TestSubscribeWaitBlocksUntilNewEvent(t *testing.T) {
	s := subscribeWaitFixture(t)
	ctx := context.Background()
	done := make(chan protocol.Message, 1)
	go func() {
		response, err := NewHandler(s).Handle(ctx, protocol.Message{Type: protocol.MessageSubscribe, WorkspaceID: "w", WaitMs: 5000})
		if err != nil {
			t.Error(err)
			return
		}
		done <- response
	}()
	select {
	case <-done:
		t.Fatal("returned before any event existed")
	case <-time.After(300 * time.Millisecond):
	}
	if err := s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte("late")}); err != nil {
		t.Fatal(err)
	}
	// The waiter asked after sequence 0, so the first event must wake it.
	select {
	case response := <-done:
		var events []domain.EventEnvelope
		if err := json.Unmarshal(response.Payload, &events); err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || string(events[0].Payload) != "late" {
			t.Fatalf("events = %#v", events)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("long-poll did not wake on new event")
	}
}

func TestSubscribeWaitTimesOutEmpty(t *testing.T) {
	s := subscribeWaitFixture(t)
	start := time.Now()
	response, err := NewHandler(s).Handle(context.Background(), protocol.Message{Type: protocol.MessageSubscribe, WorkspaceID: "w", WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Fatalf("returned too fast: %v", elapsed)
	}
	var events []domain.EventEnvelope
	if err := json.Unmarshal(response.Payload, &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %#v", events)
	}
}

func TestSubscribeWaitIsCapped(t *testing.T) {
	old := maxSubscribeWait
	maxSubscribeWait = 200 * time.Millisecond
	defer func() { maxSubscribeWait = old }()
	s := subscribeWaitFixture(t)
	start := time.Now()
	if _, err := NewHandler(s).Handle(context.Background(), protocol.Message{Type: protocol.MessageSubscribe, WorkspaceID: "w", WaitMs: 3600000}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("wait not capped: %v", elapsed)
	}
}
