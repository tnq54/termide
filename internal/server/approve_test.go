package server

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func approveFixture(t *testing.T) (*store.Store, *Handler, *Supervisor) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Description: "task", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	sup := NewSupervisor(s, &emittingRunner{text: "gated-output"}, "owner")
	return s, NewDurableHandler(s, sup), sup
}

func startDangerous(t *testing.T, h *Handler, ctx context.Context) {
	t.Helper()
	response, err := h.Handle(ctx, protocol.Message{
		Type: protocol.MessageStartTask, WorkspaceID: "w", TaskID: "t",
		Payload: []byte(`[{"id":"a","role":"coder","command":"rm -rf build"}]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageEvent {
		t.Fatal(response)
	}
}

func pendingApprovalID(t *testing.T, s *store.Store) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		pending, err := s.ListPendingApprovals(context.Background(), "t")
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) == 1 {
			return pending[0].ID
		}
		if time.Now().After(deadline) {
			t.Fatal("approval was not requested")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestApproveResumesGatedAgent(t *testing.T) {
	ctx := context.Background()
	s, h, sup := approveFixture(t)
	startDangerous(t, h, ctx)
	id := pendingApprovalID(t, s)
	payload, err := json.Marshal(protocol.ApproveDecision{ID: id, Approved: true})
	if err != nil {
		t.Fatal(err)
	}
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageApprove, WorkspaceID: "w", TaskID: "t", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageEvent || response.Event == nil || response.Event.Type != domain.EventApproval {
		t.Fatal(response)
	}
	if err := sup.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	task, err := s.GetTask(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskCompleted {
		t.Fatalf("task = %#v", task)
	}
}

func TestDenyFailsTaskWithoutRunningCommand(t *testing.T) {
	ctx := context.Background()
	s, h, sup := approveFixture(t)
	startDangerous(t, h, ctx)
	id := pendingApprovalID(t, s)
	payload, err := json.Marshal(protocol.ApproveDecision{ID: id, Approved: false, Reason: "too risky"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageApprove, WorkspaceID: "w", TaskID: "t", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if err := sup.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	task, err := s.GetTask(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskFailed {
		t.Fatalf("task = %#v", task)
	}
	events, err := s.EventsAfter(ctx, "w", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.TaskID == "t" && bytes.Contains(e.Payload, []byte("gated-output")) {
			t.Fatalf("denied agent ran the delegate: %#v", e)
		}
	}
}

func TestDuplicateApproveIsRejected(t *testing.T) {
	ctx := context.Background()
	s, h, _ := approveFixture(t)
	startDangerous(t, h, ctx)
	id := pendingApprovalID(t, s)
	payload, err := json.Marshal(protocol.ApproveDecision{ID: id, Approved: true})
	if err != nil {
		t.Fatal(err)
	}
	approve := protocol.Message{Type: protocol.MessageApprove, WorkspaceID: "w", TaskID: "t", Payload: payload}
	if _, err := h.Handle(ctx, approve); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Handle(ctx, approve); err == nil {
		t.Fatal("duplicate approval accepted")
	}
}

func TestCancelDeniesPendingApproval(t *testing.T) {
	ctx := context.Background()
	s, h, sup := approveFixture(t)
	startDangerous(t, h, ctx)
	pendingApprovalID(t, s)
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageCancel, WorkspaceID: "w", TaskID: "t"}); err != nil {
		t.Fatal(err)
	}
	if err := sup.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	pending, err := s.ListPendingApprovals(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending = %#v", pending)
	}
	task, err := s.GetTask(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskCancelled {
		t.Fatalf("task = %#v", task)
	}
}
