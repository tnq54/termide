//go:build !linux

package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

// bootstrapWait bounds how long --stdio waits for a freshly spawned daemon
// to serve its control socket.
const bootstrapWait = 15 * time.Second

// EnsureDaemon dials the supervisor, spawning a daemon from the current
// executable when nothing listens. Flags mirror the --daemon mode.
func EnsureDaemon(ctx context.Context, socketPath, database, opencode, lspPython string) (net.Conn, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate server executable: %w", err)
	}
	return EnsureSupervisor(ctx, socketPath, bootstrapWait, func(ctx context.Context) error {
		return spawnDaemon(self, socketPath, database, opencode, lspPython)
	})
}

func spawnDaemon(self, socketPath, database, opencode, lspPython string) error {
	args := []string{"--daemon", "--socket", socketPath, "--database", database, "--opencode", opencode}
	if lspPython != "" {
		args = append(args, "--lsp-python", lspPython)
	}
	cmd := exec.Command(self, args...)
	cmd.Stdin = nil
	logFile, err := os.OpenFile(socketPath+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
