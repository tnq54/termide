package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// startTestDaemon listens on sock like a freshly spawned daemon would.
func startTestDaemon(sock string, started chan struct{}) func(context.Context) error {
	return func(ctx context.Context) error {
		listener, err := ListenControl(sock)
		if err != nil {
			return err
		}
		select {
		case started <- struct{}{}:
		default:
		}
		go func() {
			<-ctx.Done()
			listener.Close()
			os.Remove(sock)
		}()
		return nil
	}
}

func TestEnsureSupervisorStartsDaemonWhenSocketMissing(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "termide.sock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	started := make(chan struct{}, 1)
	calls := 0
	conn, err := EnsureSupervisor(ctx, sock, 4*time.Second, func(ctx context.Context) error {
		calls++
		return startTestDaemon(sock, started)(ctx)
	})
	if err != nil {
		t.Fatalf("EnsureSupervisor = %v", err)
	}
	defer conn.Close()
	if calls != 1 {
		t.Fatalf("start called %d times, want 1", calls)
	}
	select {
	case <-started:
	default:
		t.Fatal("daemon was not started")
	}
	if _, err := DialControl(ctx, sock); err != nil {
		t.Fatalf("socket not serving after bootstrap: %v", err)
	}
}

func TestEnsureSupervisorReusesRunningDaemon(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "termide.sock")
	listener, err := ListenControl(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	defer os.Remove(sock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	called := false
	conn, err := EnsureSupervisor(ctx, sock, time.Second, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("EnsureSupervisor = %v", err)
	}
	defer conn.Close()
	if called {
		t.Fatal("start called despite a listening daemon")
	}
}

func TestEnsureSupervisorPropagatesStartError(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "termide.sock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := EnsureSupervisor(ctx, sock, time.Second, func(context.Context) error {
		return errDaemonStart
	}); err == nil {
		t.Fatal("start error swallowed")
	} else if !errors.Is(err, errDaemonStart) {
		t.Fatalf("start error masked: %v", err)
	}
}

var errDaemonStart = errors.New("daemon start failed")

func TestEnsureSupervisorTimesOutWhenDaemonNeverListens(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "termide.sock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, err := EnsureSupervisor(ctx, sock, 300*time.Millisecond, func(context.Context) error { return nil })
	if err == nil {
		t.Fatal("missing daemon accepted")
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("waited too long: %v", elapsed)
	}
}

func TestEnsureSupervisorRejectsEmptySocket(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := EnsureSupervisor(ctx, "", time.Second, func(context.Context) error { return nil }); err == nil {
		t.Fatal("empty socket accepted")
	}
}
