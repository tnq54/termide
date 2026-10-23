package agent

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
)

type OpenCodeRunner struct { binary string; args []string }
func NewOpenCodeRunner(binary string, args []string) *OpenCodeRunner { return &OpenCodeRunner{binary: binary, args: append([]string(nil), args...)} }
func (r *OpenCodeRunner) Command(spec Spec) (*exec.Cmd, error) {
	if r.binary == "" { return nil, fmt.Errorf("opencode binary is required") }
	args := append([]string(nil), r.args...)
	args = append(args, "run", spec.Command)
	return exec.Command(r.binary, args...), nil
}
func (r *OpenCodeRunner) Run(ctx context.Context, spec Spec, emit func(Event)) error {
	cmd, err := r.Command(spec); if err != nil { return err }
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	stdout, err := cmd.StdoutPipe(); if err != nil { return err }
	if err := cmd.Start(); err != nil { return err }
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() { emit(Event{AgentID: spec.ID, Kind: "output", Output: Redact(scanner.Text(), nil)}) }
	if err := scanner.Err(); err != nil { return err }
	return cmd.Wait()
}
