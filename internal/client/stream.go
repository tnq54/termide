package client

import (
	"context"
	"fmt"
	"io"

	"termide/internal/domain"
	"termide/internal/protocol"
)

// SubscribeStream upgrades a dedicated connection to push mode: it sends one
// subscribe_stream request, then emits every replayed and pushed event until
// ctx ends or the server closes the stream. The connection carries nothing
// else, so interactive traffic on other connections is unaffected. Close is
// owned here: returning closes conn, and ctx cancellation closes it to
// unblock the read loop.
func SubscribeStream(ctx context.Context, conn io.ReadWriteCloser, workspaceID string, after uint64, emit func(domain.EventEnvelope) error) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace ID is required")
	}
	if emit == nil {
		return fmt.Errorf("emit function is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	defer conn.Close()
	if err := protocol.Write(conn, protocol.Message{Type: protocol.MessageSubscribeStream, WorkspaceID: workspaceID, AfterSequence: after}); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	previous := after
	for {
		message, err := protocol.Read(conn)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		switch message.Type {
		case protocol.MessageReplay:
			events, err := decodeReplay(workspaceID, previous, message)
			if err != nil {
				return err
			}
			for _, event := range events {
				if err := emit(event); err != nil {
					return err
				}
				previous = event.Sequence
			}
		case protocol.MessageEvent:
			if message.Event == nil {
				return fmt.Errorf("event payload is required")
			}
			event := *message.Event
			if err := event.Validate(); err != nil {
				return err
			}
			if event.WorkspaceID != workspaceID || event.Sequence <= previous {
				return fmt.Errorf("invalid pushed event sequence or workspace")
			}
			if err := emit(event); err != nil {
				return err
			}
			previous = event.Sequence
		case protocol.MessageError:
			return fmt.Errorf("remote: %s", message.Payload)
		default:
			return fmt.Errorf("unexpected stream message %q", message.Type)
		}
	}
}
