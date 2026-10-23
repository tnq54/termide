package terminal

import (
	"context"
	"os/exec"
)

type Runner struct{}
func NewRunner() *Runner { return &Runner{} }

func (r *Runner) Run(ctx context.Context, command string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	return cmd.CombinedOutput()
}
