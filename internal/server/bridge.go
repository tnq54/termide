package server

import (
	"context"
	"io"

	"termide/internal/protocol"
)

type Bridge struct{ control io.ReadWriteCloser }

func NewBridge(control io.ReadWriteCloser) *Bridge { return &Bridge{control: control} }

func RunStdioBridge(ctx context.Context, session io.ReadWriteCloser, control io.ReadWriteCloser) error {
	return NewBridge(control).Serve(ctx, session)
}

func (b *Bridge) Serve(ctx context.Context, session io.ReadWriteCloser) error {
	defer session.Close()
	defer b.control.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		request, err := protocol.Read(session)
		if err != nil {
			return err
		}
		if err := protocol.Write(b.control, request); err != nil {
			return err
		}
		response, err := protocol.Read(b.control)
		if err != nil {
			return err
		}
		if err := protocol.Write(session, response); err != nil {
			return err
		}
	}
}
