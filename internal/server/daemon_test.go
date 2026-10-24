package server

import (
	"context"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func TestDaemonTaskSurvivesSessionClose(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Description: "task", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	socket := t.TempDir() + "/daemon.sock"
	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	defer daemonCancel()
	daemonDone := make(chan error, 1)
	go func() { daemonDone <- RunDaemon(daemonCtx, s, socket, &emittingRunner{text: "durable"}, "daemon") }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := DialControl(ctx, socket)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon did not listen")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Session 1: start the task, then disappear like a dropped SSH connection.
	session, err := DialControl(ctx, socket)
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.Write(session, protocol.Message{Type: protocol.MessageStartTask, RequestID: "1", WorkspaceID: "w", TaskID: "t", Payload: []byte(`[{"id":"a","role":"coder"}]`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := protocol.Read(session); err != nil {
		t.Fatal(err)
	}
	session.Close()
	// Session 2: a fresh connection observes the terminal state.
	deadline = time.Now().Add(3 * time.Second)
	for {
		task, err := s.GetTask(ctx, "t")
		if err != nil {
			t.Fatal(err)
		}
		if task.Status == domain.TaskCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not survive session close: %#v", task)
		}
		time.Sleep(20 * time.Millisecond)
	}
	daemonCancel()
	select {
	case <-daemonDone:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not shut down")
	}
}
