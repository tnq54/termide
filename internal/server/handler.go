package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"termide/internal/agent"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

type Handler struct {
	store     *store.Store
	runtime   *agent.TaskRuntime
	runner    agent.Runner
	isolate   bool
	jobsMu    sync.Mutex
	jobs      sync.WaitGroup
	cancels   map[string]context.CancelFunc
	closed    bool
	jobErrors []error
}

// Close cancels owned jobs and waits for final statuses before the store closes.
func (h *Handler) Close() error {
	h.jobsMu.Lock()
	h.closed = true
	for _, cancel := range h.cancels {
		cancel()
	}
	h.jobsMu.Unlock()
	h.jobs.Wait()
	h.jobsMu.Lock()
	defer h.jobsMu.Unlock()
	return errors.Join(h.jobErrors...)
}

func NewHandler(s *store.Store) *Handler { return &Handler{store: s, runtime: agent.NewTaskRuntime(s)} }

func NewHandlerWithRunner(s *store.Store, runner agent.Runner) *Handler {
	return &Handler{store: s, runtime: agent.NewTaskRuntime(s), runner: runner}
}

func NewIsolatedHandler(s *store.Store, runner agent.Runner) *Handler {
	return &Handler{store: s, runtime: agent.NewTaskRuntime(s), runner: runner, isolate: true}
}
func (h *Handler) HandleWithAgents(ctx context.Context, message protocol.Message, specs []agent.Spec, runner agent.Runner) (protocol.Message, error) {
	if message.Type != protocol.MessageStartTask {
		return protocol.Message{}, fmt.Errorf("unsupported task action %q", message.Type)
	}
	if err := h.runtime.Start(ctx, message.TaskID, specs, runner); err != nil {
		return protocol.Message{}, err
	}
	event := domain.EventEnvelope{WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, AgentID: "system", Sequence: 1, Type: domain.EventTaskUpdated, Payload: []byte("completed")}
	return protocol.Message{Type: protocol.MessageEvent, Event: &event}, nil
}
func (h *Handler) Handle(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	switch message.Type {
	case protocol.MessageListTasks, protocol.MessageTaskSnapshot:
		return h.handleDashboard(ctx, message)
	case protocol.MessageListFiles, protocol.MessageReadFile, protocol.MessageSaveFile:
		return h.handleFiles(ctx, message)
	case protocol.MessageOpenWorkspace:
		return h.openWorkspace(ctx, message)
	case protocol.MessageStartTask:
		return h.startTask(ctx, message)
	case protocol.MessageCancel:
		return h.cancelTask(ctx, message)
	case protocol.MessageCreateTask:
		if message.WorkspaceID == "" || message.TaskID == "" {
			return protocol.Message{}, fmt.Errorf("workspace and task IDs are required")
		}
		task := domain.Task{ID: message.TaskID, WorkspaceID: message.WorkspaceID, Description: string(message.Payload), Status: domain.TaskQueued}
		if err := h.store.CreateTask(ctx, task); err != nil {
			return protocol.Message{}, err
		}
		event := domain.EventEnvelope{WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, AgentID: "system", Sequence: 1, Type: domain.EventTaskUpdated, Payload: message.Payload}
		return protocol.Message{Type: protocol.MessageEvent, Event: &event}, nil
	default:
		return protocol.Message{}, fmt.Errorf("unsupported message type %q", message.Type)
	}
}
