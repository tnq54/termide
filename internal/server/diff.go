package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"termide/internal/git"
	"termide/internal/protocol"
)

const maxDiffPatchBytes = 64 * 1024

func (h *Handler) taskDiff(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if message.WorkspaceID == "" || message.TaskID == "" {
		return protocol.Message{}, fmt.Errorf("workspace and task IDs are required")
	}
	workspace, err := h.store.GetWorkspace(ctx, message.WorkspaceID)
	if err != nil {
		return protocol.Message{}, err
	}
	task, err := h.store.GetTask(ctx, message.TaskID)
	if err != nil {
		return protocol.Message{}, err
	}
	if task.WorkspaceID != message.WorkspaceID {
		return protocol.Message{}, fmt.Errorf("task does not belong to workspace")
	}
	root, err := filepath.EvalSymlinks(workspace.Root)
	if err != nil {
		return protocol.Message{}, err
	}
	agents, err := h.store.ListAgents(ctx, message.TaskID)
	if err != nil {
		return protocol.Message{}, err
	}
	diffs := make([]protocol.TaskDiff, 0, len(agents))
	for _, agent := range agents {
		if agent.Worktree == "" {
			continue
		}
		diff := protocol.TaskDiff{AgentID: agent.ID, Branch: agent.Branch, Worktree: agent.Worktree, Files: []string{}}
		worktree, err := checkWorktreeInsideRoot(root, agent.Worktree)
		if err != nil {
			diff.Error = err.Error()
			diffs = append(diffs, diff)
			continue
		}
		manager := git.NewDiffManager(worktree)
		files, err := manager.ChangedFiles()
		if err != nil {
			diff.Error = err.Error()
			diffs = append(diffs, diff)
			continue
		}
		diff.Files = files
		patch, err := manager.Patch()
		if err != nil {
			diff.Error = err.Error()
			diffs = append(diffs, diff)
			continue
		}
		if len(patch) > maxDiffPatchBytes {
			patch = patch[:maxDiffPatchBytes]
			diff.Truncated = true
		}
		diff.Patch = patch
		diffs = append(diffs, diff)
	}
	payload, err := json.Marshal(diffs)
	if err != nil {
		return protocol.Message{}, err
	}
	return protocol.Message{Type: protocol.MessageTaskDiffs, WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, Payload: payload}, nil
}

func checkWorktreeInsideRoot(root, worktree string) (string, error) {
	if !filepath.IsAbs(worktree) {
		return "", fmt.Errorf("worktree path is not absolute")
	}
	resolved, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return "", err
	}
	if resolved != root && !strings.HasPrefix(resolved, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("worktree escapes workspace root")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("worktree is not a directory")
	}
	return resolved, nil
}
