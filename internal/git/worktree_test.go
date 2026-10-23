package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorktreeManagerCreatesIsolatedWorktree(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) { c := exec.Command("git", args...); c.Dir = root; if out, err := c.CombinedOutput(); err != nil { t.Fatalf("git %v: %v\n%s", args, err, out) } }
	run("init", "-q"); run("config", "user.email", "test@example.com"); run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("base\n"), 0o600); err != nil { t.Fatal(err) }
	run("add", "."); run("commit", "-qm", "base")
	manager := NewWorktreeManager(root)
	path, err := manager.Create("task-1", "agent-1", "HEAD")
	if err != nil { t.Fatal(err) }
	if path == root { t.Fatal("worktree path must differ from repository root") }
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil { t.Fatalf("worktree missing: %v", err) }
}
