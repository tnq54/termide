package server

import (
	"context"
	"errors"
	"io"
	"termide/internal/protocol"
)

// Serve handles framed requests until EOF. The caller owns stream cancellation
// and must close the transport when its context is cancelled.
func (h *Handler) Serve(ctx context.Context, input io.Reader, output io.Writer) (serveErr error) {
	defer func() { serveErr = errors.Join(serveErr, h.Close()) }()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		request, err := protocol.Read(input)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		response, err := h.Handle(ctx, request)
		if err != nil {
			response = protocol.Message{Type: protocol.MessageError, Payload: []byte(err.Error())}
		}
		response.RequestID = request.RequestID
		if err := protocol.Write(output, response); err != nil {
			return err
		}
	}
}
