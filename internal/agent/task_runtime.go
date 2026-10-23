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

func (r *TaskRuntime) Start(ctx context.Context, taskID string, specs []Spec, runner Runner) error {
	task, err := r.store.GetTask(ctx, taskID); if err != nil { return err }
	if err := task.Transition(domain.TaskRunning); err != nil { return err }
	for _, spec := range specs { if err := r.store.UpsertAgent(ctx, domain.Agent{ID: spec.ID, TaskID: taskID, Role: spec.Role, Status: domain.AgentRunning}); err != nil { return err } }
	scheduler := NewScheduler(runner, len(specs))
	runErr := scheduler.Run(ctx, specs, func(event Event) {
		status := domain.AgentRunning
		if event.Kind == "completed" { status = domain.AgentCompleted }
		if event.Kind == "failed" { status = domain.AgentFailed }
		_ = r.store.UpsertAgent(ctx, domain.Agent{ID: event.AgentID, TaskID: taskID, Status: status})
	})
	if runErr != nil { _ = task.Transition(domain.TaskFailed) } else { _ = task.Transition(domain.TaskCompleted) }
	return r.store.UpdateTaskStatus(ctx, task.ID, task.Status)
}
