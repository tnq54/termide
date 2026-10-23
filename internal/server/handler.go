package server

import (
	"context"
	"fmt"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

type Handler struct { store *store.Store }
func NewHandler(s *store.Store) *Handler { return &Handler{store: s} }
func (h *Handler) Handle(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	switch message.Type {
	case protocol.MessageCreateTask:
		if message.WorkspaceID == "" || message.TaskID == "" { return protocol.Message{}, fmt.Errorf("workspace and task IDs are required") }
		task := domain.Task{ID: message.TaskID, WorkspaceID: message.WorkspaceID, Description: string(message.Payload), Status: domain.TaskQueued}
		if err := h.store.CreateTask(ctx, task); err != nil { return protocol.Message{}, err }
		event := domain.EventEnvelope{WorkspaceID: message.WorkspaceID, TaskID: message.TaskID, AgentID: "system", Sequence: 1, Type: domain.EventTaskUpdated, Payload: message.Payload}
		return protocol.Message{Type: protocol.MessageEvent, Event: &event}, nil
	default:
		return protocol.Message{}, fmt.Errorf("unsupported message type %q", message.Type)
	}
}
