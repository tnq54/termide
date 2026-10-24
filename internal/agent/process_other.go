//go:build !linux

package agent

import "os/exec"

// The production runner targets Linux VPS hosts. Other platforms retain
// CommandContext's direct-process cancellation without group guarantees.
func configureProcessCancellation(cmd *exec.Cmd) {}
