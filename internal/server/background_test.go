package server

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"termide/internal/agent"
	"termide/internal/client"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
	"testing"
	"time"
)

type waitingRunner struct {
	entered chan struct{}
	exited  chan struct{}
}

type closeStoreRunner struct{ s *store.Store }

func (r closeStoreRunner) Run(context.Context, agent.Spec, func(agent.Event)) error {
	return r.s.Close()
}

func TestBackgroundFinalizationErrorIsReportedOnClose(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	s.CreateWorkspace(ctx, domain.Workspace{ID: "w"})
	s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskQueued})
	h := NewHandlerWithRunner(s, closeStoreRunner{s})
	payload, _ := json.Marshal([]agent.Spec{{ID: "a"}})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageStartTask, WorkspaceID: "w", TaskID: "t", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	h.jobs.Wait()
	if err := h.Close(); err == nil {
		t.Fatal("finalization failure was lost")
	}
}

func (r waitingRunner) Run(ctx context.Context, _ agent.Spec, _ func(agent.Event)) error {
	close(r.entered)
	defer close(r.exited)
	<-ctx.Done()
	return ctx.Err()
}

func TestBackgroundStartLeavesTransportUsableAndCancelsOnDisconnect(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	runner := waitingRunner{make(chan struct{}), make(chan struct{})}
	h := NewHandlerWithRunner(s, runner)
	left, right := net.Pipe()
	remote := client.NewRemote(left)
	serverCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { defer right.Close(); done <- h.Serve(serverCtx, right, right) }()
	t.Cleanup(func() {
		cancel()
		remote.Close()
		select {
		case <-done:
			task, err := s.GetTask(context.Background(), "t")
			if err != nil || task.Status != domain.TaskCancelled {
				t.Errorf("final task: %+v, %v", task, err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})
	callCtx, stop := context.WithTimeout(ctx, time.Second)
	defer stop()
	payload, _ := json.Marshal([]agent.Spec{{ID: "a", Role: "coder"}})
	request := protocol.Message{Type: protocol.MessageStartTask, WorkspaceID: "w", TaskID: "t", Payload: payload}
	if _, err := remote.Call(callCtx, request); err != nil {
		t.Fatalf("start blocked behind runner: %v", err)
	}
	select {
	case <-runner.entered:
	case <-time.After(time.Second):
		t.Fatal("runner did not enter")
	}
	if _, err := remote.Call(callCtx, request); err == nil {
		t.Fatal("duplicate start accepted")
	}
	response, err := remote.Call(callCtx, protocol.Message{Type: protocol.MessageTaskSnapshot, WorkspaceID: "w", TaskID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot domain.TaskSnapshot
	if err := json.Unmarshal(response.Payload, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Task.Status != domain.TaskRunning {
		t.Fatal(snapshot.Task.Status)
	}
	filePayload, _ := json.Marshal(protocol.FileRequest{Path: "main.py"})
	response, err = remote.Call(callCtx, protocol.Message{Type: protocol.MessageReadFile, WorkspaceID: "w", Payload: filePayload})
	if err != nil || response.Type != protocol.MessageFileResult {
		t.Fatalf("editor unavailable during execution: %v", err)
	}
	remote.Close()
	select {
	case <-runner.exited:
	case <-time.After(time.Second):
		t.Fatal("disconnect did not cancel runner")
	}
}
