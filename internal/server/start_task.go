package server

import (
	"context"
	"encoding/json"
	"fmt"

	"termide/internal/agent"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func (h *Handler) startTask(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if message.WorkspaceID == "" || message.TaskID == "" {
		return protocol.Message{}, fmt.Errorf("workspace and task IDs are required")
	}
	if h.supervisor != nil {
		return h.startDurableTask(ctx, message)
	}
	if h.runner == nil {
		return protocol.Message{}, fmt.Errorf("task runner is not configured")
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
	var specs []agent.Spec
	if err := json.Unmarshal(message.Payload, &specs); err != nil {
		return protocol.Message{}, fmt.Errorf("invalid agent plan: %w", err)
	}
	if len(specs) == 0 {
		return protocol.Message{}, fmt.Errorf("agent plan must not be empty")
	}
	for i := range specs {
		if specs[i].Command == "" {
			specs[i].Command = task.Description
		}
	}
	runner := h.runner
	if h.isolate {
		runner = agent.NewIsolatedRunner(h.store, message.TaskID, runner)
	}
	runner = agent.NewGateRunner(h.store, runner, message.TaskID, workspace.Root, gatePollInterval)
	h.jobsMu.Lock()
	if h.closed {
		h.jobsMu.Unlock()
		return protocol.Message{}, fmt.Errorf("task supervisor is closed")
	}
	run, err := h.runtime.Prepare(ctx, message.TaskID, specs, runner)
	if err != nil {
		h.jobsMu.Unlock()
		return protocol.Message{}, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	if h.cancels == nil {
		h.cancels = make(map[string]context.CancelFunc)
	}
	h.cancels[message.TaskID] = cancel
	h.jobs.Add(1)
	h.jobsMu.Unlock()
	go func() {
		defer h.jobs.Done()
		defer cancel()
		// Runtime persists completion, failure or cancellation before returning.
		err := run(runCtx)
		h.jobsMu.Lock()
		if err != nil {
			h.jobErrors = append(h.jobErrors, fmt.Errorf("task %s: %w", message.TaskID, err))
		}
		delete(h.cancels, message.TaskID)
		h.jobsMu.Unlock()
	}()
	event := domain.EventEnvelope{
		WorkspaceID: message.WorkspaceID,
		TaskID:      message.TaskID,
		AgentID:     "system",
		Sequence:    1,
		Type:        domain.EventTaskUpdated,
		Payload:     []byte("running"),
	}
	return protocol.Message{Type: protocol.MessageEvent, WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, Event: &event}, nil
}
