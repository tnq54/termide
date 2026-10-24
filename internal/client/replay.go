package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
)

// SubscribeWait long-polls for events after `after`, waiting up to wait for
// new events. An empty result means the wait expired, not an error. The
// caller stays in control: no background goroutine is started.
func (c *Remote) SubscribeWait(ctx context.Context, workspaceID string, after uint64, wait time.Duration) ([]domain.EventEnvelope, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if wait <= 0 {
		return nil, fmt.Errorf("wait duration must be positive")
	}
	response, err := c.Call(ctx, protocol.Message{
		Type:          protocol.MessageSubscribe,
		WorkspaceID:   workspaceID,
		AfterSequence: after,
		WaitMs:        uint64(wait / time.Millisecond),
	})
	if err != nil {
		return nil, err
	}
	return decodeReplay(workspaceID, after, response)
}

const (
	MaxReplayEvents       = 10000
	MaxReplayPayloadBytes = 2 * 1024 * 1024
)

// Subscribe returns events strictly after after for one workspace. The helper
// validates the replay response before exposing it to callers.
func (c *Remote) Subscribe(ctx context.Context, workspaceID string, after uint64) ([]domain.EventEnvelope, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	response, err := c.Call(ctx, protocol.Message{
		Type:          protocol.MessageSubscribe,
		WorkspaceID:   workspaceID,
		AfterSequence: after,
	})
	if err != nil {
		return nil, err
	}
	return decodeReplay(workspaceID, after, response)
}

func decodeReplay(workspaceID string, after uint64, response protocol.Message) ([]domain.EventEnvelope, error) {
	if response.Type != protocol.MessageReplay || response.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("invalid replay response identity")
	}
	if len(response.Payload) > MaxReplayPayloadBytes {
		return nil, fmt.Errorf("replay payload exceeds limit")
	}
	var events []domain.EventEnvelope
	if err := json.Unmarshal(response.Payload, &events); err != nil {
		return nil, fmt.Errorf("decode replay: %w", err)
	}
	if len(events) > MaxReplayEvents {
		return nil, fmt.Errorf("replay contains too many events")
	}
	previous := after
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("replay event %d: %w", i, err)
		}
		if event.WorkspaceID != workspaceID || event.Sequence <= previous {
			return nil, fmt.Errorf("invalid replay event %d sequence or workspace", i)
		}
		previous = event.Sequence
	}
	return events, nil
}
