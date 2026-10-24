package server

import (
	"context"
	"encoding/json"
	"fmt"

	"termide/internal/agent"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func NewDurableHandler(s *store.Store, supervisor *Supervisor) *Handler {
	return &Handler{store: s, runtime: agent.NewTaskRuntime(s), supervisor: supervisor}
}

func (h *Handler) startDurableTask(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if message.WorkspaceID == "" || message.TaskID == "" {
		return protocol.Message{}, fmt.Errorf("workspace and task IDs are required")
	}
	if _, err := h.store.GetWorkspace(ctx, message.WorkspaceID); err != nil {
		return protocol.Message{}, err
	}
	var specs []agent.Spec
	if err := json.Unmarshal(message.Payload, &specs); err != nil {
		return protocol.Message{}, fmt.Errorf("invalid agent plan: %w", err)
	}
	if _, err := h.supervisor.Start(ctx, message.WorkspaceID, message.TaskID, specs); err != nil {
		return protocol.Message{}, err
	}
	event := domain.EventEnvelope{WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, AgentID: "system", Sequence: 1, Type: domain.EventTaskUpdated, Payload: []byte("running")}
	return protocol.Message{Type: protocol.MessageEvent, WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, Event: &event}, nil
}
