package git

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

type WorktreeManager struct{ root, base string }

func NewWorktreeManager(root string) *WorktreeManager {
	abs, err := filepath.Abs(root)
	if err == nil {
		root = abs
	}
	namespace := fmt.Sprintf("%x", sha256.Sum256([]byte(root)))
	return &WorktreeManager{root: root, base: filepath.Join(filepath.Dir(root), ".termide-worktrees", namespace)}
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func (m *WorktreeManager) Create(taskID, agentID, baseRef string) (string, error) {
	return m.CreateContext(context.Background(), taskID, agentID, baseRef)
}
func (m *WorktreeManager) CreateContext(ctx context.Context, taskID, agentID, baseRef string) (string, error) {
	if !identifier.MatchString(taskID) || !identifier.MatchString(agentID) {
		return "", fmt.Errorf("invalid identifier")
	}
	path := filepath.Join(m.base, taskID, agentID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	branch := fmt.Sprintf("termide/%s/%s", taskID, agentID)
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, "--", path, baseRef)
	cmd.Dir = m.root
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %w: %s", err, output)
	}
	return path, nil
}
