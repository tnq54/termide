package agent

import (
	"context"
	"testing"

	"termide/internal/domain"
	"termide/internal/store"
)

type immediateRunner struct{}
func (immediateRunner) Run(_ context.Context, spec Spec, emit func(Event)) error {
	emit(Event{AgentID: spec.ID, Kind: "started"})
	emit(Event{AgentID: spec.ID, Kind: "completed"})
	return nil
}

func TestTaskRuntimePersistsTaskAndAgents(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil { t.Fatal(err) }
	defer s.Close()
	if err := s.CreateWorkspace(context.Background(), domain.Workspace{ID: "w1", Name: "demo", Host: "local", Root: "/tmp/demo"}); err != nil { t.Fatal(err) }
	runtime := NewTaskRuntime(s)
	if err := runtime.Create(context.Background(), domain.Task{ID: "t1", WorkspaceID: "w1", Description: "test", Status: domain.TaskQueued}, []Spec{{ID: "a1", Role: "coder"}, {ID: "a2", Role: "tester"}}); err != nil { t.Fatal(err) }
	agents, err := s.ListAgents(context.Background(), "t1")
	if err != nil { t.Fatal(err) }
	if len(agents) != 2 || agents[0].ID != "a1" { t.Fatalf("agents = %#v", agents) }
}

func TestTaskRuntimeRunsAndPersistsCompletion(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil { t.Fatal(err) }
	defer s.Close()
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w1", Name: "demo", Host: "local", Root: "/tmp/demo"}); err != nil { t.Fatal(err) }
	runtime := NewTaskRuntime(s)
	task := domain.Task{ID: "t1", WorkspaceID: "w1", Description: "test", Status: domain.TaskQueued}
	if err := runtime.Create(ctx, task, []Spec{{ID: "a1", Role: "coder"}}); err != nil { t.Fatal(err) }
	if err := runtime.Start(ctx, task.ID, []Spec{{ID: "a1", Role: "coder"}}, immediateRunner{}); err != nil { t.Fatal(err) }
	got, err := s.GetTask(ctx, task.ID); if err != nil { t.Fatal(err) }
	if got.Status != domain.TaskCompleted { t.Fatalf("status = %q", got.Status) }
	agents, err := s.ListAgents(ctx, task.ID); if err != nil { t.Fatal(err) }
	if len(agents) != 1 || agents[0].Status != domain.AgentCompleted { t.Fatalf("agents = %#v", agents) }
	if agents[0].Role != "coder" { t.Fatalf("agent role was lost: %#v", agents[0]) }
}
