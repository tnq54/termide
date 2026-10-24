package agent

import (
	"context"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/store"
)

type recordingRunner struct {
	calls chan Spec
	err   error
}

func (r *recordingRunner) Run(_ context.Context, spec Spec, _ func(Event)) error {
	r.calls <- spec
	return r.err
}

func gateFixture(t *testing.T) (*store.Store, *recordingRunner) {
	t.Helper()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertAgent(ctx, domain.Agent{ID: "a", TaskID: "t", Role: "coder", Status: domain.AgentRunning}); err != nil {
		t.Fatal(err)
	}
	return s, &recordingRunner{calls: make(chan Spec, 4)}
}

func TestGateRunnerRunsSafeCommandImmediately(t *testing.T) {
	s, delegate := gateFixture(t)
	gate := NewGateRunner(s, delegate, "t", "/root", 10*time.Millisecond)
	if err := gate.Run(context.Background(), Spec{ID: "a", Command: "git status"}, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	select {
	case spec := <-delegate.calls:
		if spec.ID != "a" {
			t.Fatal(spec)
		}
	default:
		t.Fatal("delegate was not called")
	}
	if pending, err := s.ListPendingApprovals(context.Background(), "t"); err != nil || len(pending) != 0 {
		t.Fatalf("pending = %#v, err = %v", pending, err)
	}
}

func TestGateRunnerRejectsBlockedCommand(t *testing.T) {
	s, delegate := gateFixture(t)
	gate := NewGateRunner(s, delegate, "t", "/root", 10*time.Millisecond)
	if err := gate.Run(context.Background(), Spec{ID: "a", Command: ""}, func(Event) {}); err == nil {
		t.Fatal("blocked command accepted")
	}
	select {
	case spec := <-delegate.calls:
		t.Fatalf("delegate ran blocked command: %#v", spec)
	default:
	}
	if pending, err := s.ListPendingApprovals(context.Background(), "t"); err != nil || len(pending) != 0 {
		t.Fatalf("pending = %#v, err = %v", pending, err)
	}
}

func TestGateRunnerWaitsForApprovalThenRuns(t *testing.T) {
	s, delegate := gateFixture(t)
	gate := NewGateRunner(s, delegate, "t", "/root", 10*time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- gate.Run(context.Background(), Spec{ID: "a", Command: "rm -rf build"}, func(Event) {}) }()
	deadline := time.Now().Add(2 * time.Second)
	var pending []domain.ApprovalRequest
	for {
		var err error
		pending, err = s.ListPendingApprovals(context.Background(), "t")
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("approval was not requested")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case spec := <-delegate.calls:
		t.Fatalf("delegate ran before approval: %#v", spec)
	default:
	}
	if err := s.ResolveApproval(context.Background(), pending[0].ID, true, "ok"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("gate did not resume after approval")
	}
	select {
	case <-delegate.calls:
	default:
		t.Fatal("delegate was not called after approval")
	}
}

func TestGateRunnerDeniesWithoutRunning(t *testing.T) {
	s, delegate := gateFixture(t)
	gate := NewGateRunner(s, delegate, "t", "/root", 10*time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- gate.Run(context.Background(), Spec{ID: "a", Command: "sudo reboot"}, func(Event) {}) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		pending, err := s.ListPendingApprovals(context.Background(), "t")
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) == 1 {
			if err := s.ResolveApproval(context.Background(), pending[0].ID, false, "no"); err != nil {
				t.Fatal(err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("approval was not requested")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("denied command reported success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("gate did not return after denial")
	}
	select {
	case spec := <-delegate.calls:
		t.Fatalf("delegate ran denied command: %#v", spec)
	default:
	}
}
