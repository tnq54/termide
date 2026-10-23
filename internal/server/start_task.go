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
	if h.runner == nil {
		return protocol.Message{}, fmt.Errorf("task runner is not configured")
	}
	if _, err := h.store.GetWorkspace(ctx, message.WorkspaceID); err != nil {
		return protocol.Message{}, err
	}
	var specs []agent.Spec
	if err := json.Unmarshal(message.Payload, &specs); err != nil {
		return protocol.Message{}, fmt.Errorf("invalid agent plan: %w", err)
	}
	if len(specs) == 0 {
		return protocol.Message{}, fmt.Errorf("agent plan must not be empty")
	}
	if err := h.runtime.Start(ctx, message.TaskID, specs, h.runner); err != nil {
		return protocol.Message{}, err
	}
	event := domain.EventEnvelope{
		WorkspaceID: message.WorkspaceID,
		TaskID:      message.TaskID,
		AgentID:     "system",
		Type:        domain.EventTaskUpdated,
		Payload:     []byte("completed"),
	}
	return protocol.Message{Type: protocol.MessageEvent, WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, Event: &event}, nil
}
