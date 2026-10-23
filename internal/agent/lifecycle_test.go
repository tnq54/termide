package agent

import (
	"context"
	"errors"
	"termide/internal/domain"
	"termide/internal/store"
	"testing"
)

type runnerFunc func(context.Context, Spec, func(Event)) error

func (f runnerFunc) Run(ctx context.Context, s Spec, emit func(Event)) error { return f(ctx, s, emit) }

func runtimeFixture(t *testing.T) (*store.Store, *TaskRuntime) {
	t.Helper()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	r := NewTaskRuntime(s)
	if err := r.Create(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskQueued}, nil); err != nil {
		t.Fatal(err)
	}
	return s, r
}

func TestRuntimePropagatesFailureAndPersistsTerminalState(t *testing.T) {
	s, r := runtimeFixture(t)
	failure := errors.New("runner failed")
	err := r.Start(context.Background(), "t", []Spec{{ID: "a", Role: "coder"}}, runnerFunc(func(context.Context, Spec, func(Event)) error { return failure }))
	if !errors.Is(err, failure) {
		t.Fatalf("lost runner error: %v", err)
	}
	task, _ := s.GetTask(context.Background(), "t")
	agents, _ := s.ListAgents(context.Background(), "t")
	if task.Status != domain.TaskFailed || len(agents) != 1 || agents[0].Status != domain.AgentFailed {
		t.Fatalf("task=%+v agents=%+v", task, agents)
	}
}

func TestRuntimeTracksRunningAndCompletionWithoutRunnerEvents(t *testing.T) {
	s, r := runtimeFixture(t)
	runner := runnerFunc(func(ctx context.Context, _ Spec, emit func(Event)) error {
		task, err := s.GetTask(ctx, "t")
		if err != nil {
			return err
		}
		if task.Status != domain.TaskRunning {
			return errors.New("task not persisted as running")
		}
		emit(Event{AgentID: "spoofed", Kind: "output", Output: "hello"})
		return nil
	})
	if err := r.Start(context.Background(), "t", []Spec{{ID: "a", Role: "coder"}}, runner); err != nil {
		t.Fatal(err)
	}
	agents, _ := s.ListAgents(context.Background(), "t")
	if len(agents) != 1 || agents[0].Status != domain.AgentCompleted {
		t.Fatal(agents)
	}
	events, err := s.EventsAfter(context.Background(), "w", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("output not journaled")
	}
	for _, event := range events {
		if event.AgentID != "a" {
			t.Fatalf("untrusted agent ID persisted: %s", event.AgentID)
		}
	}
}

func TestRuntimePersistsCancellation(t *testing.T) {
	s, r := runtimeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := r.Start(ctx, "t", []Spec{{ID: "a", Role: "coder"}}, runnerFunc(func(ctx context.Context, _ Spec, _ func(Event)) error { cancel(); return ctx.Err() }))
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	task, _ := s.GetTask(context.Background(), "t")
	agents, _ := s.ListAgents(context.Background(), "t")
	if task.Status != domain.TaskCancelled || agents[0].Status != domain.AgentCancelled {
		t.Fatalf("task=%+v agents=%+v", task, agents)
	}
}

func TestRuntimeRejectsEmptyPlan(t *testing.T) {
	_, r := runtimeFixture(t)
	if err := r.Start(context.Background(), "t", nil, immediateRunner{}); err == nil {
		t.Fatal("empty plan completed")
	}
}

func TestRuntimeRejectsDuplicateStartWhileRunning(t *testing.T) {
	s, r := runtimeFixture(t)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	specs := []Spec{{ID: "a", Role: "coder"}}
	runner := runnerFunc(func(ctx context.Context, _ Spec, _ func(Event)) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { done <- r.Start(ctx, "t", specs, runner) }()
	<-started
	err := r.Start(context.Background(), "t", specs, immediateRunner{})
	close(release)
	if err == nil {
		t.Error("duplicate start accepted")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	task, err := s.GetTask(context.Background(), "t")
	if err != nil || task.Status != domain.TaskCompleted {
		t.Fatalf("%+v %v", task, err)
	}
}
