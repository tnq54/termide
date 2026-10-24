package client

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type processStream struct {
	input  io.WriteCloser
	output io.ReadCloser
	cmd    *exec.Cmd
	once   sync.Once
}

func StartProcess(cmd *exec.Cmd) (io.ReadWriteCloser, error) {
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return nil, err
	}
	return &processStream{input: input, output: output, cmd: cmd}, nil
}
func (s *processStream) Read(p []byte) (int, error)  { return s.output.Read(p) }
func (s *processStream) Write(p []byte) (int, error) { return s.input.Write(p) }
func (s *processStream) Close() error {
	s.once.Do(func() { s.input.Close(); s.output.Close(); s.cmd.Process.Kill(); s.cmd.Wait() })
	return nil
}

// SSHCommand uses the user's OpenSSH config and pre-established known_hosts.
// It deliberately allocates no PTY so terminal processing cannot corrupt frames.
func SSHCommand(ctx context.Context, host string) (*exec.Cmd, error) {
	if host == "" || strings.HasPrefix(host, "-") || strings.ContainsAny(host, " \t\r\n\x00") {
		return nil, fmt.Errorf("invalid SSH host alias")
	}
	return exec.CommandContext(ctx, "ssh", "-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "--", host, "termide-server --stdio"), nil
}

func DialSSH(ctx context.Context, host string) (*Remote, error) {
	stream, err := DialSSHStream(ctx, host)
	if err != nil {
		return nil, err
	}
	return NewRemote(stream), nil
}

// DialSSHStream opens a raw framed-transport stream to termide-server. Use it
// for push-mode subscriptions that must not share the interactive Remote.
func DialSSHStream(ctx context.Context, host string) (io.ReadWriteCloser, error) {
	cmd, err := SSHCommand(ctx, host)
	if err != nil {
		return nil, err
	}
	return StartProcess(cmd)
}
