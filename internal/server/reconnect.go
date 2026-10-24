package server

import (
	"context"
	"encoding/json"
	"fmt"

	"termide/internal/protocol"
)

const (
	maxReplayEvents       = 10000
	maxReplayPayloadBytes = 2 * 1024 * 1024
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
	if len(events) > maxReplayEvents {
		return protocol.Message{}, fmt.Errorf("replay contains too many events")
	}
	payload, err := json.Marshal(events)
	if err != nil {
		return protocol.Message{}, err
	}
	if len(payload) > maxReplayPayloadBytes {
		return protocol.Message{}, fmt.Errorf("replay payload exceeds limit")
	}
	return protocol.Message{Type: protocol.MessageReplay, WorkspaceID: message.WorkspaceID, AfterSequence: message.AfterSequence, Payload: payload}, nil
}
