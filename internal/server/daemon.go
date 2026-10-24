package server

import (
	"context"
	"errors"
	"time"

	"termide/internal/agent"
	"termide/internal/store"
)

// shutdownGrace bounds daemon shutdown: in-flight runs are left alone so a
// later daemon recovers them via lease expiry.
const shutdownGrace = 10 * time.Second

// DaemonOptions carries optional daemon integrations.
type DaemonOptions struct {
	// LSPBin/LSPArgs configure the Python language server command.
	LSPBin  string
	LSPArgs []string
}

// RunDaemon recovers expired runs, serves the control socket until ctx ends,
// then shuts the supervisor down gracefully. The caller owns database locking.
func RunDaemon(ctx context.Context, s *store.Store, socketPath string, runner agent.Runner, owner string, opts DaemonOptions) error {
	supervisor := NewSupervisor(s, runner, owner)
	supervisor.SetLSP(opts.LSPBin, opts.LSPArgs)
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return supervisor.Shutdown(shutdownCtx)
	case err := <-serveErr:
		_ = listener.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return errors.Join(err, supervisor.Shutdown(shutdownCtx))
	}
}
