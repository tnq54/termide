package store

import (
	"context"
	"testing"

	"termide/internal/domain"
)

func TestApprovalResolveIsSingleWinner(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir() + "/state.db")
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
	req := domain.ApprovalRequest{ID: "ap-1", TaskID: "t", AgentID: "a", Command: "rm -rf build", Reason: "cleanup"}
	if err := s.CreateApproval(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateApproval(ctx, req); err == nil {
		t.Fatal("duplicate pending approval accepted")
	}
	pending, err := s.ListPendingApprovals(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Status != domain.ApprovalPending {
		t.Fatalf("pending = %#v", pending)
	}
	if err := s.ResolveApproval(ctx, "ap-1", true, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.ResolveApproval(ctx, "ap-1", false, "too late"); err == nil {
		t.Fatal("second resolution accepted")
	}
	got, err := s.GetApproval(ctx, "ap-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.ApprovalApproved {
		t.Fatalf("got = %#v", got)
	}
	if pending, err := s.ListPendingApprovals(ctx, "t"); err != nil || len(pending) != 0 {
		t.Fatalf("pending = %#v, err = %v", pending, err)
	}
}
