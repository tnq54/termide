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
	if emit == nil {
		return fmt.Errorf("emit function is required")
	}
	stream, catchUp, err := OpenStream(ctx, conn, workspaceID, after)
	if err != nil {
		return err
	}
	defer stream.Close()
	for _, event := range catchUp {
		if err := emit(event); err != nil {
			return err
		}
	}
	for {
		event, err := stream.Next(ctx)
		if err != nil {
			return err
		}
		if err := emit(event); err != nil {
			return err
		}
	}
}

// Stream is one push-mode connection frame reader. Use Next in a loop;
// Close ends the stream. A Stream is not safe for concurrent use.
type Stream struct {
	conn      io.ReadWriteCloser
	workspace string
	previous  uint64
	stop      func() bool
}

// OpenStream sends subscribe_stream and reads the replay catch-up. The caller
// owns the stream: Close it when done, and cancel ctx to unblock Next.
func OpenStream(ctx context.Context, conn io.ReadWriteCloser, workspaceID string, after uint64) (*Stream, []domain.EventEnvelope, error) {
	if workspaceID == "" {
		return nil, nil, fmt.Errorf("workspace ID is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	stream := &Stream{conn: conn, workspace: workspaceID, previous: after, stop: stop}
	if err := protocol.Write(conn, protocol.Message{Type: protocol.MessageSubscribeStream, WorkspaceID: workspaceID, AfterSequence: after}); err != nil {
		stream.Close()
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, err
	}
	message, err := protocol.Read(conn)
	if err != nil {
		stream.Close()
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, err
	}
	if message.Type == protocol.MessageError {
		stream.Close()
		return nil, nil, fmt.Errorf("remote: %s", message.Payload)
	}
	events, err := decodeReplay(workspaceID, after, message)
	if err != nil {
		stream.Close()
		return nil, nil, err
	}
	for _, event := range events {
		stream.previous = event.Sequence
	}
	return stream, events, nil
}

// Next blocks for the next pushed event frame.
func (s *Stream) Next(ctx context.Context) (domain.EventEnvelope, error) {
	stop := context.AfterFunc(ctx, func() { s.conn.Close() })
	defer stop()
	message, err := protocol.Read(s.conn)
	if err != nil {
		if ctx.Err() != nil {
			return domain.EventEnvelope{}, ctx.Err()
		}
		return domain.EventEnvelope{}, err
	}
	if message.Type == protocol.MessageError {
		return domain.EventEnvelope{}, fmt.Errorf("remote: %s", message.Payload)
	}
	if message.Type != protocol.MessageEvent || message.Event == nil {
		return domain.EventEnvelope{}, fmt.Errorf("unexpected stream message %q", message.Type)
	}
	event := *message.Event
	if err := event.Validate(); err != nil {
		return domain.EventEnvelope{}, err
	}
	if event.WorkspaceID != s.workspace || event.Sequence <= s.previous {
		return domain.EventEnvelope{}, fmt.Errorf("invalid pushed event sequence or workspace")
	}
	s.previous = event.Sequence
	return event, nil
}

// Close ends the stream and releases the connection.
func (s *Stream) Close() error {
	if s.stop != nil {
		s.stop()
	}
	return s.conn.Close()
}
