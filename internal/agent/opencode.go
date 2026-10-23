package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

type OpenCodeRunner struct {
	binary string
	args   []string
}

func NewOpenCodeRunner(binary string, args []string) *OpenCodeRunner {
	return &OpenCodeRunner{binary: binary, args: append([]string(nil), args...)}
}
func (r *OpenCodeRunner) Command(spec Spec) (*exec.Cmd, error) {
	if r.binary == "" {
		return nil, fmt.Errorf("opencode binary is required")
	}
	args := append([]string(nil), r.args...)
	args = append(args, "run", spec.Command)
	cmd := exec.Command(r.binary, args...)
	cmd.Dir = spec.WorkingDir
	return cmd, nil
}
func (r *OpenCodeRunner) Run(ctx context.Context, spec Spec, emit func(Event)) error {
	cmd, err := r.Command(spec)
	if err != nil {
		return err
	}
	dir := cmd.Dir
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		emit(Event{AgentID: spec.ID, Kind: "output", Output: Redact(scanner.Text(), nil)})
	}
	if scanner.Err() != nil {
		cmd.Process.Kill()
	}
	return errors.Join(scanner.Err(), cmd.Wait())
}
