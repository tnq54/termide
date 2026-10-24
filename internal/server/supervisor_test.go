package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"termide/internal/agent"
	"termide/internal/domain"
	"termide/internal/store"
)

type durableRunner struct {
	started chan struct{}
	done    chan struct{}
	once    sync.Once
}

func (r *durableRunner) Run(ctx context.Context, _ agent.Spec, _ func(agent.Event)) error {
	r.once.Do(func() { close(r.started) })
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestSupervisorOutlivesRequestContext(t *testing.T) {
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
	runner := &durableRunner{started: make(chan struct{}), done: make(chan struct{})}
	sup := NewSupervisor(s, runner, "owner")
	requestCtx, cancel := context.WithCancel(ctx)
	if _, err := sup.Start(requestCtx, "w", "t", []agent.Spec{{ID: "a", Role: "coder", Command: "git status"}}); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	close(runner.done)
	if err := sup.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	task, err := s.GetTask(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskCompleted {
		t.Fatal(task)
	}
	agents, err := s.ListAgents(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 || agents[0].Status != domain.AgentCompleted {
		t.Fatalf("agents = %#v", agents)
	}
}
