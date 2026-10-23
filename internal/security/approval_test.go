package security

import (
	"testing"

	"termide/internal/domain"
)

func TestApprovalManagerPausesAndResumesRequest(t *testing.T) {
	m := NewApprovalManager()
	r := domain.ApprovalRequest{ID: "ap-1", TaskID: "task-1", AgentID: "agent-1", Command: "rm -rf build"}
	if err := m.Request(r); err != nil { t.Fatal(err) }
	pending, ok := m.Pending("ap-1")
	if !ok || pending.Command != r.Command { t.Fatalf("pending = %#v, ok=%v", pending, ok) }
	if err := m.ApproveOnce("ap-1"); err != nil { t.Fatal(err) }
	if _, ok := m.Pending("ap-1"); ok { t.Fatal("approved request should no longer be pending") }
}

func TestApprovalManagerRejectsUnknownRequest(t *testing.T) {
	if err := NewApprovalManager().ApproveOnce("missing"); err == nil { t.Fatal("expected unknown approval error") }
}
