package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"termide/internal/domain"
	"termide/internal/store"
	"testing"
)

func TestRuntimeRunsAgentsInSeparateWorktrees(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	git("init", "-q")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "base")
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: root}); err != nil {
		t.Fatal(err)
	}
	runtime := NewTaskRuntime(s)
	specs := []Spec{{ID: "a", Role: "coder"}, {ID: "b", Role: "tester"}}
	if err := runtime.Create(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskQueued}, specs); err != nil {
		t.Fatal(err)
	}
	writer := runnerFunc(func(_ context.Context, spec Spec, _ func(Event)) error {
		return os.WriteFile(filepath.Join(spec.WorkingDir, "result.txt"), []byte(spec.ID), 0600)
	})
	isolated := NewIsolatedRunner(s, "t", writer)
	if err := runtime.Start(ctx, "t", specs, isolated); err != nil {
		t.Fatal(err)
	}
	agents, err := s.ListAgents(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 || agents[0].Worktree == agents[1].Worktree {
		t.Fatal(agents)
	}
	for _, a := range agents {
		data, err := os.ReadFile(filepath.Join(a.Worktree, "result.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != a.ID || a.Status != domain.AgentCompleted || a.Role == "" || a.Branch == "" {
			t.Fatal(a)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "result.txt")); !os.IsNotExist(err) {
		t.Fatal("main workspace modified")
	}
}
