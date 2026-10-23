package agent

import (
	"context"
	"fmt"

	"termide/internal/domain"
	"termide/internal/store"
)

type TaskRuntime struct { store *store.Store }
func NewTaskRuntime(s *store.Store) *TaskRuntime { return &TaskRuntime{store: s} }
func (r *TaskRuntime) Create(ctx context.Context, task domain.Task, specs []Spec) error {
	if task.ID == "" || task.WorkspaceID == "" { return fmt.Errorf("task identity is required") }
	if err := r.store.CreateTask(ctx, task); err != nil { return err }
	for _, spec := range specs {
		agent := domain.Agent{ID: spec.ID, TaskID: task.ID, Role: spec.Role, Status: domain.AgentQueued}
		if err := r.store.UpsertAgent(ctx, agent); err != nil { return err }
	}
	return nil
}
