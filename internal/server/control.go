package server

import (
	"context"
	"errors"
	"fmt"
	"io"

	"termide/internal/protocol"
)

// ServeControl serves the full workspace protocol on one control connection.
// Start and cancel are executed by the durable supervisor, so tasks survive
// the loss of any single session; every other request is handled identically
// to a session-scoped handler.
func ServeControl(ctx context.Context, stream io.ReadWriteCloser, supervisor *Supervisor) error {
	if supervisor == nil {
		return fmt.Errorf("supervisor is required")
	}
	handler := supervisor.Handler()
	defer stream.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		request, err := protocol.Read(stream)
		if err != nil {
			return err
		}
		if request.Type == protocol.MessageSubscribeStream {
			return handler.serveStream(ctx, stream, request)
		}
		response, err := handler.Handle(ctx, request)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			response = protocol.Message{Type: protocol.MessageError, Payload: []byte(err.Error())}
		}
		response.RequestID = request.RequestID
		if err := protocol.Write(stream, response); err != nil {
			return err
		}
	}
}
