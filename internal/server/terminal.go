package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"termide/internal/protocol"
	"termide/internal/security"
)

const (
	commandTimeout   = 30 * time.Second
	maxCommandOutput = 64 * 1024
)

func (h *Handler) runCommand(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	if message.WorkspaceID == "" {
		return protocol.Message{}, fmt.Errorf("workspace ID is required")
	}
	workspace, err := h.store.GetWorkspace(ctx, message.WorkspaceID)
	if err != nil {
		return protocol.Message{}, err
	}
	var request protocol.CommandRequest
	if err := json.Unmarshal(message.Payload, &request); err != nil {
		return protocol.Message{}, fmt.Errorf("invalid command request: %w", err)
	}
	switch security.Classify(request.Command, workspace.Root) {
	case security.Safe:
	case security.Blocked:
		return protocol.Message{}, fmt.Errorf("command is blocked by policy")
	default:
		return protocol.Message{}, fmt.Errorf("command requires approval: start a task instead")
	}
	runCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "sh", "-c", request.Command)
	cmd.Dir = workspace.Root
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	runErr := cmd.Run()
	result := protocol.CommandResult{Output: output.String()}
	if output.Len() > maxCommandOutput {
		result.Output = output.String()[:maxCommandOutput]
		result.Truncated = true
	}
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else if runErr != nil {
		return protocol.Message{}, runErr
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return protocol.Message{}, err
	}
	return protocol.Message{Type: protocol.MessageCommandResult, WorkspaceID: message.WorkspaceID, Payload: payload}, nil
}
