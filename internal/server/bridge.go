package server

import (
	"context"
	"io"

	"termide/internal/protocol"
)

type Bridge struct{ control io.ReadWriteCloser }

func NewBridge(control io.ReadWriteCloser) *Bridge { return &Bridge{control: control} }

// stdioStream splits a session across separate input and output streams, as
// required for stdio mode where stdin is read-only and stdout is write-only.
// Close is a no-op: the process does not own the standard streams.
type stdioStream struct {
	r io.Reader
	w io.Writer
}

func Stdio(r io.Reader, w io.Writer) io.ReadWriteCloser { return stdioStream{r: r, w: w} }

func (s stdioStream) Read(p []byte) (int, error)  { return s.r.Read(p) }
func (s stdioStream) Write(p []byte) (int, error) { return s.w.Write(p) }
func (stdioStream) Close() error                  { return nil }

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
		if request.Type == protocol.MessageSubscribeStream {
			return b.pumpStream(session)
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

// pumpStream forwards pushed frames control→session until either side ends.
// Like an HTTP CONNECT tunnel, the connection carries no further requests
// after the upgrade: post-upgrade client bytes are drained and ignored. A
// background drain watches the session so a client disconnect unblocks the
// pump by closing the control connection. Both deferred closes tolerate
// double-close (processStream.Close is once-guarded; net.Pipe errors ignored).
func (b *Bridge) pumpStream(session io.ReadWriteCloser) error {
	go func() {
		_, _ = io.Copy(io.Discard, session)
		b.control.Close()
	}()
	for {
		response, err := protocol.Read(b.control)
		if err != nil {
			return err
		}
		if err := protocol.Write(session, response); err != nil {
			return err
		}
	}
}
