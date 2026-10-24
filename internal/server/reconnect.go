package server

import (
	"context"
	"encoding/json"
	"fmt"

	"termide/internal/protocol"
)

func (h *Handler) subscribe(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if message.WorkspaceID == "" {
		return protocol.Message{}, fmt.Errorf("workspace ID is required")
	}
	if _, err := h.store.GetWorkspace(ctx, message.WorkspaceID); err != nil {
		return protocol.Message{}, err
	}
	events, err := h.store.EventsAfter(ctx, message.WorkspaceID, message.AfterSequence)
	if err != nil {
		return protocol.Message{}, err
	}
	payload, err := json.Marshal(events)
	if err != nil {
		return protocol.Message{}, err
	}
	return protocol.Message{Type: protocol.MessageReplay, WorkspaceID: message.WorkspaceID, AfterSequence: message.AfterSequence, Payload: payload}, nil
}
