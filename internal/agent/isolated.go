package agent

import (
	"context"
	"fmt"
	"termide/internal/git"
	"termide/internal/store"
)

type IsolatedRunner struct {
	store  *store.Store
	taskID string
	runner Runner
}

func NewIsolatedRunner(s *store.Store, taskID string, runner Runner) *IsolatedRunner {
	return &IsolatedRunner{store: s, taskID: taskID, runner: runner}
}
func (r *IsolatedRunner) Run(ctx context.Context, spec Spec, emit func(Event)) error {
	task, err := r.store.GetTask(ctx, r.taskID)
	if err != nil {
		return err
	}
	workspace, err := r.store.GetWorkspace(ctx, task.WorkspaceID)
	if err != nil {
		return err
	}
	path, err := git.NewWorktreeManager(workspace.Root).CreateContext(ctx, task.ID, spec.ID, "HEAD")
	if err != nil {
		return err
	}
	branch := fmt.Sprintf("termide/%s/%s", task.ID, spec.ID)
	if err := r.store.UpdateAgentWorkspace(ctx, task.ID, spec.ID, branch, path); err != nil {
		return err
	}
	spec.WorkingDir = path
	return r.runner.Run(ctx, spec, emit)
}
