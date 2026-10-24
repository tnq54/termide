package server

import (
	"context"
	"encoding/json"
	"fmt"

	"termide/internal/domain"
	"termide/internal/protocol"
)

func (h *Handler) approve(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if message.WorkspaceID == "" || message.TaskID == "" {
		return protocol.Message{}, fmt.Errorf("workspace and task IDs are required")
	}
	var decision protocol.ApproveDecision
	if err := json.Unmarshal(message.Payload, &decision); err != nil {
		return protocol.Message{}, fmt.Errorf("invalid approval decision: %w", err)
	}
	if decision.ID == "" {
		return protocol.Message{}, fmt.Errorf("approval ID is required")
	}
	task, err := h.store.GetTask(ctx, message.TaskID)
	if err != nil {
		return protocol.Message{}, err
	}
	if task.WorkspaceID != message.WorkspaceID {
		return protocol.Message{}, fmt.Errorf("task does not belong to workspace")
	}
	approval, err := h.store.GetApproval(ctx, decision.ID)
	if err != nil {
		return protocol.Message{}, err
	}
	if approval.TaskID != message.TaskID {
		return protocol.Message{}, fmt.Errorf("approval does not belong to task")
	}
	if err := h.store.ResolveApproval(ctx, decision.ID, decision.Approved, decision.Reason); err != nil {
		return protocol.Message{}, err
	}
	event := domain.EventEnvelope{
		WorkspaceID: message.WorkspaceID,
		TaskID:      message.TaskID,
		AgentID:     approval.AgentID,
		Type:        domain.EventApproval,
		Payload:     message.Payload,
	}
	if err := h.store.AppendEvent(ctx, event); err != nil {
		return protocol.Message{}, err
	}
	return protocol.Message{Type: protocol.MessageEvent, WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, Event: &event}, nil
}

// denyPendingApprovals resolves leftover pending approvals after a task stops
// without a decision, so reconnects never observe a stuck approval.
func (h *Handler) denyPendingApprovals(ctx context.Context, taskID, reason string) {
	pending, err := h.store.ListPendingApprovals(ctx, taskID)
	if err != nil {
		return
	}
	for _, approval := range pending {
		_ = h.store.ResolveApproval(ctx, approval.ID, false, reason)
	}
}
