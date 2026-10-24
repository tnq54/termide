package server

import (
	"context"

	"termide/internal/agent"
	"termide/internal/store"
)

// RunDaemon recovers expired runs, serves the control socket until ctx ends,
// then shuts the supervisor down gracefully. The caller owns database locking.
func RunDaemon(ctx context.Context, s *store.Store, socketPath string, runner agent.Runner, owner string) error {
	supervisor := NewSupervisor(s, runner, owner)
	if _, err := supervisor.Recover(ctx); err != nil {
		return err
	}
	listener, err := ListenControl(socketPath)
	if err != nil {
		return err
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- ServeControlSocket(ctx, listener, supervisor) }()
	select {
	case <-ctx.Done():
		_ = listener.Close()
		<-serveErr
		return supervisor.Close(context.Background())
	case err := <-serveErr:
		_ = listener.Close()
		return err
	}
}
