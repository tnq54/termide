package server

import (
	"context"
	"fmt"
	"net"
	"time"
)

// dialBudget bounds each individual dial attempt so one hung dial cannot
// swallow the whole bootstrap wait.
const dialBudget = 500 * time.Millisecond

// EnsureSupervisor returns a connection to the durable supervisor, starting
// one via start when nothing listens on socketPath (first run, or a stale
// socket left by a dead daemon). start must be idempotent: a concurrent
// daemon wins the database lock and the loser exits, after which the socket
// serves normally.
func EnsureSupervisor(ctx context.Context, socketPath string, wait time.Duration, start func(ctx context.Context) error) (net.Conn, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("control socket path is required")
	}
	if conn, err := dialControlOnce(ctx, socketPath); err == nil {
		return conn, nil
	}
	if err := start(ctx); err != nil {
		return nil, fmt.Errorf("start durable supervisor: %w", err)
	}
	deadline := time.Now().Add(wait)
	for {
		conn, err := dialControlOnce(ctx, socketPath)
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("durable supervisor did not listen on %s within %s: %w", socketPath, wait, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func dialControlOnce(ctx context.Context, socketPath string) (net.Conn, error) {
	try, cancel := context.WithTimeout(ctx, dialBudget)
	defer cancel()
	return DialControl(try, socketPath)
}
