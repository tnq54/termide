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

func TestControlRoundTripStartAndCancel(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	runner := &durableRunner{started: make(chan struct{}), done: make(chan struct{})}
	sup := NewSupervisor(s, runner, "owner")
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- ServeControl(ctx, right, sup) }()
	defer func() { left.Close(); <-done; close(runner.done); _ = sup.Close(context.Background()) }()
	if err := protocol.Write(left, protocol.Message{Type: protocol.MessageStartTask, WorkspaceID: "w", TaskID: "t", Payload: []byte(`[ {"id":"a","role":"coder"} ]`)}); err != nil {
		t.Fatal(err)
	}
	response, err := protocol.Read(left)
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageEvent || response.TaskID != "t" {
		t.Fatal(response)
	}
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	if err := protocol.Write(left, protocol.Message{Type: protocol.MessageCancel, WorkspaceID: "w", TaskID: "t"}); err != nil {
		t.Fatal(err)
	}
	response, err = protocol.Read(left)
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageTaskCancelRequested {
		t.Fatal(response)
	}
}

func TestControlPassesThroughSubscribeAndRejectsUnknown(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	sup := NewSupervisor(s, &durableRunner{started: make(chan struct{}), done: make(chan struct{})}, "owner")
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- ServeControl(ctx, right, sup) }()
	defer func() { left.Close(); <-done }()
	if err := protocol.Write(left, protocol.Message{Type: protocol.MessageSubscribe, WorkspaceID: "w"}); err != nil {
		t.Fatal(err)
	}
	response, err := protocol.Read(left)
	if err != nil {
		t.Fatal(err)
	}
	var events []domain.EventEnvelope
	if response.Type != protocol.MessageReplay || json.Unmarshal(response.Payload, &events) != nil || len(events) != 1 {
		t.Fatal(response)
	}
	if err := protocol.Write(left, protocol.Message{Type: "nope"}); err != nil {
		t.Fatal(err)
	}
	response, err = protocol.Read(left)
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageError {
		t.Fatal(response)
	}
}
