package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
)

func (h *Handler) waitForEvents(ctx context.Context, workspaceID string, after uint64, waitMs uint64) ([]domain.EventEnvelope, error) {
	wait := time.Duration(waitMs) * time.Millisecond
	if wait > maxSubscribeWait {
		wait = maxSubscribeWait
	}
	deadline := time.Now().Add(wait)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		events, err := h.store.EventsAfter(ctx, workspaceID, after)
		if err != nil {
			return nil, err
		}
		if len(events) > 0 {
			return events, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return []domain.EventEnvelope{}, nil
		}
		if remaining > subscribePollEvery {
			remaining = subscribePollEvery
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(remaining):
		}
	}
}

const (
	maxReplayEvents       = 10000
	maxReplayPayloadBytes = 2 * 1024 * 1024
	subscribePollEvery    = 100 * time.Millisecond
)

// maxSubscribeWait caps one long-poll so a client can never pin a connection
// forever. Tests override it for speed.
var maxSubscribeWait = 30 * time.Second

// waitForEvents polls until events appear after `after`, the wait budget
// expires, or ctx is cancelled. It never returns an error for timeout: an
// empty replay is a valid long-poll answer.

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
	if len(events) == 0 && message.WaitMs > 0 {
		events, err = h.waitForEvents(ctx, message.WorkspaceID, message.AfterSequence, message.WaitMs)
		if err != nil {
			return protocol.Message{}, err
		}
	}
	return replayMessage(message.WorkspaceID, message.AfterSequence, events)
}

// replayMessage bounds and frames a replay payload shared by subscribe and
// stream catch-up.
func replayMessage(workspaceID string, after uint64, events []domain.EventEnvelope) (protocol.Message, error) {
	if events == nil {
		events = []domain.EventEnvelope{}
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
	return protocol.Message{Type: protocol.MessageReplay, WorkspaceID: workspaceID, AfterSequence: after, Payload: payload}, nil
}
