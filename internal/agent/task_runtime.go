package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"termide/internal/domain"
	"termide/internal/store"
)

type TaskRuntime struct{ store *store.Store }

func NewTaskRuntime(s *store.Store) *TaskRuntime { return &TaskRuntime{store: s} }
func (r *TaskRuntime) Create(ctx context.Context, task domain.Task, specs []Spec) error {
	if task.ID == "" || task.WorkspaceID == "" {
		return fmt.Errorf("task identity is required")
	}
	if err := r.store.CreateTask(ctx, task); err != nil {
		return err
	}
	for _, spec := range specs {
		agent := domain.Agent{ID: spec.ID, TaskID: task.ID, Role: spec.Role, Status: domain.AgentQueued}
		if err := r.store.UpsertAgent(ctx, agent); err != nil {
			return err
		}
	}
	return nil
}

func (r *TaskRuntime) Start(ctx context.Context, taskID string, specs []Spec, runner Runner) error {
	if runner == nil || len(specs) == 0 {
		return fmt.Errorf("runner and nonempty plan required")
	}
	agents := make([]domain.Agent, 0, len(specs))
	seen := map[string]bool{}
	for _, spec := range specs {
		if spec.ID == "" || seen[spec.ID] {
			return fmt.Errorf("agent IDs must be nonempty and unique")
		}
		seen[spec.ID] = true
		agents = append(agents, domain.Agent{ID: spec.ID, TaskID: taskID, Role: spec.Role})
	}
	task, err := r.store.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if err := r.store.BeginTask(ctx, taskID, agents); err != nil {
		return err
	}
	wrapped := &persistedRunner{store: r.store, runner: runner, task: task}
	runErr := NewScheduler(wrapped, 4).Run(ctx, specs, func(Event) {})
	status := domain.TaskCompleted
	if runErr != nil {
		status = domain.TaskFailed
	}
	if ctx.Err() != nil {
		status = domain.TaskCancelled
		runErr = errors.Join(runErr, ctx.Err())
	}
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if ctx.Err() != nil {
		runErr = errors.Join(runErr, r.store.CancelQueuedAgents(finalCtx, taskID))
	}
	return errors.Join(runErr, r.store.UpdateTaskStatus(finalCtx, taskID, status))
}

type persistedRunner struct {
	store  *store.Store
	runner Runner
	task   domain.Task
}

func (r *persistedRunner) Run(ctx context.Context, spec Spec, _ func(Event)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.store.UpdateAgentStatus(ctx, r.task.ID, spec.ID, domain.AgentRunning); err != nil {
		return err
	}
	var mu sync.Mutex
	var eventErr error
	err := r.runner.Run(ctx, spec, func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		if eventErr != nil {
			return
		}
		event.AgentID = spec.ID
		payload, encodeErr := json.Marshal(event)
		if encodeErr != nil {
			eventErr = encodeErr
			return
		}
		eventErr = r.store.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: r.task.WorkspaceID, TaskID: r.task.ID, AgentID: spec.ID, Type: domain.EventOutput, Payload: payload})
	})
	mu.Lock()
	err = errors.Join(err, eventErr)
	mu.Unlock()
	status := domain.AgentCompleted
	if err != nil {
		status = domain.AgentFailed
	}
	if ctx.Err() != nil {
		status = domain.AgentCancelled
		err = errors.Join(err, ctx.Err())
	}
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return errors.Join(err, r.store.UpdateAgentStatus(finalCtx, r.task.ID, spec.ID, status))
}
