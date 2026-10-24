package server

import (
	"context"
	"fmt"
	"io"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
)

const streamPollEvery = 100 * time.Millisecond

// streamAdapter joins a separate reader and writer into the duplex stream
// serveStream expects. Close is a no-op: the caller owns both halves.
type streamAdapter struct {
	io.Reader
	io.Writer
}

func (streamAdapter) Close() error { return nil }

// serveStream upgrades a connection to push mode: one replay catch-up, then
// one event frame per new event until ctx ends, a write fails, or the client
// goes away. The caller must not read further requests on the connection
// afterwards. A background drain watches the stream so a departed client
// promptly ends the push loop instead of leaking a poller.
func (h *Handler) serveStream(ctx context.Context, stream io.ReadWriteCloser, request protocol.Message) error {
	if request.WorkspaceID == "" {
		return writeStreamError(stream, request, fmt.Errorf("workspace ID is required"))
	}
	if _, err := h.store.GetWorkspace(ctx, request.WorkspaceID); err != nil {
		return writeStreamError(stream, request, err)
	}
	pushCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		var one [1]byte
		_, _ = stream.Read(one[:])
		cancel()
	}()
	cursor := request.AfterSequence
	events, err := h.store.EventsAfter(pushCtx, request.WorkspaceID, cursor)
	if err != nil {
		return writeStreamError(stream, request, err)
	}
	if err := h.writeReplay(stream, request, events); err != nil {
		return err
	}
	for _, event := range events {
		if event.Sequence > cursor {
			cursor = event.Sequence
		}
	}
	for {
		if err := pushCtx.Err(); err != nil {
			return err
		}
		events, err := h.store.EventsAfter(pushCtx, request.WorkspaceID, cursor)
		if err != nil {
			return err
		}
		for _, event := range events {
			out := protocol.Message{Type: protocol.MessageEvent, RequestID: request.RequestID, WorkspaceID: request.WorkspaceID, TaskID: event.TaskID, Event: &event}
			if err := protocol.Write(stream, out); err != nil {
				return err
			}
			cursor = event.Sequence
		}
		select {
		case <-pushCtx.Done():
			return pushCtx.Err()
		case <-time.After(streamPollEvery):
		}
	}
}

func (h *Handler) writeReplay(output io.Writer, request protocol.Message, events []domain.EventEnvelope) error {
	response, err := replayMessage(request.WorkspaceID, request.AfterSequence, events)
	if err != nil {
		return err
	}
	response.RequestID = request.RequestID
	return protocol.Write(output, response)
}

func writeStreamError(output io.Writer, request protocol.Message, err error) error {
	if werr := protocol.Write(output, protocol.Message{Type: protocol.MessageError, RequestID: request.RequestID, WorkspaceID: request.WorkspaceID, Payload: []byte(err.Error())}); werr != nil {
		return werr
	}
	return err
}
