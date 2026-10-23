package git

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

type WorktreeManager struct { root, base string }
func NewWorktreeManager(root string) *WorktreeManager { return &WorktreeManager{root: root, base: filepath.Join(filepath.Dir(root), ".termide-worktrees")} }
func (m *WorktreeManager) Create(taskID, agentID, baseRef string) (string, error) {
	path := filepath.Join(m.base, taskID, agentID)
	if err := exec.Command("mkdir", "-p", filepath.Dir(path)).Run(); err != nil { return "", err }
	branch := fmt.Sprintf("termide/%s/%s", taskID, agentID)
	cmd := exec.CommandContext(context.Background(), "git", "worktree", "add", "-b", branch, path, baseRef)
	cmd.Dir = m.root
	if output, err := cmd.CombinedOutput(); err != nil { return "", fmt.Errorf("git worktree add: %w: %s", err, output) }
	return path, nil
}
