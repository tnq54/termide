package client

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	"termide/internal/protocol"
)

// Remote serializes request/response exchanges on one transport. Cancellation
// closes the connection: callers must reconnect rather than reuse a partial frame.
type Remote struct {
	stream   io.ReadWriteCloser
	gate     chan struct{}
	sequence uint64
	once     sync.Once
	closeErr error
}

func NewRemote(stream io.ReadWriteCloser) *Remote {
	return &Remote{stream: stream, gate: make(chan struct{}, 1)}
}

func (c *Remote) Close() error {
	c.once.Do(func() { c.closeErr = c.stream.Close() })
	return c.closeErr
}

func (c *Remote) Call(ctx context.Context, request protocol.Message) (protocol.Message, error) {
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return protocol.Message{}, ctx.Err()
	}
	defer func() { <-c.gate }()
	if err := ctx.Err(); err != nil {
		return protocol.Message{}, err
	}
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	c.sequence++
	request.RequestID = strconv.FormatUint(c.sequence, 10)
	if err := protocol.Write(c.stream, request); err != nil {
		c.Close()
		if ctx.Err() != nil {
			return protocol.Message{}, ctx.Err()
		}
		return protocol.Message{}, err
	}
	response, err := protocol.Read(c.stream)
	if err != nil {
		c.Close()
		if ctx.Err() != nil {
			return protocol.Message{}, ctx.Err()
		}
		return protocol.Message{}, err
	}
	if response.RequestID != request.RequestID {
		c.Close()
		return protocol.Message{}, fmt.Errorf("response request ID mismatch")
	}
	if response.Type == protocol.MessageError {
		return response, fmt.Errorf("remote: %s", response.Payload)
	}
	return response, nil
}
