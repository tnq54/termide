package server

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func diffFixture(t *testing.T) (*store.Store, *Handler, string) {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test")
	run("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("print('review me')\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertAgent(ctx, domain.Agent{ID: "a", TaskID: "t", Role: "coder", Status: domain.AgentCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAgentWorkspace(ctx, "t", "a", "termide/t/a", root); err != nil {
		t.Fatal(err)
	}
	return s, NewHandler(s), root
}

func requestDiff(t *testing.T, h *Handler, workspace, task string) []protocol.TaskDiff {
	t.Helper()
	response, err := h.Handle(context.Background(), protocol.Message{Type: protocol.MessageTaskDiff, WorkspaceID: workspace, TaskID: task})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageTaskDiffs || response.WorkspaceID != workspace || response.TaskID != task {
		t.Fatalf("response = %#v", response)
	}
	var diffs []protocol.TaskDiff
	if err := json.Unmarshal(response.Payload, &diffs); err != nil {
		t.Fatal(err)
	}
	return diffs
}

func TestTaskDiffShowsWorktreePatch(t *testing.T) {
	_, h, _ := diffFixture(t)
	diffs := requestDiff(t, h, "w", "t")
	if len(diffs) != 1 {
		t.Fatalf("diffs = %#v", diffs)
	}
	if diffs[0].AgentID != "a" || len(diffs[0].Files) != 1 || diffs[0].Files[0] != "main.py" {
		t.Fatalf("diff = %#v", diffs[0])
	}
	if !strings.Contains(diffs[0].Patch, "review me") {
		t.Fatalf("patch = %q", diffs[0].Patch)
	}
}

func TestTaskDiffRejectsForeignTask(t *testing.T) {
	s, h, other := diffFixture(t)
	ctx := context.Background()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "other", Root: other}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "x", WorkspaceID: "other", Status: domain.TaskCompleted}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageTaskDiff, WorkspaceID: "w", TaskID: "x"}); err == nil {
		t.Fatal("foreign task accepted")
	}
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageTaskDiff, WorkspaceID: "w", TaskID: "missing"}); err == nil {
		t.Fatal("missing task accepted")
	}
}

func TestTaskDiffSkipsAgentsWithoutWorktree(t *testing.T) {
	s, h, _ := diffFixture(t)
	ctx := context.Background()
	if err := s.UpsertAgent(ctx, domain.Agent{ID: "b", TaskID: "t", Role: "tester", Status: domain.AgentCompleted}); err != nil {
		t.Fatal(err)
	}
	diffs := requestDiff(t, h, "w", "t")
	if len(diffs) != 1 || diffs[0].AgentID != "a" {
		t.Fatalf("diffs = %#v", diffs)
	}
}
