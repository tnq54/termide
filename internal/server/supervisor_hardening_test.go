package server

import (
	"context"
	"testing"
	"time"

	"termide/internal/agent"
	"termide/internal/domain"
	"termide/internal/store"
)

func mustWorkspaceTask(t *testing.T, s *store.Store, workspaceID, taskID, otherWorkspaceID string) {
	t.Helper()
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: workspaceID}); err != nil {
		t.Fatal(err)
	}
	if otherWorkspaceID != "" {
		if err := s.CreateWorkspace(ctx, domain.Workspace{ID: otherWorkspaceID}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateTask(ctx, domain.Task{ID: taskID, WorkspaceID: workspaceID, Description: "task", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
}

func TestSupervisorStartRejectsForeignWorkspace(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mustWorkspaceTask(t, s, "w", "t", "other")
	sup := NewSupervisor(s, &durableRunner{started: make(chan struct{}), done: make(chan struct{})}, "owner")
	if _, err := sup.Start(ctx, "other", "t", []agent.Spec{{ID: "a", Role: "coder"}}); err == nil {
		t.Fatal("cross-workspace start accepted")
	}
	task, err := s.GetTask(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskQueued {
		t.Fatalf("task mutated: %#v", task)
	}
}

type emittingRunner struct{ text string }

func (r *emittingRunner) Run(_ context.Context, _ agent.Spec, emit func(agent.Event)) error {
	emit(agent.Event{Output: r.text})
	return nil
}

func TestSupervisorPersistsOutputEvents(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mustWorkspaceTask(t, s, "w", "t", "")
	sup := NewSupervisor(s, &emittingRunner{text: "hello-agent"}, "owner")
	if _, err := sup.Start(ctx, "w", "t", []agent.Spec{{ID: "a", Role: "coder"}}); err != nil {
		t.Fatal(err)
	}
	if err := sup.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	events, err := s.EventsAfter(ctx, "w", 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.TaskID == "t" && e.AgentID == "a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("agent output event missing: %#v", events)
	}
}

func TestSupervisorStartRejectsDuplicateAndForeignAgentIDs(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mustWorkspaceTask(t, s, "w", "t1", "")
	if err := s.CreateTask(ctx, domain.Task{ID: "t2", WorkspaceID: "w", Description: "task", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	sup := NewSupervisor(s, &emittingRunner{text: "x"}, "owner")
	if _, err := sup.Start(ctx, "w", "t1", []agent.Spec{{ID: "a", Role: "coder"}, {ID: "a", Role: "tester"}}); err == nil {
		t.Fatal("duplicate agent IDs accepted")
	}
	if _, err := s.GetTaskRun(ctx, "t1"); err == nil {
		t.Fatal("failed start left a run record")
	}
	if _, err := sup.Start(ctx, "w", "t1", []agent.Spec{{ID: "shared", Role: "coder"}}); err != nil {
		t.Fatal(err)
	}
	if err := sup.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := sup.Start(ctx, "w", "t2", []agent.Spec{{ID: "shared", Role: "coder"}}); err == nil {
		t.Fatal("agent ID owned by another task accepted")
	}
	task, err := s.GetTask(ctx, "t2")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskQueued {
		t.Fatalf("rejected start mutated task: %#v", task)
	}
}

func TestSupervisorRecoversExpiredRun(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mustWorkspaceTask(t, s, "w", "t", "")
	runner := &durableRunner{started: make(chan struct{}), done: make(chan struct{})}
	old := NewSupervisor(s, runner, "old-owner")
	if _, err := old.Start(ctx, "w", "t", []agent.Spec{{ID: "a", Role: "coder"}}); err != nil {
		t.Fatal(err)
	}
	// Simulate a crashed owner: expire its lease while the run is in-flight.
	// The old supervisor is abandoned without Wait/Close, like a killed process.
	run, err := s.GetTaskRun(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenewRunLease(ctx, run.RunID, "old-owner", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	// While the expired run is unrecovered, a concurrent Start must be rejected:
	// the task never returns to queued.
	if _, err := NewSupervisor(s, &emittingRunner{text: "x"}, "racing").Start(ctx, "w", "t", []agent.Spec{{ID: "b", Role: "coder"}}); err == nil {
		t.Fatal("start during expired lease accepted")
	}
	next := NewSupervisor(s, &emittingRunner{text: "recovered"}, "new-owner")
	recovered, err := next.Recover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 {
		t.Fatalf("recovered = %#v", recovered)
	}
	if err := next.Wait(ctx); err != nil {
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
