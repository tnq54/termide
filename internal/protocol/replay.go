package protocol

import (
	"context"
	"fmt"

	"termide/internal/domain"
)

type EventSource interface { EventsAfter(context.Context, string, uint64) ([]domain.EventEnvelope, error) }
func Replay(ctx context.Context, source EventSource, workspaceID string, after uint64) ([]domain.EventEnvelope, error) {
	if source == nil { return nil, fmt.Errorf("event source is required") }
	return source.EventsAfter(ctx, workspaceID, after)
}
