package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func TestSubscribeStreamReplaysThenPushes(t *testing.T) {
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
	sup := NewSupervisor(s, &emittingRunner{text: "x"}, "owner")
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- ServeControl(ctx, right, sup) }()
	defer func() { left.Close(); <-done }()
	if err := protocol.Write(left, protocol.Message{Type: protocol.MessageSubscribeStream, RequestID: "1", WorkspaceID: "w"}); err != nil {
		t.Fatal(err)
	}
	replay, err := protocol.Read(left)
	if err != nil {
		t.Fatal(err)
	}
	var events []domain.EventEnvelope
	if replay.Type != protocol.MessageReplay || json.Unmarshal(replay.Payload, &events) != nil || len(events) != 1 {
		t.Fatalf("replay = %#v", replay)
	}
	if err := s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte("new")}); err != nil {
		t.Fatal(err)
	}
	left.SetReadDeadline(time.Now().Add(3 * time.Second))
	pushed, err := protocol.Read(left)
	if err != nil {
		t.Fatal(err)
	}
	if pushed.Type != protocol.MessageEvent || pushed.Event == nil || string(pushed.Event.Payload) != "new" {
		t.Fatalf("pushed = %#v", pushed)
	}
}

func TestSubscribeStreamRejectsUnknownWorkspace(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	sup := NewSupervisor(s, &emittingRunner{text: "x"}, "owner")
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- ServeControl(context.Background(), right, sup) }()
	defer func() { left.Close(); <-done }()
	if err := protocol.Write(left, protocol.Message{Type: protocol.MessageSubscribeStream, WorkspaceID: "missing"}); err != nil {
		t.Fatal(err)
	}
	response, err := protocol.Read(left)
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageError {
		t.Fatalf("response = %#v", response)
	}
}

func TestBridgePumpsStreamToSession(t *testing.T) {
	controlClient, controlServer := net.Pipe()
	defer controlClient.Close()
	defer controlServer.Close()
	sessionClient, sessionServer := net.Pipe()
	defer sessionClient.Close()
	defer sessionServer.Close()
	bridgeDone := make(chan error, 1)
	go func() { bridgeDone <- NewBridge(controlClient).Serve(context.Background(), sessionServer) }()
	if err := protocol.Write(sessionClient, protocol.Message{Type: protocol.MessageSubscribeStream, RequestID: "9", WorkspaceID: "w"}); err != nil {
		t.Fatal(err)
	}
	request, err := protocol.Read(controlServer)
	if err != nil {
		t.Fatal(err)
	}
	if request.Type != protocol.MessageSubscribeStream {
		t.Fatal(request)
	}
	payload, _ := json.Marshal([]domain.EventEnvelope{})
	if err := protocol.Write(controlServer, protocol.Message{Type: protocol.MessageReplay, RequestID: "9", WorkspaceID: "w", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	sessionClient.SetReadDeadline(time.Now().Add(2 * time.Second))
	first, err := protocol.Read(sessionClient)
	if err != nil {
		t.Fatal(err)
	}
	if first.Type != protocol.MessageReplay {
		t.Fatalf("first = %#v", first)
	}
	if err := protocol.Write(controlServer, protocol.Message{Type: protocol.MessageEvent, RequestID: "9", WorkspaceID: "w", TaskID: "t", Event: &domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 1, Type: domain.EventOutput}}); err != nil {
		t.Fatal(err)
	}
	sessionClient.SetReadDeadline(time.Now().Add(2 * time.Second))
	second, err := protocol.Read(sessionClient)
	if err != nil {
		t.Fatal(err)
	}
	if second.Type != protocol.MessageEvent {
		t.Fatalf("second = %#v", second)
	}
	sessionClient.Close()
	select {
	case <-bridgeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge pump did not exit")
	}
}
