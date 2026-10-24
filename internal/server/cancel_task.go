package server

import (
	"context"
	"fmt"
	"termide/internal/protocol"
)

func (h *Handler) cancelTask(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if message.WorkspaceID == "" || message.TaskID == "" {
		return protocol.Message{}, fmt.Errorf("workspace and task IDs are required")
	}
	if h.supervisor != nil {
		if err := h.supervisor.Cancel(ctx, message.WorkspaceID, message.TaskID); err != nil {
			return protocol.Message{}, err
		}
		return protocol.Message{Type: protocol.MessageTaskCancelRequested, WorkspaceID: message.WorkspaceID, TaskID: message.TaskID}, nil
	}
	task, err := h.store.GetTask(ctx, message.TaskID)
	if err != nil {
		return protocol.Message{}, err
	}
	if task.WorkspaceID != message.WorkspaceID {
		return protocol.Message{}, fmt.Errorf("task does not belong to workspace")
	}
	h.jobsMu.Lock()
	defer h.jobsMu.Unlock()
	if h.closed {
		return protocol.Message{}, fmt.Errorf("task supervisor is closed")
	}
	cancel, ok := h.cancels[message.TaskID]
	if !ok {
		return protocol.Message{}, fmt.Errorf("task is not running in this server session")
	}
	cancel()
	// Runtime owns final status persistence. An acknowledgement must not claim
	// cancellation has completed while the process is still shutting down.
	return protocol.Message{Type: protocol.MessageTaskCancelRequested, WorkspaceID: message.WorkspaceID, TaskID: message.TaskID}, nil
}
